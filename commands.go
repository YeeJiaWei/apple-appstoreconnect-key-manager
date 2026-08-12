package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// parseInterspersed parses flags that appear before, after, or between
// positional arguments, which Go's flag package stops at by default.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return positional, nil
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}
}

// cmdAdd registers a key and copies its .p8 into the home store.
func cmdAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	keyID := fs.String("key-id", "", "App Store Connect key ID (required)")
	issuer := fs.String("issuer", "", "issuer ID, or developer ID for individual keys (required)")
	subject := fs.String("subject", "team", "'user' for an individual key, 'team' for a team key")
	file := fs.String("file", "", "path to the .p8 private key (required)")
	note := fs.String("note", "", "free-text reminder of what this key is for")
	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	var alias string
	if len(positional) > 0 {
		alias = positional[0]
	}
	if alias == "" || *keyID == "" || *issuer == "" || *file == "" {
		return fmt.Errorf("usage: asckey add <name> --key-id <id> --issuer <uuid> --file <path.p8> [--subject user|team] [--note text]")
	}
	if *subject != "user" && *subject != "team" {
		return fmt.Errorf("--subject must be 'user' or 'team', got %q", *subject)
	}
	pem, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	if !strings.Contains(string(pem), "BEGIN PRIVATE KEY") {
		return fmt.Errorf("%s does not look like a PKCS#8 .p8 key", *file)
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	entry := KeyEntry{KeyID: *keyID, IssuerID: *issuer, Subject: *subject, Note: *note}
	dir, err := keyDir(alias)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	dest, err := keyFile(alias, entry)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dest, pem, 0o600); err != nil {
		return err
	}
	cfg.Keys[alias] = entry
	if cfg.Default == "" {
		cfg.Default = alias
	}
	if err := saveConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("added %q (%s, %s key) -> %s\n", alias, entry.KeyID, entry.Subject, dest)
	return nil
}

// cmdList prints every registered key, marking the default.
func cmdList(args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if len(cfg.Keys) == 0 {
		fmt.Println("no keys registered — see `asckey add --help`")
		return nil
	}
	names := make([]string, 0, len(cfg.Keys))
	for k := range cfg.Keys {
		names = append(names, k)
	}
	sort.Strings(names)
	fmt.Printf("%-16s %-14s %-38s %-8s %s\n", "NAME", "KEY ID", "ISSUER", "SUBJECT", "NOTE")
	for _, n := range names {
		e := cfg.Keys[n]
		marker := n
		if n == cfg.Default {
			marker = n + " *"
		}
		fmt.Printf("%-16s %-14s %-38s %-8s %s\n", marker, e.KeyID, e.IssuerID, e.Subject, e.Note)
	}
	if cfg.Default != "" {
		fmt.Printf("\n* = default when a project has no .asckey file\n")
	}
	return nil
}

// cmdUse writes the .asckey marker into the current directory.
func cmdUse(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: asckey use <name>")
	}
	alias := args[0]
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if _, ok := cfg.Keys[alias]; !ok {
		return fmt.Errorf("key %q is not registered (see `asckey list`)", alias)
	}
	body := "# asckey — which App Store Connect key this project uploads with.\n" +
		"# Local only: do not commit. The .p8 itself lives in ~/.asckey/keys/.\n" +
		alias + "\n"
	if err := os.WriteFile(projectFile, []byte(body), 0o600); err != nil {
		return err
	}
	cwd, _ := os.Getwd()
	fmt.Printf("wrote %s/%s -> %s\n", cwd, projectFile, alias)
	return nil
}

// cmdDefault sets the fallback key used when no .asckey marker is found.
func cmdDefault(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: asckey default <name>")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if _, ok := cfg.Keys[args[0]]; !ok {
		return fmt.Errorf("key %q is not registered (see `asckey list`)", args[0])
	}
	cfg.Default = args[0]
	if err := saveConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("default key is now %q\n", args[0])
	return nil
}

// cmdShow reports which key the current directory resolves to and why.
func cmdShow(args []string) error {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	key := fs.String("key", "", "override the resolved key")
	if _, err := parseInterspersed(fs, args); err != nil {
		return err
	}
	r, err := resolveAlias(*key)
	if err != nil {
		return err
	}
	fmt.Printf("name      %s\n", r.Alias)
	fmt.Printf("key id    %s\n", r.Entry.KeyID)
	fmt.Printf("issuer    %s\n", r.Entry.IssuerID)
	fmt.Printf("subject   %s\n", r.Entry.Subject)
	if r.Entry.Note != "" {
		fmt.Printf("note      %s\n", r.Entry.Note)
	}
	fmt.Printf("key file  %s\n", r.File)
	fmt.Printf("resolved  %s\n", r.Source)
	return nil
}

// cmdEnv prints the effective resolved paths and settings, noting which came from the environment.
func cmdEnv(args []string) error {
	home, err := storeDir()
	if err != nil {
		return err
	}
	homeSrc := "default"
	if os.Getenv("ASCKEY_HOME") != "" {
		homeSrc = "env"
	}

	cfgPath, err := configPath()
	if err != nil {
		return err
	}
	cfgSrc := "default"
	if os.Getenv("ASCKEY_CONFIG") != "" {
		cfgSrc = "env"
	}

	keysDir, ok, err := envPath("ASCKEY_KEYS_DIR")
	if err != nil {
		return err
	}
	keysSrc := "default"
	if ok {
		keysSrc = "env"
	} else {
		keysDir = filepath.Join(home, "keys")
	}

	key := os.Getenv("ASCKEY_KEY")
	keyDisplay, keySrc := "(unset)", "default"
	if key != "" {
		keyDisplay, keySrc = key, "env"
	}

	platform := os.Getenv("ASCKEY_PLATFORM")
	platformSrc := "default"
	if platform != "" {
		platformSrc = "env"
	} else {
		platform = "ios"
	}

	rows := [][2]string{
		{"ASCKEY_HOME", home},
		{"ASCKEY_CONFIG", cfgPath},
		{"ASCKEY_KEYS_DIR", keysDir},
		{"ASCKEY_KEY", keyDisplay},
		{"ASCKEY_PLATFORM", platform},
	}
	srcs := []string{homeSrc, cfgSrc, keysSrc, keySrc, platformSrc}
	for i, r := range rows {
		fmt.Printf("%-16s %-40s (%s)\n", r[0], r[1], srcs[i])
	}
	return nil
}

// cmdRemove drops a key from the registry and deletes its stored .p8.
func cmdRemove(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: asckey rm <name>")
	}
	alias := args[0]
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if _, ok := cfg.Keys[alias]; !ok {
		return fmt.Errorf("key %q is not registered", alias)
	}
	dir, err := keyDir(alias)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	delete(cfg.Keys, alias)
	if cfg.Default == alias {
		cfg.Default = ""
	}
	if err := saveConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("removed %q and deleted %s\n", alias, dir)
	return nil
}
