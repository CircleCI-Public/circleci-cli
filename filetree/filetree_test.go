package filetree_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/circleci/circleci-cli/filetree"
)

var _ = Describe("filetree", func() {
	var (
		tempRoot string
	)

	BeforeEach(func() {
		var err error
		tempRoot, err = ioutil.TempDir("", "circleci-cli-test-")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tempRoot)).To(Succeed())
	})

	Describe("NewTree", func() {
		BeforeEach(func() {
			var err error
			_, err = os.OpenFile(
				filepath.Join(tempRoot, "foo"),
				os.O_RDWR|os.O_CREATE,
				0600,
			)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Builds a tree of the nested file-structure", func() {
			tree, err := filetree.NewTree(tempRoot)

			Expect(err).ToNot(HaveOccurred())
			Expect(tree.FullPath).To(Equal(tempRoot))
			Expect(tree.Info.Name()).To(Equal(filepath.Base(tempRoot)))

			fooPath := filepath.Join(tempRoot, "foo")
			Expect(tree.Children[0].Info.Name()).To(Equal("foo"))
			Expect(tree.Children[0].FullPath).To(Equal(fooPath))
		})
	})
})

func TestCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Filetree Suite")
}
