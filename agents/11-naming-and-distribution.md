# Naming and Distribution

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

Consider notifying users when a new version is available:

```
$ myapp deploy production
[...]
✓ Deployed successfully.

A new version of myapp is available: v2.5.0 (you have v2.4.1)
Update with: brew upgrade myapp
```

Guidelines for update notifications:
- Check asynchronously — don't slow down the main command
- Show at the end of output, not the beginning
- Respect a `MYAPP_NO_UPDATE_CHECK=1` environment variable to disable
- Cache the check result so you're not hitting the network every invocation

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
