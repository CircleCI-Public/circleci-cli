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

package cmdutil

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// compatEnvAnnotation marks a flag as a 0.1.x compatibility shim and names the
// environment variable its value is canonicalized into. Only flags carrying this
// annotation are treated as shims, so an unrelated local flag of the same name
// (for example the `runner config --token` value) is never mistaken for one.
const compatEnvAnnotation = "compat:env"

// AddV0AuthCompatFlags registers hidden --host and --token flags on fs, which the
// 0.1.x CLI accepted and v1 replaced with the CIRCLE_HOST and CIRCLE_TOKEN
// environment variables. Pass the PersistentFlags of a command group so every
// subcommand under it inherits them; 0.1.x defined both globally, so a script may
// attach them to any command in the group it calls.
//
// Published orbs pass these on every invocation — circleci/orb-tools runs
// `orb validate --host ... --token ...` — and orb versions are pinned in
// consumers' configs, so rejecting the flags breaks pipelines that the consumer
// cannot fix by updating the CLI. ApplyCompatFlags folds the values into the env
// vars that EffectiveHost and EffectiveToken read.
//
// A command needing --host or --token for something else declares its own, which
// shadows this one and carries no annotation, so ApplyCompatFlags leaves it alone.
// `runner config --token` and `setup --token` both rely on that.
func AddV0AuthCompatFlags(fs *pflag.FlagSet) {
	addCompatFlag(fs, "host", "CIRCLE_HOST", "CircleCI host (deprecated: set CIRCLE_HOST)")
	addCompatFlag(fs, "token", "CIRCLE_TOKEN", "CircleCI API token (deprecated: set CIRCLE_TOKEN)")
}

// AddV0OrgCompatFlags registers hidden --org-id and --org-slug flags that the
// 0.1.x CLI used before both were folded into --org, which accepts either a slug
// or a UUID. All three bind to org, so whichever the caller passes lands in the
// same place the canonical flag would. They are deliberately not marked mutually
// exclusive: 0.1.x accepted --org-id and --org-slug together, and rejecting that
// combination would break the pinned orb versions this shim exists to support.
func AddV0OrgCompatFlags(cmd *cobra.Command, org *string) {
	for _, name := range []string{"org-id", "org-slug"} {
		cmd.Flags().StringVar(org, name, "", "Organization slug or UUID (deprecated: use --org)")
		_ = cmd.Flags().MarkHidden(name)
	}
}

func addCompatFlag(fs *pflag.FlagSet, name, env, usage string) {
	fs.String(name, "", usage)
	_ = fs.MarkHidden(name)
	_ = fs.SetAnnotation(name, compatEnvAnnotation, []string{env})
}

// ApplyCompatFlags copies the value of every compatibility flag on cmd into the
// environment variable it maps to. Call it before configuration is loaded.
//
// A flag that was passed overwrites the variable, because that is the precedence
// 0.1.x gave it: an explicit --token beat CIRCLE_TOKEN. orb-tools relies on it —
// its publish job takes the name of the variable holding the publishing token, so
// it passes a token that is deliberately not the ambient CIRCLE_TOKEN. Letting the
// environment win would silently publish as the wrong identity. Visit only reports
// flags the caller actually set, so an omitted flag never clears anything.
func ApplyCompatFlags(cmd *cobra.Command) {
	cmd.Flags().Visit(func(f *pflag.Flag) {
		env, ok := f.Annotations[compatEnvAnnotation]
		if !ok || len(env) == 0 || f.Value.String() == "" {
			return
		}
		_ = os.Setenv(env[0], f.Value.String())
	})
}
