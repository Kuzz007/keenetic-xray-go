package keeneticroute

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestIsPrivateIPv4(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"172.32.0.1", false}, // just outside the 172.16/12 range
		{"172.15.0.1", false},
		{"127.0.0.1", false},
		{"8.8.8.8", false},
		{"", false},
		{"not-an-ip", false},
		{"192.168.1.1/24", false}, // CIDR suffix must be rejected, not stripped
		{"192.168.1", false},
		{"192.168.1.1.1", false},
		{"192.168.1.999", false},
	}
	for _, tc := range cases {
		t.Run(tc.ip, func(t *testing.T) {
			if got := IsPrivateIPv4(tc.ip); got != tc.want {
				t.Errorf("IsPrivateIPv4(%q) = %v, want %v", tc.ip, got, tc.want)
			}
		})
	}
}

func TestDetectLANIP_FromRunningConfig(t *testing.T) {
	run := func(cmd string) (string, error) {
		if cmd == "show running-config" {
			return "interface Bridge0\n ip address 192.168.1.1 255.255.255.0\n!\n", nil
		}
		return "", fmt.Errorf("unexpected command %q", cmd)
	}

	ip, err := DetectLANIP(run, "")
	if err != nil {
		t.Fatalf("DetectLANIP: %v", err)
	}
	if ip != "192.168.1.1" {
		t.Errorf("ip = %q, want 192.168.1.1", ip)
	}
}

func TestDetectLANIP_FromShowInterfaceFallback(t *testing.T) {
	run := func(cmd string) (string, error) {
		switch {
		case cmd == "show running-config":
			return "interface Vlan1\n ip address 10.99.0.1 255.255.255.0\n!\n", nil // not a candidate interface
		case cmd == "show interface Bridge0":
			return "type: Bridge\n address: 192.168.88.1\n", nil
		default:
			return "", fmt.Errorf("unexpected command %q", cmd)
		}
	}

	ip, err := DetectLANIP(run, "")
	if err != nil {
		t.Fatalf("DetectLANIP: %v", err)
	}
	if ip != "192.168.88.1" {
		t.Errorf("ip = %q, want 192.168.88.1", ip)
	}
}

func TestDetectLANIP_RejectsCGNATLikeWANAddress(t *testing.T) {
	// A WAN-side interface carrying a CGNAT address must never be mistaken
	// for the LAN just because ndmc happily returns something for it --
	// only the candidate LAN interface names are ever queried.
	run := func(cmd string) (string, error) {
		return "", fmt.Errorf("no such interface")
	}
	if _, err := DetectLANIP(run, ""); err == nil {
		t.Error("expected error when no candidate interface resolves")
	}
}

func TestDetectLANIP_FallsBackToCache(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "router-lan-ip")
	writeLANIPCache(cache, "192.168.50.1")

	run := func(cmd string) (string, error) {
		return "", fmt.Errorf("ndmc unavailable")
	}

	ip, err := DetectLANIP(run, cache)
	if err != nil {
		t.Fatalf("DetectLANIP: %v", err)
	}
	if ip != "192.168.50.1" {
		t.Errorf("ip = %q, want cached 192.168.50.1", ip)
	}
}

func TestDetectLANIP_NeverInventsLoopback(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "router-lan-ip") // no cache file written

	run := func(cmd string) (string, error) {
		return "", fmt.Errorf("ndmc unavailable")
	}

	if ip, err := DetectLANIP(run, cache); err == nil {
		t.Errorf("expected error with no live detection and no cache, got ip=%q", ip)
	}
}

func TestDetectLANIP_WritesCacheOnSuccess(t *testing.T) {
	dir := t.TempDir()
	cache := filepath.Join(dir, "router-lan-ip")

	run := func(cmd string) (string, error) {
		if cmd == "show running-config" {
			return "interface Bridge0\n ip address 192.168.1.1 255.255.255.0\n!\n", nil
		}
		return "", fmt.Errorf("unexpected command %q", cmd)
	}
	if _, err := DetectLANIP(run, cache); err != nil {
		t.Fatalf("DetectLANIP: %v", err)
	}

	got, err := readLANIPCache(cache)
	if err != nil {
		t.Fatalf("readLANIPCache: %v", err)
	}
	if got != "192.168.1.1" {
		t.Errorf("cached ip = %q, want 192.168.1.1", got)
	}
}
