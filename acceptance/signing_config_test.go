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
	"net/http"
	"net/url"
	"runtime"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/httprecorder"
)

// --- signing-config create ---

func TestSigningConfigCreate(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddIOSCert(testIOSOrgID, fakeIOSCert("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Distribution.p12"))
	env := setupIOSEnv(t, fake)

	dir := t.TempDir()
	profilePath := writeBinaryFile(t, dir, "MyApp.mobileprovision", "fake-profile-bytes")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"signing-config", "create",
			"--org", testIOSOrgID,
			"--name", "production-signing",
			"--cert-id", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			"--profile", profilePath,
		},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodPost,
			URL:    url.URL{Path: "/api/v3/signing/configs"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {apiclient.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(`{"data":{"attributes":{"name":"production-signing","provisioning_profiles":[{"file_name":"MyApp.mobileprovision","blob":"ZmFrZS1wcm9maWxlLWJ5dGVz"}]},"references":{"org":{"id":"11111111-1111-1111-1111-111111111111"},"signing_certificate":{"id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}}}}`),
		}, ignoreCommonHeaders))
	})
}

func TestSigningConfigCreate_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddIOSCert(testIOSOrgID, fakeIOSCert("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Distribution.p12"))
	env := setupIOSEnv(t, fake)

	dir := t.TempDir()
	profilePath := writeBinaryFile(t, dir, "MyApp.mobileprovision", "fake-profile-bytes")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"signing-config", "create",
			"--org", testIOSOrgID,
			"--name", "production-signing",
			"--cert-id", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			"--profile", profilePath,
			"--json",
		},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestSigningConfigCreate_MissingProfile(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := setupIOSEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"signing-config", "create",
			"--org", testIOSOrgID,
			"--name", "x",
			"--cert-id", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestSigningConfigCreate_UnknownCertID(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := setupIOSEnv(t, fake)

	dir := t.TempDir()
	profilePath := writeBinaryFile(t, dir, "MyApp.mobileprovision", "fake-profile-bytes")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"signing-config", "create",
			"--org", testIOSOrgID,
			"--name", "production-signing",
			"--cert-id", "dddddddd-dddd-dddd-dddd-dddddddddddd",
			"--profile", profilePath,
		},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestSigningConfigCreate_DuplicateName(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddIOSCert(testIOSOrgID, fakeIOSCert("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Distribution.p12"))
	fake.AddIOSBundle(testIOSOrgID, fakeIOSSigningConfig("e1111111-1111-1111-1111-111111111111", "production-signing", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Distribution.p12",
		"MyApp.mobileprovision"))
	env := setupIOSEnv(t, fake)

	dir := t.TempDir()
	profilePath := writeBinaryFile(t, dir, "MyApp.mobileprovision", "fake-profile-bytes")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"signing-config", "create",
			"--org", testIOSOrgID,
			"--name", "production-signing",
			"--cert-id", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
			"--profile", profilePath,
		},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 4, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- signing-config list ---

func TestSigningConfigList(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddIOSBundle(testIOSOrgID, fakeIOSSigningConfig("e1111111-1111-1111-1111-111111111111", "production-signing", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Distribution.p12",
		"MyApp.mobileprovision"))
	fake.AddIOSBundle(testIOSOrgID, fakeIOSSigningConfig("e2222222-2222-2222-2222-222222222222", "staging-signing", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Distribution.p12",
		"MyAppStaging.mobileprovision",
		"MyAppExtensionStaging.mobileprovision"))
	env := setupIOSEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"signing-config", "list", "--org", testIOSOrgID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestSigningConfigList_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddIOSBundle(testIOSOrgID, fakeIOSSigningConfig("e1111111-1111-1111-1111-111111111111", "production-signing", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Distribution.p12",
		"MyApp.mobileprovision"))
	fake.AddIOSBundle(testIOSOrgID, fakeIOSSigningConfig("e2222222-2222-2222-2222-222222222222", "staging-signing", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Distribution.p12",
		"MyAppStaging.mobileprovision",
		"MyAppExtensionStaging.mobileprovision"))
	env := setupIOSEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"signing-config", "list", "--org", testIOSOrgID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestSigningConfigList_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddIOSBundle(testIOSOrgID, fakeIOSSigningConfig("e1111111-1111-1111-1111-111111111111", "production-signing", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Distribution.p12",
		"MyApp.mobileprovision"))
	fake.AddIOSBundle(testIOSOrgID, fakeIOSSigningConfig("e2222222-2222-2222-2222-222222222222", "staging-signing", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Distribution.p12",
		"MyAppStaging.mobileprovision",
		"MyAppExtensionStaging.mobileprovision"))
	env := setupIOSEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"signing-config", "list", "--org", testIOSOrgID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestSigningConfigList_Empty(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := setupIOSEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"signing-config", "list", "--org", testIOSOrgID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- signing-config delete ---

func TestSigningConfigDelete(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddIOSBundle(testIOSOrgID, fakeIOSSigningConfig("eccccccc-cccc-cccc-cccc-cccccccccccc", "old", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Distribution.p12"))
	env := setupIOSEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"signing-config", "delete", "eccccccc-cccc-cccc-cccc-cccccccccccc", "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, fake.DeletedIOSBundle("eccccccc-cccc-cccc-cccc-cccccccccccc"))

	t.Run("check request", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.LastRequest(), &httprecorder.Request{
			Method: http.MethodDelete,
			URL:    url.URL{Path: "/api/v3/signing/configs/eccccccc-cccc-cccc-cccc-cccccccccccc"},
			Header: http.Header{
				"Authorization": {"Bearer test-token"},
				"User-Agent":    {apiclient.UserAgent(runtime.GOOS, runtime.GOARCH, "dev", "")},
			},
			Body: new(""),
		}, ignoreCommonHeaders))
	})
}

func TestSigningConfigDelete_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := setupIOSEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"signing-config", "delete", "eddddddd-dddd-dddd-dddd-dddddddddddd", "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestSigningConfigDelete_RequiresForce(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddIOSBundle(testIOSOrgID, fakeIOSSigningConfig("eccccccc-cccc-cccc-cccc-cccccccccccc", "old", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "Distribution.p12"))
	env := setupIOSEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"signing-config", "delete", "eccccccc-cccc-cccc-cccc-cccccccccccc"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 6, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	assert.Check(t, !fake.DeletedIOSBundle("eccccccc-cccc-cccc-cccc-cccccccccccc"))
}
