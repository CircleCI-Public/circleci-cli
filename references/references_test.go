package references_test

import (
	"github.com/CircleCI-Public/circleci-cli/references"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Parsing Orbs", func() {

	It("Should parse dev versions", func() {
		Expect(references.IsDevVersion("dev:master")).To(BeTrue())
		Expect(references.IsDevVersion("1.2.1")).To(BeFalse())
		Expect(references.IsDevVersion("")).To(BeFalse())
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

	It("Should split correctly when dev label contains a fwslash", func() {
		ns, orb, version, err := references.SplitIntoOrbNamespaceAndVersion("foo/bar@dev:bah/bah")
		Expect(ns).To(Equal("foo"))
		Expect(orb).To(Equal("bar"))
		Expect(version).To(Equal("dev:bah/bah"))
		Expect(err).ShouldNot(HaveOccurred())
	})
})
