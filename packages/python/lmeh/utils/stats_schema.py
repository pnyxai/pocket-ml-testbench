"""Strict parsing of statistics documents written by the manager/migration.

The Pydantic models in :mod:`packages.python.protocol.protocol` describe the
on-disk schema.  They intentionally have no field defaults: a missing field is
a schema error, while a zero-valued field is a legitimate "no samples" value.
The helpers here keep the failure messages actionable by adding the supplier /
task / taxonomy context that the model itself does not know.
"""

from __future__ import annotations

from pydantic import ValidationError

from packages.python.protocol.protocol import (
    RawTaskStats,
    TaskStats,
    TaxonomyNodeSummary,
)


class StatsSchemaError(ValueError):
    """Raised when a statistics document does not match the new schema."""


def _validation_message(exc: ValidationError) -> str:
    details = []
    for error in exc.errors():
        location = ".".join(str(part) for part in error["loc"]) or "<root>"
        details.append(f"{location}: {error['msg']}")
    return "; ".join(details)


def _require_mapping(value, path: str, context: str) -> dict:
    if not isinstance(value, dict):
        raise StatsSchemaError(f"{context}: missing or invalid '{path}' object")
    return value


def _validate(model, value, path: str, context: str):
    data = _require_mapping(value, path, context)
    try:
        return model.model_validate(data)
    except ValidationError as exc:
        raise StatsSchemaError(
            f"{context}: invalid '{path}': {_validation_message(exc)}"
        ) from exc


def parse_raw_task_stats(value, context: str) -> RawTaskStats:
    """Parse raw buffer/task statistics, failing on any missing field."""

    return _validate(RawTaskStats, value, "stats", context)


def parse_task_stats(value, context: str) -> TaskStats:
    """Parse aggregated taxonomy statistics, failing on any missing field."""

    return _validate(TaskStats, value, "stats", context)


def parse_taxonomy_node(node, context: str) -> TaxonomyNodeSummary:
    """Parse one taxonomy summary node, including ``sample_min``."""

    data = _require_mapping(node, "taxonomy node", context)
    if "stats" not in data:
        raise StatsSchemaError(f"{context}: missing 'stats'")
    if "sample_min" not in data:
        raise StatsSchemaError(f"{context}: missing 'sample_min'")

    stats = parse_task_stats(data["stats"], context)
    try:
        return TaxonomyNodeSummary(stats=stats, sample_min=data["sample_min"])
    except ValidationError as exc:
        raise StatsSchemaError(
            f"{context}: invalid 'sample_min': {_validation_message(exc)}"
        ) from exc
