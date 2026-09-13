# Architecture

## Platform Support

Linux is Oikos's production target. The Linux build provides cgroup v2
resource enforcement, Unix-domain plugin sockets, and Unix process groups.

The public platform boundaries live in `internal/platform` and
`internal/resource`. They keep operating-system imports behind build tags.
Darwin and Windows builds are compile/API checks, not claims of full production
support. Features that require Linux cgroups, Unix sockets, or Unix process
groups return an explicit `ErrNotSupported` rather than silently degrading.

Cross-platform validation uses compile-only commands because a binary built for
another OS cannot execute on the host:

```sh
GOOS=linux go test ./...
for os in darwin windows; do
  mkdir -p "/tmp/oikos-cross/$os"
  GOOS="$os" go list ./... | while read -r pkg; do
    name=$(printf '%s' "$pkg" | tr '/.' '__')
    GOOS="$os" go test -c -o "/tmp/oikos-cross/$os/$name.test" "$pkg"
  done
done
```

The Linux command runs tests on the host. The Darwin and Windows commands only
compile test binaries; those binaries cannot execute on a Linux host.
