# Fase 6 Benchmark Methodology

`measure-daemon.sh` records commit, Go/Docker/kernel versions and one-second
RSS samples. It requires an existing daemon PID and writes only to the chosen
output directory.

`load-servers.sh` accepts 0, 10, 50, or 100 as workload sizes, but does not
pretend to generate production load. Wings comparison and production load
results are **pending manual verification** on a dedicated VM.

`run-chaos.sh` requires `OIKOS_CHAOS_ENABLE=1` and records scenarios without
performing destructive host operations by default.
