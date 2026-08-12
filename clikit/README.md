# clikit

The CircleCI CLI's CLI kit, published as its own Go module so extensions can look
and behave exactly like the CLI they extend.

```sh
go get github.com/CircleCI-Public/circleci-cli/clikit
```

## No CLI framework required

`iostream.New` takes a plain struct, agnostic to any CLI framework:

```go
// cobra
ctx := iostream.New(cmd.Context(), iostream.Options{
	In:    cmd.InOrStdin(),
	Out:   cmd.OutOrStdout(),
	Err:   cmd.ErrOrStderr(),
	Quiet: quiet,
	Debug: debug,
	Theme: theme,
})

// kong
ctx := iostream.New(context.Background(), iostream.Options{
	In:    os.Stdin,
	Out:   os.Stdout,
	Err:   os.Stderr,
	Quiet: cli.Quiet,
	Debug: cli.Debug,
	Theme: cli.Theme,
})
```

## Packages

| Package         | What it gives you                                                                                                                                                        |
|-----------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `iostream`      | `Streams` — the one thing every command should write through. TTY/color/interactivity detection, theme resolution, spinner, pager, prompts, `PrintJSON`, `PrintMarkdown` |
| `errors`        | `CLIError` (code, title, message, suggestions, ref) and the CLI's exit code constants                                                                                    |
| `ui`            | Bubble Tea models behind the `iostream` prompts: text, select, confirm, secret, spinner, markdown viewport, theme picker                                                 |
| `ui/components` | Reusable widgets: select list, pager, tabs, help, link, preview pane, token input, window titles                                                                         |
| `ui/theme`      | Colour tokens, icons and lipgloss styles — the CLI's palette                                                                                                             |
| `mdtable`       | GitHub-Flavored Markdown table builder. Render it through `iostream.PrintMarkdown` for the CLI's table look                                                              |
| `jq`            | `--jq` expression evaluation over JSON                                                                                                                                   |
| `jsoncolor`     | ANSI-colorised, indented JSON writer                                                                                                                                     |
| `browser`       | Open a URL, or print it when there is no browser                                                                                                                         |
| `closer`        | `io.Closer` error handling for deferred `Close()`                                                                                                                        |
