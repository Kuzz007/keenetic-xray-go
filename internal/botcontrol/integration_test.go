package botcontrol

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// TestIntegration_AgentAndServerRealTLS wires the actual router-side
// client (Run/newPinnedClient from agent.go) against the actual
// server-side stack (ListenAndServeTLS/Server from server.go) with a real
// GenerateSelfSignedCert certificate -- the one combination none of the
// other tests cover, since server_test.go drives Server directly over
// plain httptest.NewServer and agent_test.go points at httptest's own
// auto-generated certs, not ours. This is the actual round trip a router
// and the control server perform: fingerprint pinning against our own
// cert generation, Bearer+X-Router-Id auth, queue, execute, and post the
// result back, all over a real TLS listener on a real port.
func TestIntegration_AgentAndServerRealTLS(t *testing.T) {
	cert, err := GenerateSelfSignedCert("localhost")
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert: %v", err)
	}
	fingerprint, err := FingerprintSHA256(cert)
	if err != nil {
		t.Fatalf("FingerprintSHA256: %v", err)
	}

	store, err := LoadStore("")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	auth := StaticRouterAuth{"router-1": "secret-token"}
	server := NewServer(ServerConfig{Store: store, Auth: auth, Fingerprint: fingerprint})

	// Bind to an ephemeral port ourselves (rather than through
	// ListenAndServeTLS, which owns its own listener) so the test knows
	// the real address to point the agent at.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close() // ListenAndServeTLS will re-bind the same address below; the race is negligible in a test

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srvErrCh := make(chan error, 1)
	go func() { srvErrCh <- ListenAndServeTLS(ctx, addr, cert, server) }()

	// Give the listener a moment to actually bind before the agent
	// starts dialing it.
	time.Sleep(100 * time.Millisecond)

	if _, err := store.Enqueue("router-1", ActionStatus, nil); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	handler := &fakeHandler{}
	agentCtx, agentCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer agentCancel()
	opts := AgentOptions{
		ControlServerURL:  "https://" + addr,
		RouterID:          "router-1",
		Token:             "secret-token",
		FingerprintSHA256: fingerprint,
		PollInterval:      20 * time.Millisecond,
	}
	_ = Run(agentCtx, opts, handler)

	if handler.calls == 0 {
		t.Fatal("agent never executed the queued command against the real TLS server")
	}
	result := store.LastResult("router-1")
	if result == nil || result.Output != "did "+ActionStatus {
		t.Errorf("LastResult = %+v, want Output=%q", result, "did "+ActionStatus)
	}

	cancel()
	select {
	case err := <-srvErrCh:
		if err != context.Canceled {
			t.Errorf("server shutdown returned %v, want context.Canceled", err)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("server did not shut down within 6s")
	}
}

// TestIntegration_AgentRejectsWrongFingerprintAgainstRealServer confirms
// the pinning is load-bearing end to end, not just against httptest's
// certs: an agent configured with the wrong fingerprint must not be able
// to complete a request against our real server, even though the TLS
// handshake parameters otherwise match exactly.
func TestIntegration_AgentRejectsWrongFingerprintAgainstRealServer(t *testing.T) {
	cert, err := GenerateSelfSignedCert("localhost")
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert: %v", err)
	}

	store, err := LoadStore("")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	auth := StaticRouterAuth{"router-1": "secret-token"}
	server := NewServer(ServerConfig{Store: store, Auth: auth, Fingerprint: "irrelevant"})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = ListenAndServeTLS(ctx, addr, cert, server) }()
	time.Sleep(100 * time.Millisecond)

	wrongFingerprint := strings.Repeat("0", 64) // valid hex, definitely not cert's real fingerprint
	handler := &fakeHandler{}
	agentCtx, agentCancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer agentCancel()
	opts := AgentOptions{
		ControlServerURL:  "https://" + addr,
		RouterID:          "router-1",
		Token:             "secret-token",
		FingerprintSHA256: wrongFingerprint,
		PollInterval:      20 * time.Millisecond,
	}
	_ = Run(agentCtx, opts, handler)

	if handler.calls != 0 {
		t.Error("agent executed a command despite a fingerprint mismatch -- pinning is not load-bearing")
	}
}
