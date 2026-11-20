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

package extschema

import "encoding/json"

type exportDoc struct {
	Extensions []exportExt `json:"extensions"`
}

type exportExt struct {
	Name          string       `json:"name"`
	Version       int          `json:"version"`
	MaxAttributes int          `json:"max_attributes"`
	Attributes    []exportAttr `json:"attributes"`
}

type exportAttr struct {
	ShortName string   `json:"short_name"`
	Property  string   `json:"property"`
	Type      string   `json:"type"` // "int" or "enum"
	Min       *int     `json:"min,omitempty"`
	Max       *int     `json:"max,omitempty"`
	Values    []string `json:"values,omitempty"`
	Present   string   `json:"present"`
}

// Export returns the schema as indented JSON with a final newline. The
// output is the same on every call.
func Export() ([]byte, error) {
	doc := exportDoc{Extensions: make([]exportExt, 0, len(specs))}
	for _, s := range specs {
		ext := exportExt{
			Name:          s.name,
			Version:       s.version,
			MaxAttributes: maxAttributes,
			Attributes:    make([]exportAttr, 0, len(s.attrs)),
		}
		for _, a := range s.attrs {
			ea := exportAttr{ShortName: a.short, Property: a.property, Present: a.present}
			switch a.kind {
			case kindInt:
				lo, hi := a.min, a.max
				ea.Type, ea.Min, ea.Max = "int", &lo, &hi
			case kindEnum:
				ea.Type, ea.Values = "enum", a.values
			}
			ext.Attributes = append(ext.Attributes, ea)
		}
		doc.Extensions = append(doc.Extensions, ext)
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
