# Oikos Production Validation Benchmarks

This directory contains production-facing benchmark methodology and evidence.
Development-only benchmark helpers are intentionally excluded from the
production checkout.

## Measurement Rules

- Record commit, Go version, Docker Engine version, Linux distribution/kernel,
  CPU model/count, RAM, and storage type.
- Record server count, egg image, workload command, warm-up period, sample
  interval, and raw output path.
- Measure RSS from `/proc/<pid>/status` and CPU from process CPU time over a
  known interval. Report units explicitly.
- Never present missing production measurements as zero or as an estimate.

## Status

- Oikos idle baseline: measured, see `idle-resource-usage.md`.
- Wings head-to-head: **pending manual verification**.
- Production load at 10/50/100 servers: **pending manual verification**.
- Penetration and chaos validation on production VM: **pending manual verification**.
