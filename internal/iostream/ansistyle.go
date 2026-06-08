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

import "charm.land/glamour/v2/ansi"

// ansiStyle is the name of our custom markdown theme. It is deliberately not a
// glamour built-in (it does not appear in styles.DefaultStyles); styleConfig
// special-cases it and returns ansiStyleConfig.
const ansiStyle = "ansi"

// ansiStyleConfig is a markdown style that uses only the standard 16 ANSI
// colors (codes "0"–"15"). Unlike glamour's built-in dark/light styles — which
// hard-code 256-color and hex values — this style defers to whatever palette
// the user's terminal is configured with, so it looks correct in any color
// scheme (including custom themes and high-contrast setups).
//
// Color codes:
//
//	0 black    1 red      2 green    3 yellow   4 blue    5 magenta   6 cyan    7 white
//	8 br-black 9 br-red  10 br-green 11 br-yellow 12 br-blue 13 br-magenta 14 br-cyan 15 br-white
var ansiStyleConfig = ansi.StyleConfig{
	Document: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			BlockPrefix: "\n",
			BlockSuffix: "\n",
		},
		Margin: new(uint(2)),
	},
	BlockQuote: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Color: new("8"),
		},
		Indent:      new(uint(1)),
		IndentToken: new("│ "),
	},
	List: ansi.StyleList{
		LevelIndent: 2,
	},
	Heading: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			BlockSuffix: "\n",
			Color:       new("12"),
			Bold:        new(true),
		},
	},
	H1: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix:          " ",
			Suffix:          " ",
			Color:           new("15"),
			BackgroundColor: new("4"),
			Bold:            new(true),
		},
	},
	H2: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "## ",
		},
	},
	H3: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "### ",
		},
	},
	H4: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "#### ",
		},
	},
	H5: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "##### ",
		},
	},
	H6: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: "###### ",
			Color:  new("6"),
			Bold:   new(false),
		},
	},
	Strikethrough: ansi.StylePrimitive{
		CrossedOut: new(true),
	},
	Emph: ansi.StylePrimitive{
		Italic: new(true),
	},
	Strong: ansi.StylePrimitive{
		Bold: new(true),
	},
	HorizontalRule: ansi.StylePrimitive{
		Color:  new("8"),
		Format: "\n--------\n",
	},
	Item: ansi.StylePrimitive{
		BlockPrefix: "• ",
	},
	Enumeration: ansi.StylePrimitive{
		BlockPrefix: ". ",
	},
	Task: ansi.StyleTask{
		Ticked:   "[✓] ",
		Unticked: "[ ] ",
	},
	Link: ansi.StylePrimitive{
		Color:     new("6"),
		Underline: new(true),
	},
	LinkText: ansi.StylePrimitive{
		Color: new("14"),
		Bold:  new(true),
	},
	Image: ansi.StylePrimitive{
		Color:     new("13"),
		Underline: new(true),
	},
	ImageText: ansi.StylePrimitive{
		Color:  new("8"),
		Format: "Image: {{.text}} →",
	},
	Code: ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: " ", // non-breaking space to prevent hard breaks
			Suffix: " ",
			Color:  new("9"),
		},
	},
	CodeBlock: ansi.StyleCodeBlock{
		StyleBlock: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: new("7"),
			},
			Margin: new(uint(2)),
		},
		// No Chroma block on purpose: the chroma syntax highlighter only
		// understands hex / named colors (fixed RGB), not ANSI palette
		// indices, so any highlighting here would stop adapting to the
		// terminal's own palette. Code blocks render in the plain ANSI
		// CodeBlock color above instead — same approach as the ascii style.
	},
	Table: ansi.StyleTable{
		CenterSeparator: new("┼"),
		ColumnSeparator: new("│"),
		RowSeparator:    new("─"),
	},
	DefinitionDescription: ansi.StylePrimitive{
		BlockPrefix: "\n🠶 ",
	},
}
