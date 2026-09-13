# Load Test: 100 Servers

Status: **pending manual verification**.

On a dedicated VM, run separate scenarios with 10, 50, and 100 active server
containers using the same egg and lightweight workload. Record successful
create/start/stop/delete commands, p95/p99 response time, RSS/CPU, Docker API
errors, disk usage, and recovery behavior. Use a five-minute warm-up and repeat
each scenario three times. Stop before host exhaustion and retain raw logs.
