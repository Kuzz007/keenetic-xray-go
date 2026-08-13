package botcontrol

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func fingerprintOf(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	sum := sha256.Sum256(srv.Certificate().Raw)
	return hex.EncodeToString(sum[:])
}

func TestNewPinnedClient_AcceptsMatchingFingerprint(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := newPinnedClient(fingerprintOf(t, srv))
	if err != nil {
		t.Fatalf("newPinnedClient: %v", err)
	}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestNewPinnedClient_RejectsWrongFingerprint(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wrongFingerprint := hex.EncodeToString(sha256.New().Sum(nil)) // sha256 of empty input -- not this server's cert
	client, err := newPinnedClient(wrongFingerprint)
	if err != nil {
		t.Fatalf("newPinnedClient: %v", err)
	}
	if _, err := client.Get(srv.URL); err == nil {
		t.Error("expected the request to fail with a mismatched fingerprint")
	}
}

func TestNewPinnedClient_InvalidFingerprintHex(t *testing.T) {
	if _, err := newPinnedClient("not-valid-hex!!"); err == nil {
		t.Error("expected error for invalid hex fingerprint")
	}
}

func TestAgentOptions_Validate(t *testing.T) {
	valid := AgentOptions{ControlServerURL: "https://x", RouterID: "r", Token: "t", FingerprintSHA256: "ab"}
	if err := valid.validate(); err != nil {
		t.Errorf("valid options should validate: %v", err)
	}

	cases := []AgentOptions{
		{RouterID: "r", Token: "t", FingerprintSHA256: "ab"},
		{ControlServerURL: "https://x", Token: "t", FingerprintSHA256: "ab"},
		{ControlServerURL: "https://x", RouterID: "r", FingerprintSHA256: "ab"},
		{ControlServerURL: "https://x", RouterID: "r", Token: "t"},
	}
	for i, c := range cases {
		if err := c.validate(); err == nil {
			t.Errorf("case %d: expected error for incomplete options %+v", i, c)
		}
	}
}

type fakeHandler struct {
	calls int32
}

func (h *fakeHandler) Handle(_ context.Context, cmd Command) (string, error) {
	atomic.AddInt32(&h.calls, 1)
	return "did " + cmd.Action, nil
}

func TestRun_PollsExecutesAndPostsResult(t *testing.T) {
	var mu sync.Mutex
	served := false
	var gotResult ResultRequest

	mux := http.NewServeMux()
	mux.HandleFunc("/agent/poll", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if served {
			_ = json.NewEncoder(w).Encode(PollResponse{})
			return
		}
		served = true
		_ = json.NewEncoder(w).Encode(PollResponse{Command: &Command{ID: "1", Action: ActionStatus}})
	})
	mux.HandleFunc("/agent/result", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		_ = json.NewDecoder(r.Body).Decode(&gotResult)
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	handler := &fakeHandler{}
	opts := AgentOptions{
		ControlServerURL:  srv.URL,
		RouterID:          "router-1",
		Token:             "test-token",
		FingerprintSHA256: fingerprintOf(t, srv),
		PollInterval:      10 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_ = Run(ctx, opts, handler)

	if atomic.LoadInt32(&handler.calls) == 0 {
		t.Fatal("handler was never called")
	}
	mu.Lock()
	defer mu.Unlock()
	if gotResult.Result.CommandID != "1" || gotResult.Result.Output != "did status" {
		t.Errorf("gotResult = %+v, want CommandID=\"1\" Output=\"did status\"", gotResult)
	}
}

func TestRun_WrongTokenNeverCallsHandler(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/agent/poll", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	handler := &fakeHandler{}
	opts := AgentOptions{
		ControlServerURL:  srv.URL,
		RouterID:          "router-1",
		Token:             "wrong-token",
		FingerprintSHA256: fingerprintOf(t, srv),
		PollInterval:      10 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = Run(ctx, opts, handler)

	if atomic.LoadInt32(&handler.calls) != 0 {
		t.Error("handler should never be called when the server rejects the token")
	}
}
