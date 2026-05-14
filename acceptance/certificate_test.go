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
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

// Shared by certificate_test.go and signing_config_test.go.
const testIOSOrgID = "11111111-1111-1111-1111-111111111111"

func fakeIOSCert(id, fileName string) map[string]any {
	return map[string]any{
		"id":        id,
		"file_name": fileName,
		"cert_type": "distribution",
	}
}

// fakeIOSSigningConfig builds a fixture matching the API list-response shape.
// certFileName populates the nested `certificate` ref; the FK fields used by
// the fake's cert-in-use check are prefixed with `_` and stripped on the wire.
func fakeIOSSigningConfig(id, name, certID, certFileName string, profileFiles ...string) map[string]any {
	profiles := make([]map[string]any, len(profileFiles))
	for i, f := range profileFiles {
		profiles[i] = map[string]any{"file_name": f}
	}
	return map[string]any{
		"id":                    id,
		"name":                  name,
		"certificate":           map[string]any{"file_name": certFileName, "cert_type": "distribution"},
		"provisioning_profiles": profiles,
		"_cert_id":              certID,
		"_org_id":               testIOSOrgID,
	}
}

func setupIOSEnv(t *testing.T, fake *fakes.CircleCI) *testenv.TestEnv {
	t.Helper()
	env := testenv.New(t)
	env.Token = testToken
	env.CircleCIURL = fake.URL()
	return env
}

// writeBinaryFile writes content to <dir>/<name>. Returns name (caller passes
// the bare name as --cert-file / --profile and uses dir as WorkDir, so any
// path captured in golden output stays stable across runs).
func writeBinaryFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	assert.NilError(t, os.WriteFile(path, []byte(content), 0o600))
	return name
}

// --- certificate upload ---

func TestCertificateUpload(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := setupIOSEnv(t, fake)

	dir := t.TempDir()
	certPath := writeBinaryFile(t, dir, "Distribution.p12", "fake-p12-bytes")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"certificate", "upload",
			"--org-id", testIOSOrgID,
			"--cert-file", certPath,
			"--password", "hunter2",
		},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestCertificateUpload_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := setupIOSEnv(t, fake)

	dir := t.TempDir()
	certPath := writeBinaryFile(t, dir, "Distribution.p12", "fake-p12-bytes")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"certificate", "upload",
			"--org-id", testIOSOrgID,
			"--cert-file", certPath,
			"--password", "hunter2",
			"--json",
		},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestCertificateUpload_PasswordFromStdin(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := setupIOSEnv(t, fake)

	dir := t.TempDir()
	certPath := writeBinaryFile(t, dir, "Distribution.p12", "fake-p12-bytes")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"certificate", "upload",
			"--org-id", testIOSOrgID,
			"--cert-file", certPath,
			"--password", "-",
			"--json",
		},
		Env:     env.Environ(),
		WorkDir: dir,
		Stdin:   bytes.NewBufferString("hunter2\n"),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestCertificateUpload_MissingPasswordNonInteractive(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := setupIOSEnv(t, fake)

	dir := t.TempDir()
	certPath := writeBinaryFile(t, dir, "Distribution.p12", "x")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"certificate", "upload",
			"--org-id", testIOSOrgID,
			"--cert-file", certPath,
		},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestCertificateUpload_EmptyFile(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := setupIOSEnv(t, fake)

	dir := t.TempDir()
	certPath := writeBinaryFile(t, dir, "Empty.p12", "")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"certificate", "upload",
			"--org-id", testIOSOrgID,
			"--cert-file", certPath,
			"--password", "x",
		},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestCertificateUpload_NoToken(t *testing.T) {
	env := testenv.New(t)
	dir := t.TempDir()
	certPath := writeBinaryFile(t, dir, "Cert.p12", "x")

	result := binary.RunCLI(t, binary.RunOpts{
		Binary: binaryPath,
		Args: []string{
			"certificate", "upload",
			"--org-id", testIOSOrgID,
			"--cert-file", certPath,
			"--password", "p",
		},
		Env:     env.Environ(),
		WorkDir: dir,
	})

	assert.Equal(t, result.ExitCode, 3, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- certificate list ---

func TestCertificateList(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddIOSCert(testIOSOrgID, fakeIOSCert("cert-aaaa", "Distribution.p12"))
	fake.AddIOSCert(testIOSOrgID, fakeIOSCert("cert-bbbb", "Development.p12"))
	env := setupIOSEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"certificate", "list", "--org-id", testIOSOrgID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestCertificateList_Color(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddIOSCert(testIOSOrgID, fakeIOSCert("cert-aaaa", "Distribution.p12"))
	fake.AddIOSCert(testIOSOrgID, fakeIOSCert("cert-bbbb", "Development.p12"))
	env := setupIOSEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"certificate", "list", "--org-id", testIOSOrgID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
		TTY:     true,
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestCertificateList_JSON(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddIOSCert(testIOSOrgID, fakeIOSCert("cert-aaaa", "Distribution.p12"))
	fake.AddIOSCert(testIOSOrgID, fakeIOSCert("cert-bbbb", "Development.p12"))
	env := setupIOSEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"certificate", "list", "--org-id", testIOSOrgID, "--json"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".json"))
}

func TestCertificateList_Empty(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := setupIOSEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"certificate", "list", "--org-id", testIOSOrgID},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestCertificateList_NoOrgIDOutsideGit(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := setupIOSEnv(t, fake)

	// t.TempDir is not a git repo, so auto-detection fails and the user is
	// pointed at --org-id.
	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"certificate", "list"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 2, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

// --- certificate delete ---

func TestCertificateDelete(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddIOSCert(testIOSOrgID, fakeIOSCert("cert-zzzz", "Old.p12"))
	env := setupIOSEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"certificate", "delete", "cert-zzzz", "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 0, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, fake.DeletedIOSCert("cert-zzzz"))
}

func TestCertificateDelete_NotFound(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	env := setupIOSEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"certificate", "delete", "cert-missing", "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 5, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestCertificateDelete_RequiresForce(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddIOSCert(testIOSOrgID, fakeIOSCert("cert-zzzz", "Old.p12"))
	env := setupIOSEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"certificate", "delete", "cert-zzzz"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 6, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	assert.Check(t, !fake.DeletedIOSCert("cert-zzzz"))
}

func TestCertificateDelete_InUse(t *testing.T) {
	fake := fakes.NewCircleCI(t)
	fake.AddIOSCert(testIOSOrgID, fakeIOSCert("cert-zzzz", "Old.p12"))
	fake.AddIOSBundle(testIOSOrgID, fakeIOSSigningConfig("cfg-1111", "production-signing", "cert-zzzz", "Old.p12",
		"MyApp.mobileprovision"))
	env := setupIOSEnv(t, fake)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"certificate", "delete", "cert-zzzz", "--force"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Equal(t, result.ExitCode, 4, "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	assert.Check(t, !fake.DeletedIOSCert("cert-zzzz"))
}
