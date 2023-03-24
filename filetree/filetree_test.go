package filetree_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/filetree"
)

var _ = Describe("filetree", func() {
	var (
		tempRoot string
		subDir   string
	)

	BeforeEach(func() {
		var err error
		tempRoot, err = os.MkdirTemp("", "circleci-cli-test-")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tempRoot)).To(Succeed())
	})

	Describe("NewTree", func() {
		var subDirFile, emptyDir string

		BeforeEach(func() {
			subDir = filepath.Join(tempRoot, "sub_dir")
			subDirFile = filepath.Join(tempRoot, "sub_dir", "sub_dir_file.yml")
			emptyDir = filepath.Join(tempRoot, "empty_dir")

			Expect(os.Mkdir(subDir, 0700)).To(Succeed())
			Expect(os.WriteFile(subDirFile, []byte("foo:\n  bar:\n    baz"), 0600)).To(Succeed())
			Expect(os.Mkdir(emptyDir, 0700)).To(Succeed())

		})

		It("Throws an error if content is unmarshallable", func() {
			anotherDir := filepath.Join(tempRoot, "another_dir")
			anotherDirFile := filepath.Join(tempRoot, "another_dir", "another_dir_file.yml")
			Expect(os.Mkdir(anotherDir, 0700)).To(Succeed())
			Expect(os.WriteFile(anotherDirFile, []byte("1some: in: valid: yaml"), 0600)).To(Succeed())
			tree, err := filetree.NewTree(tempRoot)
			Expect(err).ToNot(HaveOccurred())

			_, err = yaml.Marshal(tree)
			Expect(err).To(MatchError("yaml: mapping values are not allowed in this context"))
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

			Expect(tree.Children[0].Info.Name()).To(Equal("empty_dir"))
			Expect(tree.Children[0].FullPath).To(Equal(emptyDir))

			Expect(tree.Children[1].Info.Name()).To(Equal("sub_dir"))
			Expect(tree.Children[1].FullPath).To(Equal(subDir))

			Expect(tree.Children[1].Children).To(HaveLen(1))
			Expect(tree.Children[1].Children[0].Info.Name()).To(Equal("sub_dir_file.yml"))
			Expect(tree.Children[1].Children[0].FullPath).To(Equal(subDirFile))
		})

		It("renders to YAML", func() {
			tree, err := filetree.NewTree(tempRoot)
			Expect(err).ToNot(HaveOccurred())

			out, err := yaml.Marshal(tree)
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(MatchYAML(`empty_dir: {}
sub_dir:
  sub_dir_file:
    foo:
      bar:
        baz
`))
		})

		It("Only checks specified directories", func() {
			tree, err := filetree.NewTree(tempRoot, "sub_dir")
			Expect(err).ToNot(HaveOccurred())

			out, err := yaml.Marshal(tree)
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(MatchYAML(`sub_dir:
  sub_dir_file:
    foo:
      bar:
        baz
`))
		})
	})

	Describe("MarshalYAML", func() {
		var subDir, subDirFile, emptyDir string

		BeforeEach(func() {
			subDir = filepath.Join(tempRoot, "sub_dir")
			subDirFile = filepath.Join(tempRoot, "sub_dir", "sub_dir_file.yml")
			emptyDir = filepath.Join(tempRoot, "empty_dir")

			Expect(os.Mkdir(subDir, 0700)).To(Succeed())
			Expect(os.WriteFile(subDirFile, []byte("foo:\n  bar:\n    baz"), 0600)).To(Succeed())
			Expect(os.Mkdir(emptyDir, 0700)).To(Succeed())

		})
		It("renders to YAML", func() {
			tree, err := filetree.NewTree(tempRoot)
			Expect(err).ToNot(HaveOccurred())

			out, err := yaml.Marshal(tree)
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(MatchYAML(`empty_dir: {}
sub_dir:
  sub_dir_file:
    foo:
      bar:
        baz
`))
		})
	})
})

func TestCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Filetree Suite")
}
