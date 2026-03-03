package references_test

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/references"
)

func TestIsDevVersion(t *testing.T) {
	assert.Assert(t, references.IsDevVersion("dev:master"))
	assert.Assert(t, !references.IsDevVersion("1.2.1"))
	assert.Assert(t, !references.IsDevVersion(""))
}

func TestSplitIntoOrbNamespaceAndVersion(t *testing.T) {
	namespace, orb, version, err := references.SplitIntoOrbNamespaceAndVersion("foo/bar@dev:baz")
	assert.NilError(t, err)
	assert.Equal(t, namespace, "foo")
	assert.Equal(t, orb, "bar")
	assert.Equal(t, version, "dev:baz")
}

func TestSplitIntoOrbNamespaceAndVersionInvalid(t *testing.T) {
	_, _, _, err := references.SplitIntoOrbNamespaceAndVersion("asdasd")
	assert.Error(t, err, "invalid orb reference 'asdasd': expected a namespace, orb and version in the format 'namespace/orb@version'")
}

func TestSplitIntoOrbAndNamespace(t *testing.T) {
	ns, orb, err := references.SplitIntoOrbAndNamespace("cat/dog")
	assert.NilError(t, err)
	assert.Equal(t, ns, "cat")
	assert.Equal(t, orb, "dog")
}

func TestSplitIntoOrbAndNamespaceInvalid(t *testing.T) {
	_, _, err := references.SplitIntoOrbAndNamespace("catdog")
	assert.Error(t, err, "invalid orb catdog, expected a namespace and orb in the form 'namespace/orb'")
}

func TestSplitDevLabelWithSlash(t *testing.T) {
	ns, orb, version, err := references.SplitIntoOrbNamespaceAndVersion("foo/bar@dev:bah/bah")
	assert.NilError(t, err)
	assert.Equal(t, ns, "foo")
	assert.Equal(t, orb, "bar")
	assert.Equal(t, version, "dev:bah/bah")
}
