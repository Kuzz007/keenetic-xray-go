package failover

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Kuzz007/keenetic-xray-go/internal/config"
	"github.com/Kuzz007/keenetic-xray-go/internal/xrayctl"
)

// Paths are the filesystem locations the daemon reads/writes.
type Paths struct {
	XrayBinary       string
	ProductionConfig string
	PretestConfig    string
}

// realActions implements Actions against real xray-core processes (via
// xrayctl.Supervisor) and real config files (via internal/config). It is
// the only piece of this package that touches a process or the network;
// Machine itself never does.
type realActions struct {
	paths Paths
	cfg   *config.Config
	socks string // "127.0.0.1:<SOCKSPort>", precomputed once

	prod    *xrayctl.Supervisor
	pretest *xrayctl.Supervisor
}

func newRealActions(paths Paths, cfg *config.Config) *realActions {
	return &realActions{
		paths: paths,
		cfg:   cfg,
		socks: fmt.Sprintf("127.0.0.1:%d", cfg.Failover.SOCKSPort),
		prod: &xrayctl.Supervisor{
			BinaryPath: paths.XrayBinary,
			ConfigPath: paths.ProductionConfig,
			Name:       "production",
		},
	}
}

func (a *realActions) ProbeLive(ctx context.Context) error {
	return xrayctl.Probe(ctx, xrayctl.ProbeOptions{
		SOCKSAddr: a.socks,
		URL:       a.cfg.Failover.HealthCheckURL,
		Timeout:   a.probeTimeout(),
	})
}

func (a *realActions) ProbeIsolated(ctx context.Context) error {
	return xrayctl.Probe(ctx, xrayctl.ProbeOptions{
		SOCKSAddr: fmt.Sprintf("127.0.0.1:%d", a.cfg.Failover.PretestPort),
		URL:       a.cfg.Failover.HealthCheckURL,
		Timeout:   a.probeTimeout(),
	})
}

func (a *realActions) probeTimeout() time.Duration {
	interval := time.Duration(a.cfg.Failover.CheckIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = time.Duration(config.DefaultFailoverConfig().CheckIntervalSeconds) * time.Second
	}
	return interval
}

func (a *realActions) SwitchLiveTo(ctx context.Context, role Role) error {
	profile := a.cfg.Primary()
	if role == RoleBackup {
		profile = a.cfg.Backup()
	}
	if profile == nil {
		return fmt.Errorf("no %s profile configured", role)
	}

	data, err := config.GenerateXrayConfig(config.XrayConfigOptions{
		SOCKSPort: a.cfg.Failover.SOCKSPort,
		HTTPPort:  a.cfg.Failover.HTTPPort,
		Outbound:  *profile,
	})
	if err != nil {
		return fmt.Errorf("generating production config: %w", err)
	}
	if err := os.WriteFile(a.paths.ProductionConfig, data, 0o600); err != nil {
		return fmt.Errorf("writing production config: %w", err)
	}

	return a.prod.Restart()
}

func (a *realActions) StartIsolatedPretest(ctx context.Context) error {
	primary := a.cfg.Primary()
	if primary == nil {
		return fmt.Errorf("no primary profile configured")
	}

	data, err := config.GenerateXrayConfig(config.XrayConfigOptions{
		SOCKSPort: a.cfg.Failover.PretestPort,
		Outbound:  *primary,
	})
	if err != nil {
		return fmt.Errorf("generating pretest config: %w", err)
	}
	if err := os.WriteFile(a.paths.PretestConfig, data, 0o600); err != nil {
		return fmt.Errorf("writing pretest config: %w", err)
	}

	if a.pretest != nil {
		a.pretest.Stop()
	}
	a.pretest = &xrayctl.Supervisor{
		BinaryPath: a.paths.XrayBinary,
		ConfigPath: a.paths.PretestConfig,
		Name:       "pretest",
	}
	return a.pretest.Start()
}

func (a *realActions) StopIsolatedPretest(ctx context.Context) error {
	if a.pretest != nil {
		a.pretest.Stop()
		a.pretest = nil
	}
	return nil
}

// Daemon drives a Machine on a real ticker, using real xray-core processes
// and config files. Subscription refresh is deliberately not wired in
// here -- it's CLI/bot-triggered (M5/M9), not something the core failover
// loop needs to own.
type Daemon struct {
	machine *Machine
	actions *realActions
	cfg     *config.Config
}

// NewDaemon builds a Daemon wired to real components.
func NewDaemon(paths Paths, cfg *config.Config) *Daemon {
	actions := newRealActions(paths, cfg)
	machine := NewMachine(failoverConfig(cfg.Failover), actions, nil, StateActivePrimary)
	return &Daemon{machine: machine, actions: actions, cfg: cfg}
}

func failoverConfig(c config.FailoverConfig) Config {
	return Config{
		FailuresRequired:          c.FailuresRequired,
		RecoverySuccessesRequired: c.RecoverySuccessesRequired,
		CooldownCycles:            c.CooldownCycles,
		RollbackBackoffSeconds:    c.RollbackBackoffSeconds,
	}
}

// State returns the daemon's current failover state.
func (d *Daemon) State() State { return d.machine.State() }

// Run starts the production xray-core process on primary and then drives
// the state machine's Tick once per CheckIntervalSeconds until ctx is
// cancelled. cfg must already have a primary and backup profile
// configured (run `keenetic-xray setup` first otherwise).
func (d *Daemon) Run(ctx context.Context) error {
	if d.cfg.Primary() == nil || d.cfg.Backup() == nil {
		return fmt.Errorf("failover: primary and backup profiles must be configured first (run `keenetic-xray setup`)")
	}

	if err := d.actions.SwitchLiveTo(ctx, RolePrimary); err != nil {
		return fmt.Errorf("starting production instance: %w", err)
	}
	defer d.actions.prod.Stop()

	ticker := time.NewTicker(d.actions.probeTimeout())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_ = d.actions.StopIsolatedPretest(ctx)
			return ctx.Err()
		case <-ticker.C:
			d.machine.Tick(ctx)
		}
	}
}
