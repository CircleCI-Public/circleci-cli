package filetree

import (
	"fmt"
	"os"
	"path/filepath"
)

type Node struct {
	FullPath string
	Info     *os.FileInfo
	Children []*Node
	Parent   *Node
}

func NewTree(root string) (*Node, error) {
	parents := make(map[string]*Node)
	// need to pass this in as an array
	exclude := []string{"path/to/skip"}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// check if file is in exclude slice and skip it
		name := info.Name()
		_, exists := exclude[name]
		if exists {
			fmt.Printf("skipping a dir without errors: %+v \n", info.Name())
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
		}

	}
	return node, err
}
