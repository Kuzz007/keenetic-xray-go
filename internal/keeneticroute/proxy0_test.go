package keeneticroute

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// fakeNDMC records every command it's given and simulates just enough
// running-config state for CurrentUpstream/InterfaceExists to work
// against it.
type fakeNDMC struct {
	commands []string
	exists   map[string]bool
	upstream map[string]string
	failOn   map[string]bool
}

func newFakeNDMC() *fakeNDMC {
	return &fakeNDMC{
		exists:   make(map[string]bool),
		upstream: make(map[string]string),
		failOn:   make(map[string]bool),
	}
}

func (f *fakeNDMC) run(cmd string) (string, error) {
	f.commands = append(f.commands, cmd)
	if f.failOn[cmd] {
		return "", fmt.Errorf("simulated failure for %q", cmd)
	}

	switch {
	case cmd == "show running-config":
		var b strings.Builder
		for iface, upstream := range f.upstream {
			fmt.Fprintf(&b, "interface %s\n proxy upstream %s\n!\n", iface, upstream)
		}
		return b.String(), nil
	case strings.HasPrefix(cmd, "show interface "):
		iface := strings.TrimPrefix(cmd, "show interface ")
		if f.exists[iface] {
			return "type: Proxy\n", nil
		}
		return "", fmt.Errorf("no such interface %s", iface)
	case strings.HasPrefix(cmd, "interface "):
		rest := strings.TrimPrefix(cmd, "interface ")
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return "", fmt.Errorf("bad interface command %q", cmd)
		}
		iface := fields[0]
		f.exists[iface] = true
		if len(fields) >= 4 && fields[1] == "proxy" && fields[2] == "upstream" {
			f.upstream[iface] = fields[3]
		}
		return "", nil
	default:
		return "", nil
	}
}

func TestInterfaceExists(t *testing.T) {
	f := newFakeNDMC()
	if InterfaceExists(f.run, "Proxy0") {
		t.Error("Proxy0 should not exist yet")
	}
	f.exists["Proxy0"] = true
	if !InterfaceExists(f.run, "Proxy0") {
		t.Error("Proxy0 should exist")
	}
}

func TestEnsureProxyInterface_FullSequence(t *testing.T) {
	f := newFakeNDMC()
	if err := EnsureProxyInterface(f.run, "Proxy0", "192.168.1.1", 1080); err != nil {
		t.Fatalf("EnsureProxyInterface: %v", err)
	}

	want := []string{
		"interface Proxy0",
		"interface Proxy0 proxy protocol socks5",
		"interface Proxy0 proxy socks5-udp",
		"interface Proxy0 proxy upstream 192.168.1.1 1080",
		"interface Proxy0 description keenetic-xray",
		"interface Proxy0 no ip global",
		"interface Proxy0 up",
	}
	if !reflect.DeepEqual(f.commands, want) {
		t.Errorf("commands =\n  %#v\nwant\n  %#v", f.commands, want)
	}
}

func TestEnsureProxyInterface_BestEffortStepsDontAbort(t *testing.T) {
	f := newFakeNDMC()
	f.failOn["interface Proxy0 proxy socks5-udp"] = true
	f.failOn["interface Proxy0 description keenetic-xray"] = true
	f.failOn["interface Proxy0 no ip global"] = true

	if err := EnsureProxyInterface(f.run, "Proxy0", "192.168.1.1", 1080); err != nil {
		t.Fatalf("best-effort step failures should not abort setup: %v", err)
	}
	if f.upstream["Proxy0"] != "192.168.1.1" {
		t.Errorf("upstream = %q, want 192.168.1.1", f.upstream["Proxy0"])
	}
}

func TestEnsureProxyInterface_RequiredStepFails(t *testing.T) {
	f := newFakeNDMC()
	f.failOn["interface Proxy0 proxy upstream 192.168.1.1 1080"] = true

	if err := EnsureProxyInterface(f.run, "Proxy0", "192.168.1.1", 1080); err == nil {
		t.Error("expected error when a required step fails")
	}
}

func TestFixUpstream_ReportsMismatchOnReadback(t *testing.T) {
	sleepFunc = func(time.Duration) {} // don't actually wait in tests
	defer func() { sleepFunc = time.Sleep }()

	f := newFakeNDMC()
	f.exists["Proxy0"] = true
	// Simulate ndm silently keeping the old upstream despite a
	// successful-looking "proxy upstream" call, by overriding run to
	// ignore the upstream-setting command.
	run := func(cmd string) (string, error) {
		if cmd == "interface Proxy0 proxy upstream 192.168.1.1 1080" {
			f.commands = append(f.commands, cmd)
			return "", nil // accepted, but doesn't actually update f.upstream
		}
		return f.run(cmd)
	}
	f.upstream["Proxy0"] = "10.0.0.1"

	err := FixUpstream(run, "Proxy0", "192.168.1.1", 1080)
	if err == nil {
		t.Fatal("expected error when readback doesn't match the requested upstream")
	}
	if !strings.Contains(err.Error(), "10.0.0.1") || !strings.Contains(err.Error(), "192.168.1.1") {
		t.Errorf("error should mention both values, got: %v", err)
	}
}

func TestFixUpstream_Success(t *testing.T) {
	sleepFunc = func(time.Duration) {}
	defer func() { sleepFunc = time.Sleep }()

	f := newFakeNDMC()
	f.exists["Proxy0"] = true
	f.upstream["Proxy0"] = "10.0.0.1"

	if err := FixUpstream(f.run, "Proxy0", "192.168.1.1", 1080); err != nil {
		t.Fatalf("FixUpstream: %v", err)
	}
	if f.upstream["Proxy0"] != "192.168.1.1" {
		t.Errorf("upstream = %q, want 192.168.1.1", f.upstream["Proxy0"])
	}

	var sawDown, sawUp, sawSave bool
	for _, c := range f.commands {
		switch c {
		case "interface Proxy0 down":
			sawDown = true
		case "interface Proxy0 up":
			sawUp = true
		case "system configuration save":
			sawSave = true
		}
	}
	if !sawDown || !sawUp || !sawSave {
		t.Errorf("expected down/up/save cycle, got commands: %v", f.commands)
	}
}

func TestCurrentUpstream(t *testing.T) {
	f := newFakeNDMC()
	f.upstream["Proxy0"] = "192.168.1.1"

	got, err := CurrentUpstream(f.run, "Proxy0")
	if err != nil {
		t.Fatalf("CurrentUpstream: %v", err)
	}
	if got != "192.168.1.1" {
		t.Errorf("got %q, want 192.168.1.1", got)
	}
}

func TestCurrentUpstream_NotFound(t *testing.T) {
	f := newFakeNDMC()
	if _, err := CurrentUpstream(f.run, "Proxy0"); err == nil {
		t.Error("expected error when interface has no recorded upstream")
	}
}

func TestSetup_CreatesFreshInterface(t *testing.T) {
	f := newFakeNDMC()
	if err := Setup(f.run, "Proxy0", "192.168.1.1", 1080); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if !f.exists["Proxy0"] {
		t.Error("Proxy0 should have been created")
	}
	if f.upstream["Proxy0"] != "192.168.1.1" {
		t.Errorf("upstream = %q, want 192.168.1.1", f.upstream["Proxy0"])
	}
}

func TestSetup_FixesExistingInterface(t *testing.T) {
	sleepFunc = func(time.Duration) {}
	defer func() { sleepFunc = time.Sleep }()

	f := newFakeNDMC()
	f.exists["Proxy0"] = true
	f.upstream["Proxy0"] = "10.0.0.1" // stale, e.g. after a DHCP lease change

	if err := Setup(f.run, "Proxy0", "192.168.1.1", 1080); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	if f.upstream["Proxy0"] != "192.168.1.1" {
		t.Errorf("upstream = %q, want 192.168.1.1", f.upstream["Proxy0"])
	}
	// The fresh-interface creation sequence must not have run again.
	for _, c := range f.commands {
		if c == "interface Proxy0 proxy protocol socks5" {
			t.Error("Setup should not re-run full creation on an existing interface")
		}
	}
}
