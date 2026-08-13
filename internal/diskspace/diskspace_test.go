package diskspace

import "testing"

func TestFreeBytes_CurrentDir(t *testing.T) {
	free, err := FreeBytes(".")
	if err != nil {
		t.Fatalf("FreeBytes: %v", err)
	}
	if free <= 0 {
		t.Errorf("FreeBytes(.) = %d, want > 0", free)
	}
}

func TestFreeBytes_Tmp(t *testing.T) {
	free, err := FreeBytes(t.TempDir())
	if err != nil {
		t.Fatalf("FreeBytes: %v", err)
	}
	if free <= 0 {
		t.Errorf("FreeBytes(tempdir) = %d, want > 0", free)
	}
}

func TestFreeBytes_NonexistentPath(t *testing.T) {
	if _, err := FreeBytes("/this/path/should/not/exist/hopefully"); err == nil {
		t.Error("expected error for a nonexistent path")
	}
}
