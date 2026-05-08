# laghaim-go

Go rewrite workspace for the Laghaim server stack.

## Scope of this bootstrap

This repo now contains the initial rewrite foundation:

- baseline design docs in `docs/`
- Go project scaffold in `cmd/` and `internal/`
- protocol framing primitives in `internal/protocol/`
- session/ticket manager in `internal/session/`
- initial MySQL migrations in `migrations/`

## Reference sources

Primary reference repo:
- `/Users/daidai/ai/laghaim-server`

Client reference used during bootstrap:
- `/tmp/laghaim-client`
- `/tmp/laghaim-client-ref`
- upstream: <https://github.com/Topstack-first/20210521_directx_laghaim>

## Verification

```bash
make test
```

## Important note

The Python reference README claims XOR transport, but the current client `rnpacket.cpp`
uses SEED block encryption for framed packets. The Go rewrite keeps a `legacyxor`
package only to preserve the old reverse-note formula; it is not treated as the
primary verified wire codec.
