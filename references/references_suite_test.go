package references_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestReferences(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "References Suite")
}
