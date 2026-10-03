"""Statistics helpers for numerical task metrics.

The on-disk task statistics (see ``NumericalStatsRecord`` in the Go manager) are
split into scalar metrics (``score``, ``time``) and vector metrics (``cost``).
A vector (``StatVector``) is a variable length list such as
``[prompt_tokens, completion_tokens, cached_tokens]`` where any element can be
``None`` because the backend did not inform it (``cached_tokens`` is missing
when ``usage.prompt_tokens_details`` is absent).

Every helper here skips ``None`` values independently per position and returns
``None`` for positions that received no value at all, so the number of samples
backing each position (``n``) can differ.

This module intentionally depends only on the standard library so that the
logic can be unit tested without the full workflow stack.
"""

from __future__ import annotations

from collections.abc import Iterable, Sequence
from math import sqrt

StatVector = list[float | None]

NumberOrVector = int | Sequence[int] | None


def _as_vector(value: NumberOrVector) -> list[int]:
    if value is None:
        return []
    if isinstance(value, int):
        return [value]
    return list(value)


def extract_cost(requests: Iterable) -> StatVector:
    """Build the cost vector of a single sample from its (possibly repeated)
    responses.

    ``prompt_tokens`` and ``completion_tokens`` are summed across every request
    exposing a ``resp.usage``. The cached token count (index ``2``) is summed
    only when *every* contributing request informs
    ``usage.prompt_tokens_details.cached_tokens``; if any request does not
    report it, the sample cached value is ``None`` (i.e. "not informed").

    Returns an empty list when no request exposes usage information.
    """
    prompt_tokens = 0.0
    completion_tokens = 0.0
    cached_tokens = 0.0
    found_usage = False
    all_cached_informed = True

    for req in requests:
        usage = getattr(getattr(req, "resp", None), "usage", None)
        if usage is None:
            continue
        found_usage = True
        prompt_tokens += float(getattr(usage, "prompt_tokens", 0) or 0)
        completion_tokens += float(getattr(usage, "completion_tokens", 0) or 0)

        details = getattr(usage, "prompt_tokens_details", None)
        cached = (
            getattr(details, "cached_tokens", None) if details is not None else None
        )
        if cached is None:
            all_cached_informed = False
        else:
            cached_tokens += float(cached)

    if not found_usage:
        return []

    if not all_cached_informed:
        return [prompt_tokens, completion_tokens, None]

    return [prompt_tokens, completion_tokens, cached_tokens]


class ScalarStatsAccumulator:
    """Combines the scalar statistics of several datasets/children.

    - ``mean``/``median`` are the arithmetic mean of the incoming values.
    - ``std`` combines the incoming standard errors of the mean, i.e.
      ``sqrt(sum((std_i / sqrt(n_i)) ** 2))``.
    - ``n`` is the minimum ``n_i`` (coverage).
    """

    def __init__(self) -> None:
        self._mean_total = 0.0
        self._mean_count = 0
        self._median_total = 0.0
        self._median_count = 0
        self._sem_sq_total = 0.0
        self._sem_count = 0
        self._min_n: int | None = None

    def add_mean(self, value: float | None) -> None:
        if value is None:
            return
        self._mean_total += float(value)
        self._mean_count += 1

    def add_median(self, value: float | None) -> None:
        if value is None:
            return
        self._median_total += float(value)
        self._median_count += 1

    def add_sem(self, std: float | None, n: int | None) -> None:
        if std is None or n is None or n <= 0:
            return
        self._sem_sq_total += (float(std) / sqrt(n)) ** 2
        self._sem_count += 1

    def add_min(self, n: int | None) -> None:
        if n is None:
            return
        if self._min_n is None or n < self._min_n:
            self._min_n = n

    def mean(self) -> float:
        return self._mean_total / self._mean_count if self._mean_count else 0.0

    def median(self) -> float:
        return self._median_total / self._median_count if self._median_count else 0.0

    def std(self) -> float:
        return sqrt(self._sem_sq_total) if self._sem_count else 0.0

    def n(self) -> int:
        return self._min_n if self._min_n is not None else 0


class VectorStatsAccumulator:
    """Per-position, null-aware combination of vector statistics.

    Follows the same rules as :class:`ScalarStatsAccumulator` but independently
    for every position. Positions that only ever receive ``None``/absent values
    yield ``None`` for mean/median/std and ``0`` for ``n``.
    """

    def __init__(self) -> None:
        self._length = 0
        self._mean_total: list[float] = []
        self._mean_count: list[int] = []
        self._median_total: list[float] = []
        self._median_count: list[int] = []
        self._sem_sq_total: list[float] = []
        self._sem_count: list[int] = []
        self._min_n: list[int | None] = []

    def _grow(self, size: int) -> None:
        if size <= self._length:
            return
        self._length = size
        for lst in (
            self._mean_total,
            self._median_total,
            self._sem_sq_total,
        ):
            lst.extend([0.0] * (size - len(lst)))
        for lst in (
            self._mean_count,
            self._median_count,
            self._sem_count,
        ):
            lst.extend([0] * (size - len(lst)))
        self._min_n.extend([None] * (size - len(self._min_n)))

    def add_mean(self, vector: Sequence[float | None] | None) -> None:
        if not vector:
            return
        self._grow(len(vector))
        for position, value in enumerate(vector):
            if value is None:
                continue
            self._mean_total[position] += float(value)
            self._mean_count[position] += 1

    def add_median(self, vector: Sequence[float | None] | None) -> None:
        if not vector:
            return
        self._grow(len(vector))
        for position, value in enumerate(vector):
            if value is None:
                continue
            self._median_total[position] += float(value)
            self._median_count[position] += 1

    def add_sem(
        self,
        std_vector: Sequence[float | None] | None,
        n: NumberOrVector,
    ) -> None:
        if not std_vector:
            return
        self._grow(len(std_vector))
        n_vector = _as_vector(n)
        for position, value in enumerate(std_vector):
            if value is None:
                continue
            n_pos = (
                n
                if isinstance(n, int)
                else (n_vector[position] if position < len(n_vector) else 0)
            )
            if not n_pos or n_pos <= 0:
                continue
            self._sem_sq_total[position] += (float(value) / sqrt(n_pos)) ** 2
            self._sem_count[position] += 1

    def add_min(self, n: NumberOrVector) -> None:
        if n is None:
            return
        n_vector = _as_vector(n)
        if not n_vector:
            return
        self._grow(len(n_vector))
        for position, value in enumerate(n_vector):
            if value is None:
                continue
            if self._min_n[position] is None or value < self._min_n[position]:
                self._min_n[position] = value

    def mean(self) -> StatVector:
        return [
            (
                self._mean_total[position] / self._mean_count[position]
                if self._mean_count[position] > 0
                else None
            )
            for position in range(self._length)
        ]

    def median(self) -> StatVector:
        return [
            (
                self._median_total[position] / self._median_count[position]
                if self._median_count[position] > 0
                else None
            )
            for position in range(self._length)
        ]

    def std(self) -> StatVector:
        return [
            (
                sqrt(self._sem_sq_total[position])
                if self._sem_count[position] > 0
                else None
            )
            for position in range(self._length)
        ]

    def n(self) -> list[int]:
        return [
            self._min_n[position]
            if self._min_n[position] is not None
            else 0
            for position in range(self._length)
        ]
