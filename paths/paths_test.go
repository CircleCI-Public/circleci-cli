package paths

import (
	"github.com/CircleCI-Public/circleci-cli/git"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("open", func() {
	It("can build urls", func() {
		Expect(ProjectUrl(&git.Remote{
			Project:      "foo",
			Organization: "bar",
			VcsType:      git.GitHub,
		})).To(Equal("https://app.circleci.com/pipelines/github/bar/foo"))
	})

	It("escapes garbage", func() {
		Expect(ProjectUrl(&git.Remote{
			Project:      "/one/two",
			Organization: "%^&*()[]",
			VcsType:      git.Bitbucket,
		})).To(Equal("https://app.circleci.com/pipelines/bitbucket/%25%5E&%2A%28%29%5B%5D/%2Fone%2Ftwo"))
	})
})
