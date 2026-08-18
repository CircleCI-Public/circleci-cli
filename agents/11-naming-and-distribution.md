# Naming and Distribution

**Rules**

1. **Command names are lowercase, short, hyphen-separated**, descriptive of purpose,
   and never shadow a standard UNIX tool.
2. **Semantic versioning, with `--version` / `-V`** exposing the version plus build
   context (Go version, OS/arch, commit, build time), and git tags matching.
3. **Ship through the package managers users actually have**, plus direct binary
   downloads for anyone who can't use one, and keep installation to a single
   command.
4. **Provide shell completion** for bash, zsh and fish.
5. **Update notices never block, never touch stdout, and go quiet when they can't
   help** — no TTY on either stream, CI, agents/MCP, no token, dev builds, or
   opted out. They run in the background during `PersistentPreRunE` and are drained
   in `PersistentPostRunE`, which only runs on success, so a notice never lands on
   top of an error. Implementation in `internal/update`.
6. **Stay channel-agnostic in user-facing upgrade text.** This CLI ships through
   seven channels, so link the release page rather than naming one package
   manager's upgrade command.

---

How you name your command and how you distribute it affects whether users can find, install, and remember your tool.

---

## Naming Your Command

### Use lowercase
Command names should always be lowercase. Mixed case is harder to type and inconsistent with convention.

```sh
# Good
myapp
my-tool
deploy-helper

# Bad
MyApp
myTool
DeployHelper
```

### Keep it short
Short names reduce typing burden and are easier to remember. Most successful CLI tools have names of 2-8 characters.

```sh
git     # 3 chars
npm     # 3 chars
docker  # 6 chars
kubectl # 7 chars
```

### Use hyphens for multi-word names
Hyphens are the standard word separator in command names:

```sh
my-tool       # good
my_tool       # unusual (underscores common in env vars, not commands)
mytool        # fine if short, but can be hard to read
```

### Avoid conflicts with common UNIX tools
Don't name your tool something that shadows a well-known command:

```sh
# Dangerous — shadows standard UNIX tools
ls
cp
find
test
printf

# Also avoid near-conflicts that cause confusion
grep   → use a different name
curl   → use a different name
```

### Make the name descriptive
The name should hint at the tool's purpose without requiring documentation:

```sh
dbmigrate    # clearly about database migrations
imgopt       # clearly about image optimization
portfwd      # clearly about port forwarding
```

Avoid overly generic names that don't hint at purpose: `runner`, `helper`, `tool`, `manager`.

---

## Versioning

### Use semantic versioning
Follow [SemVer](https://semver.org/): `MAJOR.MINOR.PATCH`

- **MAJOR:** Breaking changes
- **MINOR:** New features, backward compatible
- **PATCH:** Bug fixes, backward compatible

### Expose the version via flag
Always implement `--version` / `-V`:

```sh
$ myapp --version
myapp 2.4.1
```

Consider including more context for debugging:
```sh
$ myapp --version
myapp 2.4.1
  Go version: go1.21.0
  OS/Arch:    linux/amd64
  Commit:     a3f2b1c
  Built:      2024-01-15T14:30:00Z
```

### Tag your releases
Use git tags matching the version: `v2.4.1`. This enables users to install specific versions.

---

## Distribution

### Provide multiple installation methods

Support the package managers your users actually use. At minimum:

| Platform | Method |
|----------|--------|
| macOS | Homebrew (`brew install myapp`) |
| Linux (Debian/Ubuntu) | APT or direct `.deb` |
| Linux (RHEL/Fedora) | RPM or direct `.rpm` |
| Windows | Chocolatey, Scoop, or WinGet |
| Cross-platform | npm, pip, or language-specific package manager |
| Any platform | Direct binary download from GitHub releases |

### Single-binary distribution
Tools written in compiled languages (Go, Rust) can distribute as a single executable file — no runtime dependencies to install. This dramatically reduces installation complexity and is worth considering during language selection.

### Keep installation simple
The ideal installation experience is a single command:

```sh
# Perfect
brew install myapp

# Good
curl -fsSL https://myapp.com/install.sh | sh

# Acceptable
npm install -g myapp
pip install myapp --break-system-packages
```

Document the installation method in your README prominently — it's often the first thing new users look for.

### Provide direct download links
Always offer direct binary downloads for users who can't use package managers (corporate environments, air-gapped systems). Host on GitHub Releases or a permanent URL.

```
https://github.com/myorg/myapp/releases/latest/download/myapp-linux-amd64
https://github.com/myorg/myapp/releases/latest/download/myapp-darwin-arm64
https://github.com/myorg/myapp/releases/latest/download/myapp-windows-amd64.exe
```

---

## Shell Completion

Providing shell completion dramatically improves the user experience. Users can tab-complete subcommands, flags, and even argument values.

```sh
# Install completion for bash
myapp completion bash >> ~/.bashrc

# Install completion for zsh
myapp completion zsh >> ~/.zshrc

# Install completion for fish
myapp completion fish > ~/.config/fish/completions/myapp.fish
```

Most argument parsing libraries (cobra, click, clap, etc.) can generate completion scripts automatically.

---

## Update Notifications

`circleci` notifies users after a successful command when a newer release
exists. The message is deliberately channel-agnostic — we ship through seven
channels, so it links the GitHub release page for the new version rather than
naming one package manager's upgrade command:

```
$ circleci run get
[...]

A new version of circleci is available: 1.2.0 → 1.3.0
https://github.com/CircleCI-Public/circleci-cli/releases/tag/v1.3.0
```

The implementation lives in `internal/update` (business logic) and is wired in
`internal/cmd/root/root.go`. What it does and why:

- **Never blocks.** The check runs in a background goroutine started in the root
  `PersistentPreRunE`; the result is drained in `PersistentPostRunE`, which
  cobra runs only when the command succeeded — so the notice always follows all
  output and never lands on top of an error.
- **stderr, after output.** stdout stays clean for pipelines. Both stdout *and*
  stderr must be TTYs, so any pipe or redirect on either stream silences it.
- **Own endpoint, cached twice.** The version comes from
  `GET /api/v3/tool/releases?filter[tool]=circleci-cli` (cached server-side), behind a
  one-method `update.Source` seam. A `state.yml` under `$XDG_STATE_HOME` caches
  the result for 24h (matched to the ~daily release cadence) so we hit the
  network at most once per window.
- **Blanket 6h delay.** A release stays quiet for 6h after publication so the
  package managers have time to catch up. No channel detection. The value is
  kept well under our ~daily release cadence — a delay at or above the cadence
  would leave the newest release perpetually inside the window and nobody would
  ever be nagged — and matches the observed ~1-6h bot-moderated propagation of
  homebrew-core and winget-pkgs.
- **Off when it can't help.** Disabled for `version == "dev"`, in CI, for agents
  and MCP, when no token is configured (the endpoint needs auth), and via
  `CIRCLE_NO_UPDATE_CHECK` or `circleci setting set update-check off`.

---

## Summary Checklist

- [ ] Command name is lowercase
- [ ] Name uses hyphens for word separation
- [ ] Name doesn't shadow common UNIX commands
- [ ] `--version` / `-V` flag implemented
- [ ] Semantic versioning used (`MAJOR.MINOR.PATCH`)
- [ ] Multiple installation methods documented (package manager + direct download)
- [ ] Direct binary downloads available for major platforms
- [ ] Shell completion scripts provided (bash, zsh, fish)
- [ ] GitHub release tags match version numbers
