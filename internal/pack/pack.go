// Copyright (c) 2026 Circle Internet Services, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT

// Package pack merges a directory tree of YAML files into a single YAML document.
//
// The merge rules are consistent across all CircleCI YAML packing (config and orbs):
//
//   - Files at the pack root are merged at the top level of the output.
//   - Subdirectory names become top-level YAML keys; files inside become entries
//     under that key, keyed by filename without extension.
//   - Files whose names begin with "@" are merged at the current directory level
//     rather than nested under a key (used for base files like @orb.yml).
//   - Dotfiles and non-YAML files are skipped.
//   - Packing a single file round-trips it through the YAML parser (normalising
//     formatting) and returns the result.
package pack

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Pack reads all YAML files under rootPath and merges them into a single YAML
// document. If rootPath is a single file it is parsed and re-serialised.
func Pack(rootPath string) (string, error) {
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("accessing %q: no such file or directory", rootPath)
		}
		return "", fmt.Errorf("accessing %q: %w", rootPath, err)
	}

	var result map[string]any
	if info.IsDir() {
		result, err = packDir(absRoot, absRoot)
	} else {
		result, err = packFile(absRoot)
	}
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	err = enc.Encode(result)
	if err != nil {
		return "", fmt.Errorf("marshaling result: %w", err)
	}
	return buf.String(), nil
}

// packDir recursively merges all YAML files under dirPath.
// absRoot identifies which level is the "root" for top-level merge behaviour.
func packDir(dirPath, absRoot string) (map[string]any, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("reading directory %q: %w", dirPath, err)
	}

	subtree := make(map[string]any)
	isRoot := dirPath == absRoot
	dirBasename := nameWithoutExt(filepath.Base(dirPath))

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(dirPath, name)

		if strings.HasPrefix(name, ".") {
			continue
		}

		if entry.IsDir() {
			childMap, err := packDir(fullPath, absRoot)
			if err != nil {
				return nil, err
			}
			existing, _ := subtree[name].(map[string]any)
			subtree[name] = mergeMaps(existing, childMap)
			continue
		}

		if !isYAMLName(name) {
			continue
		}

		content, err := packFile(fullPath)
		if err != nil {
			return nil, err
		}

		switch {
		case isRoot:
			// Files directly under the pack root are merged at the top level.
			subtree = mergeMaps(subtree, content)
		case isSpecialName(name):
			// @*.yml files are merged at the current directory level rather than
			// nested under a key. The parent-dir key is consulted to pull in any
			// content already accumulated there (matching the original filetree behaviour).
			existing, _ := subtree[dirBasename].(map[string]any)
			subtree = mergeMaps(subtree, existing, content)
		default:
			key := nameWithoutExt(name)
			existing, _ := subtree[key].(map[string]any)
			subtree[key] = mergeMaps(existing, content)
		}
	}
	return subtree, nil
}

// packFile reads and parses a single YAML file, returning its root map.
func packFile(path string) (map[string]any, error) {
	b, err := os.ReadFile(path) //#nosec:G304 // path is a resolved filesystem path under the user-supplied pack root directory
	if err != nil {
		return nil, fmt.Errorf("reading %q: %w", path, err)
	}
	var v any
	if err := yaml.Unmarshal(b, &v); err != nil {
		return nil, fmt.Errorf("parsing %q: %w", path, err)
	}
	if v == nil {
		return make(map[string]any), nil
	}
	m := toStringMap(v)
	if m == nil {
		return nil, fmt.Errorf("%q must have a YAML map at the root, got %T", path, v)
	}
	return m, nil
}

// mergeMaps shallow-merges any number of maps into a new map[string]any.
// nil inputs are silently skipped.
func mergeMaps(maps ...any) map[string]any {
	result := make(map[string]any)
	for _, m := range maps {
		if m == nil {
			continue
		}
		for k, v := range toStringMap(m) {
			result[k] = v
		}
	}
	return result
}

// toStringMap converts map[string]any or map[any]any (yaml.v2 legacy) to map[string]any.
func toStringMap(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		return m
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			if ks, ok := k.(string); ok {
				out[ks] = val
			}
		}
		return out
	}
	return nil
}

var (
	yamlRe    = regexp.MustCompile(`.+\.(yml|yaml)$`)
	specialRe = regexp.MustCompile(`^@.+\.(yml|yaml)$`)
)

func isYAMLName(name string) bool    { return yamlRe.MatchString(name) }
func isSpecialName(name string) bool { return specialRe.MatchString(name) }

func nameWithoutExt(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}
