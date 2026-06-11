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

package root

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

func stringifyReference(cmd *cobra.Command) string {
	var buf = bytes.Buffer{}
	for _, c := range cmd.Commands() {
		if c.Hidden {
			continue
		}
		cmdRef(&buf, c, 2)
	}
	return buf.String()
}

func cmdRef(w io.Writer, cmd *cobra.Command, depth int) {
	_, _ = fmt.Fprintf(w, "%s `%s`\n\n", strings.Repeat("#", depth), cmd.UseLine())
	_, _ = fmt.Fprintf(w, "%s\n\n", cmd.Short)

	flags := cmd.Flags()
	if flags.HasAvailableFlags() {
		_, _ = fmt.Fprintf(w, "%s\n\n", mdTable("Flag", "Description", flagRows(flags)))
	}

	if len(cmd.Aliases) > 0 {
		_, _ = fmt.Fprintf(w, "%s\n\n", "Aliases:")
		aliasList := buildAliasList(cmd, cmd.Aliases)
		for i, a := range aliasList {
			aliasList[i] = "`" + a + "`"
		}
		_, _ = fmt.Fprintf(w, "\n%s\n\n", dedent(strings.Join(aliasList, ", ")))
	}

	for _, c := range cmd.Commands() {
		if c.Hidden {
			continue
		}
		cmdRef(w, c, depth+1)
	}
}
