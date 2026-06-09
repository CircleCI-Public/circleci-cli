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

package iostream

import (
	"sort"

	"charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
)

// themeStyles is the resolved markdown style for every theme name the CLI
// accepts. It is effectively constant: built once at startup from glamour's
// built-in DefaultStyles plus our custom ansiStyle, then never mutated.
//
// Each built-in style is copied (DefaultStyles holds pointers to shared,
// package-level configs — we must not mutate those) and its inline-code padding
// is stripped. The built-in styles wrap inline code spans with a U+00A0
// NO-BREAK SPACE on each side; that stops word-wrap from breaking a span, but
// terminals treat NBSP as part of the adjacent word, so double-clicking an
// inline value (e.g. a UUID) also selects the invisible padding. Clearing the
// prefix/suffix avoids that. Our ansiStyle already uses no padding.
var themeStyles = func() map[string]ansi.StyleConfig {
	m := make(map[string]ansi.StyleConfig, len(styles.DefaultStyles)+1)
	for name, styleConfig := range styles.DefaultStyles {
		sc := *styleConfig
		sc.Code.Prefix = ""
		sc.Code.Suffix = ""
		m[name] = sc
	}
	sc := ansiStyleConfig
	sc.Code.Prefix = ""
	sc.Code.Suffix = ""

	m[ansiStyle] = sc
	return m
}()

// themeAuto detects the terminal's background and picks the dark or light
// style accordingly. It is the default value of the --theme flag and the
// "theme" CLI setting.
const themeAuto = "auto"

// ValidThemes returns the theme names accepted by the --theme flag and the
// "theme" CLI setting, sorted for stable display. This is the single source of
// truth for theme validation: "auto" detects the terminal background, "ansi"
// is our custom 16-color style, and the rest are glamour's built-in styles.
func ValidThemes() []string {
	themes := make([]string, 0, 1+len(themeStyles))
	themes = append(themes, themeAuto)
	for name := range themeStyles {
		themes = append(themes, name)
	}
	sort.Strings(themes)
	return themes
}

// IsValidTheme reports whether theme is one of the names returned by ValidThemes.
func IsValidTheme(theme string) bool {
	_, ok := themeStyles[theme]
	if !ok {
		return theme == themeAuto
	}
	return true
}
