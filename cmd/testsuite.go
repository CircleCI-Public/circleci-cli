package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/CircleCI-Public/circleci-cli/prompt"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type Testsuite struct {
	Name             string           `yaml:"name"`
	Runner           string           `yaml:"runner,omitempty"`
	Discover         string           `yaml:"discover,omitempty"`
	Run              string           `yaml:"run,omitempty"`
	Analysis         string           `yaml:"analysis,omitempty"`
	AnalysisBaseline string           `yaml:"analysis-baseline,omitempty"`
	FileMapper       string           `yaml:"file-mapper,omitempty"`
	Outputs          TestSuiteOutputs `yaml:"outputs,omitempty"`
	Options          TestSuiteOptions `yaml:"options,omitempty"`
}

type TestSuiteOutputs struct {
	Junit      string `yaml:"junit,omitempty"`
	GoCoverage string `yaml:"go-coverage,omitempty"`
	LCov       string `yaml:"lcov,omitempty"`
}

type TestSuiteOptions struct {
	// Timeout in minutes for tests to become available when running in parallel
	Timeout int `yaml:"timeout,omitempty"`
	// If true we will perform test selection on non-default branches
	AdaptiveTesting bool `yaml:"adaptive-testing,omitempty"`
	// FullTestRunPaths are paths that might have an indirect impact on tests
	// and should run the full test suite if a change is detected.
	// Each file supports the filepath.Match syntax.
	FullTestRunPaths     []string `yaml:"full-test-run-paths,omitempty"`
	TestAnalysisDuration int      `yaml:"test-analysis-duration,omitempty"`
	DynamicBatching      *bool    `yaml:"dynamic-batching,omitempty"`
	ImpactKey            string   `yaml:"impact-key,omitempty"`
}

type Selection string

const (
	// SelectionAll selects all discovered tests.
	SelectionAll Selection = "all"
	// SelectionImpacted selects only the tests impacted by changes.
	SelectionImpacted Selection = "impacted"
	// SelectionNone selects no tests.
	SelectionNone Selection = "none"
)

// TestFramework represents a supported test framework
type TestFramework struct {
	Name              string
	DiscoverCommand   string
	RunCommand        string
	AnalysisCommand   string
	FileMapperCommand string
	JunitPath         string
	CoverageFormat    string // "lcov" or "go-coverage"
	CoveragePath      string
	Description       string
}

var testFrameworks = []TestFramework{
	{
		Name:            "Jest",
		DiscoverCommand: "jest --listTests",
		RunCommand:      `JEST_JUNIT_OUTPUT_FILE="<< outputs.junit >>" jest --runInBand --reporters=jest-junit --bail << test.atoms >>`,
		AnalysisCommand: `jest --runInBand --silent --coverage --coverageProvider=v8 --coverageReporters=lcovonly --coverage-directory="$(dirname << outputs.lcov >>)" --bail << test.atoms >> && cat "$(dirname << outputs.lcov >>)"/*.info > << outputs.lcov >>`,
		JunitPath:       "test-reports/junit.xml",
		CoverageFormat:  "lcov",
		CoveragePath:    "test-reports/lcov.info",
		Description:     "JavaScript testing framework",
	},
	{
		Name:            "Yarn Jest",
		DiscoverCommand: "yarn --silent test --listTests",
		RunCommand:      `JEST_JUNIT_OUTPUT_FILE="<< outputs.junit >>" yarn test --runInBand --reporters=jest-junit --bail << test.atoms >>`,
		AnalysisCommand: `yarn test --runInBand --coverage --coverageProvider=v8 --coverageReporters=lcovonly --coverage-directory="$(dirname << outputs.lcov >>)" --bail << test.atoms >> && cat "$(dirname << outputs.lcov >>)"/*.info > << outputs.lcov >>`,
		JunitPath:       "test-reports/junit.xml",
		CoverageFormat:  "lcov",
		CoveragePath:    "test-reports/lcov.info",
		Description:     "Jest with Yarn package manager",
	},
	{
		Name:            "Vitest",
		DiscoverCommand: "vitest list --filesOnly",
		RunCommand:      `vitest run --reporter=junit --outputFile="<< outputs.junit >>" --bail 0 << test.atoms >>`,
		AnalysisCommand: `vitest run --coverage.enabled --coverage.all=false --coverage.reporter=lcov --coverage.provider=v8 --coverage.reportsDirectory="$(dirname << outputs.lcov >>)" --silent --bail 0 << test.atoms >> && cat "$(dirname << outputs.lcov >>)"/*.info > << outputs.lcov >>`,
		JunitPath:       "test-reports/junit.xml",
		CoverageFormat:  "lcov",
		CoveragePath:    "test-reports/lcov.info",
		Description:     "Fast Vite-native testing framework",
	},
	{
		Name:            "pytest",
		DiscoverCommand: `pytest --collect-only -qq | sed 's/:.*//' | sort -u`,
		RunCommand:      `pytest --disable-pytest-warnings --no-header --quiet --tb=short --junit-xml="<< outputs.junit >>" << test.atoms >>`,
		AnalysisCommand: `pytest --disable-pytest-warnings --no-header --quiet --tb=short --cov --cov-report=lcov:<< outputs.lcov >> << test.atoms >>`,
		JunitPath:       "test-reports/junit.xml",
		CoverageFormat:  "lcov",
		CoveragePath:    "test-reports/lcov.info",
		Description:     "Python testing framework",
	},
	{
		Name:              "Go test",
		DiscoverCommand:   `go list -f '{{ if or (len .TestGoFiles) (len .XTestGoFiles) }} {{ .ImportPath }} {{end}}' ./...`,
		RunCommand:        `go test -race -count=1 << test.atoms >>`,
		AnalysisCommand:   `go test -coverprofile="<< outputs.go-coverage >>" -cover -coverpkg ./... << test.atoms >>`,
		FileMapperCommand: `go list -f '{{range .TestGoFiles}}{{$.Dir}}/{{.}}{{"\n"}}{{end}}{{range .XTestGoFiles}}{{$.Dir}}/{{.}}{{"\n"}}{{end}}' << test.atoms >>`,
		JunitPath:         "test-reports/junit.xml",
		CoverageFormat:    "go-coverage",
		CoveragePath:      "test-reports/go-coverage.txt",
		Description:       "Go standard testing",
	},
	{
		Name:              "gotestsum",
		DiscoverCommand:   `go list -f '{{ if or (len .TestGoFiles) (len .XTestGoFiles) }} {{ .ImportPath }} {{end}}' ./...`,
		RunCommand:        `go tool gotestsum --junitfile="<< outputs.junit >>" -- -race -count=1 << test.atoms >>`,
		AnalysisCommand:   `go tool gotestsum -- -coverprofile="<< outputs.go-coverage >>" -cover -coverpkg ./... << test.atoms >>`,
		FileMapperCommand: `go list -f '{{range .TestGoFiles}}{{$.Dir}}/{{.}}{{"\n"}}{{end}}{{range .XTestGoFiles}}{{$.Dir}}/{{.}}{{"\n"}}{{end}}' << test.atoms >>`,
		JunitPath:         "test-reports/junit.xml",
		CoverageFormat:    "go-coverage",
		CoveragePath:      "test-reports/go-coverage.txt",
		Description:       "Go test runner with enhanced output",
	},
	{
		Name:        "Custom",
		Description: "Configure commands manually",
	},
}

func newTestsuiteCommand() *cobra.Command {
	testsuiteCmd := &cobra.Command{
		Use:   "testsuite",
		Short: "Operate on testsuites",
		Long: `Configure and manage test suites for Smarter Testing.

Smarter Testing optimizes test runs by:
- Running only tests impacted by code changes
- Evenly distributing tests across parallel execution nodes`,
	}

	initsuiteCmd := &cobra.Command{
		Short: "Initialize a testsuite configuration",
		Use:   "init",
		Long: `Interactive setup wizard for Smarter Testing.

This command will guide you through:
1. Test suite naming
2. Test framework selection
3. Output path configuration
4. Test discovery (with automatic validation)
5. Test execution (with automatic validation)
6. (Optional) Test impact analysis with coverage
7. Additional options and settings

All commands are automatically validated before proceeding to ensure they work correctly.
The configuration will be saved to .circleci/test-suites.yml`,
		RunE: testsuiteInitRunE,
	}

	testsuiteCmd.AddCommand(initsuiteCmd)

	return testsuiteCmd
}

func testsuiteInitRunE(cmd *cobra.Command, args []string) error {
	// Welcome message
	fmt.Println("\n=== CircleCI Smarter Testing Setup ===")
	fmt.Println("Welcome! This wizard will help you configure Smarter Testing for your project.")
	fmt.Println("\nSmarter Testing features:")
	fmt.Println("  • Run only tests impacted by code changes")
	fmt.Println("  • Evenly distribute tests across parallel execution nodes")
	fmt.Println("  • Faster CI/CD pipelines with intelligent test selection")

	testsuiteFile := ".circleci/test-suites.yml"

	// Step 1: Get test suite name
	fmt.Println("Step 1: Test Suite Name")
	fmt.Println("────────────────────────")
	var name string
	for name == "" {
		name = prompt.ReadStringFromUser("Enter a name for your test suite", "ci tests")
		if name == "" {
			fmt.Println("⚠ Name is required")
		}
	}

	// Check if test suite already exists
	if exists, err := testsuiteExistsInFile(testsuiteFile, name); err != nil && !os.IsNotExist(err) {
		return err
	} else if exists {
		if !prompt.AskUserToConfirm(fmt.Sprintf("Test suite '%s' already exists. Overwrite?", name)) {
			return fmt.Errorf("test suite '%s' already exists", name)
		}
	}

	// Step 2: Select test framework
	fmt.Println("\nStep 2: Test Framework")
	fmt.Println("──────────────────────")
	framework, err := selectTestFramework()
	if err != nil {
		return err
	}

	testsuite := &Testsuite{
		Name:    name,
		Outputs: TestSuiteOutputs{},
		Options: TestSuiteOptions{},
	}

	// Step 3: Configure output paths first (needed for validation)
	fmt.Println("\nStep 3: JUnit Output Path")
	fmt.Println("─────────────────────────")
	fmt.Println("Specify where JUnit XML test results should be saved.")
	junitPath := prompt.ReadStringFromUser("JUnit output path", framework.JunitPath)
	if junitPath == "" {
		junitPath = "test-reports/junit.xml"
	}
	testsuite.Outputs.Junit = junitPath

	// Step 4: Configure discover command
	fmt.Println("\nStep 4: Test Discovery")
	fmt.Println("──────────────────────")
	fmt.Println("The discover command finds all test atoms (runnable test units) in your project.")
	if framework.Name != "Custom" {
		fmt.Printf("Recommended command for %s:\n  %s\n\n", framework.Name, framework.DiscoverCommand)
	}

	var discoveredTests []string
	testsuite.Discover, discoveredTests, err = configureCommand("discover", framework.DiscoverCommand,
		"This command should output a list of test atoms (one per line or space-separated)", testsuite.Outputs, nil)
	if err != nil {
		return err
	}

	// Step 5: Configure run command
	fmt.Println("\nStep 5: Test Execution")
	fmt.Println("──────────────────────")
	fmt.Println("The run command executes the discovered tests.")
	fmt.Println("Use << test.atoms >> for the test list and << outputs.junit >> for the output file.")
	if framework.Name != "Custom" {
		fmt.Printf("\nRecommended command for %s:\n  %s\n\n", framework.Name, framework.RunCommand)
	}

	testsuite.Run, discoveredTests, err = configureCommand("run", framework.RunCommand,
		"This command should run the specified tests and output JUnit XML results", testsuite.Outputs, discoveredTests)
	if err != nil {
		return err
	}

	// Step 6: Enable Smarter Testing (Test Impact Analysis)
	fmt.Println("\nStep 6: Enable Smarter Testing (Optional)")
	fmt.Println("──────────────────────────────────────────")
	fmt.Println("Test Impact Analysis runs only tests affected by your code changes.")
	fmt.Println("This requires:")
	fmt.Println("  • An analysis command that runs tests with code coverage")
	fmt.Println("  • Initial analysis run on your main branch (can be slower)")
	fmt.Println("  • Subsequent runs on feature branches will be much faster")

	enableAdaptiveTesting := prompt.AskUserToConfirm("Enable Test Impact Analysis?")

	if enableAdaptiveTesting {
		testsuite.Options.AdaptiveTesting = true

		// Configure coverage output path first (needed for validation)
		fmt.Println("\nCoverage Output Path")
		fmt.Println("────────────────────")

		coverageFormat := framework.CoverageFormat
		if framework.Name == "Custom" || coverageFormat == "" {
			coveragePrompt := &survey.Select{
				Message: "Select coverage format:",
				Options: []string{"lcov", "go-coverage"},
			}
			if err := survey.AskOne(coveragePrompt, &coverageFormat); err != nil {
				return err
			}
		}

		coveragePath := prompt.ReadStringFromUser(
			fmt.Sprintf("Coverage output path (%s format)", coverageFormat),
			framework.CoveragePath,
		)

		if coverageFormat == "lcov" {
			testsuite.Outputs.LCov = coveragePath
		} else {
			testsuite.Outputs.GoCoverage = coveragePath
		}

		// Configure analysis command
		fmt.Println("\nConfiguring Test Impact Analysis")
		fmt.Println("─────────────────────────────────")
		fmt.Println("The analysis command runs tests individually with code coverage.")
		fmt.Println("Use << test.atoms >> for the test and << outputs.lcov >> or << outputs.go-coverage >> for coverage output.")

		if framework.Name != "Custom" {
			fmt.Printf("\nRecommended command for %s:\n  %s\n\n", framework.Name, framework.AnalysisCommand)
		}

		testsuite.Analysis, _, err = configureCommand("analysis", framework.AnalysisCommand,
			"This command should run a single test with coverage instrumentation enabled", testsuite.Outputs, discoveredTests)
		if err != nil {
			return err
		}

		// Configure file-mapper (when test atoms are not file names)
		fmt.Println("\nFile Mapper Configuration (Optional)")
		fmt.Println("─────────────────────────────────────")
		fmt.Println("If your test atoms are NOT file names (e.g., Go packages, Java classes),")
		fmt.Println("a file-mapper command is needed to map each test atom to its source file.")
		fmt.Println("This is used during analysis and test selection.")

		if framework.FileMapperCommand != "" {
			fmt.Printf("\nRecommended command for %s:\n  %s\n\n", framework.Name, framework.FileMapperCommand)

			if prompt.AskUserToConfirmWithDefault("Configure file-mapper for this framework?", true) {
				if prompt.AskUserToConfirmWithDefault("Use recommended file-mapper command?", true) {
					testsuite.FileMapper = framework.FileMapperCommand
					fmt.Println("✓ Using recommended file-mapper command")
				} else {
					customMapper := prompt.ReadStringFromUser("Enter custom file-mapper command", framework.FileMapperCommand)
					if customMapper != "" {
						testsuite.FileMapper = customMapper
						fmt.Println("✓ Custom file-mapper command configured")
					}
				}
			}
		} else {
			fmt.Println("\nNo default file-mapper for this framework (usually not needed).")
			if prompt.AskUserToConfirm("Configure a custom file-mapper?") {
				customMapper := prompt.ReadStringFromUser("Enter file-mapper command", "")
				if customMapper != "" {
					testsuite.FileMapper = customMapper
					fmt.Println("✓ Custom file-mapper command configured")
				}
			}
		}

		// Configure analysis baseline (advanced/optional)
		fmt.Println("\nAnalysis Baseline (Advanced - Optional)")
		fmt.Println("────────────────────────────────────────")
		fmt.Println("If tests show many files impacting them (e.g., 150+ files), this may be caused")
		fmt.Println("by shared setup code. An analysis-baseline command can filter out this noise.")
		fmt.Println("This requires creating a minimal no-op test that only does imports/setup.")

		if prompt.AskUserToConfirm("Configure analysis-baseline? (skip if unsure)") {
			fmt.Println("\nCreate a minimal test file (e.g., src/baseline/noop.test.ts) that only")
			fmt.Println("imports frameworks but doesn't run any test logic, then configure the command.")

			baselineCmd := prompt.ReadStringFromUser("Analysis baseline command", "")
			if baselineCmd != "" {
				testsuite.AnalysisBaseline = baselineCmd
				fmt.Println("✓ Analysis baseline command configured")
			}
		}

		// Configure analysis duration
		fmt.Println("\nAnalysis Duration Limit (Optional)")
		fmt.Println("──────────────────────────────────")
		fmt.Println("Set a time limit (in minutes) for test analysis to prevent long-running jobs.")
		fmt.Println("Analysis can continue in subsequent runs if not completed.")

		if prompt.AskUserToConfirm("Set an analysis duration limit?") {
			durationStr := prompt.ReadStringFromUser("Duration in minutes", "15")
			if duration, err := strconv.Atoi(durationStr); err == nil && duration > 0 {
				testsuite.Options.TestAnalysisDuration = duration
			}
		}

		// Configure impact key for multiple test suites
		fmt.Println("\nImpact Key (Optional)")
		fmt.Println("─────────────────────")
		fmt.Println("If you have multiple test suites in one repository, set a unique impact key.")
		fmt.Println("This groups impact analysis data separately for each test suite.")

		if prompt.AskUserToConfirm("Set a custom impact key?") {
			impactKey := prompt.ReadStringFromUser("Impact key", "default")
			if impactKey != "" && impactKey != "default" {
				testsuite.Options.ImpactKey = impactKey
			}
		}
	}

	// Step 7: Additional options
	fmt.Println("\nStep 7: Additional Options")
	fmt.Println("──────────────────────────")

	// Dynamic batching
	fmt.Println("\nDynamic Test Splitting distributes tests across parallel nodes using a shared queue.")
	fmt.Println("Disable this if your test runner has slow startup time.")
	if !prompt.AskUserToConfirmWithDefault("Enable dynamic test splitting?", true) {
		disabled := false
		testsuite.Options.DynamicBatching = &disabled
	}

	// Full test run paths
	fmt.Println("\nFull Test Run Paths")
	fmt.Println("───────────────────")
	fmt.Println("Files that trigger a full test run when modified (e.g., package.json, go.mod).")
	fmt.Println("Default paths: .circleci/*.yml, go.mod, go.sum, package*.json, yarn.lock, project.clj")

	if prompt.AskUserToConfirm("Use default full test run paths?") {
		testsuite.Options.FullTestRunPaths = []string{
			".circleci/*.yml",
			"go.mod",
			"go.sum",
			"package-lock.json",
			"package.json",
			"project.clj",
			"yarn.lock",
		}
	} else if prompt.AskUserToConfirm("Add custom paths?") {
		fmt.Println("Enter paths one at a time (empty to finish):")
		paths := []string{}
		for {
			path := prompt.ReadStringFromUser("Path (or empty to finish)", "")
			if path == "" {
				break
			}
			paths = append(paths, path)
		}
		testsuite.Options.FullTestRunPaths = paths
	}

	// Save configuration
	fmt.Println("\nSaving Configuration")
	fmt.Println("────────────────────")

	if err := saveTestsuite(testsuiteFile, testsuite); err != nil {
		return err
	}

	fmt.Printf("✓ Configuration saved to %s\n", testsuiteFile)

	// Offer to test the configuration
	fmt.Println("\n" + strings.Repeat("─", 60))
	fmt.Println("🧪 Test Your Configuration")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("Would you like to test your configuration now?")
	fmt.Println("This will run your test suite locally with the configured commands.")

	if prompt.AskUserToConfirmWithDefault("Test configuration now?", true) {
		fmt.Println("\n▶ Running test suite...")
		fmt.Printf("Command: circleci run testsuite \"%s\" --local --test-analysis=impacted --test-selection=all\n\n", name)

		testCmd := exec.Command("circleci", "run", "testsuite", name, "--local", "--test-analysis=impacted", "--test-selection=all")
		testCmd.Stdout = os.Stdout
		testCmd.Stderr = os.Stderr
		testCmd.Stdin = os.Stdin

		if err := testCmd.Run(); err != nil {
			fmt.Printf("\n⚠ Test run encountered an error: %v\n", err)
			fmt.Println("This is normal if your project setup differs from CI.")
			fmt.Println("Review the output above to identify any issues.")
		} else {
			fmt.Println("\n✅ Test run completed successfully!")

			// Check if impact.json was created
			impactFilePath := ".circleci/impact.json"
			if _, err := os.Stat(impactFilePath); err == nil {
				fmt.Printf("✓ Test impact data created: %s\n", impactFilePath)
				fmt.Println("  This file contains the mapping between tests and code they cover.")
			} else if enableAdaptiveTesting {
				fmt.Printf("⚠ Test impact data not found: %s\n", impactFilePath)
				fmt.Println("  This file should be created when analysis runs successfully.")
				fmt.Println("  Check the output above for any analysis errors.")
			}
		}
	}

	// Show next steps
	printNextSteps(name, enableAdaptiveTesting)

	return nil
}

// selectTestFramework prompts the user to select a test framework
func selectTestFramework() (TestFramework, error) {
	options := make([]string, len(testFrameworks))
	for i, fw := range testFrameworks {
		if fw.Description != "" {
			options[i] = fmt.Sprintf("%s - %s", fw.Name, fw.Description)
		} else {
			options[i] = fw.Name
		}
	}

	frameworkPrompt := &survey.Select{
		Message: "Select your test framework:",
		Options: options,
	}

	var selected string
	if err := survey.AskOne(frameworkPrompt, &selected); err != nil {
		return TestFramework{}, err
	}

	// Extract framework name from selection
	selectedName := strings.Split(selected, " - ")[0]
	for _, fw := range testFrameworks {
		if fw.Name == selectedName {
			return fw, nil
		}
	}

	return testFrameworks[len(testFrameworks)-1], nil // Return Custom as fallback
}

// configureCommand prompts for and validates a command with automatic execution
func configureCommand(commandName, defaultCmd, description string, outputs TestSuiteOutputs, discoveredTests []string) (string, []string, error) {
	fmt.Printf("\n%s\n", description)

	var lastFailedCommand string
	firstAttempt := true

	for {
		var command string

		// If we have a failed command, offer to edit it
		if lastFailedCommand != "" {
			fmt.Println("\n" + strings.Repeat("─", 60))
			fmt.Printf("Failed command:\n  %s\n\n", lastFailedCommand)

			editChoice := &survey.Select{
				Message: "What would you like to do?",
				Options: []string{
					"Edit the command and retry",
					"Start over with recommended command",
					"Enter a completely new command",
				},
			}

			var choice string
			if err := survey.AskOne(editChoice, &choice); err != nil {
				return "", nil, err
			}

			switch choice {
			case "Edit the command and retry":
				fmt.Println("\nEdit your command:")
				command = prompt.ReadStringFromUser("Command", lastFailedCommand)
			case "Start over with recommended command":
				if defaultCmd != "" {
					command = defaultCmd
					fmt.Printf("Using recommended command:\n  %s\n", command)
				} else {
					fmt.Println("No recommended command available. Please enter a new command.")
					command = prompt.ReadStringFromUser("Command", "")
				}
			case "Enter a completely new command":
				command = prompt.ReadStringFromUser("Command", "")
			}
		} else {
			// First attempt - ask if they want to use recommended command
			useDefault := false
			if defaultCmd != "" && firstAttempt {
				useDefault = prompt.AskUserToConfirmWithDefault("Use recommended command?", true)
			}

			if useDefault {
				command = defaultCmd
			} else {
				command = prompt.ReadStringFromUser("Enter command", defaultCmd)
			}

			firstAttempt = false
		}

		if command == "" {
			fmt.Printf("⚠ %s command is required\n", commandName)
			lastFailedCommand = ""
			continue
		}

		// Automatically validate the command
		fmt.Printf("\n🔍 Validating %s command...\n", commandName)

		var err error
		var newDiscoveredTests []string
		switch commandName {
		case "discover":
			newDiscoveredTests, err = testDiscoverCommand(command)
			if err == nil {
				discoveredTests = newDiscoveredTests
			}
		case "run":
			err = testRunCommand(command, outputs, discoveredTests)
		case "analysis":
			err = testAnalysisCommand(command, outputs, discoveredTests)
		default:
			// Unknown command type, skip validation
			return command, discoveredTests, nil
		}

		if err != nil {
			fmt.Printf("\n❌ Command validation failed:\n")
			fmt.Printf("   %v\n\n", err)
			fmt.Println("💡 Tips:")
			printCommandTroubleshootingTips(commandName)

			// Save the failed command for editing
			lastFailedCommand = command
			continue
		}

		fmt.Printf("✅ Command validated successfully!\n")
		return command, discoveredTests, nil
	}
}

// testDiscoverCommand runs the discover command to validate it works
func testDiscoverCommand(command string) ([]string, error) {
	fmt.Println("   Executing discover command...")

	// Remove template variables for testing
	testCmd := strings.ReplaceAll(command, "<< outputs.junit >>", "test-output.xml")
	testCmd = strings.ReplaceAll(testCmd, "<< outputs.lcov >>", "test-output.lcov")
	testCmd = strings.ReplaceAll(testCmd, "<< outputs.go-coverage >>", "test-output.txt")
	testCmd = strings.ReplaceAll(testCmd, "<< test.atoms >>", "")

	cmd := exec.Command("sh", "-c", testCmd)
	output, err := cmd.CombinedOutput()

	if err != nil {
		outputStr := string(output)
		if len(outputStr) > 500 {
			outputStr = outputStr[:500] + "...\n(output truncated)"
		}
		return nil, fmt.Errorf("command execution failed (exit code %d)\n   Command: %s\n   Output:\n%s",
			cmd.ProcessState.ExitCode(), testCmd, outputStr)
	}

	// Check if we got some output
	outputStr := strings.TrimSpace(string(output))
	if len(outputStr) == 0 {
		return nil, fmt.Errorf("command produced no output - expected a list of test atoms")
	}

	// Parse discovered tests (both space-separated and newline-separated)
	lines := strings.Split(outputStr, "\n")
	var tests []string
	for _, line := range lines {
		lineTests := strings.Fields(line)
		tests = append(tests, lineTests...)
	}

	if len(tests) == 0 {
		return nil, fmt.Errorf("no test atoms found in output")
	}

	fmt.Printf("   ✓ Found %d test atom(s) across %d line(s)\n", len(tests), len(lines))

	// Show first few tests as sample
	sampleCount := 3
	if len(tests) < sampleCount {
		sampleCount = len(tests)
	}
	fmt.Printf("   Sample tests: %v\n", tests[:sampleCount])

	return tests, nil
}

// testRunCommand validates the run command with a sample test
func testRunCommand(command string, _ TestSuiteOutputs, discoveredTests []string) error {
	fmt.Println("   Executing run command with discovered test...")

	// Use a random discovered test
	var sampleTest string
	if len(discoveredTests) > 0 {
		// Pick a random test
		sampleTest = discoveredTests[len(discoveredTests)/2] // Use middle test for variety
		fmt.Printf("   Using test: %s\n", sampleTest)
	} else {
		return fmt.Errorf("no discovered tests available for validation")
	}

	// Replace template variables
	testCmd := command
	testCmd = strings.ReplaceAll(testCmd, "<< test.atoms >>", sampleTest)

	// Create temp directory for outputs
	tmpDir, err := os.MkdirTemp("", "circleci-testsuite-validation-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	junitPath := tmpDir + "/junit.xml"
	testCmd = strings.ReplaceAll(testCmd, "<< outputs.junit >>", junitPath)
	testCmd = strings.ReplaceAll(testCmd, "<< outputs.lcov >>", tmpDir+"/lcov.info")
	testCmd = strings.ReplaceAll(testCmd, "<< outputs.go-coverage >>", tmpDir+"/coverage.txt")

	cmd := exec.Command("sh", "-c", testCmd)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Check if we got output
	if len(outputStr) == 0 {
		return fmt.Errorf("command produced no output - may not be configured correctly")
	}

	// Check for common error patterns that indicate misconfiguration
	lowerOutput := strings.ToLower(outputStr)
	if strings.Contains(lowerOutput, "command not found") {
		return fmt.Errorf("test runner not found - ensure the test framework is installed\n   Output: %s",
			truncateOutput(outputStr, 300))
	}

	// Check if the test passed (exit code 0)
	if err != nil {
		exitCode := -1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		return fmt.Errorf("test execution failed (exit code %d)\n   The test must pass for validation.\n   Output:\n%s",
			exitCode, truncateOutput(outputStr, 500))
	}

	// Check if JUnit output was created
	if _, err := os.Stat(junitPath); err != nil {
		fmt.Printf("   ⚠ Warning: JUnit output file not created at %s\n", junitPath)
		fmt.Printf("   The command ran successfully but may not be generating test results.\n")
	} else {
		fmt.Printf("   ✓ JUnit output file created\n")
	}

	fmt.Printf("   ✓ Test passed successfully\n")

	return nil
}

// testAnalysisCommand validates the analysis command
func testAnalysisCommand(command string, outputs TestSuiteOutputs, discoveredTests []string) error {
	fmt.Println("   Executing analysis command with discovered test...")

	// Use a random discovered test
	var sampleTest string
	if len(discoveredTests) > 0 {
		// Pick a random test (use first one for consistency)
		sampleTest = discoveredTests[0]
		fmt.Printf("   Using test: %s\n", sampleTest)
	} else {
		return fmt.Errorf("no discovered tests available for validation")
	}

	// Replace template variables
	testCmd := command
	testCmd = strings.ReplaceAll(testCmd, "<< test.atoms >>", sampleTest)

	// Create temp directory for outputs
	tmpDir, err := os.MkdirTemp("", "circleci-testsuite-analysis-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	lcovPath := tmpDir + "/lcov.info"
	coveragePath := tmpDir + "/coverage.txt"
	testCmd = strings.ReplaceAll(testCmd, "<< outputs.lcov >>", lcovPath)
	testCmd = strings.ReplaceAll(testCmd, "<< outputs.go-coverage >>", coveragePath)
	testCmd = strings.ReplaceAll(testCmd, "<< outputs.junit >>", tmpDir+"/junit.xml")

	cmd := exec.Command("sh", "-c", testCmd)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Check if we got output
	if len(outputStr) == 0 {
		return fmt.Errorf("command produced no output - may not be configured correctly")
	}

	// Check for common error patterns
	lowerOutput := strings.ToLower(outputStr)
	if strings.Contains(lowerOutput, "command not found") {
		return fmt.Errorf("test runner or coverage tool not found\n   Output: %s",
			truncateOutput(outputStr, 300))
	}

	// Check if the analysis passed (exit code 0)
	if err != nil {
		exitCode := -1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		return fmt.Errorf("analysis execution failed (exit code %d)\n   The test must pass with coverage for validation.\n   Output:\n%s",
			exitCode, truncateOutput(outputStr, 500))
	}

	// Determine which coverage file to check based on outputs
	var coverageFileToCheck string
	var coverageFormat string
	if outputs.LCov != "" {
		coverageFileToCheck = lcovPath
		coverageFormat = "lcov"
	} else if outputs.GoCoverage != "" {
		coverageFileToCheck = coveragePath
		coverageFormat = "go-coverage"
	}

	// Check if coverage file was created
	if coverageFileToCheck != "" {
		if stat, err := os.Stat(coverageFileToCheck); err != nil {
			return fmt.Errorf("coverage file not created at %s\n   The analysis command must generate coverage output\n   Check that coverage flags are correct", coverageFileToCheck)
		} else if stat.Size() == 0 {
			return fmt.Errorf("coverage file is empty at %s\n   The analysis command generated a file but no coverage data\n   Check that coverage is properly configured", coverageFileToCheck)
		} else {
			fmt.Printf("   ✓ Coverage file created (%s format, %d bytes)\n", coverageFormat, stat.Size())
		}
	}

	fmt.Printf("   ✓ Analysis completed successfully with coverage\n")

	return nil
}

// truncateOutput truncates output to a maximum length
func truncateOutput(output string, maxLen int) string {
	if len(output) > maxLen {
		return output[:maxLen] + "...\n(output truncated)"
	}
	return output
}

// printCommandTroubleshootingTips provides helpful tips for common issues
func printCommandTroubleshootingTips(commandName string) {
	switch commandName {
	case "discover":
		fmt.Println("   • Ensure your test framework is installed")
		fmt.Println("   • Verify the command works in your terminal")
		fmt.Println("   • Check that test files exist in your project")
		fmt.Println("   • The command should output a list of test files or packages")
	case "run":
		fmt.Println("   • Ensure the command includes << test.atoms >> for the test list")
		fmt.Println("   • Verify << outputs.junit >> is used for JUnit output path")
		fmt.Println("   • Check that your test runner is installed")
		fmt.Println("   • The selected test must pass (exit code 0) for validation")
		fmt.Println("   • If the test is failing, fix the test or try a different one")
		fmt.Println("   • Make sure the command can run tests individually")
	case "analysis":
		fmt.Println("   • Ensure the command includes coverage flags")
		fmt.Println("   • Verify << test.atoms >> is used for the test to analyze")
		fmt.Println("   • Check that << outputs.lcov >> or << outputs.go-coverage >> is specified")
		fmt.Println("   • Ensure coverage tools are installed (e.g., coverage.py, istanbul, c8)")
		fmt.Println("   • The test must pass AND generate coverage data")
		fmt.Println("   • Verify coverage output path matches the template variable")
		fmt.Println("   • Check that coverage provider is correctly configured")
	}
}

// saveTestsuite saves the test suite configuration to the file
func saveTestsuite(filename string, testsuite *Testsuite) error {
	// Check if file exists and read existing content
	var existingTestsuites []Testsuite
	if file, err := os.Open(filename); err == nil {
		defer file.Close()
		decoder := yaml.NewDecoder(file)
		for {
			var doc Testsuite
			if err := decoder.Decode(&doc); err != nil {
				if err.Error() == "EOF" {
					break
				}
				return fmt.Errorf("error decoding existing YAML: %w", err)
			}
			// Skip the testsuite with the same name (we're overwriting it)
			if doc.Name != testsuite.Name {
				existingTestsuites = append(existingTestsuites, doc)
			}
		}
	}

	// Add the new testsuite
	existingTestsuites = append(existingTestsuites, *testsuite)

	// Create or truncate the file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", filename, err)
	}
	defer file.Close()

	// Write all testsuites as separate YAML documents
	encoder := yaml.NewEncoder(file)
	defer encoder.Close()

	for i, ts := range existingTestsuites {
		if i > 0 {
			// Add document separator between test suites
			if _, err := file.WriteString("---\n"); err != nil {
				return fmt.Errorf("failed to write document separator: %w", err)
			}
		}
		if err := encoder.Encode(&ts); err != nil {
			return fmt.Errorf("failed to encode testsuite: %w", err)
		}
	}

	return nil
}

// testsuiteExistsInFile checks if a test suite with the given name exists
func testsuiteExistsInFile(filename string, testsuiteName string) (bool, error) {
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to open %s: %w", filename, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	for {
		var doc Testsuite
		if err := decoder.Decode(&doc); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return false, fmt.Errorf("error decoding YAML: %w", err)
		}
		if doc.Name == testsuiteName {
			return true, nil
		}
	}
	return false, nil
}

// printNextSteps shows the user what to do next
func printNextSteps(testsuiteName string, adaptiveTesting bool) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("🎉 Test Suite Configuration Complete!")
	fmt.Println(strings.Repeat("=", 60))

	fmt.Println()
	fmt.Println("📋 Next Steps:")
	fmt.Println()

	fmt.Println("1. Test your configuration locally:")
	fmt.Printf("   $ circleci run testsuite \"%s\" --local\n", testsuiteName)
	fmt.Println()

	fmt.Println("2. Update your .circleci/config.yml to use the test suite:")
	fmt.Println("   Replace your existing test command with:")
	fmt.Printf("   - run: circleci run testsuite \"%s\"\n", testsuiteName)
	fmt.Println("   - store_test_results:")
	fmt.Println("       path: test-reports  # Match your junit output path")
	fmt.Println()

	if adaptiveTesting {
		fmt.Println("3. Run initial test analysis:")
		fmt.Println("   On your first run, analysis will be performed to build impact data.")
		fmt.Println("   This may take longer but subsequent runs will be much faster.")
		fmt.Println()

		fmt.Println("4. Configure branch behavior (optional):")
		fmt.Println("   By default:")
		fmt.Println("   - Main branch: runs analysis for impacted tests")
		fmt.Println("   - Other branches: runs only impacted tests (selection mode)")
		fmt.Println("   Customize with --test-analysis and --test-selection flags")
		fmt.Println()

		fmt.Println("5. Consider higher parallelism for analysis:")
		fmt.Println("   Analysis runs tests individually, so more parallel nodes speed it up:")
		fmt.Println("   parallelism: << pipeline.git.branch == \"main\" and 10 or 2 >>")
		fmt.Println()
	} else {
		fmt.Println("3. (Optional) Enable Test Impact Analysis later:")
		fmt.Println("   Run 'circleci testsuite init' again and enable it,")
		fmt.Println("   or manually add the 'analysis' command to your config.")
		fmt.Println()
	}

	fmt.Println("📚 Documentation:")
	fmt.Println("   https://circleci.com/docs/smarter-testing")

	fmt.Println()
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()
}
