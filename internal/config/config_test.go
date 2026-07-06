package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureListFileCreatesMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "domain.list")

	if err := ensureListFile(path, []string{"example.com", "foo.com"}); err != nil {
		t.Fatalf("ensureListFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got, want := string(data), "example.com\nfoo.com\n"; got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestEnsureListFileDoesNotOverwriteExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "domain.list")
	if err := os.WriteFile(path, []byte("custom.com\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := ensureListFile(path, []string{"example.com"}); err != nil {
		t.Fatalf("ensureListFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got, want := string(data), "custom.com\n"; got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}

func TestLoadListFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "domain.list")
	content := "# comment\n\n example.com \n# another\nfoo.com\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, exists, err := loadListFile(path)
	if err != nil {
		t.Fatalf("loadListFile: %v", err)
	}
	if !exists {
		t.Fatal("expected exists=true")
	}
	want := []string{"example.com", "foo.com"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestLoadListFileMissing(t *testing.T) {
	got, exists, err := loadListFile(filepath.Join(t.TempDir(), "missing.list"))
	if err != nil {
		t.Fatalf("loadListFile: %v", err)
	}
	if exists {
		t.Fatal("expected exists=false")
	}
	if got != nil {
		t.Fatalf("expected nil entries, got %v", got)
	}
}

func TestLoadListFileExistingEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.list")
	if err := os.WriteFile(path, []byte("# comment only\n\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, exists, err := loadListFile(path)
	if err != nil {
		t.Fatalf("loadListFile: %v", err)
	}
	if !exists {
		t.Fatal("expected exists=true")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty entries, got %v", got)
	}
}

func TestLoadListFileDirectoryError(t *testing.T) {
	_, _, err := loadListFile(t.TempDir())
	if err == nil {
		t.Fatal("expected error for directory path")
	}
}
