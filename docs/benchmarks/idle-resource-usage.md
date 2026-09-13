# Idle Resource Usage

## Scope

This measures the Oikos daemon with no active game server and no real NAT
tunnel. It is a baseline, not a claim about load behavior or comparison with
Wings.

## Recorded Result

Measurement date: 2026-09-13  
Source: previously validated Fase 1-6 benchmark run, commit `982c99c`  
Host: Linux amd64, cgroup v2  
Samples: 12 samples at 5-second intervals after warm-up  
Panel: local mock Panel over mTLS  
Runtime: no active server; `/bin/true` used as the frpc process fixture

| Condition | RSS min | RSS max | RSS average | CPU max | CPU average |
|---|---:|---:|---:|---:|---:|
| 0 active servers | 20.56 MB | 20.66 MB | 20.64 MB | 0.20% | 0.02% |

Raw result:

```text
RSS_KB min=21052 max=21160 avg=21138
CPU_PCT max=0.20 avg=0.02
```

The post-cleanup rerun was not claimed as a new measurement: development
benchmark helpers and mock Panel source were intentionally removed from the
production checkout. A production checkout build exposed and fixed a separate
duration YAML serialization issue with regression test coverage.

## Interpretation

The recorded baseline is below the original idle target of 30 MB RAM and 1%
CPU. It does not include real frpc overhead, active containers, or multiple
servers.

## Pending Manual Verification

Run on a dedicated Linux VM with the production build:

- 0, 10, 50, and 100 active servers.
- Same egg image and workload for every measurement.
- At least 60 seconds of samples after a 5-minute warm-up.
- Record RSS, CPU, Docker API latency/errors, and container count.
