# Full vs Mini

Mini/Full is **not** a packaging split -- there is one `.ipk` per
architecture, always. It's a runtime flag (`"variant": "mini"|"full"` in
`config.json`), decided automatically by the `.ipk`'s postinst based on
free disk space, and changeable afterward with `keenetic-xray variant
set mini|full`.

## Why a runtime flag instead of separate packages

A stdlib-only Go binary is roughly the same size regardless of which
features are compiled in (Go statically links its runtime either way),
so gating by *binary size* wouldn't accomplish much. What actually
matters on a disk-constrained router is which features are **allowed to
run** and how much state they accumulate over time (logs, history) --
that's what Mini/Full actually controls.

## The threshold

`internal/install.DefaultMiniThresholdBytes` = **43MB**, measured at
`/opt` at postinst time. Because the `.ipk`'s `Depends: xray-core` makes
`opkg` install the Xray core *before* postinst runs, this single
free-space reading already nets out the core's own footprint for free --
no subtraction math needed. Rule: less than 43MB free at that point picks
Mini, otherwise Full.

For context, real measurements taken while building this project (not
estimates): the bare `xray` binary alone (no geodata, which this project
doesn't use anyway) is about 32-33MB regardless of architecture; the
Entware-packaged `xray-core` `.ipk` itself was about 10MB compressed at
the time of measurement. Neither number is independently re-verified by
this codebase at runtime -- if Entware's package or upstream Xray grows
significantly, the 43MB threshold may need revisiting, but there's
nothing here that depends on the exact number staying accurate; it's a
single named constant.

## What's gated

| Feature | Mini | Full |
|---|---|---|
| xray-core + failover daemon, local SOCKS5/HTTP inbound | yes | yes |
| `profile`/`subscription` CLI + `setup` wizard, incl. subscription-link parsing | yes | yes |
| Manual `subscription refresh` | yes | yes |
| Log/state retention | small | larger, including failover + bot audit history |
| Remote control agent (`agent enable`) | off by default | on by default |
| Router-level routing (`routes setup`/`sync`/`clear`/`status`) | yes | yes |

Subscription-link support is deliberately **not** Full-gated, unlike a
naive port of the reference project's split would suggest: a real
fraction of VLESS providers only ever hand out a subscription URL, never
a raw link, and gating that behind Full would lock the most disk-
constrained (often oldest-hardware) users out of onboarding entirely.
What's actually gated is remote/automatic triggering, via the same
control-agent enable flag Full/Mini already needs -- no separate axis.

Router-level routing is likewise ungated: it stores no meaningful state
of its own (the LAN-IP cache is a few bytes) and is what actually makes
the proxy useful without a manual web UI detour, which matters just as
much on a disk-constrained Mini install as on Full. This matches the
reference project, where the equivalent Proxy0/domain-routing mechanism
is shared by both its Minimal Go and full backends rather than being a
Full-only feature.
