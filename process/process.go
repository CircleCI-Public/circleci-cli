package process

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// MaybeIncludeFile replaces intsances of <<include(file)>> with the contents
// of "file", escaping instances of "<<" within the file before returning, when
// the <<include()>> parameter is the string passed.
func MaybeIncludeFile(s string, orbDirectory string) (string, error) {
	// View: https://regexr.com/599mq
	includeRegex := regexp.MustCompile(`<<[\s]*include\(([-\w\/\.]+)\)?[\s]*>>`)

	// only find up to 2 matches, because we throw an error if we find >1
	includeMatches := includeRegex.FindAllStringSubmatch(s, 2)
	if len(includeMatches) > 1 {
		return "", fmt.Errorf("multiple include statements: '%s'", s)
	}

	if len(includeMatches) == 1 {
		match := includeMatches[0]
		fullMatch, subMatch := match[0], match[1]

		// throw an error if the entire string wasn't matched
		if fullMatch != s {
			return "", fmt.Errorf("entire string must be include statement: '%s'", s)
		}

		filepath := filepath.Join(orbDirectory, subMatch)
		file, err := os.ReadFile(filepath)
		if err != nil {
			return "", fmt.Errorf("could not open %s for inclusion", filepath)
		}
		escaped := strings.ReplaceAll(string(file), "<<", "\\<<")

		return escaped, nil
	}

	return s, nil
}
