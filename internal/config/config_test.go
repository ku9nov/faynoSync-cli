package config

import "testing"

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
