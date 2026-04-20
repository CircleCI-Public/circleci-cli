package deploy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func parseFixture(t *testing.T, name string) *yaml.Node {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		t.Fatalf("parsing fixture %s: %v", name, err)
	}
	return &root
}

func TestDetectDeployJobs(t *testing.T) {
	t.Run("single deploy job is detected", func(t *testing.T) {
		root := parseFixture(t, "simple-deploy.yml")
		detected := DetectDeployJobs(root)
		if assert.Len(t, detected, 1) {
			assert.Equal(t, "deploy-prod", detected[0].Name)
			assert.False(t, detected[0].AlreadyInstrumented)
		}
	})

	t.Run("multiple deploy-ish jobs are all detected", func(t *testing.T) {
		root := parseFixture(t, "multiple-deploys.yml")
		detected := DetectDeployJobs(root)
		names := make([]string, 0, len(detected))
		for _, d := range detected {
			names = append(names, d.Name)
		}
		assert.ElementsMatch(t, []string{"deploy-staging", "deploy-prod", "release-docs"}, names)
	})

	t.Run("already instrumented jobs are flagged", func(t *testing.T) {
		root := parseFixture(t, "already-instrumented.yml")
		detected := DetectDeployJobs(root)
		if assert.Len(t, detected, 1) {
			assert.True(t, detected[0].AlreadyInstrumented, "expected deploy-prod to be flagged as already instrumented")
		}
	})

	t.Run("no deploy jobs means empty result", func(t *testing.T) {
		root := parseFixture(t, "no-deploy-jobs.yml")
		assert.Empty(t, DetectDeployJobs(root))
	})

	t.Run("missing jobs section is handled", func(t *testing.T) {
		var root yaml.Node
		err := yaml.Unmarshal([]byte("version: 2.1\n"), &root)
		assert.NoError(t, err)
		assert.Empty(t, DetectDeployJobs(&root))
	})

	t.Run("nil input does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			DetectDeployJobs(nil)
		})
	})
}

func TestIsDeployJobName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"deploy", true},
		{"deploy-prod", true},
		{"PublishDocs", true},
		{"release-to-staging", true},
		{"ship-it", true},
		{"Deploy_Production", true},
		{"build", false},
		{"test", false},
		{"lint", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, isDeployJobName(c.name))
		})
	}
}

func TestInferEnvironmentName(t *testing.T) {
	cases := []struct {
		job       string
		wantEnv   string
		wantMatch bool
	}{
		{"deploy-prod", "production", true},
		{"deploy-production", "production", true},
		{"deploy-staging", "staging", true},
		{"stage-deploy", "staging", true},
		{"deploy-dev", "development", true},
		{"deploy-development", "development", true},
		{"deploy-test", "test", true},
		{"release", "", false},
		{"publish", "", false},
		{"ship-it", "", false},
	}
	for _, c := range cases {
		t.Run(c.job, func(t *testing.T) {
			got, ok := InferEnvironmentName(c.job)
			assert.Equal(t, c.wantEnv, got)
			assert.Equal(t, c.wantMatch, ok)
		})
	}
}
