# Production Release Checklist

- [ ] Full `go test ./...`, `go vet ./...`, and formatting checks pass.
- [ ] `gosec` high gate passes and complete report reviewed.
- [ ] `govulncheck` report reviewed; residual vulnerabilities documented.
- [ ] Binary built reproducibly in CI with SHA-256 checksum.
- [ ] Release binary signed with production key.
- [ ] Semantic version tag created.
- [ ] Config/database/plugin manifest backup verified.
- [ ] Install tested from zero on clean Linux VM.
- [ ] Systemd graceful handoff verified on clean VM.
- [ ] Wings comparison and production load tests completed.
- [ ] gRPC/SFTP penetration test reviewed.
- [ ] Chaos scenarios completed with predictable logs/events.
- [ ] Threat model reviewed by a second person.

Items requiring VM, production credentials, or external systems are explicitly
pending manual verification until evidence is attached.
