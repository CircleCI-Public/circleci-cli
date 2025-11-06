package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/CircleCI-Public/circleci-cli/prompt"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type Testsuite struct {
	Name     string           `yaml:"name"`
	Discover string           `yaml:"discover"`
	Run      string           `yaml:"run"`
	Analyse  string           `yaml:"analyse"`
	Outputs  TestSuiteOutputs `yaml:"outputs"`
	Options  TestSuiteOptions `yaml:"options"`
}

type TestSuiteOutputs struct {
	Junit      string `yaml:"junit,omitempty"`
	GoCoverage string `yaml:"go-coverage,omitempty"`
	LCov       string `yaml:"lcov,omitempty"`
	GCov       string `yaml:"gcov,omitempty"`
}

type TestSuiteOptions struct {
	// Overall timeout for the entire test suite run
	Timeout time.Duration `yaml:"timeout" validate:"min=1m,max=60m"`
	// If true we will perform test selection on non-default branches
	AdaptiveTesting bool `yaml:"adaptive-testing"`
	// FullTestRunPaths are paths that might have an indirect impact on tests
	// and should run the full test suite if a change is detected.
	// Each file supports the filepath.Match syntax.
	FullTestRunPaths     []string      `yaml:"full-test-run-paths"`
	TestSelection        Selection     `yaml:"test-selection,omitempty"`
	TestAnalysis         Selection     `yaml:"test-analysis,omitempty"`
	TestAnalysisDuration time.Duration `yaml:"test-analysis-duration,omitempty" validate:"min=0,max=5h"`
	DynamicBatching      bool          `yaml:"dynamic-batching"`
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

func newTestsuiteCommand() *cobra.Command {
	testsuiteCmd := &cobra.Command{
		Use:   "testsuite",
		Short: "Operate on testsuites",
	}

	initsuiteCmd := &cobra.Command{
		Short: "Initialize a testsuite",
		Use:   "init",
		RunE:  testsuiteInitRunE,
	}

	testsuiteCmd.AddCommand(initsuiteCmd)

	return testsuiteCmd
}

func testsuiteInitRunE(cmd *cobra.Command, args []string) error {
	var name string

	configPath := ".circleci/config.yml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("No .circleci/config.yml file found. Please initialize your project first with 'circleci init' or equivalent.")
		return errors.New(".circleci/config.yml file not found ")
	}

	if prompt.AskUserToConfirm("Do you want to extract the commands from the .circleci/config.yml file?") {
		jobs, err := getJobsFromConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to get jobs from %s: %w", configPath, err)
		}
		jobNames := make([]string, 0, len(jobs))
		for jobName := range jobs {
			jobNames = append(jobNames, jobName)
		}

		prompt := &survey.Select{
			Message: "Select a job to extract the commands from",
			Options: jobNames,
		}

		var selectedJobName string
		err = survey.AskOne(prompt, &selectedJobName)
		if err != nil {
			return fmt.Errorf("failed to select job name: %w", err)
		}

		if selectedJobName == "" {
			return errors.New("job name is required")
		}

		job := jobs[selectedJobName]
		// commands := job.Steps
		// fmt.Println(commands)
		fmt.Println(job)
		return nil
	}

	testsuiteFile := ".circleci/test-suite.yml"
	var file *os.File
	if _, err := os.Stat(testsuiteFile); os.IsNotExist(err) {
		fmt.Println("No test-suite configuration file found. Creating a new one...")
		file, err = os.Create(testsuiteFile)
		if err != nil {
			return fmt.Errorf("failed to create %s: %v", testsuiteFile, err)
		}
		defer file.Close()
		fmt.Printf("Created test-suite configuration file: %s\n", testsuiteFile)
	} else {
		file, err = os.OpenFile(testsuiteFile, os.O_RDWR, 0644)
		if err != nil {
			return fmt.Errorf("failed to open %s: %v", testsuiteFile, err)
		}
		defer file.Close()
	}

	for name == "" {
		name = prompt.ReadStringFromUser("Enter a name for the testsuite", "")
		if name == "" {
			fmt.Println("Name is required")
		}
	}

	if exists, err := testsuiteExistsInFile(testsuiteFile, name); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("testsuite '%s' already exists in %s", name, testsuiteFile)
	}

	testsuite := &Testsuite{
		Name: name,
		Options: TestSuiteOptions{
			Timeout:              10 * time.Minute,
			AdaptiveTesting:      false,
			FullTestRunPaths:     []string{},
			TestSelection:        SelectionNone,
			TestAnalysis:         SelectionImpacted,
			TestAnalysisDuration: 10 * time.Minute,
			DynamicBatching:      false,
		},
		Outputs: TestSuiteOutputs{
			Junit:      "test-results/junit.xml",
			GoCoverage: "test-results/go-coverage.txt",
			LCov:       "test-results/lcov.info",
			GCov:       "test-results/gcov.info",
		},
		Discover: "",
		Run:      "",
		Analyse:  "",
	}

	err := yaml.NewEncoder(file).Encode(testsuite)
	if err != nil {
		return fmt.Errorf("failed to encode testsuite: %w", err)
	}

	fmt.Printf("Testsuite '%s' initialized\n", name)
	return nil
}

func countYamlDocuments(filename string) (int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return 0, fmt.Errorf("failed to open %s: %w", filename, err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	docCount := 0
	for {
		var doc interface{}
		if err := decoder.Decode(&doc); err != nil {
			if err.Error() == "EOF" {
				break
			}
			return 0, fmt.Errorf("error decoding YAML: %w", err)
		}
		docCount++
	}
	return docCount, nil
}

func testsuiteExistsInFile(filename string, testsuiteName string) (bool, error) {
	file, err := os.Open(filename)
	if err != nil {
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

type Config struct {
	Jobs map[string]Job `yaml:"jobs"`
}

type Job struct {
	Steps any `yaml:"steps"`
}

func getJobsFromConfig(configPath string) (map[string]Job, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", configPath, err)
	}
	defer file.Close()

	var config Config
	if err := yaml.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode YAML: %w", err)
	}
	return config.Jobs, nil
}
