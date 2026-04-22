package job

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	jobapi "github.com/CircleCI-Public/circleci-cli/api/job"
	"github.com/CircleCI-Public/circleci-cli/api/rest"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
)

type mockJobClient struct {
	steps    *jobapi.JobDetails
	stepsErr error
	logs     map[string]string
	logsErr  error
	logs404  bool
	tests    []jobapi.TestResult
	testsErr error
}

func (m *mockJobClient) GetJobSteps(_ string, _ int) (*jobapi.JobDetails, error) {
	return m.steps, m.stepsErr
}

func (m *mockJobClient) GetStepLog(_ string, _ int, _ int, _ int, logType string) (string, error) {
	if m.logs404 {
		return "", &rest.HTTPError{Code: 404}
	}
	if m.logsErr != nil {
		return "", m.logsErr
	}
	return m.logs[logType], nil
}

func (m *mockJobClient) GetTestResults(_ string, _ int) ([]jobapi.TestResult, error) {
	return m.tests, m.testsErr
}

func boolPtr(b bool) *bool { return &b }

func newTestJobCommand(client jobapi.JobClient) *jobOpts {
	return &jobOpts{client: client}
}

func Test_newLogsCommand_humanOutput(t *testing.T) {
	client := &mockJobClient{
		steps: &jobapi.JobDetails{
			BuildNum: 42,
			Steps: []jobapi.JobStep{
				{
					Name: "Run tests",
					Actions: []jobapi.JobAction{
						{Index: 0, Step: 1, Failed: boolPtr(true)},
					},
				},
			},
		},
		logs: map[string]string{
			"output": "FAIL: something broke",
			"error":  "",
		},
	}

	jos := newTestJobCommand(client)
	cmd := newLogsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"42", "--project-slug", "gh/org/repo"})

	err := cmd.Execute()
	assert.NilError(t, err)
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("Run tests")))
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("FAIL: something broke")))
}

func Test_newLogsCommand_failedOnly(t *testing.T) {
	passed := boolPtr(false)
	failed := boolPtr(true)
	client := &mockJobClient{
		steps: &jobapi.JobDetails{
			Steps: []jobapi.JobStep{
				{Name: "passing step", Actions: []jobapi.JobAction{{Index: 0, Step: 1, Failed: passed}}},
				{Name: "failing step", Actions: []jobapi.JobAction{{Index: 0, Step: 2, Failed: failed}}},
			},
		},
		logs: map[string]string{"output": "error output", "error": ""},
	}

	jos := newTestJobCommand(client)
	cmd := newLogsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"42", "--project-slug", "gh/org/repo", "--failed-only"})

	err := cmd.Execute()
	assert.NilError(t, err)
	assert.Assert(t, !bytes.Contains(buf.Bytes(), []byte("passing step")))
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("failing step")))
}

func Test_newLogsCommand_stepFilter(t *testing.T) {
	client := &mockJobClient{
		steps: &jobapi.JobDetails{
			Steps: []jobapi.JobStep{
				{Name: "build", Actions: []jobapi.JobAction{{Index: 0, Step: 1, Failed: boolPtr(false)}}},
				{Name: "test", Actions: []jobapi.JobAction{{Index: 0, Step: 2, Failed: boolPtr(true)}}},
			},
		},
		logs: map[string]string{"output": "test output", "error": ""},
	}

	jos := newTestJobCommand(client)
	cmd := newLogsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"42", "--project-slug", "gh/org/repo", "--step", "test"})

	err := cmd.Execute()
	assert.NilError(t, err)
	assert.Assert(t, !bytes.Contains(buf.Bytes(), []byte("build")))
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("test")))
}

func Test_newLogsCommand_jsonOutput(t *testing.T) {
	client := &mockJobClient{
		steps: &jobapi.JobDetails{
			Steps: []jobapi.JobStep{
				{Name: "deploy", Actions: []jobapi.JobAction{{Index: 0, Step: 1, Failed: boolPtr(true)}}},
			},
		},
		logs: map[string]string{"output": "deployed", "error": ""},
	}

	jos := newTestJobCommand(client)
	cmd := newLogsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"42", "--project-slug", "gh/org/repo", "--json"})

	err := cmd.Execute()
	assert.NilError(t, err)

	var result jobLogsJSON
	assert.NilError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, len(result.Steps), 1)
	assert.Equal(t, result.Steps[0].Step, "deploy")
	assert.Equal(t, result.Steps[0].Output, "deployed")
}

func Test_newLogsCommand_invalidJobNumber(t *testing.T) {
	jos := newTestJobCommand(&mockJobClient{})
	cmd := newLogsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"not-a-number", "--project-slug", "gh/org/repo"})
	err := cmd.Execute()
	assert.ErrorContains(t, err, "integer")
}

func Test_newLogsCommand_stepLog_404_treated_as_empty(t *testing.T) {
	client := &mockJobClient{
		steps: &jobapi.JobDetails{
			Steps: []jobapi.JobStep{
				{Name: "build", Actions: []jobapi.JobAction{{Index: 0, Step: 1, Failed: boolPtr(false)}}},
			},
		},
		logs404: true,
	}
	jos := newTestJobCommand(client)
	cmd := newLogsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"42", "--project-slug", "gh/org/repo"})
	err := cmd.Execute()
	assert.NilError(t, err)
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("build")))
}

func Test_newLogsCommand_stepLog_nonNotFound_error_propagates(t *testing.T) {
	client := &mockJobClient{
		steps: &jobapi.JobDetails{
			Steps: []jobapi.JobStep{
				{Name: "build", Actions: []jobapi.JobAction{{Index: 0, Step: 1, Failed: boolPtr(false)}}},
			},
		},
		logsErr: errors.New("network error"),
	}
	jos := newTestJobCommand(client)
	cmd := newLogsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"42", "--project-slug", "gh/org/repo"})
	err := cmd.Execute()
	assert.ErrorContains(t, err, "network error")
}

func Test_newLogsCommand_tail(t *testing.T) {
	var logLines []string
	for i := 0; i < 10; i++ {
		logLines = append(logLines, fmt.Sprintf("line %d", i))
	}
	logContent := strings.Join(logLines, "\n")

	client := &mockJobClient{
		steps: &jobapi.JobDetails{
			Steps: []jobapi.JobStep{
				{Name: "build", Actions: []jobapi.JobAction{{Index: 0, Step: 1, Failed: boolPtr(false)}}},
			},
		},
		logs: map[string]string{"output": logContent, "error": ""},
	}

	t.Run("tail 3 truncates", func(t *testing.T) {
		jos := newTestJobCommand(client)
		cmd := newLogsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"42", "--project-slug", "gh/org/repo", "--tail", "3"})
		err := cmd.Execute()
		assert.NilError(t, err)
		assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("truncated")))
		assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("line 9")))
		assert.Assert(t, !bytes.Contains(buf.Bytes(), []byte("line 0")))
	})

	t.Run("tail 0 shows all", func(t *testing.T) {
		jos := newTestJobCommand(client)
		cmd := newLogsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"42", "--project-slug", "gh/org/repo", "--tail", "0"})
		err := cmd.Execute()
		assert.NilError(t, err)
		assert.Assert(t, !bytes.Contains(buf.Bytes(), []byte("truncated")))
		assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("line 0")))
		assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("line 9")))
	})
}

func Test_newLogsCommand_stepFilter_noMatch(t *testing.T) {
	client := &mockJobClient{
		steps: &jobapi.JobDetails{
			Steps: []jobapi.JobStep{
				{Name: "build", Actions: []jobapi.JobAction{{Index: 0, Step: 1, Failed: boolPtr(false)}}},
				{Name: "test", Actions: []jobapi.JobAction{{Index: 0, Step: 2, Failed: boolPtr(true)}}},
			},
		},
		logs: map[string]string{"output": "test output", "error": ""},
	}

	jos := newTestJobCommand(client)
	cmd := newLogsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"42", "--project-slug", "gh/org/repo", "--step", "nonexistent"})

	err := cmd.Execute()
	assert.ErrorContains(t, err, "nonexistent")
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("build")))
	assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("test")))
}

func Test_newLogsCommand_listSteps(t *testing.T) {
	client := &mockJobClient{
		steps: &jobapi.JobDetails{
			Steps: []jobapi.JobStep{
				{Name: "checkout", Actions: []jobapi.JobAction{{Index: 0, Step: 1, Failed: boolPtr(false)}}},
				{Name: "run tests", Actions: []jobapi.JobAction{{Index: 0, Step: 2, Failed: boolPtr(true)}}},
			},
		},
		logs: map[string]string{},
	}

	t.Run("human", func(t *testing.T) {
		jos := newTestJobCommand(client)
		cmd := newLogsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })

		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"42", "--project-slug", "gh/org/repo", "--list-steps"})

		err := cmd.Execute()
		assert.NilError(t, err)
		assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("checkout")))
		assert.Assert(t, bytes.Contains(buf.Bytes(), []byte("run tests [failed]")))
	})

	t.Run("json", func(t *testing.T) {
		jos := newTestJobCommand(client)
		cmd := newLogsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })

		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{"42", "--project-slug", "gh/org/repo", "--list-steps", "--json"})

		err := cmd.Execute()
		assert.NilError(t, err)

		var infos []stepInfoJSON
		assert.NilError(t, json.Unmarshal(buf.Bytes(), &infos))
		assert.Equal(t, len(infos), 2)
		assert.Equal(t, infos[0].Step, "checkout")
		assert.Equal(t, infos[0].AnyFailed, false)
		assert.Equal(t, infos[1].Step, "run tests")
		assert.Equal(t, infos[1].AnyFailed, true)
	})
}

func Test_newLogsCommand_listSteps_mutuallyExclusiveFlags(t *testing.T) {
	client := &mockJobClient{
		steps: &jobapi.JobDetails{
			Steps: []jobapi.JobStep{
				{Name: "build", Actions: []jobapi.JobAction{{Index: 0, Step: 1, Failed: boolPtr(false)}}},
			},
		},
	}

	for _, extra := range [][]string{
		{"--step", "build"},
		{"--failed-only"},
		{"--tail", "5"},
	} {
		t.Run(strings.Join(extra, " "), func(t *testing.T) {
			jos := newTestJobCommand(client)
			cmd := newLogsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			args := append([]string{"42", "--project-slug", "gh/org/repo", "--list-steps"}, extra...)
			cmd.SetArgs(args)
			err := cmd.Execute()
			assert.ErrorContains(t, err, "none of the others can be")
		})
	}
}

func Test_newLogsCommand_listSteps_parallelJob_collapseToOneEntryPerStep(t *testing.T) {
	// 3 containers run the same step; only container 2 failed
	client := &mockJobClient{
		steps: &jobapi.JobDetails{
			Steps: []jobapi.JobStep{
				{
					Name: "run tests",
					Actions: []jobapi.JobAction{
						{Index: 0, Step: 1, Failed: boolPtr(false)},
						{Index: 1, Step: 1, Failed: boolPtr(false)},
						{Index: 2, Step: 1, Failed: boolPtr(true)},
					},
				},
			},
		},
	}

	jos := newTestJobCommand(client)
	cmd := newLogsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"42", "--project-slug", "gh/org/repo", "--list-steps", "--json"})

	err := cmd.Execute()
	assert.NilError(t, err)

	var infos []stepInfoJSON
	assert.NilError(t, json.Unmarshal(buf.Bytes(), &infos))
	assert.Equal(t, len(infos), 1, "parallel containers should collapse to one entry per step")
	assert.Equal(t, infos[0].Step, "run tests")
	assert.Equal(t, infos[0].AnyFailed, true)
	assert.Equal(t, len(infos[0].FailedContainers), 1)
	assert.Equal(t, infos[0].FailedContainers[0], 2)
}

func Test_newLogsCommand_containerIndex(t *testing.T) {
	client := &mockJobClient{
		steps: &jobapi.JobDetails{
			Steps: []jobapi.JobStep{
				{
					Name: "run tests",
					Actions: []jobapi.JobAction{
						{Index: 0, Step: 1, Failed: boolPtr(false)},
						{Index: 1, Step: 1, Failed: boolPtr(true)},
					},
				},
			},
		},
		logs: map[string]string{"output": "container output", "error": ""},
	}

	jos := newTestJobCommand(client)
	cmd := newLogsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"42", "--project-slug", "gh/org/repo", "--json", "--container-index", "1"})

	err := cmd.Execute()
	assert.NilError(t, err)

	var result jobLogsJSON
	assert.NilError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, len(result.Steps), 1, "only container 1 should be returned")
	assert.Equal(t, result.Steps[0].ContainerIndex, 1)
}

func Test_newLogsCommand_jsonOutput_includesJobMetadata(t *testing.T) {
	client := &mockJobClient{
		steps: &jobapi.JobDetails{
			BuildNum: 4597,
			Status:   "failed",
			Steps: []jobapi.JobStep{
				{Name: "checkout", Actions: []jobapi.JobAction{{Index: 0, Step: 1, Failed: boolPtr(false)}}},
			},
			Workflows: jobapi.JobWorkflows{JobName: "build-and-test"},
		},
		logs: map[string]string{"output": "ok", "error": ""},
	}

	jos := newTestJobCommand(client)
	cmd := newLogsCommand(jos, func(cmd *cobra.Command, args []string) error { return nil })

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"4597", "--project-slug", "gh/org/repo", "--json"})

	err := cmd.Execute()
	assert.NilError(t, err)

	var result jobLogsJSON
	assert.NilError(t, json.Unmarshal(buf.Bytes(), &result))
	assert.Equal(t, result.JobNumber, 4597)
	assert.Equal(t, result.JobName, "build-and-test")
	assert.Equal(t, result.Status, "failed")
}
