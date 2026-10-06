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

package installmethod

import (
	"slices"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestFromPath(t *testing.T) {
	tests := []struct {
		name       string
		exe        string
		dockerenv  bool
		files      []string // other paths that exist
		wantMethod string
	}{
		{
			name:       "Homebrew on Apple silicon",
			exe:        "/opt/homebrew/Cellar/circleci/1.0.51648/bin/circleci",
			wantMethod: "homebrew",
		},
		{
			name:       "Homebrew on Linux",
			exe:        "/home/linuxbrew/.linuxbrew/Cellar/circleci/1.0.51648/bin/circleci",
			wantMethod: "homebrew",
		},
		{
			name:       "Homebrew preview cask",
			exe:        "/opt/homebrew/Caskroom/circleci@next/1.0.51648/circleci",
			wantMethod: "homebrew",
		},
		{
			name:       "snap",
			exe:        "/snap/circleci/123/circleci",
			wantMethod: "snap",
		},
		{
			name:       "winget, per-user install",
			exe:        `C:\Users\dev\AppData\Local\Microsoft\WinGet\Packages\CircleCI.CLI_Microsoft.Winget.Source_8wekyb3d8bbwe\circleci.exe`,
			wantMethod: "winget",
		},
		{
			name:       "winget, machine-wide install",
			exe:        `C:\Program Files\WinGet\Packages\CircleCI.CLI_Microsoft.Winget.Source_8wekyb3d8bbwe\circleci.exe`,
			wantMethod: "winget",
		},
		{
			name:       "Chocolatey",
			exe:        `C:\ProgramData\chocolatey\lib\circleci\tools\circleci.exe`,
			wantMethod: "chocolatey",
		},
		{
			name:       "deb package",
			exe:        "/usr/bin/circleci",
			files:      []string{"/var/lib/dpkg/info/circleci.list"},
			wantMethod: "deb",
		},
		{
			name:       "rpm package",
			exe:        "/usr/bin/circleci",
			files:      []string{"/var/lib/rpm"},
			wantMethod: "rpm",
		},
		{
			name:       "rpm package on a newer Fedora",
			exe:        "/usr/bin/circleci",
			files:      []string{"/usr/lib/sysimage/rpm"},
			wantMethod: "rpm",
		},
		{
			name:       "in /usr/bin but from neither package",
			exe:        "/usr/bin/circleci",
			wantMethod: "other",
		},
		{
			name:       "install script default directory",
			exe:        "/usr/local/bin/circleci",
			wantMethod: "install-script",
		},
		{
			name:       "Docker image",
			exe:        "/usr/local/bin/circleci",
			dockerenv:  true,
			wantMethod: "docker",
		},
		{
			name:       "inside a container but not at the image path",
			exe:        "/home/circleci/bin/circleci",
			dockerenv:  true,
			wantMethod: "other",
		},
		{
			name:       "archive extracted somewhere else",
			exe:        "/home/dev/tools/circleci",
			wantMethod: "other",
		},
		{
			name:       "empty path",
			exe:        "",
			wantMethod: "other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists := func(path string) bool {
				if tt.dockerenv && path == "/.dockerenv" {
					return true
				}
				return slices.Contains(tt.files, path)
			}
			assert.Check(t, cmp.Equal(fromPath(tt.exe, exists), tt.wantMethod))
		})
	}
}

func TestUpgradeCommandFor(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		exe     string
		files   []string // paths that exist
		wantCmd string
	}{
		{name: "Homebrew formula", method: "homebrew", exe: "/opt/homebrew/Cellar/circleci/1.2.0/bin/circleci", wantCmd: "brew upgrade circleci"},
		{name: "Homebrew preview cask", method: "homebrew", exe: "/opt/homebrew/Caskroom/circleci@next/1.2.0/circleci", wantCmd: "brew upgrade --cask circleci@next"},
		{name: "snap", method: "snap", exe: "/snap/circleci/123/circleci", wantCmd: "sudo snap refresh circleci"},
		{
			name:    "winget",
			method:  "winget",
			exe:     `C:\Users\dev\AppData\Local\Microsoft\WinGet\Packages\CircleCI.CLI_Microsoft.Winget.Source_8wekyb3d8bbwe\circleci.exe`,
			wantCmd: "winget upgrade --id CircleCI.CLI",
		},
		{
			name:    "winget preview package",
			method:  "winget",
			exe:     `C:\Users\dev\AppData\Local\Microsoft\WinGet\Packages\CircleCI.CLI.Preview_Microsoft.Winget.Source_8wekyb3d8bbwe\circleci.exe`,
			wantCmd: "winget upgrade --id CircleCI.CLI.Preview",
		},
		{name: "Chocolatey", method: "chocolatey", exe: `C:\ProgramData\chocolatey\lib\circleci-cli\tools\circleci.exe`, wantCmd: "choco upgrade circleci-cli"},
		{name: "deb", method: "deb", exe: "/usr/bin/circleci", wantCmd: "sudo apt-get update && sudo apt-get install --only-upgrade circleci"},
		{name: "rpm", method: "rpm", exe: "/usr/bin/circleci", wantCmd: "sudo dnf upgrade --refresh circleci"},
		{name: "Docker image", method: "docker", exe: "/usr/local/bin/circleci", wantCmd: "docker pull circleci/circleci-cli:v1"},
		{name: "Alpine Docker image", method: "docker", exe: "/usr/local/bin/circleci", files: []string{"/etc/alpine-release"}, wantCmd: "docker pull circleci/circleci-cli:v1-alpine"},
		{name: "install script", method: "install-script", exe: "/usr/local/bin/circleci", wantCmd: installScriptCommand},
		{name: "unknown install has no command", method: "other", exe: "/home/dev/tools/circleci", wantCmd: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists := func(path string) bool { return slices.Contains(tt.files, path) }
			assert.Check(t, cmp.Equal(upgradeCommandFor(tt.method, tt.exe, exists), tt.wantCmd))
		})
	}
}

func TestForceEnv(t *testing.T) {
	t.Run("a known method overrides detection", func(t *testing.T) {
		t.Setenv(ForceEnv, "homebrew")
		assert.Check(t, cmp.Equal(Detect(), "homebrew"))
		assert.Check(t, cmp.Equal(UpgradeCommand(), "brew upgrade circleci"))
	})

	t.Run("an unknown value is ignored", func(t *testing.T) {
		t.Setenv(ForceEnv, "not-a-method")
		// The test binary runs from Go's build cache, which is no install method.
		assert.Check(t, cmp.Equal(Detect(), "other"))
		assert.Check(t, cmp.Equal(UpgradeCommand(), ""))
	})
}
