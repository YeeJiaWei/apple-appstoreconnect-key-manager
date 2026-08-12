package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	storeDirName   = ".asckey"
	configFileName = "config.json"
	projectFile    = ".asckey"
)

// KeyEntry describes one registered App Store Connect API key.
type KeyEntry struct {
	KeyID    string `json:"keyId"`
	IssuerID string `json:"issuerId"`
	Subject  string `json:"subject"` // "user" for an individual key, "team" for a team key
	Note     string `json:"note,omitempty"`
}

// Config is the on-disk registry of every key asckey knows about.
type Config struct {
	Default string              `json:"default,omitempty"`
	Keys    map[string]KeyEntry `json:"keys"`
}

// envPath resolves a path-valued env var, expanding a leading ~ and making it absolute.
// The bool return is false when the var is unset or empty, meaning the caller's default applies.
func envPath(name string) (string, bool, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", false, nil
	}
	if v == "~" || strings.HasPrefix(v, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false, err
		}
		v = filepath.Join(home, strings.TrimPrefix(v, "~"))
	}
	abs, err := filepath.Abs(v)
	if err != nil {
		return "", false, err
	}
	return abs, true, nil
}

// storeDir returns the home-directory root holding the registry and key material.
func storeDir() (string, error) {
	if v, ok, err := envPath("ASCKEY_HOME"); err != nil {
		return "", err
	} else if ok {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, storeDirName), nil
}

// configPath returns the full path to the registry file.
func configPath() (string, error) {
	if v, ok, err := envPath("ASCKEY_CONFIG"); err != nil {
		return "", err
	} else if ok {
		return v, nil
	}
	root, err := storeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, configFileName), nil
}

// keyDir returns the per-alias directory altool scans via API_PRIVATE_KEYS_DIR.
func keyDir(alias string) (string, error) {
	if v, ok, err := envPath("ASCKEY_KEYS_DIR"); err != nil {
		return "", err
	} else if ok {
		return filepath.Join(v, alias), nil
	}
	root, err := storeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "keys", alias), nil
}

// keyFile returns the .p8 path altool expects for the given alias.
func keyFile(alias string, e KeyEntry) (string, error) {
	dir, err := keyDir(alias)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "AuthKey_"+e.KeyID+".p8"), nil
}

// loadConfig reads the registry, returning an empty one when it does not exist yet.
func loadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}
	cfg := &Config{Keys: map[string]KeyEntry{}}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.Keys == nil {
		cfg.Keys = map[string]KeyEntry{}
	}
	return cfg, nil
}

// saveConfig writes the registry back with owner-only permissions.
func saveConfig(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}
