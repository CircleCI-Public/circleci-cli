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
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/CircleCI-Public/circleci-cli/internal/httpcl"
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
