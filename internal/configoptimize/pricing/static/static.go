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

// Package static is a pricing.Provider backed by a YAML table: the built-in
// default.yml, or a file the user supplies.
package static

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"slices"

	"gopkg.in/yaml.v3"
)

// Table is a loaded pricing table. It implements pricing.Provider.
type Table struct {
	flat    map[string]float64
	perMin  map[rateKey]float64
	ladders map[ladderKey][]string
	sizes   map[rateKey]size
}

type size struct{ vcpus, ramGB float64 }

type rateKey struct{ platform, class, generation string }

type ladderKey struct{ platform, generation string }

type document struct {
	FlatCharges      map[string]float64 `yaml:"flat_charges"`
	CreditsPerMinute []struct {
		Platform   string  `yaml:"platform"`
		Class      string  `yaml:"class"`
		Generation string  `yaml:"generation"`
		Credits    float64 `yaml:"credits"`
	} `yaml:"credits_per_minute"`
	Ladders []struct {
		Platform   string   `yaml:"platform"`
		Generation string   `yaml:"generation"`
		Classes    []string `yaml:"classes"` // smallest first
	} `yaml:"ladders"`
	Sizes []struct {
		Platform   string  `yaml:"platform"`
		Class      string  `yaml:"class"`
		Generation string  `yaml:"generation"`
		VCPUs      float64 `yaml:"vcpus"`
		RAMGB      float64 `yaml:"ram_gb"`
	} `yaml:"sizes"`
}

// Parse reads a pricing table. Unknown keys are an error, so a typo cannot
// silently drop a rate.
func Parse(data []byte) (*Table, error) {
	var doc document
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse pricing table: %w", err)
	}
	t := &Table{
		flat:    map[string]float64{},
		perMin:  map[rateKey]float64{},
		ladders: map[ladderKey][]string{},
		sizes:   map[rateKey]size{},
	}
	for kind, credits := range doc.FlatCharges {
		if credits < 0 {
			return nil, fmt.Errorf("parse pricing table: flat charge %q is negative", kind)
		}
		t.flat[kind] = credits
	}
	for _, r := range doc.CreditsPerMinute {
		t.perMin[rateKey{r.Platform, r.Class, r.Generation}] = r.Credits
	}
	for _, l := range doc.Ladders {
		t.ladders[ladderKey{l.Platform, l.Generation}] = slices.Clone(l.Classes)
	}
	for _, s := range doc.Sizes {
		if s.VCPUs <= 0 || s.RAMGB <= 0 {
			return nil, fmt.Errorf("parse pricing table: size of %s/%s must be positive", s.Platform, s.Class)
		}
		t.sizes[rateKey{s.Platform, s.Class, s.Generation}] = size{s.VCPUs, s.RAMGB}
	}
	return t, nil
}

// CreditsPerMinute implements pricing.Provider.
func (t *Table) CreditsPerMinute(platform, class, generation string) (float64, bool) {
	c, ok := t.perMin[rateKey{platform, class, generation}]
	return c, ok
}

// FlatCharge implements pricing.Provider.
func (t *Table) FlatCharge(kind string) (float64, bool) {
	c, ok := t.flat[kind]
	return c, ok
}

// LadderBelow implements pricing.Provider.
func (t *Table) LadderBelow(platform, class, generation string) (string, bool) {
	ladder := t.ladders[ladderKey{platform, generation}]
	i := slices.Index(ladder, class)
	if i <= 0 {
		return "", false
	}
	return ladder[i-1], true
}

// Size implements pricing.Provider.
func (t *Table) Size(platform, class, generation string) (float64, float64, bool) {
	s, ok := t.sizes[rateKey{platform, class, generation}]
	return s.vcpus, s.ramGB, ok
}

//go:embed default.yml
var defaultTable []byte

// Load reads a credit-rate table from path, or the built-in table when path is
// "". It also returns the sha256 of the bytes it parsed, which identifies the
// table a result was priced with.
func Load(path string) (*Table, string, error) {
	data := defaultTable
	if path != "" {
		b, err := os.ReadFile(path) //#nosec:G304 // a file the user names
		if err != nil {
			return nil, "", err
		}
		data = b
	}
	t, err := Parse(data)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(data)
	return t, hex.EncodeToString(sum[:]), nil
}
