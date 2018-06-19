package filetree

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/mitchellh/mapstructure"
	yaml "gopkg.in/yaml.v2"
)

// Node represents a leaf in the filetree
type Node struct {
	FullPath string      `json:"full_path"`
	Info     os.FileInfo `json:"info"`
	Children []*Node     `json:"children"`
	Parent   *Node       `json:"-"`
}

func (n Node) MarshalYAML() (interface{}, error) {
	if len(n.Children) == 0 {
		return n.marshalLeaf()
	} else {
		return n.marshalParent()
	}
}

func (n Node) marshalParent() (interface{}, error) {
	tree := map[string]interface{}{}
	for _, child := range n.Children {
		c, err := child.MarshalYAML()
		if err != nil {
			return tree, err
		}

		if len(child.siblings()) > 0 && child.onlyFile() {
			find := make(map[string]interface{})
			if err := mapstructure.Decode(c, &find); err != nil {
				panic(err)
			}
			tree[child.Parent.Info.Name()] = find
		} else {
			tree[child.Info.Name()] = c
		}
	}

	return tree, nil
}

// Returns true/false if this node is the only file of it's siblings
func (n Node) onlyFile() bool {
	if n.Info.IsDir() {
		return false
	}
	for _, v := range n.siblings() {
		if v.Info.IsDir() {
			return true
		}
	}
	return false
}

func (n Node) siblings() []*Node {
	siblings := []*Node{}
	for _, child := range n.Parent.Children {
		if child != &n {
			siblings = append(siblings, child)
		}
	}
	return siblings
}

func (n Node) marshalLeaf() (interface{}, error) {
	var content interface{}
	if n.Info.IsDir() {
		return content, nil
	}

	buf, err := ioutil.ReadFile(n.FullPath)
	if err != nil {
		return content, err
	}

	err = yaml.Unmarshal(buf, &content)

	return content, err
}

// Helper function that returns true if a path exists in excludes array
func excluded(exclude []string, path string) bool {
	for _, n := range exclude {
		if path == n {
			return true
		}
	}
	return false
}

// NewTree creates a new filetree starting at the root
func NewTree(root string) (*Node, error) {
	parents := make(map[string]*Node)
	var result *Node

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip any dotfiles automatically
		re := regexp.MustCompile(`^\..+`)
		if re.MatchString(info.Name()) {
			// Turn off logging to stdout in this package
			// fmt.Printf("Skipping: %+v\n", info.Name())
			return filepath.SkipDir
		}

		// check if file is in exclude slice and skip it
		// need to pass this in as an array
		exclude := []string{"path/to/skip"}
		if excluded(exclude, path) {
			//fmt.Printf("skipping: %+v \n", info.Name())
			return filepath.SkipDir
		}

		parents[path] = &Node{
			FullPath: path,
			Info:     info,
			Children: make([]*Node, 0),
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	for path, node := range parents {
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
