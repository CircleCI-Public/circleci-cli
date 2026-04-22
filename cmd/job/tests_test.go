package job

import (
	"bytes"
	"encoding/json"
	"testing"

	jobapi "github.com/CircleCI-Public/circleci-cli/api/job"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
)

func Test_newTestsCommand_defaultShowsOnlyFailed(t *testing.T) {
	client := &mockJobClient{
		tests: []jobapi.TestResult{
			{Name: "TestPassing", Result: "success"},
			{Name: "TestFailing", Result: "failure", Message: "boom"},
			{Name: "TestError", Result: "error", Message: "crash"},
		},
	}

	jos := newTestJobCommand(client)
	cmd := newTestsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"42", "--project-slug", "gh/org/repo"})

	err := cmd.Execute()
	assert.ErrorIs(t, err, ErrTestsFailed)
	assert.Assert(t, !bytes.Contains(buf.Bytes(), []byte("TestPassing")))
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("TestFailing")))
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("TestError")))
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("1 passed, 2 failed (total 3).")))
}

func Test_newTestsCommand_all(t *testing.T) {
	client := &mockJobClient{
		tests: []jobapi.TestResult{
			{Name: "TestPassing", Result: "success"},
			{Name: "TestFailing", Result: "failure", Message: "boom"},
		},
	}

	jos := newTestJobCommand(client)
	cmd := newTestsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"42", "--project-slug", "gh/org/repo", "--all"})

	err := cmd.Execute()
	assert.ErrorIs(t, err, ErrTestsFailed)
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("TestPassing")))
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("TestFailing")))
}

func Test_newTestsCommand_jsonOutput(t *testing.T) {
	client := &mockJobClient{
		tests: []jobapi.TestResult{
			{Name: "TestFoo", Classname: "pkg.Foo", Result: "failure", Message: "bad", RunTime: 0.3},
		},
	}

	jos := newTestJobCommand(client)
	cmd := newTestsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"42", "--project-slug", "gh/org/repo", "--json"})

	err := cmd.Execute()
	assert.ErrorIs(t, err, ErrTestsFailed)

	var summary testSummaryJSON
	assert.NilError(t, json.Unmarshal(buf.Bytes(), &summary))
	assert.Equal(t, summary.Total, 1)
	assert.Equal(t, summary.Failed, 1)
	assert.Equal(t, len(summary.Results), 1)
	assert.Equal(t, summary.Results[0].Name, "TestFoo")
	assert.Equal(t, summary.Results[0].Result, "failure")
}

func Test_newTestsCommand_noResults(t *testing.T) {
	client := &mockJobClient{tests: []jobapi.TestResult{}}

	jos := newTestJobCommand(client)
	cmd := newTestsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"42", "--project-slug", "gh/org/repo"})

	err := cmd.Execute()
	assert.NilError(t, err)
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("No test results found for this job.")))
}

func Test_newTestsCommand_invalidJobNumber(t *testing.T) {
	jos := newTestJobCommand(&mockJobClient{})
	cmd := newTestsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"abc", "--project-slug", "gh/org/repo"})
	err := cmd.Execute()
	assert.ErrorContains(t, err, "integer")
}

func Test_newTestsCommand_allPassed_summaryOnly(t *testing.T) {
	client := &mockJobClient{
		tests: []jobapi.TestResult{
			{Name: "TestA", Result: "success"},
			{Name: "TestB", Result: "success"},
		},
	}

	jos := newTestJobCommand(client)
	cmd := newTestsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"42", "--project-slug", "gh/org/repo"})

	err := cmd.Execute()
	assert.NilError(t, err)
	assert.Assert(t, !bytes.Contains(buf.Bytes(), []byte("TestA")))
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("2 passed, 0 failed (total 2).")))
}
