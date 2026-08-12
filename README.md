# asckey

App Store Connect API keys kept in one place, picked per project.

Apple's `altool` finds a `.p8` by scanning a fixed set of directories, which means one
global key per machine unless you juggle `ASC_KEY_ID` exports by hand. `asckey` replaces
that: every key is registered once under a name, and each project drops a one-line
`.asckey` file saying which name it uploads with.

Requires macOS with Xcode command line tools (`asckey` shells out to `xcrun altool`).

---

## Quick start

Five minutes from zero to your first upload.

### 1. Install

```bash
go install github.com/YeeJiaWei/apple-appstoreconnect-key-manager@latest
mv "$(go env GOPATH)/bin/apple-appstoreconnect-key-manager" "$(go env GOPATH)/bin/asckey"
```

Go names the binary after the repo, hence the rename. From a clone you can skip that:

```bash
git clone https://github.com/YeeJiaWei/apple-appstoreconnect-key-manager.git
cd apple-appstoreconnect-key-manager
go build -o "$(go env GOPATH)/bin/asckey" .
```

Either way the binary lands in `$GOPATH/bin` (usually `~/go/bin`). Make sure that's on
your `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"     # add to ~/.zshrc to make it stick
asckey help                                  # should print the usage text
```

No Go toolchain? Grab the `darwin_arm64` tarball from the
[Releases](../../releases) page, then:

```bash
tar xzf asckey_*_darwin_arm64.tar.gz
sudo mv asckey /usr/local/bin/
```

### 2. Get a key from App Store Connect

1. [App Store Connect → Users and Access → Integrations → App Store Connect API](https://appstoreconnect.apple.com/access/integrations/api)
2. Pick a tab: **Team Keys** (shared, needs Admin/App Manager) or **Individual Keys**
   (tied to your own account — this is the usual choice for a solo developer).
3. Generate a key, then note three things:
   - the **Key ID** (a short code like `ABCD1234EF`)
   - the **Issuer ID** (a UUID at the top of the page)
   - the **`.p8` file** it downloads — **Apple only lets you download this once.**

### 3. Register it

```bash
asckey add myapp \
  --key-id ABCD1234EF \
  --issuer 69a6de70-0000-0000-0000-000000000000 \
  --subject user \
  --file ~/Downloads/AuthKey_ABCD1234EF.p8 \
  --note "my personal key"
```

`--subject user` for an Individual Key, `--subject team` for a Team Key — see
[Individual vs team keys](#individual-vs-team-keys). The `.p8` is copied into
`~/.asckey/keys/myapp/` at mode `0600`; you can delete the download afterwards.

The first key you add automatically becomes the default.

### 4. Point a project at it

```bash
cd ~/projects/my-ios-app
asckey use myapp     # writes a local, uncommitted .asckey file
asckey show          # confirm: which key, and why that one
```

Add `.asckey` to the project's `.gitignore` — it names a key, so it differs per machine.

### 5. Upload

```bash
asckey validate build/ios/ipa/*.ipa    # dry run: altool --validate-app
asckey upload   build/ios/ipa/*.ipa    # the real thing: altool --upload-app
```

Globs are expanded internally and the **most recently modified** match wins, so
`*.ipa` does the right thing when old builds linger.

### Troubleshooting the first run

| Symptom | Fix |
|---|---|
| `command not found: asckey` | `$(go env GOPATH)/bin` isn't on your `PATH` — see step 1. |
| `no key selected` | You're outside a project with a `.asckey` file and no default is set. Run `asckey use <name>` here, or `asckey default <name>`. |
| `key material missing for "x"` | The registry knows the name but the `.p8` is gone. Re-add it, or `asckey rm x` and start over. |
| `key "x" is not registered` | Typo, or you're reading a different registry — check `asckey env`. |
| altool: `Unable to authenticate` | Wrong `--subject`. Individual keys need `user`; team keys need `team`. |
| altool: `unrecognized option --api-key-subject` | Xcode is too old. Individual keys need Xcode 16+. |

---

## Commands

```
asckey add <name>      Register a key and copy its .p8 into the store
asckey list            Show every registered key (* marks the default)
asckey use <name>      Write the .asckey marker into the current directory
asckey default <name>  Set the fallback key for projects with no .asckey
asckey show            Report which key the current directory resolves to
asckey env             Show the effective paths and settings
asckey upload <pkg>    Upload to App Store Connect via altool  [--platform ios]
asckey validate <pkg>  Same as upload, but --validate-app (no delivery)
asckey rm <name>       Remove a key and delete its stored .p8
```

## Configuration

Every setting has a working default — you can use `asckey` without setting any of these.
Set one only when you want to move something.

| Variable | Default | What it does |
|---|---|---|
| `ASCKEY_HOME` | `~/.asckey` | Root of the store — the registry and all key material live under it. |
| `ASCKEY_CONFIG` | `$ASCKEY_HOME/config.json` | Full path to the registry file, if you want it somewhere else entirely. |
| `ASCKEY_KEYS_DIR` | `$ASCKEY_HOME/keys` | Where the per-key directories live. |
| `ASCKEY_KEY` | *(unset)* | Key name to use, overriding any `.asckey` file. |
| `ASCKEY_PLATFORM` | `ios` | Default for `--platform` (`ios`, `macos`, `appletvos`, `visionos`). |

`asckey env` prints what is actually in effect, and whether each value came from the
environment or the built-in default:

```console
$ asckey env
ASCKEY_HOME      /Users/you/.asckey                 (default)
ASCKEY_CONFIG    /Users/you/.asckey/config.json     (default)
ASCKEY_KEYS_DIR  /Users/you/.asckey/keys            (default)
ASCKEY_KEY       (unset)                            (default)
ASCKEY_PLATFORM  ios                                (default)
```

Set a variable for one command by prefixing it, or for good by exporting it from
`~/.zshrc`:

```bash
ASCKEY_HOME=/tmp/scratch asckey list     # this command only
export ASCKEY_HOME=/Volumes/Secure/asckey  # every command in the shell
```

### The path variables

`ASCKEY_HOME`, `ASCKEY_CONFIG` and `ASCKEY_KEYS_DIR` all accept a leading `~` and may be
relative (they're resolved to absolute paths). The latter two default to sitting *inside*
`ASCKEY_HOME`, so moving the home moves everything:

```bash
export ASCKEY_HOME=/Volumes/Secure/asckey
# -> registry  /Volumes/Secure/asckey/config.json
# -> keys      /Volumes/Secure/asckey/keys/<name>/
```

Set `ASCKEY_CONFIG` or `ASCKEY_KEYS_DIR` when you want to split them apart — for example,
a registry synced between machines while the `.p8` files stay on an encrypted volume:

```bash
export ASCKEY_CONFIG=~/Dropbox/asckey/config.json
export ASCKEY_KEYS_DIR=/Volumes/Secure/asckey-keys
```

Nothing is migrated for you: point these at a new location and `asckey` sees an empty
registry until you move the old files across (or re-run `asckey add`).

Because everything relocates cleanly, a throwaway store is the safe way to experiment
without touching your real keys:

```bash
ASCKEY_HOME=$(mktemp -d) asckey add scratch --key-id … --issuer … --file …
```

### `ASCKEY_KEY`

Names the key to use, sitting between the `--key` flag and the project's `.asckey` file
in the [resolution order](#resolution-order). It's the answer for anywhere you can't or
won't commit a `.asckey` file — CI in particular:

```yaml
# GitHub Actions, GitLab CI, etc.
env:
  ASCKEY_KEY: ci-team
run: asckey upload build/ios/ipa/*.ipa
```

It's also a quick way to force one build through a different key without editing the
project file:

```bash
ASCKEY_KEY=other-team asckey validate build/ios/ipa/*.ipa
```

`asckey show` will tell you `resolved  ASCKEY_KEY env` when it's the one winning.

### `ASCKEY_PLATFORM`

Changes the default for `--platform`, for when a machine or project is mostly not iOS:

```bash
export ASCKEY_PLATFORM=macos
asckey upload dist/MyApp.pkg                  # goes as macos
asckey upload build/ios/ipa/*.ipa --platform ios  # explicit flag still wins
```

## Layout

```
~/.asckey/                                    # $ASCKEY_HOME
├── config.json                               # registry: name -> key id, issuer, subject
└── keys/
    └── myapp/
        └── AuthKey_ABCD1234EF.p8             # 0600, one dir per key
```

Each key gets its own directory so `API_PRIVATE_KEYS_DIR` can point at exactly one key
per invocation, with no filename collisions.

In a project:

```
.asckey        # plain text, one line: the key name. Not committed.
```

## Resolution order

1. `--key <name>` flag
2. `ASCKEY_KEY` environment variable
3. Nearest `.asckey` file, walking up from the current directory
4. Registry default (`asckey default <name>`)

`asckey show` prints which of these won.

Subcommands exit with altool's own exit code, so a Taskfile fails exactly as it did when
it called altool directly.

## Taskfile integration

```yaml
ios:upload:
  desc: Upload latest IPA to TestFlight (key named by the local, uncommitted .asckey file)
  cmds:
    - asckey upload {{.IPA_OUT}}/*.ipa --platform ios
```

## Individual vs team keys

| | Team key | Individual key |
|---|---|---|
| JWT claim | `iss: <issuer id>` | `sub: user` |
| Issuer ID | Required and meaningful | Required by altool, but **ignored** |
| `--subject` | `team` | `user` |
| Provisioning API | Full access | `/v1/certificates`, `/v1/bundleIds`, `/v1/profiles` return 401 |

Because an individual key's identity comes from the key itself, passing a wrong issuer
still authenticates. Don't read a successful upload as confirmation that the recorded
issuer is correct.

Individual keys need Xcode 16+ for altool's `--api-key-subject` flag. Verified on
Xcode 26.6.

## Security

- The store root and every key directory are `0700`; the registry and each `.p8` are `0600`.
- `.asckey` holds only a name — no secret — but stays uncommitted so machines can differ.
- The registry holds key IDs and issuer IDs, which are identifiers, not secrets. The `.p8`
  files are the secrets, and Apple only lets you download each one once. **Back them up**
  somewhere encrypted; there is no re-download.
- The repo's `.gitignore` blocks `*.p8`, `config.json`, `keys/` and `.asckey` so a
  misplaced `ASCKEY_HOME` can't leak key material into version control.

## Releases

Pushing a `v*` tag builds a `darwin/arm64` (Apple Silicon) binary and publishes it as a
GitHub Release; pushes to the `production` branch refresh a rolling `production-latest`
prerelease. See `.github/workflows/`.

```bash
git tag v1.0.0 && git push origin v1.0.0
```
