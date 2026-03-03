package update_test

import (
	"testing"

	"github.com/blang/semver"
	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/update"
)

func TestParseSimpleVersion(t *testing.T) {
	version, err := update.ParseHomebrewVersion("1.0.0")
	assert.NilError(t, err)
	assert.Equal(t, version.String(), "1.0.0")
}

func TestParseVersionWithRevision(t *testing.T) {
	version, err := update.ParseHomebrewVersion("0.1.15410_1")
	assert.NilError(t, err)
	assert.Equal(t, version.String(), "0.1.15410-1")
	assert.Equal(t, len(version.Pre), 1)
	assert.DeepEqual(t, version.Pre[0], semver.PRVersion{VersionNum: 1, IsNum: true})
}

func TestParseGarbageVersion(t *testing.T) {
	_, err := update.ParseHomebrewVersion("asdad.1231.-_")
	assert.Assert(t, err != nil)
	assert.ErrorContains(t, err, "failed to parse current version")
	assert.ErrorContains(t, err, "asdad.1231.-_")
}
