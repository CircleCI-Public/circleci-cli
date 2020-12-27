package references

import (
	"fmt"
	"regexp"
	"strings"
)

// SplitIntoOrbAndNamespace splits ref into 2 strings, namespace and orb.
func SplitIntoOrbAndNamespace(ref string) (namespace, orb string, err error) {
	parts := strings.Split(ref, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("Invalid orb %s. Expected a namespace and orb in the form 'namespace/orb'", ref)
	}

	return parts[0], parts[1], nil
}

// SplitIntoOrbNamespaceAndVersion splits ref into namespace, orb and version.
func SplitIntoOrbNamespaceAndVersion(ref string) (namespace, orb, version string, err error) {

	errorMessage := fmt.Errorf("Invalid orb reference '%s': Expected a namespace, orb and version in the format 'namespace/orb@version'", ref)

	re := regexp.MustCompile("^(.+)/(.+)@(.+)$")

	matches := re.FindStringSubmatch(ref)

	if len(matches) != 4 {
		return "", "", "", errorMessage
	}

	return matches[1], matches[2], matches[3], nil
}

// IsDevVersion returns true or false depending if `version` is of the form dev:...
func IsDevVersion(version string) bool {
	return strings.HasPrefix(version, "dev:")
}

// IsOrbRefWithOptionalVersion returns an error unless ref is a valid orb reference with optional version.
func IsOrbRefWithOptionalVersion(ref string) error {

	_, _, _, err := SplitIntoOrbNamespaceAndVersion(ref)

	if err == nil {
		return nil
	}

	_, _, err = SplitIntoOrbAndNamespace(ref)

	if err == nil {
		return nil
	}

	return fmt.Errorf("Invalid orb reference '%s': expected a string of the form namespace/orb or namespace/orb@version", ref)

}
