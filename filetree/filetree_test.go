package filetree_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
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
		var rootFile, subDir, subDirFile string

		BeforeEach(func() {
			rootFile = filepath.Join(tempRoot, "root_file")
			subDir = filepath.Join(tempRoot, "sub_dir")
			subDirFile = filepath.Join(tempRoot, "sub_dir", "sub_dir_file")

			Expect(ioutil.WriteFile(rootFile, []byte{}, 0600)).To(Succeed())
			Expect(os.Mkdir(subDir, 0700)).To(Succeed())
			Expect(ioutil.WriteFile(subDirFile, []byte{}, 0600)).To(Succeed())
		})

		It("Builds a tree of the nested file-structure", func() {
			tree, err := filetree.NewTree(tempRoot)

			Expect(err).ToNot(HaveOccurred())
			Expect(tree.FullPath).To(Equal(tempRoot))
			Expect(tree.Info.Name()).To(Equal(filepath.Base(tempRoot)))

			Expect(tree.Children).To(HaveLen(2))
			sort.Slice(tree.Children, func(i, j int) bool {
				return tree.Children[i].FullPath < tree.Children[j].FullPath
			})
			Expect(tree.Children[0].Info.Name()).To(Equal("root_file"))
			Expect(tree.Children[0].FullPath).To(Equal(rootFile))
			Expect(tree.Children[1].Info.Name()).To(Equal("sub_dir"))
			Expect(tree.Children[1].FullPath).To(Equal(subDir))

			Expect(tree.Children[1].Children).To(HaveLen(1))
			Expect(tree.Children[1].Children[0].Info.Name()).To(Equal("sub_dir_file"))
			Expect(tree.Children[1].Children[0].FullPath).To(Equal(subDirFile))
		})
	})
})

func TestCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Filetree Suite")
}
