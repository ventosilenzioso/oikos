# Egg Specification

An egg directory contains `egg.yaml` and its Dockerfile. Required fields:

```yaml
name: "Example"
slug: "example"
build:
  dockerfile: "./Dockerfile"
startup:
  command: "./server --port {{PORT}}"
variables:
  - name: "Port"
    env: "PORT"
    default: "25565"
    editable: true
ports:
  - name: "game"
    default: 25565
    protocol: "tcp"
```

`name`, `slug`, `build.dockerfile`, dan `startup.command` wajib. Variable
substitution memakai `{{ENV_NAME}}`. Port protocol hanya `tcp` atau `udp` dan
default port harus valid. Dockerfile tidak boleh mengandalkan akses host;
container data dipasang ke root data server oleh runtime.

Schema versioning egg dan compatibility matrix per egg: **pending manual verification**.
