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
	"context"
	"encoding/json"
	"os/exec"
	"runtime"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/extension"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

// testReference documents one subcommand, "do", plus a flag declared on the
// extension itself, so tests can check both reach the generated tool.
func testReference() *extension.Reference {
	return &extension.Reference{
		Use:  "<command>",
		Long: "A test extension with enriched command documentation.",
		Flags: []extension.ReferenceFlag{
			{Name: "verbose", Usage: "Enable verbose logging"},
		},
		Subcommands: []extension.ReferenceSub{
			{
				Name: "do",
				Use:  "<name> [flags]",
				Long: "Do a thing with the given name.",
				Args: []extension.ReferenceArg{
					{Name: "<name>", Help: "The name to operate on."},
				},
				Flags: []extension.ReferenceFlag{
					{Name: "parallel", Shorthand: "p", Type: "int", Default: "4", Usage: "Parallel workers"},
					{Name: "mode", Type: "fast|slow", Default: "fast", Usage: "How to do it"},
				},
			},
		},
	}
}

// withExtension serves extName from registry, documented by ref.
func withExtension(t *testing.T, registry *fakes.ExtensionRegistry, extName string, ref *extension.Reference) {
	t.Helper()
	registry.WithExtension(t,
		extension.Manifest{
			Name:       extName,
			BinaryName: "circleci-" + extName,
			Version:    "1.0.0",
			Path:       testBinaryPath,
			Ref:        ref,
		},
		fakes.ExtensionMeta{Arch: runtime.GOARCH, Sys: runtime.GOOS},
	)
}

func TestMCPToolsExtensionSubcommands(t *testing.T) {
	registry := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = registry.URL()

	installExtensionWithRef(t, registry, env, "testextension", testReference())
	tools := runMCPTools(t, env)

	t.Run("subcommand tool has its flags and none of the root flags", func(t *testing.T) {
		tool, ok := tools["circleci_testextension_do"]
		assert.Assert(t, ok, "subcommand tool not exported")
		assert.Check(t, cmp.DeepEqual(tool.flagNames(), []string{"mode", "parallel", "verbose"}))
		assert.Check(t, golden.String(tool.Description, t.Name()+".txt"))
	})

	t.Run("extension tool has none of the root flags", func(t *testing.T) {
		tool, ok := tools["circleci_testextension"]
		assert.Assert(t, ok, "extension tool not exported")
		assert.Check(t, cmp.Len(tool.flagNames(), 0))
	})

	t.Run("extension tool is described by its reference", func(t *testing.T) {
		tool, ok := tools["circleci_testextension"]
		assert.Assert(t, ok, "extension tool not exported")
		assert.Check(t, golden.String(tool.Description, t.Name()+".txt"))
	})

	t.Run("other tools keep the root flags", func(t *testing.T) {
		tool, ok := tools["circleci_extension_install"]
		assert.Assert(t, ok)
		assert.Check(t, cmp.Contains(tool.flagNames(), "debug"))
	})
}

func TestMCPToolsOfficialExtensionFromCachedReference(t *testing.T) {
	registry := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = registry.URL()

	withExtension(t, registry, "testsuite", testReference())

	t.Run("mcp tools documents the uninstalled extension", func(t *testing.T) {
		tools := runMCPTools(t, env)
		tool, ok := tools["circleci_testsuite_do"]
		assert.Assert(t, ok, "subcommand tool not exported for the uninstalled extension")
		assert.Check(t, cmp.DeepEqual(tool.flagNames(), []string{"mode", "parallel", "verbose"}))
		assert.Check(t, golden.String(tools["circleci_testsuite"].Description, t.Name()+".txt"))
	})

	t.Run("help reference uses the cached reference", func(t *testing.T) {
		result := binary.RunCLI(t, binary.RunOpts{
			Binary:  binaryPath,
			Args:    []string{"help", "reference"},
			Env:     env.Environ(),
			WorkDir: t.TempDir(),
		})
		assert.Check(t, cmp.Equal(result.ExitCode, 0))
		section := findSection(t, result.Stdout, "### `circleci testsuite <command>`")
		assert.Check(t, golden.String(section, t.Name()+".txt"))
	})

	t.Run("the cache is reused without fetching again", func(t *testing.T) {
		registry.RequestRecorder.Reset()
		tools := runMCPTools(t, env)
		_, ok := tools["circleci_testsuite_do"]
		assert.Check(t, ok)
		assert.Check(t, cmp.Len(registry.RequestRecorder.AllRequests(), 0))
	})
}

func TestMCPToolsOfficialExtensionRegistryUnavailable(t *testing.T) {
	// The fake registry serves no testsuite, so the reference fetch fails.
	registry := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = registry.URL()

	t.Run("the extension tool is still exported", func(t *testing.T) {
		tools := runMCPTools(t, env)
		_, ok := tools["circleci_testsuite"]
		assert.Check(t, ok)
		_, ok = tools["circleci_testsuite_do"]
		assert.Check(t, !ok, "no subcommand tools without a reference")
		assert.Check(t, cmp.Len(registry.RequestRecorder.AllRequests(), 1))
	})

	t.Run("a failed fetch is not retried straight away", func(t *testing.T) {
		registry.RequestRecorder.Reset()
		runMCPTools(t, env)
		assert.Check(t, cmp.Len(registry.RequestRecorder.AllRequests(), 0))
	})
}

func TestMCPToolsOfficialExtensionUpdateCheckDisabled(t *testing.T) {
	registry := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = registry.URL()
	env.Extra["CIRCLE_NO_UPDATE_CHECK"] = "1"

	withExtension(t, registry, "testsuite", testReference())
	tools := runMCPTools(t, env)

	_, ok := tools["circleci_testsuite_do"]
	assert.Check(t, !ok, "the reference must not be fetched when update checks are off")
	assert.Check(t, cmp.Len(registry.RequestRecorder.AllRequests(), 0))
}

// TestMCPCallExtensionSubcommandTool drives a real `mcp start` server and calls
// a generated subcommand tool, checking the extension receives the subcommand,
// its flag and its argument.
func TestMCPCallExtensionSubcommandTool(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	registry := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = registry.URL()
	installExtensionWithRef(t, registry, env, "testextension", testReference())

	session := startMCPServer(ctx, t, env)
	assert.Check(t, cmp.Equal(session.InitializeResult().Instructions, ""),
		"an extension whose reference has no instructions sends none")

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "circleci_testextension_do",
		Arguments: map[string]any{
			"flags": map[string]any{"parallel": "2"},
			"args":  []string{"thing"},
		},
	})
	assert.NilError(t, err)
	assert.Check(t, !res.IsError, "tool call reported an error")

	b, err := json.Marshal(res.StructuredContent)
	assert.NilError(t, err)
	var out struct {
		ExitCode int    `json:"exitCode"`
		Stdout   string `json:"stdout"`
	}
	assert.NilError(t, json.Unmarshal(b, &out))
	assert.Check(t, cmp.Equal(out.ExitCode, 0))
	assert.Check(t, golden.String(out.Stdout, t.Name()+".txt"))
}

// installExtensionWithRef installs extName documented by ref, with the fixed
// telemetry IDs installExtension uses.
func installExtensionWithRef(t *testing.T, registry *fakes.ExtensionRegistry, env *testenv.TestEnv, extName string, ref *extension.Reference) {
	t.Helper()
	withExtension(t, registry, extName, ref)

	result := binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"extension", "install", extName},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
	assert.Assert(t, cmp.Equal(result.ExitCode, 0))

	writeCLIConfig(t, env, "user_id: "+testUserID+"\n")
}

// TestMCPServerInstructionsFromReferences checks the server sends the
// instructions each extension's reference supplies, joined in command order:
// here an installed extension's and a cached one for uninstalled testsuite.
func TestMCPServerInstructionsFromReferences(t *testing.T) {
	ctx := iostream.Testing(context.Background())

	registry := fakes.NewExtensionRegistry(t)
	env := testenv.New(t)
	env.Token = testToken
	env.ExtensionRegistryURL = registry.URL()

	installed := testReference()
	installed.Instructions = "Use the testextension tools."
	installExtensionWithRef(t, registry, env, "testextension", installed)
	withExtension(t, registry, "testsuite", &extension.Reference{
		Instructions: "Use the testsuite tools.",
		Subcommands:  []extension.ReferenceSub{{Name: "doctor"}},
	})

	session := startMCPServer(ctx, t, env)
	assert.Check(t, golden.String(session.InitializeResult().Instructions, t.Name()+".txt"))
}

// startMCPServer runs `mcp start` over stdio and returns a connected client
// session, closed when the test ends.
func startMCPServer(ctx context.Context, t *testing.T, env *testenv.TestEnv) *mcp.ClientSession {
	t.Helper()

	serverCmd := exec.Command(binaryPath, "mcp", "start")
	serverCmd.Env = env.Environ()
	serverCmd.Dir = t.TempDir()

	client := mcp.NewClient(&mcp.Implementation{Name: "acceptance-test", Version: "dev"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: serverCmd}, nil)
	assert.NilError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}
