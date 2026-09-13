# Architecture Notes

Linux adalah target production utama. Cgroups, Unix process groups, Unix socket,
dan filesystem permission diisolasi melalui interface atau build tags.

Cross-platform compile checks dijalankan tanpa mengeksekusi binary foreign OS:

```bash
GOOS=linux go test ./...
GOOS=darwin go test -c ./internal/platform
GOOS=windows go test -c ./internal/platform
```

Darwin dan Windows compile support bukan klaim node production penuh; fitur
Linux-only mengembalikan `ErrNotSupported` di platform tersebut.
