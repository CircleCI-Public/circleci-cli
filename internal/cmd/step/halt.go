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

package step

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	clierrors "github.com/CircleCI-Public/circleci-cli/internal/errors"
)

func newHaltCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "halt",
		Short: "Halt the current job and mark it as successful",
		Long: heredoc.Doc(`
			Halt the running CircleCI job immediately and mark it as successful.

			Any steps that follow in the job configuration will not execute.

			This command delegates to circleci-agent and must be called from
			within a running CircleCI job.
		`),
		DisableFlagParsing: true,
		RunE: func(_ *cobra.Command, args []string) error {
			agent, err := exec.LookPath("circleci-agent")
			if err != nil {
				return clierrors.New(
					"step.agent_not_found",
					"circleci-agent not found",
					"circleci step halt must be called from within a running CircleCI job",
				).WithExitCode(clierrors.ExitNotFound)
			}

			argv := append([]string{agent, "step", "halt"}, args...)
			return syscall.Exec(agent, argv, os.Environ()) // #nosec
		},
	}
}
