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

package function

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
)

// blockKey is the top-level config key holding function declarations.
const blockKey = "functions"

var aliasPattern = regexp.MustCompile(`\A[A-Za-z][A-Za-z0-9_-]*\z`)

// Pin is one entry in the functions: block. Alias is the key, and the name a
// step uses to invoke the function.
type Pin struct {
	Alias    string `json:"alias"`
	Function string `json:"function"`
	Version  string `json:"version"`
}

// ListPins returns every entry in the functions: block, in config order.
//
// A functions: key that is not a map of declarations yields nothing rather than
// an error, so a config using the name to hold YAML anchors keeps working.
func ListPins(configPath string) ([]Pin, error) {
	doc, err := readConfig(configPath)
	if err != nil {
		return nil, err
	}

	block := resolve(findMappingValue(doc, blockKey))
	if block == nil || block.Kind != yaml.MappingNode {
		return nil, nil
	}

	pins := make([]Pin, 0, len(block.Content)/2)
	for i := 0; i+1 < len(block.Content); i += 2 {
		value := resolve(block.Content[i+1])
		if value == nil || value.Kind != yaml.ScalarNode {
			continue
		}
		pin := Pin{Alias: block.Content[i].Value, Function: value.Value}
		if path, version, ok := splitReference(value.Value); ok {
			pin.Function, pin.Version = path, version
		}
		pins = append(pins, pin)
	}
	return pins, nil
}

// splitReference reads a "<path>@<version>" value. A malformed value is
// reported as-is rather than hidden: config validation will reject it too.
func splitReference(value string) (path, version string, ok bool) {
	for i := len(value) - 1; i >= 0; i-- {
		if value[i] != '@' {
			continue
		}
		path, version = value[:i], value[i+1:]
		if ValidatePath(path) == nil && ValidateVersion(version) == nil {
			return path, version, true
		}
		return "", "", false
	}
	return "", "", false
}

func readConfig(configPath string) (*yaml.Node, error) {
	data, err := os.ReadFile(configPath) //nolint:gosec // configPath is user-supplied via --config
	if err != nil {
		if os.IsNotExist(err) {
			return nil, clierrors.New("function.config_not_found", "Config file not found",
				fmt.Sprintf("No config file at %q.", configPath)).
				WithSuggestions(
					"Run this from a repository with a .circleci/config.yml",
					"Point at the file explicitly: --config path/to/config.yml",
				).
				WithExitCode(clierrors.ExitNotFound)
		}
		return nil, err
	}

	var parsed yaml.Node
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, clierrors.New("function.config_invalid", "Config file could not be parsed",
			fmt.Sprintf("%s is not valid YAML: %s", configPath, err)).
			WithSuggestions("Run 'circleci config validate' to see what is wrong").
			WithExitCode(clierrors.ExitValidationFail)
	}
	if len(parsed.Content) == 0 {
		return nil, clierrors.New("function.config_empty", "Config file is empty",
			fmt.Sprintf("%s has no content.", configPath)).
			WithExitCode(clierrors.ExitValidationFail)
	}
	return parsed.Content[0], nil
}

func findMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// resolve follows an anchor reference to the node it points at.
func resolve(n *yaml.Node) *yaml.Node {
	if n != nil && n.Kind == yaml.AliasNode {
		return n.Alias
	}
	return n
}

// Reference is a pinned function reference as the config records it.
type Reference struct {
	Path    string
	Version string
}

func (r Reference) String() string { return r.Path + "@" + r.Version }

// ValidateAlias reports whether alias can key an entry in the functions:
// block. A '/' is rejected because the config reads it as naming one of the
// function's commands.
func ValidateAlias(alias string) error {
	if aliasPattern.MatchString(alias) {
		return nil
	}
	return clierrors.New("function.invalid_alias", "Invalid function alias",
		fmt.Sprintf("Alias %q is not valid. Expected a letter followed by letters, digits, '-' or '_'.", alias)).
		WithSuggestions("Pass a simpler name: circleci function add <name> --as my-function").
		WithExitCode(clierrors.ExitBadArguments)
}

// AliasFor is the default alias for a function path: its last segment.
func AliasFor(functionPath string) string {
	return path.Base(functionPath)
}

// CheckAdd reports whether AddPin could add alias, without writing anything.
func CheckAdd(configPath, alias string) error {
	doc, err := readConfig(configPath)
	if err != nil {
		return err
	}
	return checkAddable(doc, configPath, alias)
}

// AddPin adds alias → ref to the functions: block, creating the block when the
// config has none. It fails if alias is already declared.
func AddPin(configPath, alias string, ref Reference) error {
	data, doc, mode, err := loadForEdit(configPath)
	if err != nil {
		return err
	}
	if err := checkAddable(doc, configPath, alias); err != nil {
		return err
	}

	entry := alias + ": " + ref.String()
	block := findMappingValue(doc, blockKey)
	if block == nil {
		return write(configPath, appendBlock(data, entry), mode)
	}
	if emptyBlock(block) {
		return write(configPath, insertLines(data, block.Line, "  "+entry), mode)
	}
	indent := strings.Repeat(" ", block.Content[0].Column-1)
	return write(configPath, insertLines(data, blockEnd(block), indent+entry), mode)
}

func checkAddable(doc *yaml.Node, configPath, alias string) error {
	// Deliberately not resolved: an aliased block lives elsewhere in the file,
	// and appending to it would edit whatever else points at the same anchor.
	block := findMappingValue(doc, blockKey)
	switch {
	case block == nil, emptyBlock(block):
		return nil
	case block.Kind == yaml.MappingNode && block.Style == 0:
		if findMappingValue(block, alias) != nil {
			return clierrors.New("function.already_pinned", "Function already declared",
				fmt.Sprintf("Alias %q already has an entry in the functions: block.", alias)).
				WithSuggestions(
					"Pin it under a different alias: circleci function add <name> --as <alias>",
				).
				WithExitCode(clierrors.ExitBadArguments)
		}
		return nil
	default:
		return notDeclarationsErr(configPath)
	}
}

// emptyBlock reports whether the functions: key has no value at all — written
// bare, with at most a comment after the colon — so entries can go under it.
// An explicit null or ~ is a value, and is left alone.
func emptyBlock(block *yaml.Node) bool {
	return block.Kind == yaml.ScalarNode && block.Tag == "!!null" && block.Value == ""
}

func notDeclarationsErr(configPath string) error {
	return clierrors.New("function.block_not_declarations", "functions: is not a map of declarations",
		fmt.Sprintf("%s has a functions: key that is not a plain map of alias to reference, so there is nowhere to add one.", configPath)).
		WithSuggestions("Add the entry by hand, or move the existing value under a different key").
		WithExitCode(clierrors.ExitValidationFail)
}

// loadForEdit reads the config for a rewrite, returning its bytes alongside the
// parsed document. The bytes are what gets edited: the parse only locates the
// edit, so every byte the edit does not touch survives it — comments, blank
// lines, quoting and indentation alike.
func loadForEdit(configPath string) ([]byte, *yaml.Node, os.FileMode, error) {
	info, err := os.Stat(configPath) //nolint:gosec // configPath is user-supplied via --config
	if err != nil {
		if os.IsNotExist(err) {
			_, err = readConfig(configPath) // for the structured error
			return nil, nil, 0, err
		}
		return nil, nil, 0, err
	}

	data, err := os.ReadFile(configPath) //nolint:gosec // same
	if err != nil {
		return nil, nil, 0, err
	}

	doc, err := readConfig(configPath)
	if err != nil {
		return nil, nil, 0, err
	}
	if doc.Kind != yaml.MappingNode {
		return nil, nil, 0, clierrors.New("function.config_invalid", "Config file could not be parsed",
			fmt.Sprintf("%s does not contain a YAML mapping at the top level.", configPath)).
			WithExitCode(clierrors.ExitValidationFail)
	}
	return data, doc, info.Mode(), nil
}

func write(configPath string, data []byte, mode os.FileMode) error {
	return os.WriteFile(configPath, data, mode) //nolint:gosec // configPath is user-supplied via --config
}

// blockEnd is the last line belonging to a block mapping: its final entry, plus
// any comment lines indented under it, so a new entry lands at the end of the
// block rather than above a trailing comment.
func blockEnd(block *yaml.Node) int {
	end := 0
	for _, n := range block.Content {
		if l := lastLine(n); l > end {
			end = l
		}
	}
	return end
}

// lastLine is the highest line any part of n occupies.
func lastLine(n *yaml.Node) int {
	last := n.Line
	if n.Kind == yaml.ScalarNode {
		// A multi-line scalar reports only its first line.
		last += strings.Count(strings.TrimSuffix(n.Value, "\n"), "\n")
	}
	for _, c := range n.Content {
		if l := lastLine(c); l > last {
			last = l
		}
	}
	return last
}

// appendBlock adds a functions: block at the end of the file, separated from
// what precedes it by one blank line.
func appendBlock(data []byte, entry string) []byte {
	lines := splitLines(data)
	eol := fileEOL(lines)
	if n := len(lines); n > 0 {
		// Terminate the file, so the new block starts on its own line.
		if lineEOL(lines[n-1]) == "" {
			lines[n-1] += eol
		}
	}
	if n := len(lines); n > 0 && strings.TrimSpace(lines[n-1]) != "" {
		lines = append(lines, eol)
	}
	return joinLines(append(lines, blockKey+":"+eol, "  "+entry+eol))
}

// fileEOL is the line terminator the file already uses, so an appended line
// does not leave a CRLF file with mixed endings.
func fileEOL(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		if eol := lineEOL(lines[i]); eol != "" {
			return eol
		}
	}
	return "\n"
}

// insertLines returns data with lines inserted after the given 1-based line.
func insertLines(data []byte, after int, lines ...string) []byte {
	existing := splitLines(data)
	if after < 0 || after > len(existing) {
		after = len(existing)
	}

	eol := "\n"
	if after > 0 {
		if e := lineEOL(existing[after-1]); e != "" {
			eol = e
		} else {
			// Inserting after an unterminated final line: terminate it first.
			existing[after-1] += eol
		}
	}

	inserted := make([]string, 0, len(lines))
	for _, l := range lines {
		inserted = append(inserted, l+eol)
	}
	return joinLines(slices.Insert(existing, after, inserted...))
}

// splitLines splits data into lines, each keeping its own line terminator, so
// joining them reproduces the input byte for byte.
func splitLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	lines := strings.SplitAfter(string(data), "\n")
	// SplitAfter yields a trailing empty element for data ending in a newline.
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func joinLines(lines []string) []byte {
	return []byte(strings.Join(lines, ""))
}

// lineEOL is a line's terminator, empty for an unterminated final line.
func lineEOL(line string) string {
	switch {
	case strings.HasSuffix(line, "\r\n"):
		return "\r\n"
	case strings.HasSuffix(line, "\n"):
		return "\n"
	default:
		return ""
	}
}
