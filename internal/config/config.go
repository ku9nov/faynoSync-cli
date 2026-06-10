package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultServer = "https://example.com"
	DefaultOwner  = "example"
	deviceIDFile  = "device_id"

	EnvToken   = "FAYNOSYNC_TOKEN"
	EnvURL     = "FAYNOSYNC_URL"
	EnvAccount = "FAYNOSYNC_ACCOUNT"
	EnvEdge    = "FAYNOSYNC_EDGE"
)

type Config struct {
	Server string `yaml:"server"`
	Owner  string `yaml:"owner"`
	Edge   string `yaml:"edge,omitempty"`
	TUF    bool   `yaml:"tuf"`
}

type RuntimeConfig struct {
	Token  string
	Server string
	Owner  string
	Edge   string
	TUF    bool
}

type PublicRuntimeConfig struct {
	Server string
	Owner  string
	Edge   string
	TUF    bool
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

func EnsureDeviceID() (string, error) {
	configPath, err := Path()
	if err != nil {
		return "", err
	}

	configDir := filepath.Dir(configPath)
	devicePath := filepath.Join(configDir, deviceIDFile)

	raw, err := os.ReadFile(devicePath)
	if err == nil {
		deviceID := strings.TrimSpace(string(raw))
		if deviceID != "" {
			return deviceID, nil
		}
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return "", err
	}

	deviceID, err := generateDeviceID()
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(devicePath, []byte(deviceID+"\n"), 0o600); err != nil {
		return "", err
	}

	return deviceID, nil
}

func generateDeviceID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
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
	case "edge":
		cfg.Edge = value
	case "tuf":
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("invalid value for tuf: %q (expected true/false)", value)
		}
		cfg.TUF = parsed
	default:
		return fmt.Errorf("unknown key: %s (allowed: server, owner, edge, tuf)", key)
	}

	return nil
}

func Marshal(cfg Config) ([]byte, error) {
	return yaml.Marshal(cfg)
}

func LoadRuntime() (RuntimeConfig, string, error) {
	token := strings.TrimSpace(os.Getenv(EnvToken))
	if token == "" {
		return RuntimeConfig{}, "", fmt.Errorf("%s is required", EnvToken)
	}

	resolved, path, err := resolveRuntime()
	if err != nil {
		return RuntimeConfig{}, path, err
	}

	return RuntimeConfig{
		Token:  token,
		Server: resolved.Server,
		Owner:  resolved.Owner,
		Edge:   resolved.Edge,
		TUF:    resolved.TUF,
	}, path, nil
}

func LoadPublicRuntime() (PublicRuntimeConfig, string, error) {
	resolved, path, err := resolveRuntime()
	if err != nil {
		return PublicRuntimeConfig{}, path, err
	}

	return resolved, path, nil
}

func resolveRuntime() (PublicRuntimeConfig, string, error) {
	envServer := strings.TrimSpace(os.Getenv(EnvURL))
	envOwner := strings.TrimSpace(os.Getenv(EnvAccount))
	envEdge := strings.TrimSpace(os.Getenv(EnvEdge))
	needsConfig := envServer == "" || envOwner == ""

	cfg := Config{}
	path := ""
	if needsConfig {
		var err error
		cfg, path, err = Load()
		if err != nil {
			return PublicRuntimeConfig{}, "", err
		}
	}

	server := envServer
	if server == "" {
		server = strings.TrimSpace(cfg.Server)
	}

	owner := envOwner
	if owner == "" {
		owner = strings.TrimSpace(cfg.Owner)
	}

	edge := envEdge
	if edge == "" {
		edge = strings.TrimSpace(cfg.Edge)
	}

	if server == "" {
		return PublicRuntimeConfig{}, path, errors.New("server is empty: set in config or via FAYNOSYNC_URL")
	}
	if owner == "" {
		return PublicRuntimeConfig{}, path, errors.New("owner is empty: set in config or via FAYNOSYNC_ACCOUNT")
	}

	return PublicRuntimeConfig{
		Server: server,
		Owner:  owner,
		Edge:   edge,
		TUF:    cfg.TUF,
	}, path, nil
}
