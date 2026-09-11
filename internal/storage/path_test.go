package storage

import (
	"path/filepath"
	"testing"
)

func TestSafeJoin(t *testing.T) {
	base := t.TempDir()
	got, err := SafeJoin(base, "file-id", "file_720.mp4")
	if err != nil {
		t.Fatalf("SafeJoin() error = %v", err)
	}
	want := filepath.Join(base, "file-id", "file_720.mp4")
	if got != want {
		t.Fatalf("SafeJoin() = %q, want %q", got, want)
	}
}

func TestSafeJoinRejectsTraversal(t *testing.T) {
	base := t.TempDir()
	if _, err := SafeJoin(base, "..", "outside.mp4"); err == nil {
		t.Fatal("SafeJoin() accepted a path outside the storage directory")
	}
}

func TestSafeJoinRejectsEmptyBase(t *testing.T) {
	if _, err := SafeJoin("", "file-id"); err == nil {
		t.Fatal("SafeJoin() accepted an empty base path")
	}
}
