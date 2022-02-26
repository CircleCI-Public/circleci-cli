package update_test

import (
	"github.com/blang/semver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/CircleCI-Public/circleci-cli/update"
)

var _ = Describe("Homebrew Version Parsing", func() {

	It("Should parse simple versions", func() {
		version, err := update.ParseHomebrewVersion("1.0.0")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(version.String()).To(Equal("1.0.0"))
	})

	It("Should parse strings with revisions", func() {
		version, err := update.ParseHomebrewVersion("0.1.15410_1")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(version.String()).To(Equal("0.1.15410-1"))
		Expect(version.Pre).To(HaveLen(1))
		Expect(version.Pre[0]).To(Equal(semver.PRVersion{VersionNum: 1, IsNum: true}))
	})

	It("Should can deal with garbage", func() {
		_, err := update.ParseHomebrewVersion("asdad.1231.-_")
		Expect(err).To(MatchError(MatchRegexp("failed to parse current version")))
		Expect(err).To(MatchError(MatchRegexp("asdad.1231.-_")))
	})
})
