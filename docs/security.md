# Security Model

## Assets and Controls

| Asset | Threat | Control |
|---|---|---|
| mTLS cert/key | Node impersonation | Key/cert mode 0600, CA validation, rotation policy |
| Pairing token | Token theft/reuse | One-time validation and short expiry at Panel |
| Server files | Traversal/symlink escape | Central `ResolvePath`, Lstat component checks, jailed SFTP |
| Docker host | Container escape | Seccomp, optional LSM profile, user namespace, Docker least privilege |
| Plugin process | Host state access/crash | Unix socket 0600, capability allowlist, subprocess isolation |
| Update binary | Tampering/rollback loss | SHA-256, signature, version slots, state manifest, backup |

Seccomp/AppArmor/SELinux/user namespace status harus dilaporkan doctor secara
akurat. Unsupported host memakai fallback eksplisit, bukan klaim sandbox aktif.

Gosec high gate tidak menemukan high issue pada audit ini. Govulncheck masih
melaporkan dependency Docker/Moby tanpa upstream fixed version; hal tersebut
tetap dicatat sebagai residual risk dan tidak disuppress diam-diam.

Penetration test API gRPC/SFTP dan review pihak kedua: **pending manual verification**.
