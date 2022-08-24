package runner

import (
	"bytes"
	"testing"
	"time"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/api/runner"
)

func Test_generateConfig(t *testing.T) {
	token := runner.Token{
		ID:            "da73786c-ebbc-4c07-849a-5590f7eef509",
		Token:         "1a34e5519976717fb808ad8900cadbecc686facee3f9ca56c5ba1ad30e50cab7e5fa328409065c64",
		ResourceClass: "the-namespace/the-resource-class",
		Nickname:      "the-nickname",
		CreatedAt:     time.Date(2020, 03, 04, 16, 13, 53, 00, time.UTC),
	}

	b := bytes.Buffer{}
	err := generateConfig(token, &b)
	assert.NilError(t, err)
	golden.Assert(t, b.String(), "expected-config.yaml")
}
