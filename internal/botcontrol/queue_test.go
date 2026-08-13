package botcontrol

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_EnqueueDequeueFIFO(t *testing.T) {
	s, err := LoadStore("")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	id1, err := s.Enqueue("router-1", ActionStatus, nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	id2, err := s.Enqueue("router-1", ActionSwitchPrimary, []string{"x"})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if id1 == id2 {
		t.Fatalf("Enqueue returned the same ID twice: %q", id1)
	}

	got, err := s.Dequeue("router-1")
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if got == nil || got.ID != id1 || got.Action != ActionStatus {
		t.Errorf("first Dequeue = %+v, want ID=%s Action=%s", got, id1, ActionStatus)
	}

	got, err = s.Dequeue("router-1")
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if got == nil || got.ID != id2 || got.Action != ActionSwitchPrimary {
		t.Errorf("second Dequeue = %+v, want ID=%s Action=%s", got, id2, ActionSwitchPrimary)
	}

	got, err = s.Dequeue("router-1")
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if got != nil {
		t.Errorf("third Dequeue = %+v, want nil (queue empty)", got)
	}
}

func TestStore_DequeueUnknownRouterIsEmptyNotError(t *testing.T) {
	s, err := LoadStore("")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	cmd, err := s.Dequeue("never-seen")
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if cmd != nil {
		t.Errorf("Dequeue on unknown router = %+v, want nil", cmd)
	}
}

func TestStore_RecordResultAndLastResult(t *testing.T) {
	s, err := LoadStore("")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if r := s.LastResult("router-1"); r != nil {
		t.Errorf("LastResult before any result = %+v, want nil", r)
	}

	result := Result{CommandID: "abc", Output: "did status", Completed: time.Now()}
	if err := s.RecordResult("router-1", result); err != nil {
		t.Fatalf("RecordResult: %v", err)
	}

	got := s.LastResult("router-1")
	if got == nil || got.CommandID != "abc" || got.Output != "did status" {
		t.Errorf("LastResult = %+v, want CommandID=abc Output=\"did status\"", got)
	}

	// A second result replaces the first.
	result2 := Result{CommandID: "def", Output: "did switch_primary", Completed: time.Now()}
	if err := s.RecordResult("router-1", result2); err != nil {
		t.Fatalf("RecordResult: %v", err)
	}
	got = s.LastResult("router-1")
	if got == nil || got.CommandID != "def" {
		t.Errorf("LastResult after second record = %+v, want CommandID=def", got)
	}
}

func TestStore_PendingCount(t *testing.T) {
	s, err := LoadStore("")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if n := s.PendingCount("router-1"); n != 0 {
		t.Errorf("PendingCount on unknown router = %d, want 0", n)
	}

	if _, err := s.Enqueue("router-1", ActionStatus, nil); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := s.Enqueue("router-1", ActionSwitchPrimary, nil); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if n := s.PendingCount("router-1"); n != 2 {
		t.Errorf("PendingCount = %d, want 2", n)
	}

	if _, err := s.Dequeue("router-1"); err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if n := s.PendingCount("router-1"); n != 1 {
		t.Errorf("PendingCount after one Dequeue = %d, want 1", n)
	}
}

func TestStore_RouterIDsSorted(t *testing.T) {
	s, err := LoadStore("")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if _, err := s.Enqueue("zeta", ActionStatus, nil); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := s.Enqueue("alpha", ActionStatus, nil); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	ids := s.RouterIDs()
	if len(ids) != 2 || ids[0] != "alpha" || ids[1] != "zeta" {
		t.Errorf("RouterIDs = %v, want [alpha zeta]", ids)
	}
}

func TestStore_PersistsAcrossReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "queue.json")

	s1, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	id, err := s1.Enqueue("router-1", ActionStatus, nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if err := s1.RecordResult("router-2", Result{CommandID: "other", Output: "hi"}); err != nil {
		t.Fatalf("RecordResult: %v", err)
	}

	s2, err := LoadStore(path)
	if err != nil {
		t.Fatalf("reload LoadStore: %v", err)
	}
	cmd, err := s2.Dequeue("router-1")
	if err != nil {
		t.Fatalf("Dequeue after reload: %v", err)
	}
	if cmd == nil || cmd.ID != id {
		t.Errorf("Dequeue after reload = %+v, want ID=%s", cmd, id)
	}
	if r := s2.LastResult("router-2"); r == nil || r.CommandID != "other" {
		t.Errorf("LastResult after reload = %+v, want CommandID=other", r)
	}
}

func TestStore_LoadStoreMissingFileIsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.json")

	s, err := LoadStore(path)
	if err != nil {
		t.Fatalf("LoadStore on missing file: %v", err)
	}
	if len(s.RouterIDs()) != 0 {
		t.Errorf("fresh store should have no routers, got %v", s.RouterIDs())
	}
}

func TestStore_AwaitResult_ReturnsImmediatelyIfAlreadyPresent(t *testing.T) {
	s, err := LoadStore("")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if err := s.RecordResult("router-1", Result{CommandID: "abc", Output: "ok"}); err != nil {
		t.Fatalf("RecordResult: %v", err)
	}

	start := time.Now()
	result, ok := s.AwaitResult(context.Background(), "router-1", "abc", time.Second)
	if !ok || result == nil || result.Output != "ok" {
		t.Fatalf("AwaitResult = %+v, %v, want ok=true Output=ok", result, ok)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("AwaitResult took %v for an already-present result, want near-instant", elapsed)
	}
}

func TestStore_AwaitResult_SeesLaterResult(t *testing.T) {
	s, err := LoadStore("")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = s.RecordResult("router-1", Result{CommandID: "target", Output: "eventually"})
	}()

	result, ok := s.AwaitResult(context.Background(), "router-1", "target", 2*time.Second)
	if !ok || result == nil || result.Output != "eventually" {
		t.Fatalf("AwaitResult = %+v, %v, want ok=true Output=eventually", result, ok)
	}
}

func TestStore_AwaitResult_TimesOut(t *testing.T) {
	s, err := LoadStore("")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	result, ok := s.AwaitResult(context.Background(), "router-1", "never-comes", 100*time.Millisecond)
	if ok || result != nil {
		t.Errorf("AwaitResult = %+v, %v, want ok=false nil on timeout", result, ok)
	}
}

func TestStore_AwaitResult_IgnoresMismatchedCommandID(t *testing.T) {
	s, err := LoadStore("")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	if err := s.RecordResult("router-1", Result{CommandID: "stale", Output: "old"}); err != nil {
		t.Fatalf("RecordResult: %v", err)
	}
	result, ok := s.AwaitResult(context.Background(), "router-1", "fresh", 100*time.Millisecond)
	if ok || result != nil {
		t.Errorf("AwaitResult = %+v, %v, want ok=false (stale result must not match)", result, ok)
	}
}

func TestStore_AwaitResult_ContextCancelled(t *testing.T) {
	s, err := LoadStore("")
	if err != nil {
		t.Fatalf("LoadStore: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, ok := s.AwaitResult(ctx, "router-1", "id", time.Second)
	if ok || result != nil {
		t.Errorf("AwaitResult with cancelled context = %+v, %v, want ok=false", result, ok)
	}
}
