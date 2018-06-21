package filetree

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mitchellh/mapstructure"
	yaml "gopkg.in/yaml.v2"
)

// This is a quick hack of a function to convert interfaces
// into to map[string]interface{} and combine them.
func mergeTree(trees ...interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, tree := range trees {
		kvp := make(map[string]interface{})
		if err := mapstructure.Decode(tree, &kvp); err != nil {
			panic(err)
		}
		for k, v := range kvp {
			result[k] = v
		}
	}
	return result
}

// SpecialCase is a function you can pass to NewTree
// in order to override the behavior when marshalling a node.
var SpecialCase func(path string) bool

// Node represents a leaf in the filetree
type Node struct {
	FullPath string      `json:"full_path"`
	Info     os.FileInfo `json:"-"`
	Children []*Node     `json:"-"`
	Parent   *Node       `json:"-"`
}

// MarshalYAML serializes the tree into YAML
func (n Node) MarshalYAML() (interface{}, error) {
	if len(n.Children) == 0 {
		return n.marshalLeaf()
	}
	return n.marshalParent()
}

func (n Node) basename() string {
	return n.Info.Name()
}

func (n Node) name() string {
	return strings.TrimSuffix(n.basename(), filepath.Ext(n.basename()))
}

func (n Node) rootFile() bool {
	return n.Info.Mode().IsRegular() && n.root() == n.Parent
}

func (n Node) isYaml() bool {
	re := regexp.MustCompile(`.+\.(yml|yaml)$`)
	return re.MatchString(n.FullPath)
}

func (n Node) marshalParent() (interface{}, error) {
	subtree := map[string]interface{}{}
	for _, child := range n.Children {
		c, err := child.MarshalYAML()
		if err != nil {
			return subtree, err
		}

		if child.rootFile() {
			merged := mergeTree(subtree, c)
			subtree = merged
		} else if SpecialCase(child.basename()) {
			merged := mergeTree(subtree, subtree[child.Parent.name()], c)
			subtree = merged
		} else {
			merged := mergeTree(subtree[child.name()], c)
			subtree[child.name()] = merged
		}
	}

	return subtree, nil
}

// Returns the root node
func (n Node) root() *Node {
	root := n.Parent

	for root.Parent != nil {
		root = root.Parent
	}

	return root
}

func (n Node) marshalLeaf() (interface{}, error) {
	var content interface{}
	if n.Info.IsDir() {
		return content, nil
	}
	if !n.isYaml() {
		return content, nil
	}

	buf, err := ioutil.ReadFile(n.FullPath)

	if err != nil {
		return content, err
	}

	err = yaml.Unmarshal(buf, &content)

	return content, err
}

func dotfile(path string) bool {
	re := regexp.MustCompile(`^\..+`)
	return re.MatchString(path)
}

func dotfolder(info os.FileInfo) bool {
	return info.IsDir() && dotfile(info.Name())
}

// NewTree creates a new filetree starting at the root
func NewTree(root string, specialCase func(path string) bool) (*Node, error) {
	SpecialCase = specialCase
	parents := make(map[string]*Node)
	var (
		result   *Node
		pathKeys []string
	)
	absroot, err := filepath.Abs(root)
	if err != nil {
		return result, err
	}

	err = filepath.Walk(absroot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip any dotfolders that aren't the root path
		if absroot != path && dotfolder(info) {
			// Turn off logging to stdout in this package
			//fmt.Printf("Skipping dotfolder: %+v\n", path)
			return filepath.SkipDir
		}

		fp, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		pathKeys = append(pathKeys, path)
		parents[path] = &Node{
			FullPath: fp,
			Info:     info,
			Children: make([]*Node, 0),
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	for _, path := range pathKeys {
		node := parents[path]
		// skip dotfile nodes that aren't the root path
		if absroot != path && dotfile(node.Info.Name()) {
			continue
		}
		parentPath := filepath.Dir(path)
		parent, exists := parents[parentPath]
		if exists {
			node.Parent = parent
			parent.Children = append(parent.Children, node)
		} else {
			result = node
		}

	}

	return result, err
}
