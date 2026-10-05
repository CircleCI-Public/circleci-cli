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

// Detect returns how the running binary was installed: one of "homebrew",
// "snap", "winget", "chocolatey", "deb", "rpm", "docker", "install-script" or
// "other". It never fails; anything it cannot place is "other".
func Detect() string {
	exe, err := os.Executable()
	if err != nil {
		return methodOther
	}
	// Package managers link the binary onto PATH, so follow the link to where it
	// really lives: /usr/local/bin/circleci on an Intel Mac is a symlink into
	// Homebrew's Cellar.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return fromPath(exe, fileExists)
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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
