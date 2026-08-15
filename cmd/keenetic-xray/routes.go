package main

import (
	"fmt"
	"strings"

	"github.com/Kuzz007/keenetic-xray-go/internal/config"
	"github.com/Kuzz007/keenetic-xray-go/internal/keeneticroute"
)

// proxyInterfaceName is the Keenetic "Proxy" interface keenetic-xray
// wires its local SOCKS5 inbound into, so ordinary Keenetic-side
// Policy-Based Routing (and this package's own automatic domain routes)
// can select it like any other connection. Matches the reference
// project's own default name.
const proxyInterfaceName = "Proxy0"

func cmdRoutes(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: keenetic-xray routes {setup|sync|clear|status} ...")
	}
	switch args[0] {
	case "setup":
		return routesSetup()
	case "sync":
		return routesSync()
	case "clear":
		return routesClear()
	case "status":
		return routesStatus()
	default:
		return fmt.Errorf("unknown routes subcommand %q", args[0])
	}
}

func requireNDMC() error {
	if !keeneticroute.Available() {
		return fmt.Errorf("ndmc not found -- router-level routing only works on real Keenetic/Entware hardware")
	}
	return nil
}

// routesSetup is the one-shot action: detect this router's own LAN IP,
// point Proxy0 at the local SOCKS5 inbound (creating the interface if
// it doesn't exist yet, or re-pointing it if it does), and sync the
// curated domain-routing catalog through it -- everything setup's
// "router-level routing" menu item needs, in one call.
func routesSetup() error {
	if err := requireNDMC(); err != nil {
		return err
	}
	cfg, err := config.Load(configPath())
	if err != nil {
		return err
	}
	run := keeneticroute.NDMCRunner()

	lanIP, err := keeneticroute.DetectLANIP(run, lanIPCachePath())
	if err != nil {
		return fmt.Errorf("detecting router LAN IP: %w", err)
	}
	fmt.Printf("detected router LAN IP: %s\n", lanIP)

	if err := keeneticroute.Setup(run, proxyInterfaceName, lanIP, cfg.Failover.SOCKSPort); err != nil {
		return fmt.Errorf("configuring %s: %w", proxyInterfaceName, err)
	}
	fmt.Printf("%s upstream: %s:%d\n", proxyInterfaceName, lanIP, cfg.Failover.SOCKSPort)

	if err := keeneticroute.SyncRoutes(run, proxyInterfaceName); err != nil {
		return fmt.Errorf("syncing domain routes: %w", err)
	}
	fmt.Println("domain routing synced:", strings.Join(keeneticroute.Groups(), ", "))
	return nil
}

func routesSync() error {
	if err := requireNDMC(); err != nil {
		return err
	}
	if err := keeneticroute.SyncRoutes(keeneticroute.NDMCRunner(), proxyInterfaceName); err != nil {
		return err
	}
	fmt.Println("domain routing synced:", strings.Join(keeneticroute.Groups(), ", "))
	return nil
}

func routesClear() error {
	if err := requireNDMC(); err != nil {
		return err
	}
	if err := keeneticroute.ClearRoutes(keeneticroute.NDMCRunner(), proxyInterfaceName); err != nil {
		return err
	}
	fmt.Println("domain routing cleared")
	return nil
}

func routesStatus() error {
	if err := requireNDMC(); err != nil {
		return err
	}
	status, err := keeneticroute.StatusRoutes(keeneticroute.NDMCRunner())
	if err != nil {
		return err
	}
	fmt.Print(status)
	return nil
}
