package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultServer = "https://example.com"
	DefaultOwner  = "example"
)

type Config struct {
	Server string `yaml:"server"`
	Owner  string `yaml:"owner"`
}

func Default() Config {
	return Config{
		Server: DefaultServer,
		Owner:  DefaultOwner,
	}
}

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".faynosync", "config.yaml"), nil
}
func Init(cfg Config) (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}

	if err := SaveAt(path, cfg); err != nil {
		return "", err
	}

	return path, nil
}

func Load() (Config, string, error) {
	path, err := Path()
	if err != nil {
		return Config{}, "", err
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, "", errors.New("config not found, run: faynosync init")
		}
		return Config{}, "", err
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, "", err
	}

	return cfg, path, nil
}

func SaveAt(path string, cfg Config) error {
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, out, 0o644)
}

func UpdateField(cfg *Config, key, value string) error {
	switch key {
	case "server":
		cfg.Server = value
	case "owner":
		cfg.Owner = value
	default:
		return fmt.Errorf("unknown key: %s (allowed: server, owner)", key)
	}

	return nil
}

func Marshal(cfg Config) ([]byte, error) {
	return yaml.Marshal(cfg)
}
