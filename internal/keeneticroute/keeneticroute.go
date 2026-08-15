// Package keeneticroute wires the local Xray SOCKS5 inbound into
// KeeneticOS's own routing layer: a "Proxy" interface (Proxy0) whose
// upstream is this router's local SOCKS5 server, plus domain-based
// routing rules that send a curated set of commonly-blocked services
// through it via Keenetic's native dns-proxy/object-group mechanism.
//
// This ports a working, real-hardware-verified mechanism from the
// author's earlier keenetic_xray_installer project (see
// xray-keenetic-routes.sh, vless-go-lan-ip.sh, vless-go-recover.sh
// there) rather than reinventing it -- notably the LAN-IP detection
// logic, whose complexity is earned: an earlier, simpler version that
// fell back to 127.0.0.1 on detection failure caused a periodic
// re-apply to silently replace a working upstream with a loopback
// address and break the proxy until someone re-ran the installer by
// hand. Everything here talks to the router through ndmc, KeeneticOS's
// own configuration CLI (present on stock firmware, reachable from the
// Entware shell) -- never iptables/TPROXY.
package keeneticroute

import "os/exec"

// Runner executes a single ndmc CLI command (equivalent to `ndmc -c
// "<cmd>"`) and returns its combined output. Real callers use
// NDMCRunner; tests substitute a fake that inspects the command string
// and returns canned output, since ndmc itself only exists on real
// Keenetic/Entware hardware.
type Runner func(cmd string) (string, error)

// NDMCRunner returns a Runner backed by the real ndmc binary.
func NDMCRunner() Runner {
	return func(cmd string) (string, error) {
		out, err := exec.Command("ndmc", "-c", cmd).CombinedOutput()
		return string(out), err
	}
}

// Available reports whether ndmc is present on this system -- false on
// any non-Keenetic dev machine or CI runner, in which case every
// operation in this package should be skipped rather than attempted.
func Available() bool {
	_, err := exec.LookPath("ndmc")
	return err == nil
}
