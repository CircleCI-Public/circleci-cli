package references_test

import (
	"github.com/CircleCI-Public/circleci-cli/references"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Parsing Orbs", func() {

	It("Should parse dev versions", func() {
		Expect(references.IsDevVersion("dev:master")).Should(BeTrue())
		Expect(references.IsDevVersion("1.2.1")).Should(BeFalse())
		Expect(references.IsDevVersion("")).Should(BeFalse())
	})

	It("Should parse full references", func() {
		namespace, orb, version, err := references.SplitIntoOrbNamespaceAndVersion("foo/bar@dev:baz")
		Expect(namespace).To(Equal("foo"))
		Expect(orb).To(Equal("bar"))
		Expect(version).To(Equal("dev:baz"))
		Expect(err).To(BeNil())

		_, _, _, err = references.SplitIntoOrbNamespaceAndVersion("asdasd")

		Expect(err).Should(MatchError("Invalid orb reference 'asdasd': Expected a namespace, orb and version in the format 'namespace/orb@version'"))
	})

	It("Should split namespace and orbs", func() {
		ns, orb, err := references.SplitIntoOrbAndNamespace("cat/dog")
		Expect(ns).To(Equal("cat"))
		Expect(orb).To(Equal("dog"))
		Expect(err).ShouldNot(HaveOccurred())

		_, _, err = references.SplitIntoOrbAndNamespace("catdog")
		Expect(err).Should(MatchError("Invalid orb catdog. Expected a namespace and orb in the form 'namespace/orb'"))

	})
})
