package failover

import (
	"context"
	"testing"

	"github.com/Kuzz007/keenetic-xray-go/internal/config"
)

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
