package filetree_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"gotest.tools/v3/assert"
	"gopkg.in/yaml.v3"

	"github.com/CircleCI-Public/circleci-cli/filetree"
)

func setupTempRoot(t *testing.T) string {
	t.Helper()
	tempRoot, err := os.MkdirTemp("", "circleci-cli-test-")
	assert.NilError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempRoot) })
	return tempRoot
}

func setupTreeDirs(t *testing.T, tempRoot string) (subDir, subDirFile, emptyDir string) {
	t.Helper()
	subDir = filepath.Join(tempRoot, "sub_dir")
	subDirFile = filepath.Join(tempRoot, "sub_dir", "sub_dir_file.yml")
	emptyDir = filepath.Join(tempRoot, "empty_dir")

	assert.NilError(t, os.Mkdir(subDir, 0700))
	assert.NilError(t, os.WriteFile(subDirFile, []byte("foo:\n  bar:\n    baz"), 0600))
	assert.NilError(t, os.Mkdir(emptyDir, 0700))
	return
}

func assertYAMLEqual(t *testing.T, got []byte, wantYAML string) {
	t.Helper()
	var gotMap, wantMap interface{}
	assert.NilError(t, yaml.Unmarshal(got, &gotMap))
	assert.NilError(t, yaml.Unmarshal([]byte(wantYAML), &wantMap))
	assert.DeepEqual(t, gotMap, wantMap)
}

func TestNewTreeUnmarshallableContent(t *testing.T) {
	tempRoot := setupTempRoot(t)
	setupTreeDirs(t, tempRoot)

	anotherDir := filepath.Join(tempRoot, "another_dir")
	anotherDirFile := filepath.Join(tempRoot, "another_dir", "another_dir_file.yml")
	assert.NilError(t, os.Mkdir(anotherDir, 0700))
	assert.NilError(t, os.WriteFile(anotherDirFile, []byte("1some: in: valid: yaml"), 0600))

	tree, err := filetree.NewTree(tempRoot)
	assert.NilError(t, err)

	_, err = yaml.Marshal(tree)
	assert.Error(t, err, "yaml: mapping values are not allowed in this context")
}

func TestNewTreeBuildsNestedStructure(t *testing.T) {
	tempRoot := setupTempRoot(t)
	subDir, subDirFile, emptyDir := setupTreeDirs(t, tempRoot)

	tree, err := filetree.NewTree(tempRoot)
	assert.NilError(t, err)
	assert.Equal(t, tree.FullPath, tempRoot)
	assert.Equal(t, tree.Info.Name(), filepath.Base(tempRoot))

	assert.Equal(t, len(tree.Children), 2)
	sort.Slice(tree.Children, func(i, j int) bool {
		return tree.Children[i].FullPath < tree.Children[j].FullPath
	})

	assert.Equal(t, tree.Children[0].Info.Name(), "empty_dir")
	assert.Equal(t, tree.Children[0].FullPath, emptyDir)

	assert.Equal(t, tree.Children[1].Info.Name(), "sub_dir")
	assert.Equal(t, tree.Children[1].FullPath, subDir)

	assert.Equal(t, len(tree.Children[1].Children), 1)
	assert.Equal(t, tree.Children[1].Children[0].Info.Name(), "sub_dir_file.yml")
	assert.Equal(t, tree.Children[1].Children[0].FullPath, subDirFile)
}

func TestNewTreeRendersToYAML(t *testing.T) {
	tempRoot := setupTempRoot(t)
	setupTreeDirs(t, tempRoot)

	tree, err := filetree.NewTree(tempRoot)
	assert.NilError(t, err)

	out, err := yaml.Marshal(tree)
	assert.NilError(t, err)
	assertYAMLEqual(t, out, `empty_dir: {}
sub_dir:
  sub_dir_file:
    foo:
      bar:
        baz
`)
}

func TestNewTreeOnlyChecksSpecifiedDirs(t *testing.T) {
	tempRoot := setupTempRoot(t)
	setupTreeDirs(t, tempRoot)

	tree, err := filetree.NewTree(tempRoot, "sub_dir")
	assert.NilError(t, err)

	out, err := yaml.Marshal(tree)
	assert.NilError(t, err)
	assertYAMLEqual(t, out, `sub_dir:
  sub_dir_file:
    foo:
      bar:
        baz
`)
}

func TestMarshalYAML(t *testing.T) {
	tempRoot := setupTempRoot(t)
	setupTreeDirs(t, tempRoot)

	tree, err := filetree.NewTree(tempRoot)
	assert.NilError(t, err)

	out, err := yaml.Marshal(tree)
	assert.NilError(t, err)
	assertYAMLEqual(t, out, `empty_dir: {}
sub_dir:
  sub_dir_file:
    foo:
      bar:
        baz
`)
}
