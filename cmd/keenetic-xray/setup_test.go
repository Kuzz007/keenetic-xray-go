package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuzz007/keenetic-xray-go/internal/config"
)

func TestRunSetup_SingleVlessLink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	t.Setenv("KEENETIC_XRAY_CONFIG", path)

	input := strings.NewReader(testVLESSURI + "\n0\n")
	if err := runSetup(input); err != nil {
		t.Fatalf("runSetup: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Profiles) != 1 {
		t.Fatalf("len(Profiles) = %d, want 1", len(cfg.Profiles))
	}
	if cfg.PrimaryIndex != 0 || cfg.BackupIndex != 0 {
		t.Errorf("primary=%d backup=%d, want 0,0 (single profile)", cfg.PrimaryIndex, cfg.BackupIndex)
	}
}

func TestRunSetup_Subscription(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, twoProfileSubBody)
	}))
	defer backend.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	t.Setenv("KEENETIC_XRAY_CONFIG", path)

	input := strings.NewReader(backend.URL + "\n0\n1\n") // subscription URL, primary=0, backup=1
	if err := runSetup(input); err != nil {
		t.Fatalf("runSetup: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Profiles) != 2 {
		t.Fatalf("len(Profiles) = %d, want 2", len(cfg.Profiles))
	}
	if cfg.PrimaryIndex != 0 || cfg.BackupIndex != 1 {
		t.Errorf("primary=%d backup=%d, want 0,1", cfg.PrimaryIndex, cfg.BackupIndex)
	}
	if cfg.Subscription == nil || cfg.Subscription.URL != backend.URL {
		t.Errorf("Subscription = %#v, want URL %s", cfg.Subscription, backend.URL)
	}
}

func TestRunSetup_DefaultBackupSelection(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, twoProfileSubBody)
	}))
	defer backend.Close()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	t.Setenv("KEENETIC_XRAY_CONFIG", path)

	// Press Enter (blank line) for both prompts -> defaults: primary=0, backup=1.
	input := strings.NewReader(backend.URL + "\n\n\n")
	if err := runSetup(input); err != nil {
		t.Fatalf("runSetup: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PrimaryIndex != 0 || cfg.BackupIndex != 1 {
		t.Errorf("primary=%d backup=%d, want 0,1 (defaults)", cfg.PrimaryIndex, cfg.BackupIndex)
	}
}

func TestRunSetup_RejectsUnrecognizedInput(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KEENETIC_XRAY_CONFIG", filepath.Join(dir, "config.json"))

	input := strings.NewReader("not-a-link-or-url\n")
	if err := runSetup(input); err == nil {
		t.Error("expected error for unrecognized input")
	}
}

// seedConfig writes a pre-configured config.json so runSetup skips the
// onboarding prompt and drops straight into the menu -- matching a
// second/later invocation of `keenetic-xray setup`, not a fresh install.
func seedConfig(t *testing.T, path string) *config.Config {
	t.Helper()
	c := config.Default()
	c.Profiles = []config.Profile{mustParseVLESS(t, testVLESSURI)}
	c.PrimaryIndex = 0
	c.BackupIndex = 0
	if err := c.Save(path); err != nil {
		t.Fatalf("seeding config: %v", err)
	}
	return c
}

func mustParseVLESS(t *testing.T, uri string) config.Profile {
	t.Helper()
	p, err := config.ParseVLESSURI(uri)
	if err != nil {
		t.Fatalf("parsing test fixture URI: %v", err)
	}
	return p
}

func TestRunSetup_AlreadyConfigured_SkipsOnboardingEntersMenu(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	t.Setenv("KEENETIC_XRAY_CONFIG", path)
	seedConfig(t, path)

	// "0" exits the menu immediately -- if onboarding had run instead, this
	// same input would be parsed as a (invalid) vless/subscription link and
	// error out, so reaching a clean nil confirms the menu path was taken.
	if err := runSetup(strings.NewReader("0\n")); err != nil {
		t.Fatalf("runSetup: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Profiles) != 1 || cfg.Profiles[0].Remark != "test" {
		t.Errorf("profiles changed unexpectedly: %#v", cfg.Profiles)
	}
}

func TestRunMenu_ListProfilesThenExit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	t.Setenv("KEENETIC_XRAY_CONFIG", path)
	seedConfig(t, path)

	if err := runSetup(strings.NewReader("2\n0\n")); err != nil {
		t.Fatalf("runSetup: %v", err)
	}
}

func TestRunMenu_RemoveProfileViaMenu(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	t.Setenv("KEENETIC_XRAY_CONFIG", path)
	c := config.Default()
	c.Profiles = []config.Profile{mustParseVLESS(t, testVLESSURI), mustParseVLESS(t, testVLESSURI)}
	c.Profiles[1].Remark = "second"
	c.PrimaryIndex, c.BackupIndex = 0, 1
	if err := c.Save(path); err != nil {
		t.Fatalf("seeding config: %v", err)
	}

	// "3" = remove a profile, "1" = index to remove (the "second" one), "0" = exit.
	if err := runSetup(strings.NewReader("3\n1\n0\n")); err != nil {
		t.Fatalf("runSetup: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Profiles) != 1 || cfg.Profiles[0].Remark != "test" {
		t.Errorf("profiles = %#v, want only the first (\"test\") left", cfg.Profiles)
	}
}

func TestRunMenu_SetPrimaryViaMenu(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	t.Setenv("KEENETIC_XRAY_CONFIG", path)
	c := config.Default()
	c.Profiles = []config.Profile{mustParseVLESS(t, testVLESSURI), mustParseVLESS(t, testVLESSURI)}
	c.Profiles[1].Remark = "second"
	c.PrimaryIndex, c.BackupIndex = 0, 1
	if err := c.Save(path); err != nil {
		t.Fatalf("seeding config: %v", err)
	}

	// "4" = set primary, "1" = index, "0" = exit.
	if err := runSetup(strings.NewReader("4\n1\n0\n")); err != nil {
		t.Fatalf("runSetup: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PrimaryIndex != 1 {
		t.Errorf("PrimaryIndex = %d, want 1", cfg.PrimaryIndex)
	}
}

func TestRunMenu_InvalidChoiceDoesNotExit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	t.Setenv("KEENETIC_XRAY_CONFIG", path)
	seedConfig(t, path)

	// "99" is not a valid choice; the menu should reprompt rather than exit
	// or error out, and then exit cleanly on "0".
	if err := runSetup(strings.NewReader("99\n0\n")); err != nil {
		t.Fatalf("runSetup: %v", err)
	}
}

func TestRunMenu_RoutesWithoutNDMCReportsErrorNotCrash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	t.Setenv("KEENETIC_XRAY_CONFIG", path)
	seedConfig(t, path)

	// "11" = router-level routing submenu, "1" = setup/repair (fails
	// gracefully since ndmc isn't present in this test environment), "0" =
	// back out of the submenu, "0" = exit the main menu.
	if err := runSetup(strings.NewReader("11\n1\n0\n0\n")); err != nil {
		t.Fatalf("runSetup should report the ndmc-missing error inline, not fail the whole menu: %v", err)
	}
}
