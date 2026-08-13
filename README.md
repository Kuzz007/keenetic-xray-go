# keenetic-xray-go

A single Xray (VLESS) failover installer and manager for Keenetic routers
running Entware, targeting **mipsel** and **aarch64** only.

Status: early development (see `docs/architecture.md` once M1+ lands).

## What this is

- One Go binary (`keenetic-xray`), one `.ipk` per architecture — no
  generated multi-script installer pipeline.
- Automatic failover between a primary and backup VLESS profile, health
  checked through the live proxy (not bare ICMP), with an isolated
  pre-test before failing back to primary.
- Accepts either a raw `vless://` link or a subscription URL during setup.
- Local interactive setup menu over SSH, plus optional remote control via
  a Telegram bot through a VPS control-server.
- Installs the Xray core from Entware's own `opkg` feed as a package
  dependency; ships no geoip/geosite routing data — whole-LAN redirection
  through the proxy port is handled by Keenetic's own Policy-Based
  Routing, not by this project.

## Relationship to `keenetic_xray_installer`

This is a separate, from-scratch project by the same author. It does not
share code with `keenetic_xray_installer`; that project remains a useful
reference for proven patterns (build flags, CI shape, failover safety
design) but nothing here is copied from it.

## Building

```sh
go build -o keenetic-xray ./cmd/keenetic-xray
```

Cross-compiling for the router architectures:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm64  go build -trimpath -ldflags "-s -w" -o dist/keenetic-xray-linux-arm64  ./cmd/keenetic-xray
CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -trimpath -ldflags "-s -w" -o dist/keenetic-xray-linux-mipsle ./cmd/keenetic-xray
```

## License

MIT — see `LICENSE`.
