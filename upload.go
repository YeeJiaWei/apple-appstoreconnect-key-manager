package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// pickPackage expands any globs in args and returns the most recently modified match.
func pickPackage(args []string) (string, error) {
	var candidates []string
	for _, a := range args {
		if strings.ContainsAny(a, "*?[") {
			matches, err := filepath.Glob(a)
			if err != nil {
				return "", err
			}
			candidates = append(candidates, matches...)
			continue
		}
		candidates = append(candidates, a)
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no package matched — pass an .ipa/.pkg path")
	}
	best, bestMod := "", int64(-1)
	for _, c := range candidates {
		st, err := os.Stat(c)
		if err != nil || st.IsDir() {
			continue
		}
		if m := st.ModTime().UnixNano(); m > bestMod {
			best, bestMod = c, m
		}
	}
	if best == "" {
		return "", fmt.Errorf("no readable package among: %s", strings.Join(candidates, ", "))
	}
	return best, nil
}

// runAltool shells out to altool with the resolved key, streaming its output.
func runAltool(args []string, validateOnly bool) error {
	fs := flag.NewFlagSet("upload", flag.ExitOnError)
	platformDefault := os.Getenv("ASCKEY_PLATFORM")
	if platformDefault == "" {
		platformDefault = "ios"
	}
	platform := fs.String("platform", platformDefault, "altool platform: ios, macos, appletvos, visionos")
	key := fs.String("key", "", "override the key resolved from .asckey")
	verbose := fs.Bool("verbose", true, "pass --verbose to altool")
	positional, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	pkg, err := pickPackage(positional)
	if err != nil {
		return err
	}
	r, err := resolveAlias(*key)
	if err != nil {
		return err
	}

	action := "--upload-app"
	if validateOnly {
		action = "--validate-app"
	}
	altoolArgs := []string{
		"altool", action,
		"-f", pkg,
		"-t", *platform,
		"--apiKey", r.Entry.KeyID,
		"--apiIssuer", r.Entry.IssuerID,
	}
	// Individual keys sign their JWT with sub=user instead of an iss claim;
	// altool only does that when told to.
	if r.Entry.Subject == "user" {
		altoolArgs = append(altoolArgs, "--api-key-subject", "user")
	}
	if *verbose {
		altoolArgs = append(altoolArgs, "--verbose")
	}

	fmt.Fprintf(os.Stderr, "asckey: %s using key %q (%s, %s) from %s\n",
		strings.TrimPrefix(action, "--"), r.Alias, r.Entry.KeyID, r.Entry.Subject, r.Source)
	fmt.Fprintf(os.Stderr, "asckey: package %s\n", pkg)

	cmd := exec.Command("xcrun", altoolArgs...)
	// altool discovers the .p8 by scanning this directory, so each key gets its own.
	cmd.Env = append(os.Environ(), "API_PRIVATE_KEYS_DIR="+r.Dir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
