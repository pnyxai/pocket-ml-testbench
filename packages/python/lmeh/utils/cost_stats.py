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


def extract_cost(requests: Iterable) -> StatVector:
    """Build the cost vector of a single sample from its (possibly repeated)
    responses.

    Each position is the **mean** over the responses that inform it, so a doc
    with several requests contributes one representative value per position:

    - ``prompt_tokens`` and ``completion_tokens`` are averaged over every
      request exposing a ``resp.usage``.
    - ``cached_tokens`` (index ``2``) is averaged over the requests that report
      ``usage.prompt_tokens_details.cached_tokens``. Requests that do not report
      it are simply excluded from that average, so partial information is kept
      instead of discarding the whole position.

    Returns an empty list when no request exposes usage information.
    """
    prompt_tokens = 0.0
    completion_tokens = 0.0
    cached_tokens = 0.0
    usage_count = 0
    cached_count = 0

    for req in requests:
        usage = getattr(getattr(req, "resp", None), "usage", None)
        if usage is None:
            continue
        usage_count += 1
        prompt_tokens += float(getattr(usage, "prompt_tokens", 0) or 0)
        completion_tokens += float(getattr(usage, "completion_tokens", 0) or 0)

        details = getattr(usage, "prompt_tokens_details", None)
        cached = (
            getattr(details, "cached_tokens", None) if details is not None else None
        )
        if cached is not None:
            cached_tokens += float(cached)
            cached_count += 1

    if usage_count == 0:
        return []

    mean_cached = cached_tokens / cached_count if cached_count > 0 else None

    return [
        prompt_tokens / usage_count,
        completion_tokens / usage_count,
        mean_cached,
    ]


def _at(values: Sequence[float | None] | None, position: int):
    if values is None or position >= len(values):
        return None
    return values[position]


class ScalarStatsAccumulator:
    """Combines the scalar statistics of several groups (datasets/children).

    For each group ``i`` we receive its ``mean``, ``median``, its standard error
    ``se_i`` and the degrees of freedom ``df_i`` associated with that error.
    Datasets use ``se_i = std_i / sqrt(n_i)`` and ``df_i = n_i - 1``; when
    combining already-aggregated children (``root_c``), the child's ``se`` and
    ``v_eff`` are passed instead.

    - ``mean``/``median`` are the arithmetic mean of the incoming values.
    - ``se`` is the standard error of that (unweighted) mean:
      ``sqrt(sum(V_i)) / K`` with ``V_i = se_i ** 2`` and ``K`` the number of
      groups.
    - ``v_eff`` is the Welch-Satterthwaite effective degrees of freedom:
      ``(sum(V_i)) ** 2 / sum(V_i ** 2 / df_i)``.
    - ``n`` is the minimum ``n_i`` (coverage), used for ``sample_min``.
    """

    def __init__(self) -> None:
        self._mean_total = 0.0
        self._mean_count = 0
        self._median_total = 0.0
        self._median_count = 0
        self._v_total = 0.0
        self._ws_denominator = 0.0
        self._min_n: int | None = None

    def add(
        self,
        mean: float | None,
        median: float | None,
        se: float | None,
        n: int | None,
        df: float | None,
    ) -> None:
        if mean is None or n is None or n <= 0:
            return
        self._mean_total += float(mean)
        self._mean_count += 1

        if median is not None:
            self._median_total += float(median)
            self._median_count += 1

        if se is not None:
            variance = float(se) ** 2
            self._v_total += variance
            if df is not None and df > 0:
                self._ws_denominator += (variance**2) / df

        if self._min_n is None or n < self._min_n:
            self._min_n = n

    def mean(self) -> float:
        return self._mean_total / self._mean_count if self._mean_count else 0.0

    def median(self) -> float:
        return self._median_total / self._median_count if self._median_count else 0.0

    def se(self) -> float:
        # Standard error of the reported (unweighted) mean.
        return sqrt(self._v_total) / self._mean_count if self._mean_count else 0.0

    def v_eff(self) -> float:
        if self._ws_denominator > 0:
            return (self._v_total**2) / self._ws_denominator
        return 0.0

    def n(self) -> int:
        return self._min_n if self._min_n is not None else 0


class VectorStatsAccumulator:
    """Per-position, null-aware combination of vector statistics.

    Follows the same rules as :class:`ScalarStatsAccumulator` independently for
    every position. ``df`` is provided per position (``n_i - 1`` for datasets,
    the child's ``v_eff`` for aggregated children). Positions that only ever
    receive ``None``/absent values yield ``None`` for mean/median/se/v_eff.
    """

    def __init__(self) -> None:
        self._length = 0
        self._mean_total: list[float] = []
        self._mean_count: list[int] = []
        self._median_total: list[float] = []
        self._median_count: list[int] = []
        self._v_total: list[float] = []
        self._ws_denominator: list[float] = []

    def _grow(self, size: int) -> None:
        if size <= self._length:
            return
        self._length = size
        for lst in (
            self._mean_total,
            self._median_total,
            self._v_total,
            self._ws_denominator,
        ):
            lst.extend([0.0] * (size - len(lst)))
        for lst in (self._mean_count, self._median_count):
            lst.extend([0] * (size - len(lst)))

    def add(
        self,
        mean_vector: Sequence[float | None] | None,
        median_vector: Sequence[float | None] | None,
        se_vector: Sequence[float | None] | None,
        df_vector: Sequence[float | None] | None,
    ) -> None:
        if not mean_vector:
            return
        self._grow(len(mean_vector))
        for position, mean in enumerate(mean_vector):
            if mean is None:
                continue
            self._mean_total[position] += float(mean)
            self._mean_count[position] += 1

            median = _at(median_vector, position)
            if median is not None:
                self._median_total[position] += float(median)
                self._median_count[position] += 1

            se = _at(se_vector, position)
            if se is not None:
                variance = float(se) ** 2
                self._v_total[position] += variance
                df = _at(df_vector, position)
                if df is not None and df > 0:
                    self._ws_denominator[position] += (variance**2) / df

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

    def se(self) -> StatVector:
        # Standard error of the reported (unweighted) mean, per position.
        return [
            (
                sqrt(self._v_total[position]) / self._mean_count[position]
                if self._mean_count[position] > 0
                else None
            )
            for position in range(self._length)
        ]

    def v_eff(self) -> StatVector:
        result: StatVector = []
        for position in range(self._length):
            if self._ws_denominator[position] > 0:
                result.append(
                    (self._v_total[position] ** 2) / self._ws_denominator[position]
                )
            elif self._mean_count[position] > 0:
                result.append(0.0)
            else:
                result.append(None)
        return result
