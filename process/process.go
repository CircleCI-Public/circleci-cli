package process

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
)

// MaybeIncludeFile replaces intsances of <<include(file)>> with the contents
// of "file", escaping instances of "<<" within the file before returning, when
// the <<include()>> parameter is the string passed.
func MaybeIncludeFile(s string, orbDirectory string) (string, error) {
	// View: https://regexr.com/582gb
	includeRegex, err := regexp.Compile(`(?U)^<<\s*include\((.*\/*[^\/]+)\)\s*?>>$`)
	if err != nil {
		return s, err
	}

	includeMatches := includeRegex.FindStringSubmatch(s)
	if len(includeMatches) > 0 {
		filepath := filepath.Join(orbDirectory, includeMatches[1])
		file, err := ioutil.ReadFile(filepath)
		if err != nil {
			return "", fmt.Errorf("could not open %s for inclusion", filepath)
		}
		escaped := strings.ReplaceAll(string(file), "<<", "\\<<")

		return escaped, nil
	}

	return s, nil
}
