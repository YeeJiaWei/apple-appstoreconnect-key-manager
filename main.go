package main

import (
	"fmt"
	"os"
	"os/exec"
)

const usage = `asckey — App Store Connect API keys kept in one place, picked per project.

Key material lives in ~/.asckey/keys/<name>/. Each project holds a local,
uncommitted .asckey file naming which key it uploads with.

USAGE
  asckey <command> [args]

COMMANDS
  add <name>      Register a key and copy its .p8 into the home store
                  --key-id <id> --issuer <uuid> --file <path.p8>
                  [--subject user|team] [--note text]
  list            Show every registered key (* marks the default)
  use <name>      Write the .asckey marker into the current directory
  default <name>  Set the fallback key for projects with no .asckey
  show            Report which key the current directory resolves to
  upload <pkg>    Upload to App Store Connect via altool  [--platform ios]
  validate <pkg>  Same as upload, but --validate-app (no delivery)
  rm <name>       Remove a key and delete its stored .p8
  env             Print the effective config paths and settings

ENVIRONMENT
  ASCKEY_HOME       Store root                (default ~/.asckey)
  ASCKEY_CONFIG     Full path to the registry (default $ASCKEY_HOME/config.json)
  ASCKEY_KEYS_DIR   Where per-alias keys live (default $ASCKEY_HOME/keys)
  ASCKEY_KEY        Key alias to use          (below --key flag, above .asckey)
  ASCKEY_PLATFORM   Default upload platform   (default ios)

EXAMPLES
  asckey add myapp --key-id ABCD1234EF --issuer 69a6de70-... \
      --file ~/Downloads/ABCD1234EF.p8 --subject user
  asckey use myapp
  asckey upload build/ios/ipa/*.ipa --platform ios
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]

	var err error
	switch cmd {
	case "add":
		err = cmdAdd(args)
	case "list", "ls":
		err = cmdList(args)
	case "use":
		err = cmdUse(args)
	case "default":
		err = cmdDefault(args)
	case "show", "which":
		err = cmdShow(args)
	case "upload":
		err = runAltool(args, false)
	case "validate":
		err = runAltool(args, true)
	case "rm", "remove":
		err = cmdRemove(args)
	case "env":
		err = cmdEnv(args)
	case "help", "-h", "--help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}

	if err != nil {
		// Surface altool's own exit code so Taskfile fails the same way it used to.
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "asckey: %v\n", err)
		os.Exit(1)
	}
}
