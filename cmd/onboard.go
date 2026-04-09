package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/CircleCI-Public/chunk-cli/envbuilder"
	"github.com/briandowns/spinner"
	"github.com/charmbracelet/lipgloss"
)

type onboardOptions struct {
	dir        string
	skipDocker bool
	skipConfig bool
	verbose    bool
}

const configTemplate = `version: 2.1

jobs:
  build-and-test:
    docker:
      - image: {{.Image}}:{{.ImageVersion}}
    steps:
      - checkout
      - run:
          name: Install dependencies
          command: {{.Install}}
      - run:
          name: Run tests
          command: {{.Test}}

workflows:
  main:
    jobs:
      - build-and-test
`

// Styles for the onboarding output.
var (
	stepStyle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#003740", Dark: "#3B6385"})
	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#161616", Dark: "#FFFFFF"})
	successStyle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#0B6623", Dark: "#4CAF50"})
	failStyle = lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#B00020", Dark: "#EF5350"})
	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#888888"})
)

func renderStep(s string) string    { return stepStyle.Render(s) }
func renderInfo(s string) string    { return infoStyle.Render(s) }
func renderSuccess(s string) string { return successStyle.Render(s) }
func renderFail(s string) string    { return failStyle.Render(s) }
func renderDim(s string) string     { return dimStyle.Render(s) }


func runOnboard(ctx context.Context, opts onboardOptions, w io.Writer) error {
	totalStart := time.Now()

	dir := opts.dir
	if dir == "" {
		dir = "."
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve directory: %w", err)
	}

	fmt.Fprintln(w)

	// Step 1: Detect environment
	s := newTimedSpinner(w, "Scanning repository...")
	s.start()

	env, err := envbuilder.DetectEnvironment(ctx, absDir)

	elapsed := s.stop()

	if err != nil {
		fmt.Fprintf(w, "  %s %s %s\n", renderFail("x"), "Scanning repository", renderDim(elapsed))
		return fmt.Errorf("failed to detect environment: %w", err)
	}

	if env.Stack == "unknown" {
		fmt.Fprintf(w, "  %s Scanning repository %s\n", renderStep("1."), renderDim(elapsed))
		fmt.Fprintf(w, "     %s\n", renderDim("Could not detect tech stack. A minimal config will be generated."))
		fmt.Fprintln(w)
	} else {
		pkgHint := detectPackageManager(env)
		fmt.Fprintf(w, "  %s Scanning repository %s\n", renderStep("1."), renderDim(elapsed))
		fmt.Fprintf(w, "     Detected: %s\n", renderInfo(fmt.Sprintf("%s (%s)", env.Stack, pkgHint)))
		fmt.Fprintf(w, "     Image:    %s\n", renderInfo(fmt.Sprintf("%s:%s", env.Image, env.ImageVersion)))
		fmt.Fprintln(w)
	}

	// Steps 2-3: Build & run tests in Docker
	testsPassed := false
	if !opts.skipDocker && env.Stack != "unknown" {
		passed, output, dockerErr := runDockerTests(ctx, absDir, env, w, opts.verbose)
		if dockerErr != nil {
			fmt.Fprintf(w, "     %s\n", renderDim(fmt.Sprintf("Skipped: %v", dockerErr)))
			fmt.Fprintln(w)
		} else if passed {
			testsPassed = true
			fmt.Fprintln(w)
		} else {
			fmt.Fprintln(w)
			if output != "" {
				printTail(w, output, 20)
			}
			fmt.Fprintf(w, "     Fix your tests and re-run %s.\n", renderInfo("circleci init"))
			fmt.Fprintln(w)
			return nil
		}
	}

	// Step 4: Generate .circleci/config.yml (only if tests passed or Docker was skipped)
	if !opts.skipConfig && (testsPassed || opts.skipDocker || env.Stack == "unknown") {
		if err := writeConfig(absDir, env, w); err != nil {
			return fmt.Errorf("failed to generate config: %w", err)
		}
	}

	// Next steps & suggestions
	printNextSteps(env, w)

	// Total time
	fmt.Fprintf(w, "  %s\n", renderDim(fmt.Sprintf("Total time: %s", formatDuration(time.Since(totalStart)))))
	fmt.Fprintln(w)

	return nil
}

func runDockerTests(ctx context.Context, dir string, env *envbuilder.Environment, w io.Writer, verbose bool) (passed bool, output string, err error) {
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		return false, "", fmt.Errorf("docker not found on PATH — install Docker or use --skip-docker")
	}

	// Write Dockerfile.test
	dockerfilePath, err := envbuilder.WriteDockerfile(dir, env)
	if err != nil {
		return false, "", fmt.Errorf("write Dockerfile: %w", err)
	}
	defer os.Remove(dockerfilePath)
	defer os.Remove(filepath.Join(dir, "Dockerfile.test.dockerignore"))

	imageName := "circleci-init-test"

	// Step 2: docker build
	var buildOutput bytes.Buffer
	buildCmd := exec.CommandContext(ctx, dockerPath, "build", "-f", "Dockerfile.test", "-t", imageName, ".")
	buildCmd.Dir = dir

	if verbose {
		fmt.Fprintf(w, "  %s Building test environment...\n", renderStep("2."))
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
	} else {
		buildCmd.Stdout = &buildOutput
		buildCmd.Stderr = &buildOutput
	}

	s := newTimedSpinner(w, "Building test environment...")
	if !verbose {
		s.start()
	}

	buildStart := time.Now()
	buildErr := buildCmd.Run()
	buildElapsed := formatDuration(time.Since(buildStart))

	if !verbose {
		s.stop()
	}

	if buildErr != nil {
		fmt.Fprintf(w, "  %s Building test environment... %s %s\n", renderStep("2."), renderFail("failed"), renderDim(buildElapsed))
		return false, buildOutput.String(), nil
	}

	if !verbose {
		fmt.Fprintf(w, "  %s Building test environment... %s %s\n", renderStep("2."), renderSuccess("done"), renderDim(buildElapsed))
	} else {
		fmt.Fprintf(w, "     %s %s\n", renderSuccess("done"), renderDim(buildElapsed))
	}

	// Step 3: docker run
	var testOutput bytes.Buffer
	runCmd := exec.CommandContext(ctx, dockerPath, "run", "--rm", imageName)

	if verbose {
		fmt.Fprintf(w, "  %s Running tests...\n", renderStep("3."))
		runCmd.Stdout = os.Stdout
		runCmd.Stderr = os.Stderr
	} else {
		runCmd.Stdout = &testOutput
		runCmd.Stderr = &testOutput
	}

	s2 := newTimedSpinner(w, "Running tests...")
	if !verbose {
		s2.start()
	}

	testStart := time.Now()
	runErr := runCmd.Run()
	testElapsed := formatDuration(time.Since(testStart))

	if !verbose {
		s2.stop()
	}

	if runErr != nil {
		fmt.Fprintf(w, "  %s Running tests... %s %s\n", renderStep("3."), renderFail("failed"), renderDim(testElapsed))
		if _, ok := runErr.(*exec.ExitError); ok {
			return false, testOutput.String(), nil
		}
		return false, testOutput.String(), fmt.Errorf("docker run failed: %w", runErr)
	}

	if !verbose {
		fmt.Fprintf(w, "  %s Running tests... %s %s\n", renderStep("3."), renderSuccess("passed"), renderDim(testElapsed))
	} else {
		fmt.Fprintf(w, "     %s %s\n", renderSuccess("passed"), renderDim(testElapsed))
	}
	return true, "", nil
}

func generateOnboardConfig(env *envbuilder.Environment) (string, error) {
	data := struct {
		Image        string
		ImageVersion string
		Install      string
		Test         string
	}{
		Image:        env.Image,
		ImageVersion: env.ImageVersion,
		Install:      env.Install,
		Test:         env.Test,
	}

	if env.Stack == "unknown" {
		data.Image = "cimg/base"
		data.ImageVersion = "current"
		data.Install = "echo 'TODO: add install command'"
		data.Test = "echo 'TODO: add test command'"
	}

	tmpl, err := template.New("config").Parse(configTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

func writeConfig(dir string, env *envbuilder.Environment, w io.Writer) error {
	configDir := filepath.Join(dir, ".circleci")
	configPath := filepath.Join(configDir, "config.yml")

	content, err := generateOnboardConfig(env)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create .circleci directory: %w", err)
	}

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	stepNum := "4."
	fmt.Fprintf(w, "  %s Generated %s\n", renderStep(stepNum), renderInfo(".circleci/config.yml"))
	fmt.Fprintln(w)

	return nil
}

func printNextSteps(env *envbuilder.Environment, w io.Writer) {
	fmt.Fprintf(w, "  %s\n", renderStep("Next steps:"))
	fmt.Fprintln(w, "    - Run `circleci signup` to create your CircleCI account")
	fmt.Fprintln(w, "    - Run `circleci init` again to push your config and trigger your first build")
	fmt.Fprintln(w)

	suggestions := []string{
		"Enable dependency caching to speed up builds",
	}

	switch env.Stack {
	case "go":
		suggestions = append(suggestions, "Add golangci-lint for Go linting")
	case "python":
		suggestions = append(suggestions, "Add ruff or flake8 for Python linting")
	case "javascript", "typescript":
		suggestions = append(suggestions, "Add eslint for JavaScript/TypeScript linting")
	case "java":
		suggestions = append(suggestions, "Add checkstyle or spotbugs for code quality")
	case "rust":
		suggestions = append(suggestions, "Add clippy for Rust linting")
	case "ruby":
		suggestions = append(suggestions, "Add rubocop for Ruby linting")
	case "php":
		suggestions = append(suggestions, "Add phpstan for static analysis")
	case "elixir":
		suggestions = append(suggestions, "Add credo for Elixir linting")
	}

	suggestions = append(suggestions, "Add parallelism to distribute tests across containers")

	fmt.Fprintf(w, "  %s\n", renderDim("Suggestions:"))
	for _, s := range suggestions {
		fmt.Fprintf(w, "    %s %s\n", renderDim("-"), renderDim(s))
	}
	fmt.Fprintln(w)
}

// detectPackageManager extracts a short package manager hint from the environment.
func detectPackageManager(env *envbuilder.Environment) string {
	install := env.Install
	switch {
	case strings.HasPrefix(install, "pnpm"):
		return "pnpm"
	case strings.HasPrefix(install, "yarn"):
		return "yarn"
	case strings.HasPrefix(install, "npm"):
		return "npm"
	case strings.HasPrefix(install, "pip"):
		return "pip"
	case strings.HasPrefix(install, "uv"):
		return "uv"
	case strings.HasPrefix(install, "pipenv"):
		return "pipenv"
	case strings.HasPrefix(install, "go mod"):
		return "go modules"
	case strings.HasPrefix(install, "cargo"):
		return "cargo"
	case strings.HasPrefix(install, "bundle"):
		return "bundler"
	case strings.HasPrefix(install, "composer"):
		return "composer"
	case strings.HasPrefix(install, "mix"):
		return "mix"
	case strings.Contains(install, "gradlew"), strings.Contains(install, "gradle"):
		return "gradle"
	case strings.Contains(install, "mvn"):
		return "maven"
	default:
		return env.Stack
	}
}

// timedSpinner wraps a spinner with a live elapsed-time counter.
type timedSpinner struct {
	s      *spinner.Spinner
	w      io.Writer
	label  string
	start_ time.Time
	done   chan struct{}
}

func newTimedSpinner(w io.Writer, label string) *timedSpinner {
	return &timedSpinner{
		s:     spinner.New(spinner.CharSets[14], 100*time.Millisecond, spinner.WithWriter(w)),
		w:     w,
		label: label,
		done:  make(chan struct{}),
	}
}

func (ts *timedSpinner) start() {
	ts.start_ = time.Now()
	ts.s.Suffix = fmt.Sprintf(" %s %s", ts.label, renderDim("0s"))
	ts.s.Start()

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ts.done:
				return
			case <-ticker.C:
				ts.s.Suffix = fmt.Sprintf(" %s %s", ts.label, renderDim(formatDuration(time.Since(ts.start_))))
			}
		}
	}()
}

func (ts *timedSpinner) stop() string {
	close(ts.done)
	ts.s.Stop()
	return formatDuration(time.Since(ts.start_))
}

// formatDuration formats a duration as a human-friendly string (e.g. "4s", "1m 15s", "2m 3s").
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// printTail prints the last n lines of output, indented.
func printTail(w io.Writer, output string, n int) {
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	start := 0
	if len(lines) > n {
		start = len(lines) - n
		fmt.Fprintf(w, "     %s\n", renderDim(fmt.Sprintf("... (%d lines omitted)", start)))
	}
	for _, line := range lines[start:] {
		fmt.Fprintf(w, "     %s\n", renderDim(line))
	}
	fmt.Fprintln(w)
}
