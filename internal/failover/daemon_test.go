package failover

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Kuzz007/keenetic-xray-go/internal/config"
)

// TestMain lets `go test` re-exec the test binary itself as a stand-in
// for the xray binary Daemon supervises via xrayctl.Supervisor -- avoids
// needing a real xray-core binary in CI. Mirrors the pattern in
// internal/xrayctl's own tests. The fake doesn't need to act as a real
// proxy for the tests here: ForceSwitch/State bypass health-check probing
// entirely, and the concurrency property under test doesn't depend on
// probes succeeding.
func TestMain(m *testing.M) {
	if os.Getenv("FAILOVER_TEST_HELPER") == "1" {
		time.Sleep(time.Hour)
		return
	}
	os.Exit(m.Run())
}

func TestDaemon_Run_RequiresPrimaryAndBackup(t *testing.T) {
	cases := []struct {
		name string
		cfg  *config.Config
	}{
		{"no profiles at all", config.Default()},
		{"primary only", func() *config.Config {
			c := config.Default()
			c.Profiles = []config.Profile{{
				UUID: "u", Address: "a", Port: 443, Network: "tcp", Security: "none", Encryption: "none",
			}}
			c.PrimaryIndex = 0
			return c
		}()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDaemon(Paths{}, tc.cfg)
			if err := d.Run(context.Background()); err == nil {
				t.Error("expected Run to error out without primary+backup configured")
			}
		})
	}
}

func TestFailoverConfig_CopiesFromConfigPackage(t *testing.T) {
	src := config.FailoverConfig{
		FailuresRequired:          3,
		RecoverySuccessesRequired: 3,
		CooldownCycles:            2,
		RollbackBackoffSeconds:    300,
	}
	got := failoverConfig(src)
	want := Config{FailuresRequired: 3, RecoverySuccessesRequired: 3, CooldownCycles: 2, RollbackBackoffSeconds: 300}
	if got != want {
		t.Errorf("failoverConfig(%+v) = %+v, want %+v", src, got, want)
	}
}

func TestDaemon_ForceSwitchAndState_NotRunning(t *testing.T) {
	cfg := config.Default()
	cfg.Profiles = []config.Profile{{UUID: "u", Address: "a", Port: 443, Network: "tcp", Security: "none", Encryption: "none"}}
	cfg.PrimaryIndex = 0
	cfg.BackupIndex = 0
	d := NewDaemon(Paths{}, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := d.ForceSwitch(ctx, RolePrimary); err == nil {
		t.Error("expected ForceSwitch to error when Run is not active")
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()
	if _, ran := d.State(ctx2); ran {
		t.Error("expected State to report not-running when Run is not active")
	}
}

// TestDaemon_ForceSwitchAndStateConcurrentWithRun is the actual
// concurrency-safety test: it calls ForceSwitch/State from a separate
// goroutine while Run's own tick loop is simultaneously live (fast
// ticks, so real concurrent activity happens, not just a single quiet
// moment) -- this is what `go test -race` is for. Machine's fields must
// never be touched by two goroutines at once; if a future change breaks
// that, this test is what would catch it.
func TestDaemon_ForceSwitchAndStateConcurrentWithRun(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Profiles = []config.Profile{
		{UUID: "p", Address: "primary.invalid", Port: 443, Network: "tcp", Security: "none", Encryption: "none", Remark: "primary"},
		{UUID: "b", Address: "backup.invalid", Port: 443, Network: "tcp", Security: "none", Encryption: "none", Remark: "backup"},
	}
	cfg.PrimaryIndex = 0
	cfg.BackupIndex = 1
	cfg.Failover.CheckIntervalSeconds = 1 // fast ticks so Tick() genuinely overlaps the test's own calls

	paths := Paths{
		XrayBinary:       os.Args[0],
		ProductionConfig: filepath.Join(dir, "production.json"),
		PretestConfig:    filepath.Join(dir, "pretest.json"),
		Env:              []string{"FAILOVER_TEST_HELPER=1"},
	}
	d := NewDaemon(paths, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runErr := make(chan error, 1)
	go func() { runErr <- d.Run(ctx) }()

	for i := 0; i < 6; i++ {
		role, want := RolePrimary, StateActivePrimary
		if i%2 == 1 {
			role, want = RoleBackup, StateActiveBackup
		}
		if err := d.ForceSwitch(ctx, role); err != nil {
			t.Fatalf("ForceSwitch(%v): %v", role, err)
		}
		state, ran := d.State(ctx)
		if !ran {
			t.Fatal("State: daemon reported not running while Run is active")
		}
		if state != want {
			t.Errorf("after ForceSwitch(%v), State() = %v, want %v", role, state, want)
		}
		time.Sleep(50 * time.Millisecond) // let a few ticks land in between
	}

	cancel()
	select {
	case err := <-runErr:
		if err != context.Canceled {
			t.Errorf("Run returned %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after ctx cancellation")
	}
}
