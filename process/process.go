package process

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
)

// MaybeIncludeFile replaces instances of <<include(file)>> with the contents
// of "file", escaping instances of "<<" within the file before returning, when
// the <<include()>> parameter is the string passed.
func MaybeIncludeFile(s string, orbDirectory string) (string, error) {
	// View: https://regexr.com/599mq
	includeRegex := regexp.MustCompile(`<<[\s]*include\(([-\w\/\.]+)\)?[\s]*>>`)

	includeMatches := includeRegex.FindAllStringSubmatch(s, -1)

	for _, match := range includeMatches {
		fullMatch, subMatch := match[0], match[1]

		filepath := filepath.Join(orbDirectory, subMatch)
		file, err := ioutil.ReadFile(filepath)

		if err != nil {
			return "", fmt.Errorf("could not open %s for inclusion", filepath)
		}

		escaped := strings.ReplaceAll(string(file), "<<", "\\<<")
		s = strings.ReplaceAll(s, fullMatch, escaped)
	}

	return s, nil
}
