package git

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Dealing with git", func() {

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
				"git@github.com:foobar/foo-service.git": &Remote{
					VcsType:      GitHub,
					Organization: "foobar",
					Project:      "foo-service",
				},

				"git@bitbucket.org:example/makefile_sh.git": &Remote{
					VcsType:      Bitbucket,
					Organization: "example",
					Project:      "makefile_sh",
				},

				"https://github.com/apple/pear.git": &Remote{
					VcsType:      GitHub,
					Organization: "apple",
					Project:      "pear",
				},

				"git@bitbucket.org:example/makefile_sh": &Remote{
					VcsType:      Bitbucket,
					Organization: "example",
					Project:      "makefile_sh",
				},

				"https://example@bitbucket.org/kiwi/fruit.git": &Remote{
					VcsType:      Bitbucket,
					Organization: "kiwi",
					Project:      "fruit",
				},

				"https://example@bitbucket.org/kiwi/fruit": &Remote{
					VcsType:      Bitbucket,
					Organization: "kiwi",
					Project:      "fruit",
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
