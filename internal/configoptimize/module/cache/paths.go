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

package cache

import (
	"strconv"
	"strings"

	"github.com/CircleCI-Public/circleci-cli/internal/configoptimize/pipelineconfig"
)

// PathClass says what a cache stores, from its save_cache paths.
type PathClass int

// Path classes: DependencyPaths when every path is a known dependency
// directory; BuildOutputPaths when every path is known build output;
// otherwise the paths are mixed, parameterized, unknown, or there is no
// visible save.
const (
	NoVisibleSave PathClass = iota
	DependencyPaths
	BuildOutputPaths
	MixedPaths
	UnknownPaths
	ParameterizedPaths
)

func (c PathClass) String() string {
	switch c {
	case DependencyPaths:
		return "dependency paths"
	case BuildOutputPaths:
		return "build output"
	case MixedPaths:
		return "a mix of dependency and other paths"
	case UnknownPaths:
		return "not on the dependency list"
	case ParameterizedPaths:
		return "set by a parameter"
	case NoVisibleSave:
	}
	return "not visible (no matching save_cache in this config)"
}

// dependencyPaths are package manager caches and install directories, the
// explicit list X1-static-v2 accepts for workflow-, pipeline- and
// revision-scoped findings. A path matches an entry, or lies under one.
var dependencyPaths = []string{
	"~/.npm", "node_modules", "~/.cache/yarn", ".yarn/cache", "~/.pnpm-store", "~/.local/share/pnpm/store",
	"~/.cache/pip", "~/.cache/pypoetry", "~/.cache/uv",
	"~/.m2", "~/.gradle/caches", "~/.gradle/wrapper",
	"~/go/pkg/mod",
	"~/.cargo/registry", "~/.cargo/git",
	"vendor/bundle", "~/.bundle",
	"~/.composer/cache",
}

// buildOutputPaths are build results and build tool caches. A scoped cache
// of these is a normal way to pass output between jobs or workflows, so it
// gives no finding (the Workspaces research page).
var buildOutputPaths = []string{
	"~/.cache/go-build", "target", "build", "dist", "out", ".next",
}

// ClassifyPath places one path, after writing /home/circleci/ as ~/ and
// dropping ./ and a trailing /.
func ClassifyPath(path string) PathClass {
	p := strings.TrimSuffix(strings.TrimPrefix(path, "./"), "/")
	if rest, ok := strings.CutPrefix(p, "/home/circleci/"); ok {
		p = "~/" + rest
	}
	under := func(list []string) bool {
		for _, e := range list {
			if p == e || strings.HasPrefix(p, e+"/") || strings.HasSuffix(p, "/"+e) {
				return true
			}
		}
		return false
	}
	switch {
	case strings.ContainsAny(p, "$*{"):
		return UnknownPaths
	case under(dependencyPaths):
		return DependencyPaths
	case under(buildOutputPaths):
		return BuildOutputPaths
	}
	return UnknownPaths
}

// savedPaths collects the paths of the saves and classifies them together.
// A path set by a parameter is traced back: if the authored path has a
// reference, the class is ParameterizedPaths.
func savedPaths(cfg *pipelineconfig.Effective, saves []*Step) ([]string, PathClass) {
	if len(saves) == 0 {
		return nil, NoVisibleSave
	}
	var paths []string
	classes := map[PathClass]bool{}
	for _, s := range saves {
		if len(s.Paths) == 0 {
			classes[UnknownPaths] = true
		}
		for i, p := range s.Paths {
			paths = append(paths, p)
			tr := cfg.Trace(s.Job, append(append([]string{}, s.Path...), "save_cache", "paths", strconv.Itoa(i)))
			if tr.Problem != "" || strings.Contains(tr.Template, "<<") {
				classes[ParameterizedPaths] = true
				continue
			}
			classes[ClassifyPath(p)] = true
		}
	}
	switch {
	case classes[ParameterizedPaths]:
		return paths, ParameterizedPaths
	case len(classes) == 1:
		for c := range classes {
			return paths, c
		}
	case classes[DependencyPaths]:
		return paths, MixedPaths
	}
	return paths, UnknownPaths
}
