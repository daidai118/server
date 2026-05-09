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

## Verification

```bash
make test
```

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
