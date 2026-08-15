package keeneticroute

import (
	"bufio"
	"fmt"
	"strings"
	"time"
)

// sleepBetweenDownUp is how long FixUpstream waits between bringing the
// interface down and back up -- load-bearing in the reference
// implementation (proxy0-lan-upstream-fix.sh): it's how long ndm needs
// to tear down the old upstream connection before re-establishing it
// with the new one. Overridable so tests don't actually wait 2 seconds.
var sleepBetweenDownUp = 2 * time.Second
var sleepFunc = time.Sleep

// InterfaceExists reports whether iface is already configured.
func InterfaceExists(run Runner, iface string) bool {
	_, err := run(fmt.Sprintf("show interface %s", iface))
	return err == nil
}

// EnsureProxyInterface performs the full declarative setup of a fresh
// Proxy interface wrapping a SOCKS5 upstream -- safe to call even if
// iface already exists (every step just re-applies the same
// configuration). Ported byte-for-byte from the command sequence used
// independently by vless-go-recover.sh, vless-go-socks-auth.sh, and
// minimal-go-backend.sh in the reference project, which agree on it
// exactly. "no ip global" keeps this interface out of the default WAN
// fallback route set so it's only ever used where a routing rule
// explicitly points at it -- without it, ndm could start sending
// ordinary internet traffic through the proxy by default.
func EnsureProxyInterface(run Runner, iface, upstreamHost string, upstreamPort int) error {
	if _, err := run(fmt.Sprintf("interface %s", iface)); err != nil {
		return fmt.Errorf("creating interface %s: %w", iface, err)
	}
	if _, err := run(fmt.Sprintf("interface %s proxy protocol socks5", iface)); err != nil {
		return fmt.Errorf("setting %s proxy protocol: %w", iface, err)
	}
	// Best-effort: UDP-over-SOCKS5 isn't supported on every firmware
	// version, and the reference project never treats its failure as fatal.
	_, _ = run(fmt.Sprintf("interface %s proxy socks5-udp", iface))

	upstreamCmd := fmt.Sprintf("interface %s proxy upstream %s %d", iface, upstreamHost, upstreamPort)
	if _, err := run(upstreamCmd); err != nil {
		return fmt.Errorf("setting %s upstream: %w", iface, err)
	}

	// Best-effort cosmetics/hardening -- not worth failing setup over.
	_, _ = run(fmt.Sprintf("interface %s description keenetic-xray", iface))
	_, _ = run(fmt.Sprintf("interface %s no ip global", iface))

	if _, err := run(fmt.Sprintf("interface %s up", iface)); err != nil {
		return fmt.Errorf("bringing %s up: %w", iface, err)
	}
	return nil
}

// FixUpstream re-points an already-existing Proxy interface at a new
// upstream host/port and cycles it down/up to force the change to take
// effect live, then reads the applied value back to confirm it actually
// stuck rather than assuming a successful ndmc exit means the change
// took. Ported from proxy0-lan-upstream-fix.sh / ensure_proxy0_lan_upstream.
func FixUpstream(run Runner, iface, upstreamHost string, upstreamPort int) error {
	upstreamCmd := fmt.Sprintf("interface %s proxy upstream %s %d", iface, upstreamHost, upstreamPort)
	if _, err := run(upstreamCmd); err != nil {
		return fmt.Errorf("setting %s upstream: %w", iface, err)
	}
	// Best-effort: bringing an already-down interface down again is a
	// harmless no-op in the reference implementation too.
	_, _ = run(fmt.Sprintf("interface %s down", iface))
	sleepFunc(sleepBetweenDownUp)
	if _, err := run(fmt.Sprintf("interface %s up", iface)); err != nil {
		return fmt.Errorf("bringing %s back up: %w", iface, err)
	}
	if _, err := run("system configuration save"); err != nil {
		return fmt.Errorf("system configuration save: %w", err)
	}

	applied, err := CurrentUpstream(run, iface)
	if err == nil && applied != "" && applied != upstreamHost {
		return fmt.Errorf("%s upstream reads back as %s, expected %s", iface, applied, upstreamHost)
	}
	return nil
}

// CurrentUpstream reads back the upstream host ndm actually has stored
// for iface by parsing `show running-config`, so a caller can verify a
// write took effect instead of assuming it did.
func CurrentUpstream(run Runner, iface string) (string, error) {
	out, err := run("show running-config")
	if err != nil {
		return "", err
	}
	inBlock := false
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 {
			continue
		}
		if fields[0] == "interface" {
			inBlock = len(fields) > 1 && fields[1] == iface
			continue
		}
		if inBlock && len(fields) >= 3 && fields[0] == "proxy" && fields[1] == "upstream" {
			return fields[2], nil
		}
	}
	return "", fmt.Errorf("no proxy upstream found for interface %s in running-config", iface)
}

// Setup ensures iface exists and points at (upstreamHost, upstreamPort),
// whichever state it started in: a fresh interface gets the full
// declarative setup below; an existing one gets its upstream re-applied
// and cycled so a changed LAN IP actually takes effect live rather than
// silently sitting unused until the next manual "down"/"up".
func Setup(run Runner, iface, upstreamHost string, upstreamPort int) error {
	if InterfaceExists(run, iface) {
		return FixUpstream(run, iface, upstreamHost, upstreamPort)
	}
	if err := EnsureProxyInterface(run, iface, upstreamHost, upstreamPort); err != nil {
		return err
	}
	_, err := run("system configuration save")
	return err
}
