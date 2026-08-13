# keenetic-xray-go

A single Xray (VLESS) failover installer and manager for Keenetic routers
running Entware, targeting **mipsel** and **aarch64** only.

Status: v0.1.0 checkpoint reached — install, configure, and run automatic
failover works end to end. Remote (Telegram bot) control is not built yet.
See `docs/architecture.md` for how it fits together and
`docs/full-vs-mini.md` for the Mini/Full variant split.

## What this is

- One Go binary (`keenetic-xray`), one `.ipk` per architecture — no
  generated multi-script installer pipeline.
- Automatic failover between a primary and backup VLESS profile, health
  checked with a real HTTP request through the live proxy (not bare
  ICMP), with an isolated pre-test before failing back to primary.
- Accepts either a raw `vless://` link or a subscription URL, both from
  the CLI (`keenetic-xray setup`) and (once built) the remote bot.
- Installs the Xray core from Entware's own `opkg` feed as a package
  dependency (`Depends: xray-core` in the `.ipk` control file); ships no
  geoip/geosite routing data — whole-LAN redirection through the local
  proxy port is handled by Keenetic's own Policy-Based Routing, not by
  this project.

## Installing

Download the `.ipk` matching your router's architecture from the
[latest release](https://github.com/Kuzz007/keenetic-xray-go/releases/latest)
and install it directly — no package feed to add first:

```sh
opkg install keenetic-xray_<version>_aarch64-3.10.ipk   # newer, ARM-based models
opkg install keenetic-xray_<version>_mipsel-3.4.ipk     # older, MIPS-based models
```

`opkg` installs the `xray-core` dependency from the Entware feed
automatically before running this package's postinst. Once installed:

```sh
keenetic-xray setup     # paste a vless:// link or a subscription URL
keenetic-xray daemon    # run the failover daemon in the foreground
```

(An init.d script starts the daemon automatically on boot/install --
`daemon` above is for running it in the foreground, e.g. to watch logs.)

## CLI reference

```
keenetic-xray version
keenetic-xray setup
keenetic-xray daemon
keenetic-xray profile {add <vless-uri>|list|remove <index>}
keenetic-xray subscription {set-url <url>|refresh|list|set-primary <i>|set-backup <i>}
keenetic-xray status
keenetic-xray doctor
keenetic-xray variant {show|set mini|set full}
```

`status`/`doctor` currently report saved configuration only (`config.json`),
not live daemon state -- there's no CLI↔daemon IPC layer yet.

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

Building a `.ipk` locally (see `docs/architecture.md` for why this script
exists instead of relying solely on goreleaser's `nfpm` integration):

```sh
sh packaging/build-ipk.sh <version> aarch64-3.10 dist/keenetic-xray-linux-arm64 keenetic-xray_<version>_aarch64-3.10.ipk
```

## License

MIT — see `LICENSE`.
