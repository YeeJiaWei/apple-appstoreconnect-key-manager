package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolved bundles an alias with its registry entry and on-disk key location.
type resolved struct {
	Alias  string
	Entry  KeyEntry
	Dir    string
	File   string
	Source string
}

// findProjectFile walks up from dir looking for the nearest .asckey marker.
func findProjectFile(dir string) (string, bool) {
	for {
		p := filepath.Join(dir, projectFile)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// readAlias pulls the first non-empty, non-comment line out of a .asckey file.
func readAlias(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		return line, nil
	}
	return "", fmt.Errorf("%s contains no key name", path)
}

// resolveAlias picks the key for the current directory: explicit flag first,
// then ASCKEY_KEY, then the nearest .asckey marker, then the registry default.
func resolveAlias(override string) (*resolved, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	alias, source := override, "--key flag"
	if alias == "" {
		if v := os.Getenv("ASCKEY_KEY"); v != "" {
			alias, source = v, "ASCKEY_KEY env"
		} else {
			cwd, err := os.Getwd()
			if err != nil {
				return nil, err
			}
			if p, ok := findProjectFile(cwd); ok {
				a, err := readAlias(p)
				if err != nil {
					return nil, err
				}
				alias, source = a, p
			} else if cfg.Default != "" {
				alias, source = cfg.Default, "registry default"
			}
		}
	}
	if alias == "" {
		return nil, errors.New("no key selected — run `asckey use <name>` in the project, or `asckey default <name>`")
	}
	entry, ok := cfg.Keys[alias]
	if !ok {
		return nil, fmt.Errorf("key %q is not registered (see `asckey list`)", alias)
	}
	dir, err := keyDir(alias)
	if err != nil {
		return nil, err
	}
	file, err := keyFile(alias, entry)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(file); err != nil {
		return nil, fmt.Errorf("key material missing for %q at %s", alias, file)
	}
	return &resolved{Alias: alias, Entry: entry, Dir: dir, File: file, Source: source}, nil
}
