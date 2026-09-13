# Graceful Update

`oikos update` adalah operasi manual. Candidate binary di-download ke temporary
file, diverifikasi checksum SHA-256 dan signature, lalu diuji dengan
`--health-check-only` sebelum masuk ke versioned slot.

Layout slot:

```text
/var/lib/oikos/versions/
  v0.1.0/oikos
  v0.2.0/oikos
  current -> v0.2.0
  previous -> v0.1.0
  state.json
```

Config, database SQLite, dan plugin manifest dibackup sebelum switch. `state.json`
menjadi sumber otoritatif pasangan current/previous; symlink adalah compatibility
projection dan diperbaiki dari state saat startup/rollback.

Container tidak direstart karena dikelola Docker daemon secara independen.
Candidate health failure atau signature/checksum failure tidak mengubah slot
aktif. Jika proses baru gagal readiness setelah switch, rollback mengembalikan
slot tervalidasi sebelumnya dan menyimpan diagnostics recovery.
