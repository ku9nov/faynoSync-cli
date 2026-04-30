package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateFieldTUF(t *testing.T) {
	cfg := Config{}

	if err := UpdateField(&cfg, "tuf", "true"); err != nil {
		t.Fatalf("UpdateField returned error: %v", err)
	}
	if !cfg.TUF {
		t.Fatal("expected cfg.TUF to be true")
	}

	if err := UpdateField(&cfg, "tuf", "false"); err != nil {
		t.Fatalf("UpdateField returned error: %v", err)
	}
	if cfg.TUF {
		t.Fatal("expected cfg.TUF to be false")
	}
}

func TestUpdateFieldTUFInvalidValue(t *testing.T) {
	cfg := Config{}

	if err := UpdateField(&cfg, "tuf", "maybe"); err == nil {
		t.Fatal("expected validation error for invalid boolean")
	}
}

func TestEnsureDeviceIDCreatesWhenMissing(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	deviceID, err := EnsureDeviceID()
	if err != nil {
		t.Fatalf("EnsureDeviceID returned error: %v", err)
	}
	if strings.TrimSpace(deviceID) == "" {
		t.Fatal("expected non-empty device id")
	}

	path := filepath.Join(tempHome, ".faynosync", "device_id")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read device id file: %v", err)
	}
	if got := strings.TrimSpace(string(raw)); got != deviceID {
		t.Fatalf("unexpected stored device id: got %q, want %q", got, deviceID)
	}
}

func TestEnsureDeviceIDReturnsExistingValue(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	dir := filepath.Join(tempHome, ".faynosync")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	want := "existing-device-id"
	if err := os.WriteFile(filepath.Join(dir, "device_id"), []byte(want+"\n"), 0o600); err != nil {
		t.Fatalf("write device id file: %v", err)
	}

	got, err := EnsureDeviceID()
	if err != nil {
		t.Fatalf("EnsureDeviceID returned error: %v", err)
	}
	if got != want {
		t.Fatalf("unexpected device id: got %q, want %q", got, want)
	}
}

func TestEnsureDeviceIDRegeneratesWhenEmpty(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	dir := filepath.Join(tempHome, ".faynosync")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	path := filepath.Join(dir, "device_id")
	if err := os.WriteFile(path, []byte("\n"), 0o600); err != nil {
		t.Fatalf("write empty device id file: %v", err)
	}

	got, err := EnsureDeviceID()
	if err != nil {
		t.Fatalf("EnsureDeviceID returned error: %v", err)
	}
	if strings.TrimSpace(got) == "" {
		t.Fatal("expected regenerated non-empty device id")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read device id file: %v", err)
	}
	if strings.TrimSpace(string(raw)) != got {
		t.Fatalf("expected regenerated device id to be persisted")
	}
}
