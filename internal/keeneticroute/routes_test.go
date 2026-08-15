package keeneticroute

import (
	"fmt"
	"strings"
	"testing"
)

// fakeRouteStore is a minimal ndmc simulation for the object-group /
// dns-proxy commands SyncRoutes and friends issue.
type fakeRouteStore struct {
	commands     []string
	groupMembers map[string][]string
	routed       map[string]bool // group -> has a dns-proxy route
}

func newFakeRouteStore() *fakeRouteStore {
	return &fakeRouteStore{
		groupMembers: make(map[string][]string),
		routed:       make(map[string]bool),
	}
}

func (f *fakeRouteStore) run(cmd string) (string, error) {
	f.commands = append(f.commands, cmd)

	switch {
	case cmd == "show running-config":
		var b strings.Builder
		for group, members := range f.groupMembers {
			fmt.Fprintf(&b, "object-group fqdn %s\n", group)
			for _, m := range members {
				fmt.Fprintf(&b, " include %s\n", m)
			}
			if f.routed[group] {
				fmt.Fprintf(&b, "dns-proxy route object-group %s Proxy0 auto\n", group)
			}
		}
		return b.String(), nil

	case strings.HasPrefix(cmd, "object-group fqdn ") && strings.Contains(cmd, "include "):
		parts := strings.Fields(cmd)
		// object-group fqdn <group> include <domain>
		group, domain := parts[2], parts[4]
		f.groupMembers[group] = append(f.groupMembers[group], domain)
		return "", nil

	case strings.HasPrefix(cmd, "object-group fqdn ") && strings.Contains(cmd, "description "):
		return "", nil

	case strings.HasPrefix(cmd, "object-group fqdn "):
		group := strings.Fields(cmd)[2]
		if _, ok := f.groupMembers[group]; !ok {
			f.groupMembers[group] = nil
		}
		return "", nil

	case strings.HasPrefix(cmd, "no object-group fqdn "):
		group := strings.Fields(cmd)[3]
		delete(f.groupMembers, group)
		return "", nil

	case strings.HasPrefix(cmd, "dns-proxy route object-group "):
		group := strings.Fields(cmd)[3]
		f.routed[group] = true
		return "", nil

	case strings.HasPrefix(cmd, "dns-proxy no route object-group ") || strings.HasPrefix(cmd, "no dns-proxy route object-group "):
		fields := strings.Fields(cmd)
		// last-but-one token before "Proxy0"/"auto" varies by exact
		// phrasing; the group name is always the 5th field in both forms.
		group := fields[4]
		f.routed[group] = false
		return "", nil

	default:
		return "", nil
	}
}

func TestSyncRoutes_CreatesGroupsAndRoutes(t *testing.T) {
	f := newFakeRouteStore()
	if err := SyncRoutes(f.run, "Proxy0"); err != nil {
		t.Fatalf("SyncRoutes: %v", err)
	}

	for _, group := range []string{"apple", "youtube", "telegram"} {
		if len(f.groupMembers[group]) == 0 {
			t.Errorf("group %q has no members", group)
		}
		if !f.routed[group] {
			t.Errorf("group %q was not routed through Proxy0", group)
		}
	}

	// The catalog-only, CIDR-only "cloudflare" group must never become an
	// object-group of its own.
	if _, ok := f.groupMembers["cloudflare"]; ok {
		t.Error("cloudflare should not be created as an object-group")
	}
}

func TestSyncRoutes_OnlyIncludesDomainsNotCIDRs(t *testing.T) {
	f := newFakeRouteStore()
	if err := SyncRoutes(f.run, "Proxy0"); err != nil {
		t.Fatalf("SyncRoutes: %v", err)
	}
	for _, domain := range f.groupMembers["apple"] {
		if strings.Contains(domain, "/") {
			t.Errorf("apple object-group contains a CIDR entry %q, want domains only", domain)
		}
	}
}

func TestSyncRoutes_SavesConfiguration(t *testing.T) {
	f := newFakeRouteStore()
	if err := SyncRoutes(f.run, "Proxy0"); err != nil {
		t.Fatalf("SyncRoutes: %v", err)
	}
	if f.commands[len(f.commands)-1] != "system configuration save" {
		t.Errorf("last command = %q, want \"system configuration save\"", f.commands[len(f.commands)-1])
	}
}

func TestClearRoutes_RemovesManagedGroups(t *testing.T) {
	f := newFakeRouteStore()
	if err := SyncRoutes(f.run, "Proxy0"); err != nil {
		t.Fatalf("SyncRoutes: %v", err)
	}
	if err := ClearRoutes(f.run, "Proxy0"); err != nil {
		t.Fatalf("ClearRoutes: %v", err)
	}
	if len(f.groupMembers) != 0 {
		t.Errorf("expected no object-groups left, got %v", f.groupMembers)
	}
}

func TestStatusRoutes_ReportsConfiguredAndMissing(t *testing.T) {
	f := newFakeRouteStore()
	if err := SyncRoutes(f.run, "Proxy0"); err != nil {
		t.Fatalf("SyncRoutes: %v", err)
	}

	status, err := StatusRoutes(f.run)
	if err != nil {
		t.Fatalf("StatusRoutes: %v", err)
	}
	if !strings.Contains(status, "apple") {
		t.Errorf("status should mention apple, got:\n%s", status)
	}
}

func TestStatusRoutes_BeforeSync(t *testing.T) {
	f := newFakeRouteStore()
	status, err := StatusRoutes(f.run)
	if err != nil {
		t.Fatalf("StatusRoutes: %v", err)
	}
	if !strings.Contains(status, "not configured") {
		t.Errorf("expected \"not configured\" markers before any sync, got:\n%s", status)
	}
}
