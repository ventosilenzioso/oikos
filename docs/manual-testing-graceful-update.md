# Manual Testing: Graceful Update

Status automated logic: verified.  
Status real systemd handoff: **pending manual verification**.

Automated tests cover the required logical order:

1. Backup config, database, and plugin manifest.
2. Health-check candidate binary.
3. Run schema migration in a transaction.
4. Drain the old daemon from new commands.
5. Let the new daemon take over Panel/monitoring connections.
6. Exit the old daemon.

## VM Procedure

1. Use a clean Linux VM with Docker and systemd enabled.
2. Install Oikos version A and create a long-running container.
3. Record container ID and start time:

```bash
docker inspect -f '{{.Id}} {{.State.StartedAt}}' <container>
```

4. Confirm `current` and `previous` slots and make a backup of config/database.
5. Run `oikos update` with a signed version B release.
6. Follow journal output and verify candidate health-check, drain, takeover,
   and old-process exit.
7. Query the Panel and local API while the handoff occurs.
8. Inspect the container again and verify ID and `StartedAt` are unchanged.
9. Simulate a failed candidate health-check and verify current slot, config,
   database, and plugin manifest remain recoverable.
10. Run `oikos doctor` after success and after rollback.

Systemd-specific socket activation, process supervision, signal delivery, and
host reboot recovery are deliberately not automated in the development sandbox.
