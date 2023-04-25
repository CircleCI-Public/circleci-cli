package filetree

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v3"
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

func (n Node) specialCase() bool {
	re := regexp.MustCompile(`^@.*\.(yml|yaml)$`)
	return re.MatchString(n.basename())
}

func (n Node) marshalParent() (interface{}, error) {
	subtree := map[string]interface{}{}
	for _, child := range n.Children {
		c, err := child.MarshalYAML()

		switch c.(type) {
		case map[string]interface{}, map[interface{}]interface{}, nil:
			if err != nil {
				return subtree, err
			}

			if child.rootFile() {
				merged := mergeTree(subtree, c)
				subtree = merged
			} else if child.specialCase() {
				merged := mergeTree(subtree, subtree[child.Parent.name()], c)
				subtree = merged
			} else {
				merged := mergeTree(subtree[child.name()], c)
				subtree[child.name()] = merged
			}
		default:
			return nil, fmt.Errorf("expected a map, got a `%T` which is not supported at this time for \"%s\"", c, child.FullPath)
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

	// TODO: this check may be unnecessary with the isYaml check
	if n.Info.IsDir() {
		return content, nil
	}
	if !isYaml(n.Info) {
		return content, nil
	}

	buf, err := os.ReadFile(n.FullPath)

	if err != nil {
		return content, err
	}

	err = yaml.Unmarshal(buf, &content)

	return content, err
}

func dotfile(info os.FileInfo) bool {
	re := regexp.MustCompile(`^\..+`)
	return re.MatchString(info.Name())
}

func dotfolder(info os.FileInfo) bool {
	return info.IsDir() && dotfile(info)
}

func isYaml(info os.FileInfo) bool {
	re := regexp.MustCompile(`.+\.(yml|yaml)$`)
	return re.MatchString(info.Name())
}

// PathNodes is a map of filepaths to tree nodes with ordered path keys.
type PathNodes struct {
	Map  map[string]*Node
	Keys []string
}

func buildTree(absRootPath string, pathNodes PathNodes) *Node {
	var rootNode *Node

	for _, path := range pathNodes.Keys {
		node := pathNodes.Map[path]
		// skip dotfile nodes that aren't the root path
		if absRootPath != path && node.Info.Mode().IsRegular() {
			if dotfile(node.Info) || !isYaml(node.Info) {
				continue
			}
		}
		parentPath := filepath.Dir(path)
		parent, exists := pathNodes.Map[parentPath]
		if exists {
			node.Parent = parent
			parent.Children = append(parent.Children, node)
		} else {
			rootNode = node
		}
	}

	return rootNode
}

func collectNodes(absRootPath string, allowedDirs map[string]string) (PathNodes, error) {
	pathNodes := PathNodes{}
	pathNodes.Map = make(map[string]*Node)
	pathNodes.Keys = []string{}

	err := filepath.Walk(absRootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip any dotfolders or dirs not explicitly allowed, if available.
		isAllowed := true
		if len(allowedDirs) > 0 && info.IsDir() {
			_, isAllowed = allowedDirs[info.Name()]
		}
		if absRootPath != path && (dotfolder(info) || !isAllowed) {
			// Turn off logging to stdout in this package
			//fmt.Printf("Skipping dotfolder: %+v\n", path)
			return filepath.SkipDir
		}

		fp, err := filepath.Abs(path)
		if err != nil {
			return err
		}

		pathNodes.Keys = append(pathNodes.Keys, path)
		pathNodes.Map[path] = &Node{
			FullPath: fp,
			Info:     info,
			Children: make([]*Node, 0),
		}
		return nil
	})

	return pathNodes, err
}

// NewTree creates a new filetree starting at the root
func NewTree(rootPath string, allowedDirectories ...string) (*Node, error) {
	allowedDirs := make(map[string]string)

	for _, dir := range allowedDirectories {
		allowedDirs[dir] = "1"
	}

	absRootPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}

	pathNodes, err := collectNodes(absRootPath, allowedDirs)
	if err != nil {
		return nil, err
	}

	rootNode := buildTree(absRootPath, pathNodes)

	return rootNode, err
}
