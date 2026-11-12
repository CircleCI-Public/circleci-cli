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

package configaudit

import (
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	orbRefRE      = regexp.MustCompile(`^[\w-]+/[\w-]+(@(.+))?$`)
	exactOrbVerRE = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	paramRefRE    = regexp.MustCompile(`^<<\s*parameters\.([\w-]+)\s*>>$`)
)

const orbPinFix = "pin the orb to an exact version (name@X.Y.Z)"

// checkUnpinnedOrb flags orbs that are not pinned to an exact X.Y.Z version.
// Inline orb definitions (mappings) are not references and are skipped.
func checkUnpinnedOrb(d *Doc, emit func(Finding)) {
	for _, kv := range mapPairs(d.orbs) {
		n := kv[1]
		if n == nil || n.Kind != yaml.ScalarNode {
			continue
		}
		m := orbRefRE.FindStringSubmatch(n.Value)
		if m == nil {
			continue
		}
		var msg string
		switch version := m[2]; {
		case m[1] == "":
			msg = "orb " + n.Value + " has no version"
		case version == "volatile":
			msg = "orb " + n.Value + " uses @volatile"
		case strings.HasPrefix(version, "dev:"):
			msg = "orb " + n.Value + " is a dev version"
		case !exactOrbVerRE.MatchString(version):
			msg = "orb " + n.Value + " is a major-version alias"
		default:
			continue
		}
		pos := locate(n, d.lines, 0, len(n.Value))
		emit(Finding{RuleID: "unpinned-orb", Message: msg, Fix: orbPinFix, Position: &pos})
	}
}

// checkUnpinnedImage flags docker images in jobs and executors that do not
// carry a digest.
func checkUnpinnedImage(d *Doc, emit func(Finding)) {
	for _, section := range []*yaml.Node{d.jobs, d.executors} {
		for _, kv := range mapPairs(section) {
			owner := kv[1]
			if owner == nil || owner.Kind != yaml.MappingNode {
				continue
			}
			for _, entry := range seqItems(mapGet(owner, "docker")) {
				img := mapGet(entry, "image")
				if img == nil || img.Kind != yaml.ScalarNode {
					continue
				}
				checkImage(d, owner, img, emit)
			}
		}
	}
}

func checkImage(d *Doc, owner, img *yaml.Node, emit func(Finding)) {
	ref := img.Value
	confidence := ConfidenceHigh
	var via string
	if m := paramRefRE.FindStringSubmatch(ref); m != nil {
		def := mapGet(mapGet(mapGet(owner, "parameters"), m[1]), "default")
		if def == nil || def.Kind != yaml.ScalarNode {
			return
		}
		ref = def.Value
		confidence = ConfidenceLow
		via = " (default of parameter " + m[1] + ")"
	}
	if strings.Contains(ref, "@sha256:") {
		return
	}
	severity, msg := SeverityMedium, "image "+ref+via+" is pinned to a tag, not a digest"
	switch tag, hasTag := imageTag(ref); {
	case !hasTag:
		severity, msg = SeverityHigh, "image "+ref+via+" has no tag or digest"
	case tag == "latest":
		severity, msg = SeverityHigh, "image "+ref+via+" uses the latest tag"
	}
	pos := locate(img, d.lines, 0, len(img.Value))
	emit(Finding{
		RuleID:     "unpinned-image",
		Severity:   severity,
		Confidence: confidence,
		Message:    msg,
		Position:   &pos,
	})
}

// imageTag returns the tag of an image reference. A colon before the last
// slash is a registry port, not a tag.
func imageTag(ref string) (string, bool) {
	i := strings.LastIndex(ref, ":")
	if i < 0 || i < strings.LastIndex(ref, "/") {
		return "", false
	}
	return ref[i+1:], true
}
