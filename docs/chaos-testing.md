# Chaos Testing

The safe harness records Docker unavailable, disk unwritable/full, Panel
unreachable, and SQLite locked scenarios. Use `OIKOS_CHAOS_ENABLE=1` explicitly.
System-level failure injection and production VM validation are **pending manual
verification**. Expected output must include exit code, stdout/stderr, and
observable health/event result for each scenario.
