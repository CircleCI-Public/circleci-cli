package job

import (
	"bytes"
	"fmt"
	"testing"

	jobapi "github.com/CircleCI-Public/circleci-cli/api/job"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

type stubImageTagClient struct {
	details *jobapi.JobDetails
	outputs map[string][]jobapi.StepOutput
}

func (s *stubImageTagClient) GetJobDetails(vcs string, org string, repo string, jobNumber int) (*jobapi.JobDetails, error) {
	return s.details, nil
}

func (s *stubImageTagClient) GetStepOutput(outputURL string) ([]jobapi.StepOutput, error) {
	if out, ok := s.outputs[outputURL]; ok {
		return out, nil
	}
	return nil, fmt.Errorf("unexpected output url %q", outputURL)
}

func TestJobImageTagCommand(t *testing.T) {
	stub := &stubImageTagClient{
		details: &jobapi.JobDetails{
			Steps: []jobapi.Step{
				{
					Name: "Build Docker image",
					Actions: []jobapi.StepAction{
						{Name: "Build Docker image", OutputURL: "https://example.com/out/build"},
					},
				},
				{
					Name: "Push Docker images",
					Actions: []jobapi.StepAction{
						{Name: "Push Docker images", OutputURL: "https://example.com/out/push"},
					},
				},
			},
		},
		outputs: map[string][]jobapi.StepOutput{
			"https://example.com/out/build": {
				{Message: "docker build -t repo/app:abc123 .\n"},
			},
			"https://example.com/out/push": {
				{Message: "docker push repo/app:abc123\n"},
				{Message: "docker push repo/app:def456\n"},
			},
		},
	}

	opts := &jobOpts{jobClient: stub}
	noValidator := func(_ *cobra.Command, _ []string) error { return nil }

	t.Run("extracts image tags from default steps", func(t *testing.T) {
		cmd := newImageTagCommand(opts, noValidator)
		cmd.SetArgs([]string{"gh/test-org/test-repo", "123"})

		var outBuf, errBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Equal(t, "repo/app:abc123\nrepo/app:def456\n", outBuf.String()+errBuf.String())
	})

	t.Run("supports custom regex capture group", func(t *testing.T) {
		custom := &stubImageTagClient{
			details: &jobapi.JobDetails{
				Steps: []jobapi.Step{
					{
						Name: "Build Docker image",
						Actions: []jobapi.StepAction{
							{Name: "Build Docker image", OutputURL: "https://example.com/out/custom"},
						},
					},
				},
			},
			outputs: map[string][]jobapi.StepOutput{
				"https://example.com/out/custom": {
					{Message: "IMAGE_TAG=abc123\n"},
				},
			},
		}

		cmd := newImageTagCommand(&jobOpts{jobClient: custom}, noValidator)
		cmd.SetArgs([]string{"gh/test-org/test-repo", "123", "--regex", `IMAGE_TAG=([^\n]+)`, "--step", "Build Docker image"})

		var outBuf, errBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetErr(&errBuf)

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Equal(t, "abc123\n", outBuf.String()+errBuf.String())
	})
}
