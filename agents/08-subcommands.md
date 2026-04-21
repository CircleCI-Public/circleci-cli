# Subcommands

Subcommands let you group related functionality under a single tool. Used well, they create an intuitive namespace. Used poorly, they create a confusing maze.

---

## Command Structure Models

There are two common models for structuring multi-command CLIs. Choose one and be consistent:

**Space-separated subcommands** (e.g., git, docker):
```sh
git add file.txt
git commit -m "message"
docker build .
docker run image
```

**Colon-namespaced topic:command** (e.g., Heroku CLI):
```sh
heroku apps:create my-app
heroku apps:favorites:add my-app
heroku pg:credentials:rotate
```

In the colon-namespace model: **topics are plural nouns** (the resource being managed); **commands are verbs** (the action). If a single word isn't possible, kebab-case is the fallback — but treat it as a last resort, not the default.

**Important rule in the colon-namespace model:** The root topic command IS the list. Never create an explicit `*:list` command:
```sh
heroku config        ← lists config vars (correct)
heroku config:list   ← don't create this
```

---

## When to Use Subcommands

Use subcommands when your tool has multiple distinct operations that share a context:

```sh
git add          # git operates on a repository — subcommands make sense
git commit
git push

docker build     # docker operates on containers/images
docker run
docker ps
```

Don't use subcommands just to namespace flags. If you find yourself writing `myapp config --set foo=bar`, a subcommand structure may be artificial.

---

## Structure

### Top-level command
The root command should provide:
- A brief description of the tool
- A list of available subcommands
- How to get help (`--help`, `help <subcommand>`)

```
$ myapp
A tool for managing your application deployments.

Usage: myapp <command> [options]

Commands:
  deploy      Deploy your application to an environment
  rollback    Roll back to a previous deployment
  status      Show current deployment status
  logs        Stream application logs
  config      Manage configuration

Run 'myapp <command> --help' for command-specific help.
```

### Subcommand help
Each subcommand must have its own help text accessible via `--help`:

```sh
myapp deploy --help
myapp deploy -h
myapp help deploy
```

---

## Naming Subcommands

### Noun-first vs. verb-first: choose one and be consistent

Two common orderings exist. Pick one for your CLI and never mix them:

**Noun-first** (resource-centric — Thoughtworks, Heroku-style):
```sh
myapp apps create     # apps → create
myapp apps list       # apps → list
myapp config set KEY  # config → set
```
Groups all operations on a resource together in help output. Reads like "apps: create one."

**Verb-first** (action-centric — git-style):
```sh
myapp create app     # create → app
myapp list apps      # list → apps
myapp deploy         # verb only
```
Emphasizes what you're doing over what you're doing it to.

Noun-first tends to scale better for CLIs managing multiple resource types, because related commands naturally cluster together. Verb-first is more familiar in traditional UNIX tools.

### Be consistent within your tool
If you use `list` in one place, don't use `ls` in another. Pick one convention and stick to it.

### Common, conventional names
Follow these patterns where they apply:

| Command | Purpose |
|---------|---------|
| `create` / `new` | Create a resource |
| `delete` / `remove` / `rm` | Delete a resource |
| `list` / `ls` | List resources |
| `show` / `get` / `describe` | Show details of a resource |
| `update` / `edit` | Modify a resource |
| `start` / `stop` / `restart` | Change resource state |
| `init` | Initialize something new |
| `run` / `exec` | Execute something |

### Support aliases for common abbreviations
Common tools support abbreviations. Providing aliases reduces friction:

```sh
myapp ls       # alias for myapp list
myapp rm       # alias for myapp delete
myapp ps       # alias for myapp status
```

---

## Ordering in Help Text

List the most commonly-used subcommands first. Users scan help text top-to-bottom and should find the most useful commands without scrolling.

```
Commands:
  deploy      Deploy your application              ← used constantly
  status      Show deployment status               ← used constantly  
  logs        Stream application logs              ← used often
  rollback    Roll back to a previous deployment   ← used sometimes
  config      Manage configuration settings        ← used rarely
  completion  Install shell completion             ← used once
```

---

## Nested Subcommands

Use nesting sparingly. One level of nesting is usually enough. Two levels start to feel complex; three levels is almost always too deep.

```sh
# Good: one level
myapp config set API_KEY=abc123
myapp config get API_KEY
myapp config list

# Questionable: two levels
myapp project environment variable set KEY=VALUE

# Too deep: three levels
myapp project environment variable secret set KEY=VALUE
```

If you find yourself going deep, consider whether the levels are all necessary or if some can be collapsed.

---

## Global vs. Subcommand Flags

Some flags apply globally (to all subcommands); others apply only to specific subcommands. Be clear about which is which and be consistent.

```sh
# Global flags — apply to every subcommand
myapp --config ~/.myapp/config.yml deploy production
myapp --verbose list

# Subcommand-specific flags
myapp deploy production --strategy rolling
myapp logs --tail 100 --since 1h
```

Global flags typically go *before* the subcommand. Subcommand flags go *after*.

---

## Summary Checklist

- [ ] Subcommands used for logically distinct operations, not just namespacing
- [ ] Root command shows list of subcommands and how to get help
- [ ] Each subcommand has its own `--help` text
- [ ] Consistent naming convention within the tool (not mixing `list` and `ls`)
- [ ] Common aliases provided where useful
- [ ] Most-used subcommands listed first in help
- [ ] Nesting kept shallow (one level preferred, two levels max)
- [ ] Global vs. subcommand-specific flags clearly distinguished
