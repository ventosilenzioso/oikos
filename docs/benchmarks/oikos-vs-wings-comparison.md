# Oikos vs Wings

Status: **pending manual verification**.

Run both daemons on separate equivalent Linux VMs:

- Ubuntu LTS, identical kernel family, CPU model/count, RAM, storage, and Docker
  Engine version.
- Oikos production commit and the exact Wings release selected by the operator;
  record both versions in the report.
- Identical egg image, server command, environment, resource limits, and server
  counts of 0, 10, 50, and 100.
- Five-minute warm-up followed by 60-second measurement with one-second samples.
- Measure daemon RSS/CPU, container start latency, command p50/p95/p99 latency,
  Docker API errors, and crash recovery time.
- Repeat each scenario three times and report mean plus variance.

Do not claim Oikos is lighter until raw measurements from both VMs are attached.
