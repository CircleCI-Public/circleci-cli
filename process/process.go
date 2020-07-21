package process

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
)

// IncludeFile replaces intsances of <<include(file)>> with the contents
// of "file", escaping instances of "<<" within the file before returning.
func IncludeFile(template string, root string) (string, error) {
	// View: https://regexr.com/582gb
	includeRegex, err := regexp.Compile(`(?U)^<<\s*include\((.*\/*[^\/]+)\)\s*?>>$`)
	if err != nil {
		return template, err
	}

	includeMatches := includeRegex.FindStringSubmatch(template)
	if len(includeMatches) > 0 {
		filepath := filepath.Join(root, includeMatches[1])
		file, err := ioutil.ReadFile(filepath)
		if err != nil {
			return "", fmt.Errorf("could not open %s for inclusion", filepath)
		}
		escaped := strings.ReplaceAll("<<", string(file), "\\<<")

		return escaped, nil
	}

	return template, nil
}
