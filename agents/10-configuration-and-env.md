# Configuration and Environment Variables

Configuration lets users set persistent defaults so they don't have to repeat themselves. Environment variables bridge configuration and scripting. Together they make your tool adaptable without being complicated.

---

## The Configuration Priority Stack

Always resolve configuration in this order (highest priority first):

1. **CLI flags** — Most explicit, most intentional. Always win.
2. **Environment variables** — Set in shell, CI, or scripts.
3. **Project config file** — `.myapp.yml` or similar in the working directory.
4. **User config file** — `~/.myapp/config.yml` or XDG equivalent.
5. **Built-in defaults** — Hardcoded in the program.

This lets users work at the right level: devops sets env vars in CI, users set their preferences in a config file, and one-off overrides are handled with flags.

---

## Configuration Files

### Location

Support standard locations in this search order:

1. Path from `--config` flag (if provided)
2. `MYAPP_CONFIG` environment variable (if set)
3. `.myapp.yml` in the current working directory (project-level)
4. `~/.myapp/config.yml` (user-level)
5. `$XDG_CONFIG_HOME/myapp/config.yml` (XDG standard, for Linux)

Document all locations in your help text and documentation.

### Format

Use a standard, human-readable format. YAML is widely used and familiar:

```yaml
# ~/.myapp/config.yml
api_url: https://api.example.com
timeout: 60
output_format: table
default_environment: staging
```

TOML is also a good choice:
```toml
# ~/.myapp/config.toml
api_url = "https://api.example.com"
timeout = 60
output_format = "table"
default_environment = "staging"
```

Avoid XML. Consider JSON only if you need strict typing or programmatic generation.

### Initialization

Provide a command to create an initial config file:
```sh
myapp config init        # creates ~/.myapp/config.yml with defaults
myapp config init --local # creates .myapp.yml in current directory
```

### Viewing and editing
```sh
myapp config list        # show all current configuration
myapp config get api_url # show a specific value
myapp config set timeout 120  # set a value
myapp config edit        # open config in $EDITOR
```

---

## Environment Variables

### Naming conventions

- **UPPERCASE with underscores:** `MYAPP_API_KEY`, not `myapp-api-key`
- **Prefix with program name:** `MYAPP_TIMEOUT` not just `TIMEOUT` (avoids conflicts)
- **Consistent with flag names:** `--api-key` → `MYAPP_API_KEY`

### What to expose as environment variables

- Any setting that varies between environments (dev/staging/production)
- Secrets and credentials (API keys, tokens, passwords)
- Settings useful in CI/CD pipelines
- Settings that would be tedious to pass as flags every time

### Standard environment variables to respect

These are conventions other tools have established — honor them:

| Variable | Meaning |
|----------|---------|
| `NO_COLOR` | Disable all colored output (any non-empty value) |
| `TERM=dumb` | Disable color and advanced terminal features |
| `CI` | Running in CI environment — disable interactive prompts |
| `HOME` | User home directory |
| `EDITOR` | User's preferred text editor |
| `PAGER` | User's preferred pager (`less`, `more`) |
| `LANG` / `LC_ALL` | Locale and character encoding |

### Documenting environment variables

List every environment variable your tool reads in the help text and documentation:

```
Environment Variables:
  MYAPP_API_KEY      API authentication key (overrides config file)
  MYAPP_API_URL      API base URL [default: https://api.myapp.com]
  MYAPP_TIMEOUT      Request timeout in seconds [default: 30]
  MYAPP_NO_COLOR     Disable colored output (same as --no-color)
  NO_COLOR           Disable colored output (standard convention)
```

---

## Secrets and Credentials

Never store secrets in config files that might be committed to version control. Instead:

- Accept credentials via environment variables: `MYAPP_TOKEN=...`
- Accept credentials via flags: `--token` (for one-off use)
- Store secrets in a separate, permissions-protected credentials file
- Integrate with system keychains where appropriate

When a credential is missing, give a clear error and explain how to provide it:
```
Error: No API token found.

Provide one via:
  --token <token>
  MYAPP_TOKEN environment variable
  myapp auth login (stores credentials securely)
```

---

## `--config` Flag

Always provide a `--config` flag (or `-c`) to specify an alternative config file path. This is essential for:
- CI environments with non-standard setups
- Running multiple configurations side-by-side
- Testing with different configurations

```sh
myapp --config /etc/myapp/production.yml deploy
```

---

## Summary Checklist

- [ ] Configuration priority: flags > env vars > project config > user config > defaults
- [ ] Config file location documented (multiple locations checked in order)
- [ ] Config file uses standard format (YAML, TOML, or similar)
- [ ] `myapp config init` command creates starter config
- [ ] `myapp config list/get/set` commands for managing config
- [ ] `--config` flag to specify alternate config file
- [ ] Environment variables use `MYAPP_SETTING` naming convention
- [ ] `NO_COLOR`, `TERM=dumb`, `CI` env vars respected
- [ ] All environment variables documented in help text
- [ ] Credentials never stored in plain config files committed to VCS
- [ ] Missing credentials produce clear error with instructions
