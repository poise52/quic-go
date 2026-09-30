# 3x-ui local QUIC patch

Pinned upstream: `github.com/apernet/quic-go` at
`v0.61.1-0.20260806010916-184d081eef3e`.
The root `go.mod` replaces this module with this directory.

Local changes:

- `Config.ConfigureCongestionControl` configures each new connection before
  the handshake. `(*Conn).SetCongestionControlFactory` installs Xray BBR and
  reapplies it when the path changes.
- `(*Conn).SetCubicCongestionControl` delegates to `internal/ackhandler` and
  selects the existing CUBIC or New Reno implementation.
- Path MTU changes reset the sender datagram size and the cached maximum
  payload estimate, preserving the selected controller through migration.

The patch does not copy congestion algorithms or expose internal sender types.
Run `go test -shuffle=on -count=1 . ./internal/ackhandler` and its `-race`
variant after changing the pinned dependency. Root `make test-go`, `make race`,
and CI include these packages. TUIC tests also inspect actual sender selection
and exercise authenticated TCP, native UDP and stream UDP across live
`Manager.Ensure` changes with old connections still active.
