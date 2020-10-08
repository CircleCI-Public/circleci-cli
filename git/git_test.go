package git

import (
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Dealing with git", func() {

	Context("running commands", func() {

		It("returns output in the happy path", func() {
			Expect(commandOutputOrDefault(
				exec.Command("echo", "hello"), "goodbye",
			)).To(Equal("hello"))
		})

		It("handles programs that exit with a failure", func() {

			Expect(commandOutputOrDefault(
				exec.Command("git", "this", "it", "not", "a", "command"), "goodbye",
			)).To(Equal("goodbye"))
		})

		It("handles invalid programs", func() {

			Expect(commandOutputOrDefault(
				exec.Command("this/is/not/a/command"), "morning",
			)).To(Equal("morning"))
		})

	})

	Context("can read the data to drive pipeline variables", func() {

		if os.Getenv("CIRCLECI") != "true" {
			return
		}

		It("computes the branch, tag and revision", func() {
			Expect(Branch()).To(Equal(os.Getenv("CIRCLE_BRANCH")))
			Expect(Revision()).To(Equal(os.Getenv("CIRCLE_SHA1")))
			Expect(Tag()).To(Equal(os.Getenv("CIRCLE_TAG")))
		})

	})

	Context("remotes", func() {

		Describe("integration tests", func() {

			It("fails gracefully when the remote can't be found", func() {
				// This test will fail if the current working directory has git remote
				// named 'peristeronic'.
				_, err := getRemoteUrl("peristeronic")
				Expect(err).To(MatchError("Error finding the peristeronic git remote: fatal: No such remote 'peristeronic'"))
			})

			It("can read git output", func() {
				Expect(getRemoteUrl("origin")).To(MatchRegexp(`github`))
			})

		})

		It("should parse these", func() {

			cases := map[string]*Remote{
				"git@github.com:foobar/foo-service.git": {
					VcsType:      GitHub,
					Organization: "foobar",
					Project:      "foo-service",
				},

				"git@bitbucket.org:example/makefile_sh.git": {
					VcsType:      Bitbucket,
					Organization: "example",
					Project:      "makefile_sh",
				},

				"https://github.com/apple/pear.git": {
					VcsType:      GitHub,
					Organization: "apple",
					Project:      "pear",
				},

				"git@bitbucket.org:example/makefile_sh": {
					VcsType:      Bitbucket,
					Organization: "example",
					Project:      "makefile_sh",
				},

				"https://example@bitbucket.org/kiwi/fruit.git": {
					VcsType:      Bitbucket,
					Organization: "kiwi",
					Project:      "fruit",
				},

				"https://example@bitbucket.org/kiwi/fruit": {
					VcsType:      Bitbucket,
					Organization: "kiwi",
					Project:      "fruit",
				},

				"ssh://git@github.com/cloud/rain": {
					VcsType:      GitHub,
					Organization: "cloud",
					Project:      "rain",
				},

				"ssh://git@bitbucket.org/snow/ice": {
					VcsType:      Bitbucket,
					Organization: "snow",
					Project:      "ice",
				},
			}

			for url, remote := range cases {
				Expect(findRemote(url)).To(Equal(remote))
			}

		})

		It("should not parse these", func() {

			cases := map[string]string{
				"asd/asd/asd":                "Unknown git remote: asd/asd/asd",
				"git@github.com:foo/bar/baz": "Splitting 'foo/bar/baz' into organization and project failed",
			}
			for url, message := range cases {
				_, err := findRemote(url)
				Expect(err).To(MatchError(message))
			}
		})
	})
})
