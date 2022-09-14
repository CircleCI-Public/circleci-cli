package runner

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/api/runner"
)

func Test_ResourceClass(t *testing.T) {
	runner := runnerMock{}
	cmd := newResourceClassCommand(&runnerOpts{r: &runner}, nil)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	t.Run("create", func(t *testing.T) {
		t.Run("without default token", func(t *testing.T) {
			defer runner.reset()
			defer stdout.Reset()
			defer stderr.Reset()

			cmd.SetArgs([]string{
				"create",
				"my-namespace/my-resource-class",
				"my-description",
			})

			err := cmd.Execute()
			assert.NilError(t, err)

			assert.Check(t, cmp.Equal(len(runner.resourceClasses), 1))
			assert.Check(t, cmp.Equal(runner.resourceClasses[0].ResourceClass, "my-namespace/my-resource-class"))
			assert.Check(t, cmp.Equal(runner.resourceClasses[0].Description, "my-description"))
			assert.Check(t, cmp.Contains(stdout.String(), "my-namespace/my-resource-class"))

			assert.Check(t, cmp.Equal(len(runner.tokens), 0))

			assert.Check(t, cmp.Contains(stderr.String(), terms))
		})

		t.Run("with default token", func(t *testing.T) {
			defer runner.reset()
			defer stdout.Reset()
			defer stderr.Reset()

			cmd.SetArgs([]string{
				"create",
				"my-namespace/my-other-resource-class",
				"my-description",
				"--generate-token",
			})

			err := cmd.Execute()
			assert.NilError(t, err)
			out := stdout.String()

			assert.Check(t, cmp.Equal(len(runner.resourceClasses), 1))
			assert.Check(t, cmp.Equal(runner.resourceClasses[0].ResourceClass, "my-namespace/my-other-resource-class"))
			assert.Check(t, cmp.Equal(runner.resourceClasses[0].Description, "my-description"))
			assert.Check(t, cmp.Contains(out, "my-namespace/my-other-resource-class"))

			assert.Check(t, cmp.Equal(len(runner.tokens), 1))
			assert.Check(t, cmp.Equal(runner.tokens[0].ResourceClass, "my-namespace/my-other-resource-class"))
			assert.Check(t, cmp.Equal(runner.tokens[0].Nickname, "default"))
			assert.Check(t, cmp.Contains(out, "fake-token"))

			assert.Check(t, cmp.Contains(stderr.String(), terms))
		})
	})
}

type runnerMock struct {
	resourceClasses []runner.ResourceClass
	tokens          []runner.Token
}

func (r *runnerMock) CreateResourceClass(resourceClass, desc string) (*runner.ResourceClass, error) {
	rc := runner.ResourceClass{
		ID:            "d8bc155b-5e91-4765-b327-0fa256f0229e",
		ResourceClass: resourceClass,
		Description:   desc,
	}
	r.resourceClasses = append(r.resourceClasses, rc)
	return &rc, nil
}

func (r *runnerMock) GetResourceClassByName(resourceClass string) (*runner.ResourceClass, error) {
	for _, rc := range r.resourceClasses {
		if rc.ResourceClass == resourceClass {
			return &rc, nil
		}
	}
	return nil, errors.New("not found")
}

func (r *runnerMock) GetNamespaceByResourceClass(resourceClass string) (string, error) {
	s := strings.SplitN(resourceClass, "/", 2)
	if len(s) != 2 {
		return "", fmt.Errorf("bad resource class: %q", resourceClass)
	}
	return s[0], nil
}

func (r *runnerMock) GetResourceClassesByNamespace(namespace string) ([]runner.ResourceClass, error) {
	var rcs []runner.ResourceClass
	for _, rc := range r.resourceClasses {
		if strings.Split(rc.ResourceClass, "/")[0] == namespace {
			rcs = append(rcs, rc)
		}
	}
	return rcs, nil
}

func (r *runnerMock) DeleteResourceClass(id string, force bool) error {
	for i, rc := range r.resourceClasses {
		if rc.ID == id {
			r.resourceClasses = append(r.resourceClasses[:i], r.resourceClasses[i+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}

func (r *runnerMock) CreateToken(resourceClass, nickname string) (*runner.Token, error) {
	token := runner.Token{
		ID:            "987905d7-6780-4fed-a637-37277c373629",
		Token:         "fake-token",
		ResourceClass: resourceClass,
		Nickname:      nickname,
		CreatedAt:     time.Now(),
	}
	r.tokens = append(r.tokens, token)
	return &token, nil
}

func (r *runnerMock) GetRunnerTokensByResourceClass(resourceClass string) ([]runner.Token, error) {
	var tokens []runner.Token
	for _, token := range r.tokens {
		if token.ResourceClass == resourceClass {
			tokens = append(tokens, token)
		}
	}
	return tokens, nil
}

func (r *runnerMock) DeleteToken(id string) error {
	for i, token := range r.tokens {
		if token.ID == id {
			r.tokens = append(r.tokens[:i], r.tokens[i+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}

func (r *runnerMock) GetRunnerInstances(_ string) ([]runner.RunnerInstance, error) {
	return nil, nil
}

func (r *runnerMock) reset() {
	r.resourceClasses = nil
	r.tokens = nil
}
