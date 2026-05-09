# laghaim-go

Go rewrite workspace for the Laghaim server stack.

## Scope of this bootstrap

This repo now contains the initial rewrite foundation and a tested P0 dev runtime:

- baseline design docs in `docs/`
- Go project scaffold in `cmd/` and `internal/`
- protocol framing + SEED transport in `internal/protocol/`
- session/ticket manager in `internal/session/`
- in-memory P0 auth/select + zone runtime in `internal/server/`
- starter service/repository layer in `internal/service/` and `internal/repo/`
- memory + MySQL repository implementations
- initial MySQL migrations in `migrations/`

## Reference sources

Primary reference repo:
- `/Users/daidai/ai/laghaim-server`

Client reference used during bootstrap:
- `/tmp/laghaim-client`
- `/tmp/laghaim-client-ref`
- upstream: <https://github.com/Topstack-first/20210521_directx_laghaim>

## Run the P0 dev cluster

```bash
go run ./cmd/dev-cluster -config configs/dev.yaml
```

This starts:

- auth/select gateway on `:4005`
- zone server on `:4008`

`configs/dev.yaml` now also exposes the address that will be returned in `go_world`:

- `zone_server.advertise_host`
- `zone_server.advertise_port`

For local runs keep them at `127.0.0.1:4008`; for remote clients set them to the client-reachable public/private address instead of `0.0.0.0`.

Storage backend is configured in `configs/dev.yaml`:

- `storage.backend: memory` for disposable local runs
- `storage.backend: mysql` for persistent runs backed by `internal/repo/mysql`

## Local MySQL helpers

For local socket-based MySQL / MariaDB setups:

```bash
make mysql-init
make mysql-migrate-reset
make mysql-smoke
make dev-cluster-mysql
```

Useful direct commands:

```bash
bash scripts/mysql-local.sh status
bash scripts/mysql-local.sh dsn
bash scripts/migrate-mysql.sh up
```

`mysql-smoke` runs the live MySQL-backed auth/select/zone integration test.

## Verification

```bash
make test
```

## Deployment status

Current deployable runtime is still `cmd/dev-cluster`.

The standalone `cmd/login-server`, `cmd/game-manager`, and `cmd/zone-server` binaries are bootstrap placeholders today; they are not yet wired into a distributed session/handoff topology. If you want a real playable environment right now, deploy the dev-cluster process and point `zone_server.advertise_host/advertise_port` at the address the client can actually reach.

## Important note

The Python reference README claims XOR transport, but the current client `rnpacket.cpp`
uses SEED block encryption for framed packets. The Go rewrite now implements the
client-matching SEED transport and keeps `internal/protocol/legacyxor` only as a
reverse-note compatibility aid.

The current client-visible topology is also two-stage in practice:

1. auth/select gateway
2. zone reconnect via `go_world`

So the rewrite keeps GMS as a logical service boundary even when the P0 dev runtime
co-locates it behind the gateway listener.
