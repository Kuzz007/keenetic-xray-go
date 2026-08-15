package keeneticroute

import (
	"fmt"
	"strings"
)

// quiet runs cmd and ignores its result -- used for steps the reference
// project (xray-keenetic-routes.sh's quiet_ndmc) treats as best-effort,
// e.g. removing an object-group that may not exist yet.
func quiet(run Runner, cmd string) {
	_, _ = run(cmd)
}

// SyncRoutes (re)creates the domain object-groups from the catalog and
// routes each one through iface via Keenetic's dns-proxy mechanism:
// resolving any of a group's domains sends that connection through
// iface, without touching non-matching traffic. Safe to call repeatedly
// -- it clears its own previously-managed groups first, so re-running
// after a catalog change converges rather than accumulating stale
// entries. Ported from sync_routes() in xray-keenetic-routes.sh.
func SyncRoutes(run Runner, iface string) error {
	if err := clearManagedGroups(run, iface); err != nil {
		return err
	}

	byGroup := make(map[string][]string)
	var order []string
	for _, e := range Catalog() {
		if !isDomain(e.Item) {
			continue
		}
		if _, seen := byGroup[e.Group]; !seen {
			order = append(order, e.Group)
		}
		byGroup[e.Group] = append(byGroup[e.Group], e.Item)
	}

	for _, group := range order {
		if _, err := run(fmt.Sprintf("object-group fqdn %s", group)); err != nil {
			return fmt.Errorf("creating object-group %s: %w", group, err)
		}
		quiet(run, fmt.Sprintf("object-group fqdn %s description keenetic-xray-%s", group, group))
		for _, domain := range byGroup[group] {
			if _, err := run(fmt.Sprintf("object-group fqdn %s include %s", group, domain)); err != nil {
				return fmt.Errorf("adding %s to object-group %s: %w", domain, group, err)
			}
		}
		if _, err := run(fmt.Sprintf("dns-proxy route object-group %s %s auto", group, iface)); err != nil {
			return fmt.Errorf("routing object-group %s through %s: %w", group, iface, err)
		}
	}

	if _, err := run("system configuration save"); err != nil {
		return fmt.Errorf("system configuration save: %w", err)
	}
	return nil
}

// ClearRoutes removes every object-group/dns-proxy route this package
// manages, leaving iface itself untouched.
func ClearRoutes(run Runner, iface string) error {
	if err := clearManagedGroups(run, iface); err != nil {
		return err
	}
	_, err := run("system configuration save")
	return err
}

func clearManagedGroups(run Runner, iface string) error {
	for _, group := range Groups() {
		quiet(run, fmt.Sprintf("dns-proxy no route object-group %s %s auto", group, iface))
		quiet(run, fmt.Sprintf("no dns-proxy route object-group %s %s auto", group, iface))
		quiet(run, fmt.Sprintf("no object-group fqdn %s", group))
	}
	return nil
}

// StatusRoutes reports, per managed group, whether its object-group and
// dns-proxy route currently appear in the running config.
func StatusRoutes(run Runner) (string, error) {
	out, err := run("show running-config")
	if err != nil {
		return "", err
	}

	var b strings.Builder
	for _, group := range Groups() {
		fmt.Fprintf(&b, "--- %s\n", group)
		found := false
		for _, line := range strings.Split(out, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.Contains(trimmed, "object-group fqdn "+group) || strings.Contains(trimmed, "route object-group "+group) {
				fmt.Fprintf(&b, "%s\n", trimmed)
				found = true
			}
		}
		if !found {
			fmt.Fprintf(&b, "(not configured)\n")
		}
	}
	return b.String(), nil
}
