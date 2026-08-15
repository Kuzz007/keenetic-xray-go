package keeneticroute

import "testing"

func TestCatalog_ParsesNonEmpty(t *testing.T) {
	entries := Catalog()
	if len(entries) == 0 {
		t.Fatal("Catalog() returned no entries")
	}
	for _, e := range entries {
		if e.Group == "" || e.Item == "" {
			t.Errorf("entry with empty field: %#v", e)
		}
	}
}

func TestGroups_ContainsExpectedServices(t *testing.T) {
	groups := Groups()
	want := []string{"apple", "discord", "gemini", "meta", "telegram", "tiktok", "youtube"}
	have := make(map[string]bool)
	for _, g := range groups {
		have[g] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("Groups() missing expected group %q, got %v", w, groups)
		}
	}
}

func TestGroups_ExcludesBareCIDROnlyGroup(t *testing.T) {
	// "cloudflare" only ever appears as CIDR entries in the catalog (a
	// reference/documentation group in the source project, not something
	// dns-proxy can route by domain), so it must not show up as a
	// manageable group.
	for _, g := range Groups() {
		if g == "cloudflare" {
			t.Error("cloudflare has no domain entries and should be excluded from Groups()")
		}
	}
}

func TestGroups_Deduplicated(t *testing.T) {
	groups := Groups()
	seen := make(map[string]bool)
	for _, g := range groups {
		if seen[g] {
			t.Errorf("group %q appears more than once in Groups()", g)
		}
		seen[g] = true
	}
}

func TestIsDomain(t *testing.T) {
	cases := []struct {
		item string
		want bool
	}{
		{"apple.com", true},
		{"12dagenkerstbijitunes.com", true},
		{"17.0.0.0/8", false},
		{"103.21.244.0/22", false},
		{"cloudflare", true}, // the one non-CIDR, non-dotted-domain literal in the catalog
		{"", false},
	}
	for _, tc := range cases {
		if got := isDomain(tc.item); got != tc.want {
			t.Errorf("isDomain(%q) = %v, want %v", tc.item, got, tc.want)
		}
	}
}
