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

// Package installmethod works out how the running circleci binary was
// installed, from where it lives on disk. The result is a fixed category name,
// never the path itself, so it is safe to report in telemetry.
package installmethod

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	methodChocolatey    = "chocolatey"
	methodDeb           = "deb"
	methodDocker        = "docker"
	methodHomebrew      = "homebrew"
	methodInstallScript = "install-script"
	methodOther         = "other"
	methodRPM           = "rpm"
	methodSnap          = "snap"
	methodWinget        = "winget"
)

// ForceEnv makes Detect and UpgradeCommand report the named install method
// instead of working it out from the binary's path. Acceptance tests need it
// because the binary under test always runs from a temp dir. Internal (double
// underscore); never user-set.
const ForceEnv = "__CIRCLE_INSTALL_METHOD"

// previewCaskDir is part of the path of a binary installed from the preview
// Homebrew cask, which upgrades with a different command from the formula.
const previewCaskDir = "/Caskroom/circleci@next/"

// installScriptCommand re-runs the install script, which puts the latest release
// over the old binary and uses sudo itself when the directory needs it.
const installScriptCommand = "curl -fsSL https://raw.githubusercontent.com/CircleCI-Public/circleci-cli/main/install.sh | bash"

// Detect returns how the running binary was installed: one of "homebrew",
// "snap", "winget", "chocolatey", "deb", "rpm", "docker", "install-script" or
// "other". It never fails; anything it cannot place is "other".
func Detect() string {
	method, _ := detect()
	return method
}

// UpgradeCommand returns the command that upgrades the running binary the same
// way it was installed, such as "brew upgrade circleci", or "" when the install
// method is unknown and there is no command to recommend.
func UpgradeCommand() string {
	_, command := detect()
	return command
}

func detect() (method, upgradeCommand string) {
	method, exe, exists := resolve()
	return method, upgradeCommandFor(method, exe, exists)
}

// HomebrewUpgrade returns the command line that upgrades a Homebrew install of
// the running binary, naming brew by its full path so it works where brew is
// not on PATH, as in many agent sandboxes. ok is false for any other install
// method, or when brew cannot be found.
func HomebrewUpgrade() (argv []string, ok bool) {
	method, exe, exists := resolve()
	return homebrewUpgradeFor(method, exe, exists, exec.LookPath)
}

func homebrewUpgradeFor(method, exe string, exists func(string) bool, lookPath func(string) (string, error)) ([]string, bool) {
	if method != methodHomebrew {
		return nil, false
	}
	brew := brewPath(exe, exists, lookPath)
	if brew == "" {
		return nil, false
	}
	if strings.Contains(exe, previewCaskDir) {
		return []string{brew, "upgrade", "--cask", "circleci@next"}, true
	}
	return []string{brew, "upgrade", "circleci"}, true
}

// brewPath finds the brew that manages exe: <prefix>/bin/brew, where prefix holds
// Homebrew's Cellar or Caskroom, falling back to brew on PATH.
func brewPath(exe string, exists func(string) bool, lookPath func(string) (string, error)) string {
	for _, dir := range []string{"/Cellar/", "/Caskroom/"} {
		if i := strings.Index(exe, dir); i > 0 {
			if candidate := exe[:i] + "/bin/brew"; exists(candidate) {
				return candidate
			}
		}
	}
	if p, err := lookPath("brew"); err == nil {
		return p
	}
	return ""
}

// resolve works out the install method along with the resolved executable path
// and the file check used, which the command builders need for variants.
func resolve() (method, exe string, exists func(string) bool) {
	if forced := os.Getenv(ForceEnv); isMethod(forced) {
		return forced, "", func(string) bool { return false }
	}

	exe, err := os.Executable()
	if err != nil {
		return methodOther, "", fileExists
	}
	// Package managers link the binary onto PATH, so follow the link to where it
	// really lives: /usr/local/bin/circleci on an Intel Mac is a symlink into
	// Homebrew's Cellar.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return fromPath(exe, fileExists), exe, fileExists
}

func isMethod(s string) bool {
	switch s {
	case methodChocolatey, methodDeb, methodDocker, methodHomebrew, methodInstallScript,
		methodOther, methodRPM, methodSnap, methodWinget:
		return true
	}
	return false
}

// fromPath classifies a resolved executable path. exists reports whether a file
// is present, so tests can stand in for the filesystem.
func fromPath(exe string, exists func(string) bool) string {
	// Windows paths are case-insensitive, and installers disagree on casing.
	lower := strings.ToLower(exe)

	switch {
	case strings.Contains(exe, "/Cellar/"), strings.Contains(exe, "/Caskroom/"):
		return methodHomebrew
	case strings.HasPrefix(exe, "/snap/"):
		return methodSnap
	case strings.Contains(lower, `\winget\packages\`):
		return methodWinget
	case strings.Contains(lower, `\chocolatey\`):
		return methodChocolatey
	case exe == "/usr/bin/circleci":
		// Where the deb and rpm packages install the binary. Which one matters,
		// because they upgrade with different commands. dpkg keeps a file list
		// for every package it installed, so that names the deb; otherwise an rpm
		// database means the rpm. Without either, something else put it here.
		switch {
		case exists("/var/lib/dpkg/info/circleci.list"):
			return methodDeb
		case exists("/var/lib/rpm"), exists("/usr/lib/sysimage/rpm"):
			return methodRPM
		default:
			return methodOther
		}
	case exe == "/usr/local/bin/circleci":
		// The Docker images and the install script both put the binary here, so
		// /.dockerenv is what tells them apart. A binary copied here by hand also
		// lands in install-script; that is the best this path can say.
		if exists("/.dockerenv") {
			return methodDocker
		}
		return methodInstallScript
	default:
		return methodOther
	}
}

// upgradeCommandFor returns the command that upgrades an install of the given
// method. exe and exists pick between variants of one method: the preview
// Homebrew cask and winget package, and the Alpine Docker image.
func upgradeCommandFor(method, exe string, exists func(string) bool) string {
	switch method {
	case methodHomebrew:
		if strings.Contains(exe, previewCaskDir) {
			return "brew upgrade --cask circleci@next"
		}
		return "brew upgrade circleci"
	case methodSnap:
		return "sudo snap refresh circleci"
	case methodWinget:
		if strings.Contains(strings.ToLower(exe), `\circleci.cli.preview_`) {
			return "winget upgrade --id CircleCI.CLI.Preview"
		}
		return "winget upgrade --id CircleCI.CLI"
	case methodChocolatey:
		return "choco upgrade circleci-cli"
	case methodDeb:
		// apt only sees the new version once its package lists are refreshed, and
		// install --only-upgrade upgrades just this package, where apt-get upgrade
		// would upgrade everything on the machine.
		return "sudo apt-get update && sudo apt-get install --only-upgrade circleci"
	case methodRPM:
		return "sudo dnf upgrade --refresh circleci"
	case methodDocker:
		if exists("/etc/alpine-release") {
			return "docker pull circleci/circleci-cli:v1-alpine"
		}
		return "docker pull circleci/circleci-cli:v1"
	case methodInstallScript:
		return installScriptCommand
	default:
		return ""
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
