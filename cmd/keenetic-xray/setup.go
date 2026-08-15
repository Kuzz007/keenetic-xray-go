package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Kuzz007/keenetic-xray-go/internal/config"
	"github.com/Kuzz007/keenetic-xray-go/internal/subscription"
)

// cmdSetup is the interactive configuration menu -- the local SSH-driven
// counterpart to the remote bot. On a fresh install (no profiles yet) it
// starts by prompting for a vless:// link or subscription URL, same as
// before; either way it then drops into a persistent menu covering
// everything else (profiles, subscription refresh, status, doctor,
// variant, remote agent) so this is the one command worth remembering,
// not just a first-run-only wizard.
func cmdSetup(args []string) error {
	return runSetup(os.Stdin)
}

func runSetup(stdin io.Reader) error {
	reader := bufio.NewReader(stdin)
	cfg, err := config.Load(configPath())
	if err != nil {
		return err
	}

	if len(cfg.Profiles) == 0 {
		fmt.Println("keenetic-xray setup")
		if err := onboardFromLink(reader); err != nil {
			return err
		}
	}

	return runMenu(reader)
}

// onboardFromLink prompts for a vless:// link or subscription URL, then a
// primary/backup selection, and saves the result -- replacing whatever
// profile list (and subscription linkage) was there before. This is the
// original one-shot setup flow, now also reachable as a menu item for
// reconfiguring from scratch later.
func onboardFromLink(reader *bufio.Reader) error {
	fmt.Println("Paste a vless:// link, or a subscription http(s):// URL:")
	fmt.Print("> ")
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return fmt.Errorf("reading input: %w", err)
	}
	input := strings.TrimSpace(line)
	if input == "" {
		return fmt.Errorf("no input given")
	}

	cfg, err := config.Load(configPath())
	if err != nil {
		return err
	}

	var profiles []config.Profile
	switch {
	case strings.HasPrefix(input, "vless://"):
		p, err := config.ParseVLESSURI(input)
		if err != nil {
			return fmt.Errorf("parsing vless link: %w", err)
		}
		profiles = []config.Profile{p}
		// A raw single link replaces any previous subscription linkage --
		// otherwise a later "refresh subscription" would silently pull in
		// an unrelated server list this profile has nothing to do with.
		cfg.Subscription = nil
	case strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://"):
		fmt.Println("fetching subscription...")
		result, err := subscription.Refresh(context.Background(), input, "", "")
		if err != nil {
			return fmt.Errorf("fetching subscription: %w", err)
		}
		for _, w := range result.Warnings {
			fmt.Println("warning:", w)
		}
		profiles = result.Profiles
		cfg.Subscription = &config.Subscription{URL: input, LastFetchedAt: time.Now()}
	default:
		return fmt.Errorf("input doesn't look like a vless:// link or an http(s):// subscription URL")
	}

	if len(profiles) == 0 {
		return fmt.Errorf("no usable vless:// profiles found")
	}
	cfg.Profiles = profiles

	fmt.Println("\nAvailable profiles:")
	for i, p := range profiles {
		fmt.Printf("  %d: %s -- %s:%d\n", i, p.Remark, p.Address, p.Port)
	}

	primaryIdx, err := promptIndex(reader, "Select PRIMARY", len(profiles), 0)
	if err != nil {
		return err
	}
	backupIdx := primaryIdx
	if len(profiles) > 1 {
		defaultBackup := 1
		if primaryIdx == 1 {
			defaultBackup = 0
		}
		backupIdx, err = promptIndex(reader, "Select BACKUP", len(profiles), defaultBackup)
		if err != nil {
			return err
		}
	}

	cfg.PrimaryIndex = primaryIdx
	cfg.BackupIndex = backupIdx
	if cfg.Subscription != nil {
		cfg.Subscription.PrimaryKey = profiles[primaryIdx].Remark
		cfg.Subscription.BackupKey = profiles[backupIdx].Remark
	}

	if err := cfg.Save(configPath()); err != nil {
		return err
	}

	fmt.Printf("\nSaved: primary=%s, backup=%s\n", profiles[primaryIdx].Remark, profiles[backupIdx].Remark)
	return nil
}

// runMenu is the persistent management loop. Every action reloads config
// fresh from disk and delegates to the exact same functions the flat
// `profile`/`subscription`/`status`/`doctor`/`variant`/`agent` subcommands
// use, so the menu can't drift from their behavior. Reaching end-of-input
// (e.g. piped/scripted stdin, or Ctrl-D) at the menu prompt exits cleanly
// rather than erroring -- there's no "you forgot to type 0" failure mode.
func runMenu(reader *bufio.Reader) error {
	for {
		cfg, err := config.Load(configPath())
		if err != nil {
			return err
		}
		printMenu(cfg)
		fmt.Print("> ")
		line, err := reader.ReadString('\n')
		choice := strings.TrimSpace(line)
		if choice == "" {
			if err != nil {
				return nil
			}
			fmt.Println("invalid choice")
			continue
		}

		if menuErr := dispatchMenuChoice(reader, choice); menuErr != nil {
			if menuErr == errExitMenu {
				return nil
			}
			fmt.Println("error:", menuErr)
		}
	}
}

var errExitMenu = fmt.Errorf("exit menu")

func printMenu(cfg *config.Config) {
	fmt.Println()
	fmt.Println("=== keenetic-xray ===")
	fmt.Printf("variant: %s | profiles: %d", cfg.Variant, len(cfg.Profiles))
	if p := cfg.Primary(); p != nil {
		fmt.Printf(" | primary: %s", p.Remark)
	}
	if b := cfg.Backup(); b != nil {
		fmt.Printf(" | backup: %s", b.Remark)
	}
	fmt.Println()
	fmt.Println("1) Set profiles from a vless:// link or subscription URL (replaces current list)")
	fmt.Println(" 2) List profiles")
	fmt.Println(" 3) Remove a profile")
	fmt.Println(" 4) Set primary profile")
	fmt.Println(" 5) Set backup profile")
	fmt.Println(" 6) Refresh subscription")
	fmt.Println(" 7) Status")
	fmt.Println(" 8) Doctor (diagnostics)")
	fmt.Println(" 9) Variant (mini/full)")
	fmt.Println("10) Remote control agent")
	fmt.Println("11) Router-level routing (Proxy0 + auto domain routing)")
	fmt.Println(" 0) Exit")
}

func dispatchMenuChoice(reader *bufio.Reader, choice string) error {
	switch choice {
	case "0", "q", "exit", "quit":
		return errExitMenu
	case "1":
		return onboardFromLink(reader)
	case "2":
		return profileList()
	case "3":
		return menuRemoveProfile(reader)
	case "4":
		return menuSetRole(reader, true)
	case "5":
		return menuSetRole(reader, false)
	case "6":
		return subscriptionRefresh()
	case "7":
		return cmdStatus(nil)
	case "8":
		return cmdDoctor(nil)
	case "9":
		return menuVariant(reader)
	case "10":
		return menuAgent(reader)
	case "11":
		return menuRoutes(reader)
	default:
		fmt.Println("invalid choice")
		return nil
	}
}

func menuRemoveProfile(reader *bufio.Reader) error {
	if err := profileList(); err != nil {
		return err
	}
	idx, err := promptLine(reader, "Index to remove: ")
	if err != nil {
		return err
	}
	return profileRemove([]string{idx})
}

func menuSetRole(reader *bufio.Reader, primary bool) error {
	if err := profileList(); err != nil {
		return err
	}
	idx, err := promptLine(reader, fmt.Sprintf("Index for %s: ", roleWord(primary)))
	if err != nil {
		return err
	}
	return subscriptionSetRole([]string{idx}, primary)
}

func menuVariant(reader *bufio.Reader) error {
	if err := variantShow(); err != nil {
		return err
	}
	fmt.Print("Set variant (mini/full, blank to keep current): ")
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return fmt.Errorf("reading input: %w", err)
	}
	choice := strings.TrimSpace(line)
	if choice == "" {
		return nil
	}
	return variantSet([]string{choice})
}

func menuAgent(reader *bufio.Reader) error {
	if err := agentStatus(); err != nil {
		return err
	}
	fmt.Println(" 1) Configure   2) Enable   3) Disable   0) Back")
	fmt.Print("> ")
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return fmt.Errorf("reading input: %w", err)
	}
	switch strings.TrimSpace(line) {
	case "1":
		serverURL, err := promptLine(reader, "Control server URL: ")
		if err != nil {
			return err
		}
		routerID, err := promptLine(reader, "Router ID: ")
		if err != nil {
			return err
		}
		fingerprint, err := promptLine(reader, "Server fingerprint (SHA256): ")
		if err != nil {
			return err
		}
		token, err := promptLine(reader, "Router token: ")
		if err != nil {
			return err
		}
		return agentConfigure([]string{serverURL, routerID, fingerprint, token})
	case "2":
		return agentSetEnabled(true)
	case "3":
		return agentSetEnabled(false)
	default:
		return nil
	}
}

// menuRoutes wires the local SOCKS5 proxy into Keenetic's own routing
// (a Proxy0 interface plus automatic domain-based routing for common
// blocked services) via keeneticroute -- only meaningful on real
// Keenetic/Entware hardware (ndmc must be present).
func menuRoutes(reader *bufio.Reader) error {
	fmt.Println(" 1) Set up / repair (detect LAN IP, create or fix Proxy0, sync domain routes)")
	fmt.Println(" 2) Re-sync domain routes only")
	fmt.Println(" 3) Clear domain routes")
	fmt.Println(" 4) Status")
	fmt.Println(" 0) Back")
	fmt.Print("> ")
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return fmt.Errorf("reading input: %w", err)
	}
	switch strings.TrimSpace(line) {
	case "1":
		return routesSetup()
	case "2":
		return routesSync()
	case "3":
		return routesClear()
	case "4":
		return routesStatus()
	default:
		return nil
	}
}

// promptLine prints label, reads one line, and returns it trimmed. Unlike
// promptIndex there's no default -- every caller needs a real value (an
// index, a URL, a token) rather than something it's safe to skip.
func promptLine(reader *bufio.Reader, label string) (string, error) {
	fmt.Print(label)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("reading input: %w", err)
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return "", fmt.Errorf("no input given")
	}
	return value, nil
}

// promptIndex reads a line, re-prompting on invalid input; an empty line
// (just Enter) accepts def.
func promptIndex(reader *bufio.Reader, label string, count, def int) (int, error) {
	for {
		fmt.Printf("%s [0-%d, default %d]: ", label, count-1, def)
		line, err := reader.ReadString('\n')
		if err != nil && line == "" {
			return 0, fmt.Errorf("reading input: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			return def, nil
		}
		idx, err := strconv.Atoi(line)
		if err != nil || idx < 0 || idx >= count {
			fmt.Println("invalid selection, try again")
			continue
		}
		return idx, nil
	}
}
