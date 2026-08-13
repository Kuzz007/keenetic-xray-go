# Architecture

## One binary, dispatched by subcommand

`cmd/keenetic-xray` is the only binary this project ships. There is no
separate CLI/daemon/agent binary split: a stdlib-only Go binary is
roughly the same size (~5-6MB stripped, measured directly) regardless of
which features are compiled in, since Go statically links its runtime
either way -- splitting into multiple binaries would buy little and
would reintroduce, at the binary level, the "many near-duplicate
artifacts" problem this project deliberately avoids at the installer
level (see below).

Current subcommands: `version`, `setup`, `daemon`, `profile`,
`subscription`, `status`, `doctor`, `variant`, and the hidden `internal
postinst-setup` / `internal prerm-cleanup` used only by the `.ipk`'s
packaging scripts. `agent` (the control-server polling loop) is not
built yet.

## Package layout

| Package | Responsibility |
|---|---|
| `internal/config` | `Config`/`Profile` types, `config.json` load/save/validate, `vless://` URI parsing and formatting, Xray-core JSON config generation |
| `internal/subscription` | Fetching, base64-decoding, and parsing V2Ray-style subscription URLs; remark-based re-matching of the previously-selected primary/backup across a refresh |
| `internal/xrayctl` | `Supervisor` (start/stop/restart an xray-core process, crash-restart with backoff) and `Probe` (health check through the live SOCKS5 proxy, including a minimal hand-rolled SOCKS5 CONNECT client -- this project has zero third-party Go dependencies, and `net/http.Transport` doesn't speak SOCKS5 on its own) |
| `internal/failover` | The failover state machine (`state.go`, pure logic, no I/O, driven via an `Actions` interface so it's unit-testable with fakes) and its wiring to real `xrayctl`/`config` (`daemon.go`) |
| `internal/diskspace` | Resolves the real filesystem behind a path (Entware routinely symlinks `/opt` to a USB mount, not the internal flash overlay) and reports free space via `statfs` |
| `internal/install` | The `.ipk` postinst/prerm logic: directory setup, the Mini/Full decision, and the "never overwrite an existing config.json" upgrade guarantee |

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

`packaging/build-ipk.sh` builds the `.ipk` by hand (an `ar` archive of
`debian-binary` + `control.tar.gz` + `data.tar.gz` -- the format doesn't
need dedicated tooling). This is deliberately the primary path, not a
fallback behind `nfpm`: it was hand-verified (built a real `.ipk` from a
cross-compiled binary and inspected its structure directly) in an
environment with no `opkg` to test installation against, so the already-
verified path was kept over an nfpm integration that couldn't be
verified the same way.

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

## What this project deliberately does not do

- **No geoip/geosite routing data.** Outbound selection is just
  primary-vs-backup; there's no domain/IP-based split-tunneling. Routing
  all LAN traffic through the local proxy port is Keenetic's own
  Policy-Based Routing feature, configured separately in the router's
  web UI -- this project never touches `iptables`/TPROXY.
- **No CLI↔daemon IPC yet.** `status`/`doctor` read `config.json`
  directly; they report saved configuration, not live daemon state. A
  future version could add a Unix-socket protocol for this, but it
  wasn't needed to make the CLI/wizard genuinely useful, so it wasn't
  built speculatively.
- **No remote bot control yet.** Planned as a VPS control-server +
  polling router agent (the same shape the reference `keenetic_xray_installer`
  project uses, reimplemented fresh) -- deliberately sequenced last since
  it's the single largest remaining scope item and everything else here
  is independently useful without it.
