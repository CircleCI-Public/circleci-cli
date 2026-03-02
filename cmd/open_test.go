package cmd

import (
	"testing"

	"github.com/CircleCI-Public/circleci-cli/git"
	"gotest.tools/v3/assert"
)

func TestProjectUrl(t *testing.T) {
	t.Run("can build urls", func(t *testing.T) {
		got := projectUrl(&git.Remote{
			Project:      "foo",
			Organization: "bar",
			VcsType:      git.GitHub,
		})
		assert.Equal(t, got, "https://app.circleci.com/pipelines/github/bar/foo")
	})

	t.Run("escapes garbage", func(t *testing.T) {
		got := projectUrl(&git.Remote{
			Project:      "/one/two",
			Organization: "%^&*()[]",
			VcsType:      git.Bitbucket,
		})
		assert.Equal(t, got, "https://app.circleci.com/pipelines/bitbucket/%25%5E&%2A%28%29%5B%5D/%2Fone%2Ftwo")
	})
}
