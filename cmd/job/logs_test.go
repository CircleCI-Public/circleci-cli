package job

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	jobapi "github.com/CircleCI-Public/circleci-cli/api/job"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

type stubJobClient struct {
	details *jobapi.JobDetails
	outputs map[string][]jobapi.StepOutput

	gotVCS       string
	gotOrg       string
	gotRepo      string
	gotJobNumber int
}

func (s *stubJobClient) GetJobDetails(vcs string, org string, repo string, jobNumber int) (*jobapi.JobDetails, error) {
	s.gotVCS = vcs
	s.gotOrg = org
	s.gotRepo = repo
	s.gotJobNumber = jobNumber
	return s.details, nil
}

func (s *stubJobClient) GetStepOutput(outputURL string) ([]jobapi.StepOutput, error) {
	if out, ok := s.outputs[outputURL]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected output url %q", outputURL)
}

func TestJobLogsCommand(t *testing.T) {
	secretOutputURL := "https://circleci.example/api/private/output/presigned/123?token=SECRET_TOKEN"

	stub := &stubJobClient{
		details: &jobapi.JobDetails{
			BuildNum: 123,
			Status:   "success",
			Steps: []jobapi.Step{
				{
					Name: "Checkout code",
					Actions: []jobapi.StepAction{
						{Name: "Checkout code", OutputURL: "https://example.com/out/1"},
					},
				},
				{
					Name: "Build Docker image",
					Actions: []jobapi.StepAction{
						{Name: "Build Docker image", OutputURL: secretOutputURL, Status: "success", Type: "test"},
					},
				},
			},
		},
		outputs: map[string][]jobapi.StepOutput{
			"https://example.com/out/1": {
				{Message: "checkout\n"},
			},
			secretOutputURL: {
				{Message: "docker build -t repo/app:abc123 .\n"},
			},
		},
	}

	opts := &jobOpts{jobClient: stub}
	noValidator := func(_ *cobra.Command, _ []string) error { return nil }

	t.Run("prints full job output by default", func(t *testing.T) {
		cmd := newLogsCommand(opts, noValidator)
		cmd.SetArgs([]string{"gh/test-org/test-repo", "123"})

		var outBuf, errBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Equal(t, "checkout\ndocker build -t repo/app:abc123 .\n", outBuf.String()+errBuf.String())

		assert.Equal(t, "github", stub.gotVCS)
		assert.Equal(t, "test-org", stub.gotOrg)
		assert.Equal(t, "test-repo", stub.gotRepo)
		assert.Equal(t, 123, stub.gotJobNumber)
	})

	t.Run("filters by step name (substring match)", func(t *testing.T) {
		cmd := newLogsCommand(opts, noValidator)
		cmd.SetArgs([]string{"gh/test-org/test-repo", "123", "--step", "Build"})

		var outBuf, errBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Equal(t, "docker build -t repo/app:abc123 .\n", outBuf.String()+errBuf.String())
	})

	t.Run("filters by step index", func(t *testing.T) {
		cmd := newLogsCommand(opts, noValidator)
		cmd.SetArgs([]string{"gh/test-org/test-repo", "123", "--step", "2"})

		var outBuf, errBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Equal(t, "docker build -t repo/app:abc123 .\n", outBuf.String()+errBuf.String())
	})

	t.Run("json output does not include output urls", func(t *testing.T) {
		cmd := newLogsCommand(opts, noValidator)
		cmd.SetArgs([]string{"gh/test-org/test-repo", "123", "--json"})

		var outBuf, errBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)

		err := cmd.Execute()
		assert.NoError(t, err)

		gotRaw := outBuf.String() + errBuf.String()
		assert.NotContains(t, gotRaw, "SECRET_TOKEN")

		var parsed logsResponse
		assert.NoError(t, json.Unmarshal([]byte(gotRaw), &parsed))
		assert.Equal(t, "gh/test-org/test-repo", parsed.ProjectSlug)
		assert.Equal(t, 123, parsed.JobNumber)
		assert.Len(t, parsed.Steps, 2)
		assert.Equal(t, "Checkout code", parsed.Steps[0].Name)
		assert.Equal(t, "Build Docker image", parsed.Steps[1].Name)
	})
}
