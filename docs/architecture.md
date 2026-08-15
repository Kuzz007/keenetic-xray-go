# Architecture

## One binary, dispatched by subcommand

`cmd/keenetic-xray` is the only binary this project ships *for the
router*. There is no separate CLI/daemon/agent binary split on that side:
a stdlib-only Go binary is roughly the same size (~5-6MB stripped,
measured directly) regardless of which features are compiled in, since Go
statically links its runtime either way -- splitting into multiple
binaries would buy little and would reintroduce, at the binary level, the
"many near-duplicate artifacts" problem this project deliberately avoids
at the installer level (see below). `cmd/keenetic-xray-control-server` is
a genuinely separate binary, but deliberately so -- see
`docs/bot-control-design.md` for why the VPS side isn't part of this
split.

Current subcommands: `version`, `setup` (also the persistent interactive
management menu, not just a first-run wizard), `daemon`, `profile`,
`subscription`, `status`, `doctor`, `variant`, `agent` (configure/enable/
disable/status for the control-server polling loop -- Full variant only,
see `docs/bot-control-design.md`), `routes` (setup/sync/clear/status for
the Keenetic-native routing integration, see below), and the hidden
`internal postinst-setup` / `internal prerm-cleanup` used only by the
`.ipk`'s packaging scripts.

## Package layout

| Package | Responsibility |
|---|---|
| `internal/config` | `Config`/`Profile` types, `config.json` load/save/validate, `vless://` URI parsing and formatting, Xray-core JSON config generation |
| `internal/subscription` | Fetching, base64-decoding, and parsing V2Ray-style subscription URLs; remark-based re-matching of the previously-selected primary/backup across a refresh |
| `internal/xrayctl` | `Supervisor` (start/stop/restart an xray-core process, crash-restart with backoff) and `Probe` (health check through the live SOCKS5 proxy, including a minimal hand-rolled SOCKS5 CONNECT client -- this project has zero third-party Go dependencies, and `net/http.Transport` doesn't speak SOCKS5 on its own) |
| `internal/failover` | The failover state machine (`state.go`, pure logic, no I/O, driven via an `Actions` interface so it's unit-testable with fakes) and its wiring to real `xrayctl`/`config` (`daemon.go`) |
| `internal/diskspace` | Resolves the real filesystem behind a path (Entware routinely symlinks `/opt` to a USB mount, not the internal flash overlay) and reports free space via `statfs` |
| `internal/install` | The `.ipk` postinst/prerm logic: directory setup, the Mini/Full decision, and the "never overwrite an existing config.json" upgrade guarantee |
| `internal/botcontrol` | The router agent (`agent.go`, TLS-fingerprint-pinned polling client) and its command handlers (`commands.go`, thin wrappers over `internal/config`/`internal/subscription`/`internal/failover`); the control-server pieces the router never uses -- HTTP API (`server.go`), self-signed cert generation (`tls.go`), the persisted command queue (`queue.go`), and the Telegram bot (`telegram.go`) -- see `docs/bot-control-design.md` |
| `internal/keeneticroute` | Wires the local SOCKS5 inbound into KeeneticOS's own routing via `ndmc`: LAN-IP detection (`lanip.go`), the `Proxy0` interface (`proxy0.go`), and the curated domain-routing catalog (`catalog.go`, `routes.go`) -- see below |

## Why one unified `.ipk` instead of a `curl \| sh` script

Xray installers for Keenetic commonly ship as a `curl | sh` one-liner
that fetches and runs a shell script. This project ships a single
compressed `.ipk` per architecture instead, installed with `opkg install
<file-or-url>` -- no hosted package feed required, since `opkg` can
install directly from a local file or a URL.

The payoff: the control file declares `Depends: xray-core`, so `opkg`'s
own dependency resolution installs the Xray core from Entware's feed
*before* running this package's postinst -- "core first, then script" is
enforced by `opkg` itself, not a hand-rolled sequencing check.

`packaging/build-ipk.sh` builds the `.ipk` by hand: a single
gzip-compressed tar containing `./debian-binary`, `./data.tar.gz`, and
`./control.tar.gz`, in that order. **This is not the `ar`-wrapped format
`.deb` uses**, despite that being the commonly-documented convention for
`.ipk` too -- real Entware's own feed doesn't use it. That wrong
assumption originally shipped here and cost five failed real-hardware
install attempts to find: `ar t` on a real Entware-served `.ipk`
(`xray-core`'s own package, fetched directly from `bin.entware.net`)
fails with "invalid ar magic", while `tar tzf` lists its three members
directly. This is deliberately the primary path, not a fallback behind
`nfpm`: it's now verified end-to-end on real router hardware (`opkg
install` succeeds, the daemon starts via the generated init.d script),
not just structurally inspected.

## Failover state machine

Four states: `ACTIVE_PRIMARY`, `ACTIVE_BACKUP` (reached only after a
failed recovery confirmation, waiting out a backoff), `TESTING_RECOVERY`,
`COOLDOWN`. Health checks are a real HTTP GET through the live SOCKS5
proxy, never bare ICMP or a raw TCP connect -- those don't prove the
VLESS/TLS/auth path actually works, and ICMP is frequently filtered
independently of tunnel health anyway.

The mechanism is deliberately **asymmetric** even though the trigger
counts are symmetric (3 consecutive results, matching the original
spec): failing away from primary happens immediately on 3 consecutive
live-probe failures, with no pre-test, since there's nothing to protect
by double-checking a path already known to be bad. Failing back to
primary requires 3 consecutive successes against an *isolated, zero-risk*
throwaway `xray` instance before production traffic is touched at all,
then one live confirmation check -- and rolls back with a 5-minute
backoff if that confirmation fails, rather than retrying against a
flapping primary every 30 seconds.

## Router-level routing (`internal/keeneticroute`)

Outbound selection inside Xray itself is still just primary-vs-backup --
no geoip/geosite data, no per-domain outbound routing in the Xray config.
What *is* domain-aware is a separate, later layer: wiring the resulting
local SOCKS5 proxy into KeeneticOS's own routing so LAN traffic actually
reaches it, without the user hand-configuring the web UI.

This ports a working mechanism from the author's earlier
`keenetic_xray_installer` project (`xray-keenetic-routes.sh`,
`vless-go-lan-ip.sh`, the various `proxy0`/`recover` helpers there)
rather than reinventing it, since its rough edges were each earned by a
real failure mode on real hardware:

- **LAN IP detection is intentionally conservative.** The `Proxy0`
  interface's upstream must be the router's real LAN-facing address
  (tried in order against `Bridge0`/`Home`/`br0`, three different ways --
  `show running-config`, `show interface`, and the kernel's own interface
  list -- validated as a real RFC1918 address each time), never
  `127.0.0.1`: KeeneticOS's own proxy client (`ndm`) can't reach the
  Entware userland's SOCKS5 server over loopback even though they're the
  same physical box. `internal/keeneticroute/lanip.go`'s doc comment has
  the full story of why an earlier, simpler version that fell back to
  `127.0.0.1` on detection failure was actively harmful (a periodic
  re-apply silently replaced a working upstream with a dead loopback
  address). Detection never invents an address -- on failure it falls
  back to a cached last-known-good value rather than guessing, and
  callers must leave any existing upstream untouched if even that fails.
- **The `Proxy0` interface is created (or repaired) via `ndmc`**,
  KeeneticOS's own configuration CLI, reachable from the Entware shell on
  stock firmware -- `interface Proxy0` / `proxy protocol socks5` /
  `proxy upstream <lan-ip> <socks-port>` / `no ip global` (keeps it out
  of the default WAN fallback route set, so it's only used where a rule
  explicitly points at it) / `up`. Re-pointing an *existing* interface's
  upstream additionally cycles it down/up and reads the applied value
  back to confirm the change actually took -- ndmc accepting a command
  doesn't guarantee ndm applied it live.
- **Domain routing uses Keenetic's own `dns-proxy`/`object-group`
  mechanism**, not `iptables`/TPROXY: a curated catalog
  (`internal/keeneticroute/catalog.go`, ported verbatim from the
  reference project) of domains for commonly-blocked services (Apple,
  Meta/WhatsApp/Instagram, Telegram, TikTok, YouTube, Discord, Gemini)
  becomes one `object-group fqdn` per service, each routed through
  `Proxy0` with `dns-proxy route object-group <name> Proxy0 auto` --
  resolving one of those domains sends that connection through the
  proxy, nothing else is affected. `routes sync` clears and recreates
  its own managed groups on every run (converges on catalog changes
  rather than accumulating stale entries); `routes clear` removes them;
  `routes status` reports what's currently applied.

Every operation here goes through a `Runner` (`cmd string) (string,
error)`) rather than calling `ndmc` directly, so the whole package is
unit-testable with a fake on any machine -- `keeneticroute.Available()`
gates real usage on `ndmc` actually being present, which is false on
every dev machine and CI runner and true only on real Keenetic hardware.

## What this project deliberately does not do

- **No geoip/geosite routing data inside Xray.** Outbound selection is
  just primary-vs-backup; domain-based routing happens one layer up, in
  Keenetic's own `dns-proxy` mechanism (above), not in the Xray config
  itself.
- **No CLI↔daemon IPC yet.** `status`/`doctor` read `config.json`
  directly; they report saved configuration, not live daemon state. The
  bot-control agent doesn't need this either -- it runs *inside* `daemon`
  as a goroutine (see `docs/bot-control-design.md`), sharing the same
  in-memory `*failover.Daemon` rather than talking to it over IPC. A
  future version could still add a Unix-socket protocol for the CLI's
  benefit, but it wasn't needed to make the CLI/wizard genuinely useful,
  so it wasn't built speculatively.
