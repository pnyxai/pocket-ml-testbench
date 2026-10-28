"""Helpers for detecting documents written before the PR #245 stats schema.

The new readers expect nested ``stats`` (``buffers_numerical``,
``suppliers_snapshots.tasks``) and ``stats`` on every taxonomy node.  Old
documents keep the flat statistics / ``score_dev`` fields instead.  Rather than
silently treating them as zero/no-data, the summarizer fails fast and tells the
operator to run the one-time migration.
"""


def _sample_count(doc: dict) -> int:
    return int(doc.get("num_samples") or doc.get("samples") or 0)


def is_legacy_buffer_stats(doc: dict) -> bool:
    """True when a buffer/task document has old flat stats and real samples."""
    return doc.get("stats") is None and _sample_count(doc) > 0


def is_legacy_taxonomy_node(node: dict) -> bool:
    """True when a taxonomy summary node still uses the old flat schema."""
    return node.get("stats") is None
