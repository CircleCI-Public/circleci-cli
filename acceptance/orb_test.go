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

package acceptance_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
	"github.com/go-git/go-git/v6"
	gitconfig "github.com/go-git/go-git/v6/config"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
)

const testOrbID = "orb00001-0000-4000-8000-000000000001"
const testOrbVersionID = "orbv0001-0000-4000-8000-000000000001"
const testOrbCategoryID = "orbc0001-0000-4000-8000-000000000001"
const testOrbNsID = "orbns001-0000-4000-8000-000000000001"

// testOrbUUID is a valid UUID (all hex) used where uuid.Parse must succeed.
const testOrbUUID = "a1b2c3d4-0000-4000-8000-000000000001"
const testOrbName = "myorg/my-orb"
const testOrbNsName = "myorg"
const testOrbShortName = "my-orb"

// Valid hex UUIDs: ProjectRef.ID is a uuid.UUID and has to parse.
const testOrbProjectID = "00000000-0000-4000-8000-0000000000b1"
const testOrbOrgID = "00000000-0000-4000-8000-0000000000b2"
const testOrbVersion = "1.0.0"
const testOrbSource = "version: 2.1\ndescription: My test orb\n"

func setupOrbFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake := fakes.NewCircleCI(t)
	fake.AddNamespace(testOrbNsID, testOrbNsName)
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return fake, env
}

// --- orb list ---

func TestOrbList(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbPackage(testOrbID, testOrbNsID, testOrbNsName, testOrbShortName, false, true)
	fake.AddOrbVersion(testOrbVersionID, testOrbID, testOrbName, testOrbVersion, testOrbSource, "")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestOrbList_JSON(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbPackage(testOrbID, testOrbNsID, testOrbNsName, testOrbShortName, false, true)
	fake.AddOrbVersion(testOrbVersionID, testOrbID, testOrbName, testOrbVersion, testOrbSource, "")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "list", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(len(out), 1))
	assert.Check(t, cmp.Equal(out[0]["name"], testOrbName))
	assert.Check(t, cmp.Equal(out[0]["latest_version"], testOrbVersion))
}

func TestOrbList_Namespace(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbPackage(testOrbID, testOrbNsID, testOrbNsName, testOrbShortName, false, true)
	fake.AddOrbVersion(testOrbVersionID, testOrbID, testOrbName, testOrbVersion, testOrbSource, "")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "list", testOrbNsName},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestOrbList_Empty(t *testing.T) {
	_, env := setupOrbFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- orb list-categories ---

func TestOrbListCategories(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbCategory(testOrbCategoryID, "Testing")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "list-categories"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestOrbListCategories_JSON(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbCategory(testOrbCategoryID, "Testing")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "list-categories", "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out []map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(len(out), 1))
	assert.Check(t, cmp.Equal(out[0]["id"], testOrbCategoryID))
	assert.Check(t, cmp.Equal(out[0]["name"], "Testing"))
}

// --- orb create ---

func TestOrbCreate(t *testing.T) {
	fake, env := setupOrbFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "create", testOrbName},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "Created orb"))
	assert.Check(t, cmp.Contains(result.Stdout, testOrbName))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/orb/packages"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"data":{"attributes":{"name":"myorg/my-orb","is_private":false},"references":{"namespace":{"id":"orbns001-0000-4000-8000-000000000001"}}}}`),
		}, ignoreCommonHeaders))
	})
}

func TestOrbCreate_JSON(t *testing.T) {
	_, env := setupOrbFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "create", testOrbName, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out["name"], testOrbName))
	assert.Check(t, cmp.Equal(out["namespace"], testOrbNsName))
	id, _ := out["id"].(string)
	assert.Check(t, id != "", "id should be non-empty")
}

func TestOrbCreate_InvalidRef(t *testing.T) {
	_, env := setupOrbFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "create", "invalid-no-slash"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, result.ExitCode != 0)
}

// --- orb validate ---

func TestOrbValidate_Valid(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.SetOrbValidationResponse("", true, nil, testOrbSource)

	dir := t.TempDir()
	orbFile := filepath.Join(dir, "orb.yml")
	assert.NilError(t, os.WriteFile(orbFile, []byte(testOrbSource), 0644))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "validate", orbFile},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestOrbValidate_Invalid(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.SetOrbValidationResponse("", false, []string{"orb version is required"}, "")

	dir := t.TempDir()
	orbFile := filepath.Join(dir, "bad-orb.yml")
	assert.NilError(t, os.WriteFile(orbFile, []byte("not: valid: orb\n"), 0644))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "validate", orbFile},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 7))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- orb process ---

func TestOrbProcess(t *testing.T) {
	fake, env := setupOrbFake(t)
	processedYAML := "# processed\n" + testOrbSource
	fake.SetOrbValidationResponse("", true, nil, processedYAML)

	dir := t.TempDir()
	orbFile := filepath.Join(dir, "orb.yml")
	assert.NilError(t, os.WriteFile(orbFile, []byte(testOrbSource), 0644))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "process", orbFile},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- orb publish ---

func TestOrbPublish(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbPackage(testOrbID, testOrbNsID, testOrbNsName, testOrbShortName, false, true)

	dir := t.TempDir()
	orbFile := filepath.Join(dir, "orb.yml")
	assert.NilError(t, os.WriteFile(orbFile, []byte(testOrbSource), 0644))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "publish", orbFile, testOrbName + "@1.0.0"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/orb/versions"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"data":{"attributes":{"orb_id":"orb00001-0000-4000-8000-000000000001","yaml":"version: 2.1\ndescription: My test orb\n","version":"1.0.0"}}}`),
		}, ignoreCommonHeaders))
	})
}

func TestOrbPublishPromote(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbPackage(testOrbID, testOrbNsID, testOrbNsName, testOrbShortName, false, true)
	fake.AddOrbVersion(testOrbVersionID, testOrbID, testOrbName, "dev:my-branch", testOrbSource, "")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "publish", "promote", testOrbName + "@dev:my-branch", "--bump", "patch"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/orb/versions/" + testOrbVersionID + "/promote"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"segment":"patch"}`),
		}, ignoreCommonHeaders))
	})
}

func TestOrbPublishIncrement(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbPackage(testOrbID, testOrbNsID, testOrbNsName, testOrbShortName, false, true)
	fake.AddOrbVersion(testOrbVersionID, testOrbID, testOrbName, "1.0.0", testOrbSource, "")

	dir := t.TempDir()
	orbFile := filepath.Join(dir, "orb.yml")
	assert.NilError(t, os.WriteFile(orbFile, []byte(testOrbSource), 0644))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "publish", "increment", orbFile, testOrbName, "--bump", "patch"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/orb/versions"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"data":{"attributes":{"orb_id":"orb00001-0000-4000-8000-000000000001","yaml":"version: 2.1\ndescription: My test orb\n","version":"1.0.1"}}}`),
		}, ignoreCommonHeaders))
	})
}

// --- orb source ---

func TestOrbSource(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbPackage(testOrbID, testOrbNsID, testOrbNsName, testOrbShortName, false, true)
	fake.AddOrbVersion(testOrbVersionID, testOrbID, testOrbName, testOrbVersion, testOrbSource, "")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "source", testOrbName + "@" + testOrbVersion},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestOrbSource_DefaultVersion(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbPackage(testOrbID, testOrbNsID, testOrbNsName, testOrbShortName, false, true)
	fake.AddOrbVersion(testOrbVersionID, testOrbID, testOrbName, testOrbVersion, testOrbSource, "")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "source", testOrbName},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- orb get ---

func TestOrbInfo(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbPackage(testOrbID, testOrbNsID, testOrbNsName, testOrbShortName, false, true)
	fake.AddOrbVersion(testOrbVersionID, testOrbID, testOrbName, testOrbVersion, testOrbSource, "")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "get", testOrbName},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestOrbInfo_ByID(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbPackage(testOrbUUID, testOrbNsID, testOrbNsName, testOrbShortName, false, true)
	fake.AddOrbVersion(testOrbVersionID, testOrbUUID, testOrbName, testOrbVersion, testOrbSource, "")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "get", testOrbUUID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out["id"], testOrbUUID))
	assert.Check(t, cmp.Equal(out["name"], testOrbName))
	assert.Check(t, cmp.Equal(out["namespace"], testOrbNsName))
}

func TestOrbInfo_JSON(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbPackage(testOrbID, testOrbNsID, testOrbNsName, testOrbShortName, false, true)
	fake.AddOrbVersion(testOrbVersionID, testOrbID, testOrbName, testOrbVersion, testOrbSource, "")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "get", testOrbName, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)

	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.Equal(out["name"], testOrbName))
	assert.Check(t, cmp.Equal(out["namespace"], testOrbNsName))
	assert.Check(t, cmp.Equal(out["id"], testOrbID))
}

// --- orb unlist ---

func TestOrbUnlist(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbPackage(testOrbID, testOrbNsID, testOrbNsName, testOrbShortName, false, true)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "unlist", testOrbName},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/orb/packages/" + testOrbID + "/set-listed"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"is_listed":false}`),
		}, ignoreCommonHeaders))
	})
}

func TestOrbRelist(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbPackage(testOrbID, testOrbNsID, testOrbNsName, testOrbShortName, false, true)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "unlist", testOrbName, "--restore"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/orb/packages/" + testOrbID + "/set-listed"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"is_listed":true}`),
		}, ignoreCommonHeaders))
	})
}

// --- orb diff ---

func TestOrbDiff(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbPackage(testOrbID, testOrbNsID, testOrbNsName, testOrbShortName, false, true)
	fake.AddOrbVersion("ver1-0000-0000-0000-000000000001", testOrbID, testOrbName, "1.0.0",
		"version: 2.1\ndescription: Version one\n", "")
	fake.AddOrbVersion("ver2-0000-0000-0000-000000000002", testOrbID, testOrbName, "1.1.0",
		"version: 2.1\ndescription: Version two\n", "")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "diff", testOrbName, "--from", "1.0.0", "--to", "1.1.0"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- orb add-to-category / remove-from-category ---

func TestOrbAddToCategory(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbPackage(testOrbID, testOrbNsID, testOrbNsName, testOrbShortName, false, true)
	fake.AddOrbCategory(testOrbCategoryID, "Testing")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "add-to-category", testOrbName, "Testing"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/orb/packages/" + testOrbID + "/add-category"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"category_id":"orbc0001-0000-4000-8000-000000000001"}`),
		}, ignoreCommonHeaders))
	})
}

func TestOrbRemoveFromCategory(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbPackage(testOrbID, testOrbNsID, testOrbNsName, testOrbShortName, false, true)
	fake.AddOrbCategory(testOrbCategoryID, "Testing")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "remove-from-category", testOrbName, "Testing"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/orb/packages/" + testOrbID + "/remove-category"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {httpcl.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"category_id":"orbc0001-0000-4000-8000-000000000001"}`),
		}, ignoreCommonHeaders))
	})
}

// --- orb pack ---

func TestOrbPack_File(t *testing.T) {
	_, env := setupOrbFake(t)

	dir := t.TempDir()
	orbFile := filepath.Join(dir, "orb.yml")
	assert.NilError(t, os.WriteFile(orbFile, []byte(testOrbSource), 0644))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "pack", orbFile},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestOrbPack_Directory(t *testing.T) {
	_, env := setupOrbFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args:   []string{"orb", "pack", filepath.Join("testdata", "myorb", "src")},
		Env:    env.Environ(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestOrbPack_Directory_OrbYml(t *testing.T) {
	_, env := setupOrbFake(t)

	dir := t.TempDir()
	// Create orb.yml (no @ prefix)
	baseYAML := "version: 2.1\ndescription: Fallback orb\n"
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "orb.yml"), []byte(baseYAML), 0644))

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "pack", dir},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestOrbPack_YAML11Booleans is the end-to-end check for
// https://github.com/CircleCI-Public/circleci-cli/issues/691: a boolean orb
// parameter defaulting to `on` packed to the string "on", so `orb validate` on
// the packed output rejected the parameter against its own declared type.
func TestOrbPack_YAML11Booleans(t *testing.T) {
	_, env := setupOrbFake(t)

	dir := t.TempDir()
	writeOrbFile(t, filepath.Join(dir, "src", "@orb.yml"), "version: 2.1\n")
	writeOrbFile(t, filepath.Join(dir, "src", "commands", "vpn.yml"),
		"parameters:\n"+
			"  killswitch:\n    type: boolean\n    default: on\n"+
			"  verbose:\n    type: boolean\n    default: yes\n"+
			"  quiet:\n    type: boolean\n    default: off\n"+
			// Quoted: the author asked for the string, and packing must not
			// decide otherwise.
			"  label:\n    type: string\n    default: \"on\"\n"+
			"steps:\n  - run: echo hi\n")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "pack", "src"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestOrbPack_Includes is the end-to-end check for the include directive that
// was dropped in the v1 rewrite: a step whose value is exactly
// '<< include(file) >>' must be replaced with that file's contents. See
// https://github.com/CircleCI-Public/circleci-cli/pull/737.
func TestOrbPack_Includes(t *testing.T) {
	_, env := setupOrbFake(t)

	dir := t.TempDir()
	writeOrbFile(t, filepath.Join(dir, "src", "@orb.yml"), "version: 2.1\n")
	writeOrbFile(t, filepath.Join(dir, "src", "commands", "greet.yml"),
		"steps:\n  - run: << include(scripts/greet.sh) >>\n")
	writeOrbFile(t, filepath.Join(dir, "src", "scripts", "greet.sh"),
		"echo hello\necho world\n")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "pack", "src"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestOrbPack_IncludesMultipleAndEmbedded is the end-to-end check for
// https://github.com/CircleCI-Public/circleci-cli/pull/737: a value may hold
// several '<< include(...) >>' directives and surround them with other text.
func TestOrbPack_IncludesMultipleAndEmbedded(t *testing.T) {
	_, env := setupOrbFake(t)

	dir := t.TempDir()
	writeOrbFile(t, filepath.Join(dir, "src", "@orb.yml"), "version: 2.1\n")
	writeOrbFile(t, filepath.Join(dir, "src", "commands", "greet.yml"),
		"steps:\n  - run: echo \"<< include(scripts/a.txt) >> << include(scripts/b.txt) >>\"\n")
	writeOrbFile(t, filepath.Join(dir, "src", "scripts", "a.txt"), "Hello,")
	writeOrbFile(t, filepath.Join(dir, "src", "scripts", "b.txt"), "world!")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "pack", "src"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestOrbPack_NestedDirectories is the end-to-end check for
// https://github.com/CircleCI-Public/circleci-cli/issues/755: an orb author
// organising commands/ into subdirectories. Those files used to pack into
// commands.<dir>.<file>, producing a "command" that was really a map of
// commands, so the orb was invalid.
func TestOrbPack_NestedDirectories(t *testing.T) {
	_, env := setupOrbFake(t)

	dir := t.TempDir()
	writeOrbFile(t, filepath.Join(dir, "src", "@orb.yml"), "version: 2.1\n")
	writeOrbFile(t, filepath.Join(dir, "src", "commands", "top.yml"), "steps:\n  - run: echo top\n")
	writeOrbFile(t, filepath.Join(dir, "src", "commands", "aws", "login.yml"), "steps:\n  - run: echo login\n")
	writeOrbFile(t, filepath.Join(dir, "src", "commands", "gcp", "auth.yml"), "steps:\n  - run: echo auth\n")
	writeOrbFile(t, filepath.Join(dir, "src", "jobs", "build", "unit.yml"), "steps:\n  - run: echo unit\n")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "pack", "src"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// TestOrbPack_NestedNameCollision covers the cost of flattening: two files in
// different subdirectories of one section would silently claim the same key, and
// one would win. Say so instead, naming both files.
//
// Asserted with Contains rather than a golden file because the message carries
// absolute, %q-quoted paths: a golden would embed this machine's temp directory,
// and %q escapes the separator on Windows.
func TestOrbPack_NestedNameCollision(t *testing.T) {
	_, env := setupOrbFake(t)

	dir := t.TempDir()
	writeOrbFile(t, filepath.Join(dir, "src", "@orb.yml"), "version: 2.1\n")
	writeOrbFile(t, filepath.Join(dir, "src", "commands", "aws", "login.yml"), "steps:\n  - run: echo a\n")
	writeOrbFile(t, filepath.Join(dir, "src", "commands", "gcp", "login.yml"), "steps:\n  - run: echo b\n")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "pack", "src"},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	// No half-packed document on stdout when the pack fails.
	assert.Check(t, cmp.Equal(result.Stdout, ""))
	assert.Check(t, cmp.Contains(result.Stderr, `both define "login"`))
	assert.Check(t, cmp.Contains(result.Stderr, "cannot share a name"))
	// Both source files are named, so the author knows which two to reconcile.
	assert.Check(t, cmp.Contains(result.Stderr, "aws"))
	assert.Check(t, cmp.Contains(result.Stderr, "gcp"))
}

// --- edge case: missing args ---

func TestOrbList_Namespace_NotFound(t *testing.T) {
	_, env := setupOrbFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "list", "nonexistent-ns"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5)
}

func TestOrbCreate_MissingArg(t *testing.T) {
	_, env := setupOrbFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "create"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, result.ExitCode != 0)
}

func TestOrbSource_NotFound(t *testing.T) {
	_, env := setupOrbFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "source", "nonexistent/orb@1.0.0"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, result.ExitCode != 0)
}

func TestOrbDiff_SameVersion(t *testing.T) {
	fake, env := setupOrbFake(t)
	fake.AddOrbPackage(testOrbID, testOrbNsID, testOrbNsName, testOrbShortName, false, true)
	fake.AddOrbVersion(testOrbVersionID, testOrbID, testOrbName, "1.0.0", testOrbSource, "")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "diff", testOrbName, "--from", "1.0.0", "--to", "1.0.0"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	// Same version = no diff output, exit 0
	assert.Check(t, cmp.Equal(result.ExitCode, 0))
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- orb init ---

// orbTemplateFiles are the files the fake orb template zip contains. They use
// the same placeholder tokens as the real CircleCI-Public/Orb-Template so the
// git-setup path exercises placeholder substitution.
var orbTemplateFiles = map[string]string{
	"README.md":                 "# <orb-name>\n\norg: <organization>, project: <project-name>\n**Meta** drop me\n",
	"LICENSE":                   "MIT License\n",
	"src/@orb.yml":              "version: 2.1\ndescription: <orb-name> in <namespace>\n",
	".circleci/config.yml":      "orb: <namespace>/<orb-name>\ncontext: <publishing-context>\n",
	".circleci/test-deploy.yml": "orb: <namespace>/<orb-name>\n",
}

// buildOrbTemplateZip returns a zip mimicking a GitHub zipball: every entry is
// nested under a top-level wrapper directory that the extractor must strip.
func buildOrbTemplateZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range orbTemplateFiles {
		w, err := zw.Create("Orb-Template-deadbeef/" + name)
		assert.NilError(t, err)
		_, err = w.Write([]byte(content))
		assert.NilError(t, err)
	}
	assert.NilError(t, zw.Close())
	return buf.Bytes()
}

// registerOrbTemplate serves a fake orb template (tags list + zipball) from the
// fake server's static-file route and points the CLI at it via env override.
func registerOrbTemplate(t *testing.T, fake *fakes.CircleCI, env *testenv.TestEnv) {
	t.Helper()
	zipURL := fake.URL() + "/artifacts/orb-template/orb.zip"
	tags := []map[string]string{
		{"name": "nightly", "zipball_url": zipURL},
		{"name": "v1.2.3", "zipball_url": zipURL},
	}
	tagsJSON, err := json.Marshal(tags)
	assert.NilError(t, err)

	fake.AddStaticFile("/artifacts/orb-template/tags", string(tagsJSON))
	fake.AddStaticFile("/artifacts/orb-template/orb.zip", string(buildOrbTemplateZip(t)))
	env.Extra["CIRCLE_ORB_TEMPLATE_URL"] = fake.URL() + "/artifacts/orb-template/tags"
}

// A relative <path> keeps stdout deterministic (no temp-dir prefix) so the
// output can be compared against a golden file. Files are read back from
// <workDir>/<orbDir>.
const orbInitDir = "my-orb"

func TestOrbInit_TemplateOnly(t *testing.T) {
	fake, env := setupOrbFake(t)
	registerOrbTemplate(t, fake, env)

	workDir := t.TempDir()
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "init", orbInitDir, "--template-only"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	// The wrapper directory is stripped: files land directly in the orb dir.
	// Because --template-only was used, placeholders are left untouched.
	orbDir := filepath.Join(workDir, orbInitDir)
	readme, err := os.ReadFile(filepath.Join(orbDir, "README.md"))
	assert.NilError(t, err)
	assert.Check(t, cmp.Contains(string(readme), "<orb-name>"))
	_, err = os.Stat(filepath.Join(orbDir, "src", "@orb.yml"))
	assert.NilError(t, err)
	// A public template keeps its LICENSE.
	_, err = os.Stat(filepath.Join(orbDir, "LICENSE"))
	assert.NilError(t, err)
}

func TestOrbInit_TemplateOnly_Private(t *testing.T) {
	fake, env := setupOrbFake(t)
	registerOrbTemplate(t, fake, env)

	workDir := t.TempDir()
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "init", orbInitDir, "--template-only", "--private"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	// A private orb has its MIT LICENSE removed.
	_, err := os.Stat(filepath.Join(workDir, orbInitDir, "LICENSE"))
	assert.Check(t, os.IsNotExist(err))
}

func TestOrbInit_MissingArg(t *testing.T) {
	_, env := setupOrbFake(t)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "init"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, result.ExitCode != 0)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestOrbInit_RequiresOrgWhenNonInteractive(t *testing.T) {
	fake, env := setupOrbFake(t)
	registerOrbTemplate(t, fake, env)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		// No --org and no TTY: setup cannot proceed.
		Args:    []string{"orb", "init", orbInitDir, "--skip-git"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, result.ExitCode != 0)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestOrbInit_SkipGit(t *testing.T) {
	fake, env := setupOrbFake(t) // registers namespace "myorg"
	registerOrbTemplate(t, fake, env)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "init", orbInitDir, "--org", "gh/myorg", "--skip-git"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	// The orb was reserved via the create-package endpoint.
	var created bool
	for _, req := range fake.AllRequests() {
		if req.Method == http.MethodPost && req.URL.Path == "/api/v3/orb/packages" {
			created = true
		}
	}
	assert.Check(t, created, "expected a POST to /api/v3/orb/packages")
}

func TestOrbInit_Git(t *testing.T) {
	fake, env := setupOrbFake(t)
	registerOrbTemplate(t, fake, env)
	// Resolvable by slug so orb init can find the project it just followed, and
	// known by UUID so the settings update lands, not 404s.
	fake.AddProjectBySlug("gh/myorg/my-orb", testOrbProjectID, testOrbShortName, testOrbOrgID)
	fake.SetProjectSettings(testOrbProjectID, map[string]any{"advanced": map[string]any{}})
	// Provide a git identity so the initial commit succeeds deterministically.
	env.Extra["GIT_AUTHOR_NAME"] = "Test"
	env.Extra["GIT_AUTHOR_EMAIL"] = "test@example.com"
	env.Extra["GIT_COMMITTER_NAME"] = "Test"
	env.Extra["GIT_COMMITTER_EMAIL"] = "test@example.com"

	workDir := t.TempDir()
	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"orb", "init", orbInitDir,
			"--org", "gh/myorg",
			"--remote", "https://github.com/myorg/my-orb.git",
			"--branch", "main",
		},
		Env:     env.Environ(),
		WorkDir: workDir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	// A local git repository was initialized.
	orbDir := filepath.Join(workDir, orbInitDir)
	_, err := os.Stat(filepath.Join(orbDir, ".git"))
	assert.NilError(t, err)

	// The initial commit landed on the configured branch, never go-git's default
	// "master" (PR 678) — so a master ref is never created.
	_, err = os.Stat(filepath.Join(orbDir, ".git", "refs", "heads", "master"))
	assert.Check(t, os.IsNotExist(err), "orb init must not create a master branch")

	// Template placeholders were substituted in the generated config.
	cfg, err := os.ReadFile(filepath.Join(orbDir, ".circleci", "config.yml"))
	assert.NilError(t, err)
	assert.Check(t, cmp.Contains(string(cfg), "myorg/my-orb"))
	assert.Check(t, !bytes.Contains(cfg, []byte("<orb-name>")))

	// A dev:alpha version was published and the project was followed.
	var published, followed bool
	for _, req := range fake.AllRequests() {
		if req.Method == http.MethodPost && req.URL.Path == "/api/v3/orb/versions" {
			published = true
		}
		if req.Method == http.MethodPost && req.URL.Path == fmt.Sprintf("/api/v1.1/project/%s/%s/%s/follow", "gh", "myorg", "my-orb") {
			followed = true
		}
	}
	assert.Check(t, published, "expected a POST to /api/v3/orb/versions")
	assert.Check(t, followed, "expected a follow request")

	// Dynamic config was enabled, so the orb dev kit's setup workflow will
	// actually run on the first pipeline (issue 834).
	var dynamicConfig any
	for _, req := range fake.AllRequests() {
		if req.Method == http.MethodPost &&
			req.URL.Path == "/api/v3/projects/"+testOrbProjectID+"/update-settings" {
			if req.Body == nil {
				continue
			}
			var body map[string]any
			assert.NilError(t, json.Unmarshal([]byte(*req.Body), &body))
			if v, ok := body["enable_dynamic_config"]; ok {
				dynamicConfig = v
			}
		}
	}
	assert.Check(t, cmp.Equal(dynamicConfig, true),
		"expected enable_dynamic_config: true in an update-settings request")
}

// TestOrbInit_Git_DynamicConfigFailureDoesNotAbort pins the failure mode. By the
// time dynamic config is enabled, the namespace, orb, publishing context, git
// repository and dev release all exist. If the project cannot be resolved — it is
// not registered by slug here — orb init must warn and still finish, rather than
// abort and leave the author with a half-built project and no way to resume.
func TestOrbInit_Git_DynamicConfigFailureDoesNotAbort(t *testing.T) {
	fake, env := setupOrbFake(t)
	registerOrbTemplate(t, fake, env)
	env.Extra["GIT_AUTHOR_NAME"] = "Test"
	env.Extra["GIT_AUTHOR_EMAIL"] = "test@example.com"
	env.Extra["GIT_COMMITTER_NAME"] = "Test"
	env.Extra["GIT_COMMITTER_EMAIL"] = "test@example.com"

	workDir := t.TempDir()
	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"orb", "init", orbInitDir,
			"--org", "gh/myorg",
			"--remote", "https://github.com/myorg/my-orb.git",
			"--branch", "main",
		},
		Env:     env.Environ(),
		WorkDir: workDir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "Could not enable dynamic configuration"))
	// The run still reached its end: the orb is set up and on the alpha branch.
	assert.Check(t, cmp.Contains(result.Stdout, "alpha"))
}

// TestOrbInit_Interactive_CategoryFailureDoesNotAbort is the regression test for
// the costly half of https://github.com/CircleCI-Public/circleci-cli/issues/609.
//
// Assigning a category used to abort the run on failure. That happens after the
// namespace and the orb already exist, and orb init has no resume — so the
// author's only recourse was to start over and re-answer every prompt, which is
// what the issue complains about. The registry refusing a category has to warn
// and let the rest of the setup finish.
//
// Driven through a PTY because categories are gathered interactively: the
// non-interactive path takes no category flags, so it never reaches this code.
func TestOrbInit_Interactive_CategoryFailureDoesNotAbort(t *testing.T) {
	fake, env := setupOrbFake(t)
	registerOrbTemplate(t, fake, env)
	fake.AddOrbCategory(testOrbCategoryID, "Testing")
	// The registry refuses the category, as it does past an orb's limit.
	fake.SetOrbAddCategoryStatus(http.StatusBadRequest)

	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "init", orbInitDir, "--org", "gh/myorg"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	expect := func(want string) {
		t.Helper()
		_, err := console.ExpectString(want)
		assert.NilError(t, err, "waiting for %q", want)
	}
	send := func(keys string) {
		t.Helper()
		_, err := console.Send(keys)
		assert.NilError(t, err)
	}

	expect("public or private orb")
	send("\r") // Public
	expect("automated setup")
	send("\r") // Yes, walk me through it
	expect("Enter the namespace")
	send("\r") // default: myorg
	expect("Orb name")
	send("\r") // default: my-orb

	// The prompt states the limit before the choice is made.
	expect("up to 2")
	send("\x1b[B") // down: "(done)" → "Testing"
	send("\r")     // choose it; only one category exists, so the loop ends

	expect("publishing context")
	send("n")
	expect("set up your git project")
	send("n")

	// The refusal is reported, naming the category...
	expect(`Could not add myorg/my-orb to the "Testing" category`)
	// ...and the run still reaches its normal end rather than aborting.
	expect("is ready")
}

// TestOrbInit_AdoptsExistingClone is the regression test for
// https://github.com/CircleCI-Public/circleci-cli/issues/803, following the steps
// in the report: an admin creates an empty repository, the author clones it and
// runs orb init inside. That used to dead-end on "already a git repository;
// delete its .git directory and retry", with no way through — and cloning first
// is the only route when repository creation is restricted to admins.
//
// No --remote is passed: the clone's origin is what the author intends to push
// to, so orb init has to read it rather than demand it again.
func TestOrbInit_AdoptsExistingClone(t *testing.T) {
	fake, env := setupOrbFake(t)
	registerOrbTemplate(t, fake, env)
	env.Extra["GIT_AUTHOR_NAME"] = "Test"
	env.Extra["GIT_AUTHOR_EMAIL"] = "test@example.com"
	env.Extra["GIT_COMMITTER_NAME"] = "Test"
	env.Extra["GIT_COMMITTER_EMAIL"] = "test@example.com"

	workDir := t.TempDir()
	orbDir := filepath.Join(workDir, orbInitDir)

	// The repository is named differently from the orb directory, as in the
	// issue (circleci-orb-myorbname cloned into my-orb). That difference is what
	// proves the project details come from the clone's origin rather than from
	// the directory name or a flag default.
	const cloneURL = "https://github.com/myorg/circleci-orb-myorbname.git"
	const cloneProject = "circleci-orb-myorbname"

	// Resolvable so the followed project can have dynamic config enabled on it.
	fake.AddProjectBySlug("gh/myorg/"+cloneProject, testOrbProjectID, cloneProject, testOrbOrgID)
	fake.SetProjectSettings(testOrbProjectID, map[string]any{"advanced": map[string]any{}})

	// Stand in for `git clone` of an empty repository: a repository with an
	// origin and no commits.
	assert.NilError(t, os.MkdirAll(orbDir, 0o750))
	repo, err := git.PlainInit(orbDir, false)
	assert.NilError(t, err)
	_, err = repo.CreateRemote(&gitconfig.RemoteConfig{
		Name: "origin",
		URLs: []string{cloneURL},
	})
	assert.NilError(t, err)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		// Deliberately no --remote: it comes from the clone.
		Args:    []string{"orb", "init", orbInitDir, "--org", "gh/myorg", "--branch", "main"},
		Env:     env.Environ(),
		WorkDir: workDir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Contains(result.Stdout, "Using origin from the existing repository"))
	assert.Check(t, cmp.Contains(result.Stdout, cloneURL))

	// The clone's origin survived rather than being replaced.
	reopened, err := git.PlainOpen(orbDir)
	assert.NilError(t, err)
	origin, err := reopened.Remote("origin")
	assert.NilError(t, err)
	assert.Check(t, cmp.DeepEqual(origin.Config().URLs, []string{cloneURL}))

	// And the template was committed into it, so the project is genuinely set up.
	cfg, err := os.ReadFile(filepath.Join(orbDir, ".circleci", "config.yml"))
	assert.NilError(t, err)
	assert.Check(t, cmp.Contains(string(cfg), "myorg/my-orb"))

	// The project followed, and the one dynamic config was enabled on, are both
	// derived from the clone's origin — not from the orb directory name. This is
	// the seam between adopting a clone (issue 803) and enabling dynamic config
	// (issue 834), which only exists once both are in.
	var followed, configured bool
	for _, req := range fake.AllRequests() {
		if req.Method == http.MethodPost &&
			req.URL.Path == "/api/v1.1/project/gh/myorg/"+cloneProject+"/follow" {
			followed = true
		}
		if req.Method == http.MethodPost &&
			req.URL.Path == "/api/v3/projects/"+testOrbProjectID+"/update-settings" {
			configured = true
		}
	}
	assert.Check(t, followed, "expected the project from the clone's origin to be followed")
	assert.Check(t, configured, "expected dynamic config to be enabled on that project")
	assert.Check(t, cmp.Contains(result.Stdout, "Dynamic configuration enabled"))
}

// TestOrbInit_NoRemoteAndNoRepoStillErrors keeps the other side intact: with
// nothing to adopt and no --remote, git setup cannot proceed and says so.
func TestOrbInit_NoRemoteAndNoRepoStillErrors(t *testing.T) {
	fake, env := setupOrbFake(t)
	registerOrbTemplate(t, fake, env)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"orb", "init", orbInitDir, "--org", "gh/myorg"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Check(t, cmp.Equal(result.ExitCode, 2))
	assert.Check(t, cmp.Contains(result.Stderr, "Pass --remote <url>"))
	// The suggestion names the way out that issue 803 added.
	assert.Check(t, cmp.Contains(result.Stderr, "Running inside an existing clone uses its origin automatically"))
}

// writeOrbFile writes a file for the pack tests, creating parent directories.
func writeOrbFile(t *testing.T, path, content string) {
	t.Helper()
	assert.NilError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	assert.NilError(t, os.WriteFile(path, []byte(content), 0o600))
}
