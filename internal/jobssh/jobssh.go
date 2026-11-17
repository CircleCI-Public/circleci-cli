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

// Package jobssh finds the ssh command for jobs rerun with SSH enabled.
//
// A job rerun with SSH gets an "Enable SSH" step (v3 step type enable_ssh) once
// it starts. The step runs in the background for the life of the job, and its
// output names the command to connect, e.g.
//
//	You can now SSH into this job if your SSH public key is added:
//	    $ ssh b0023961-07d3-4943-8342-a052e5e9307a-0@ssh.circleci.com
//
// Older jobs print the host and port form instead (ssh -p 54782 3.94.12.34);
// both are read the same way. A parallel job has one such step, and one
// command, per execution.
package jobssh

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
)

// StepType is the v3 step type of the step that enables SSH on a job.
const StepType = "enable_ssh"

// stepName is the step's display name, matched when a response has no type.
const stepName = "Enable SSH"

// Conn is how to connect to one execution of a job.
type Conn struct {
	Job        string    // job name
	JobID      uuid.UUID // the rerun job's ID
	Execution  int       // parallel execution index
	Executions int       // number of parallel executions the job has
	Command    string    // the ssh command, as the job printed it
	User       string    // user part of the destination, if any
	Host       string
	Port       int // 0 when the command names no port
}

// commandLine matches the line the SSH step prints the command on. The output
// uses CRLF line endings, hence the trailing \s*.
var commandLine = regexp.MustCompile(`(?m)^\s*\$ (ssh [^\r\n]*?)\s*$`)

// ParseOutput reads the ssh command from an SSH step's output. ok is false
// when the output does not hold one yet.
func ParseOutput(output []byte) (c Conn, ok bool) {
	m := commandLine.FindSubmatch(output)
	if m == nil {
		return Conn{}, false
	}
	c.Command = string(m[1])
	fields := strings.Fields(c.Command)[1:]
	for i := 0; i < len(fields); i++ {
		switch f := fields[i]; {
		case f == "-p" && i+1 < len(fields):
			c.Port, _ = strconv.Atoi(fields[i+1])
			i++
		case strings.HasPrefix(f, "-"):
		default:
			c.User, c.Host, _ = strings.Cut(f, "@")
			if c.Host == "" {
				c.User, c.Host = "", f
			}
		}
	}
	if c.Host == "" {
		return Conn{}, false
	}
	return c, true
}

// Options control Wait.
type Options struct {
	// Timeout bounds the whole wait.
	Timeout time.Duration
	// Interval is the time between polls.
	Interval time.Duration
	// OnStatus is called when what Wait is waiting for changes, with a short
	// description such as "Waiting for build to start".
	OnStatus func(string)
	// OnConn is called for each command as soon as it is found.
	OnConn func(Conn)
}

// TimeoutError means the wait ran out of time. Pending names the jobs that
// still had no command.
type TimeoutError struct {
	Timeout time.Duration
	Pending []string
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("no ssh command for %s within %s", strings.Join(e.Pending, ", "), e.Timeout)
}

// NoSSHError means a job ended without printing an ssh command, so there is
// nothing to connect to.
type NoSSHError struct {
	Job   string
	JobID uuid.UUID
}

func (e *NoSSHError) Error() string {
	return fmt.Sprintf("job %s ended without an ssh command", e.Job)
}

// target is one job Wait is looking for.
type target struct {
	name  string
	id    uuid.UUID    // zero until the job shows up in the workflow
	execs int          // number of executions, once the job has started
	conns map[int]Conn // commands found so far, by execution
}

func (t *target) done() bool { return t.execs > 0 && len(t.conns) == t.execs }

// Wait polls the rerun workflow until each named job prints its ssh command,
// and returns the commands in the order of names, then execution. Commands
// found before an error are returned with it.
func Wait(ctx context.Context, client *apiclient.Client, workflowID uuid.UUID, names []string, opts Options) ([]Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	targets := make([]*target, len(names))
	for i, n := range names {
		targets[i] = &target{name: n, conns: map[int]Conn{}}
	}
	status := ""
	setStatus := func(s string) {
		if s != status && opts.OnStatus != nil {
			status = s
			opts.OnStatus(s)
		}
	}
	found := false
	onConn := func(c Conn) {
		found = true
		if opts.OnConn != nil {
			opts.OnConn(c)
		}
	}

	for {
		msg, err := poll(ctx, client, workflowID, targets, onConn)
		if err != nil {
			return collect(targets), timeoutOr(ctx, err, opts.Timeout, targets)
		}
		if msg == "" {
			return collect(targets), nil
		}
		// Printing a command interrupts the status (the caller stops its
		// spinner), so report it again even if it has not changed: a parallel
		// job is still "waiting for the SSH step" after its first execution.
		if found {
			status, found = "", false
		}
		setStatus(msg)

		t := time.NewTimer(opts.Interval)
		select {
		case <-ctx.Done():
			t.Stop()
			return collect(targets), timeoutOr(ctx, ctx.Err(), opts.Timeout, targets)
		case <-t.C:
		}
	}
}

// poll makes one pass over the targets. It returns a description of what is
// still being waited for, or "" once every command has been found.
func poll(ctx context.Context, client *apiclient.Client, workflowID uuid.UUID, targets []*target, onConn func(Conn)) (string, error) {
	// The rerun's jobs can take a moment to appear in the new workflow.
	if slices.ContainsFunc(targets, func(t *target) bool { return t.id == uuid.Nil }) {
		jobs, err := client.GetWorkflowJobsV3(ctx, workflowID)
		if err != nil {
			return "", err
		}
		for _, t := range targets {
			for _, j := range jobs {
				if j.Name == t.name {
					t.id = j.ID
				}
			}
		}
	}

	var queued, starting []string
	for _, t := range targets {
		if t.done() {
			continue
		}
		if t.id == uuid.Nil {
			queued = append(queued, t.name)
			continue
		}
		job, err := client.GetJobV3(ctx, t.id)
		if err != nil {
			return "", err
		}
		// A job that has ended has no SSH session left to join, whatever its
		// steps say. This also catches a job that ended before it ever started
		// (canceled while queued, not run, infrastructure failure), which has no
		// executions and would otherwise read as still queued.
		if job.Phase == apiclient.PhaseEnded {
			return "", &NoSSHError{Job: t.name, JobID: t.id}
		}
		if apiclient.PhaseNotStarted(job.Phase) || len(job.Executions) == 0 {
			queued = append(queued, t.name)
			continue
		}
		t.execs = len(job.Executions)
		for _, exec := range job.Executions {
			if _, ok := t.conns[exec.Index]; ok {
				continue
			}
			step, ok := sshStep(exec.Steps)
			if !ok || apiclient.PhaseNotStarted(step.Phase) {
				continue
			}
			out, err := client.GetJobStdout(ctx, t.id, exec.Index, step.Num)
			if err != nil {
				return "", err
			}
			conn, ok := ParseOutput(out)
			if !ok {
				// The step has started but not printed the command yet. If it
				// has already ended, it never will.
				if step.Phase == apiclient.PhaseEnded {
					return "", &NoSSHError{Job: t.name, JobID: t.id}
				}
				continue
			}
			conn.Job, conn.JobID, conn.Execution, conn.Executions = t.name, t.id, exec.Index, t.execs
			t.conns[exec.Index] = conn
			if onConn != nil {
				onConn(conn)
			}
		}
		if !t.done() {
			starting = append(starting, t.name)
		}
	}

	switch {
	case len(queued) > 0:
		return "Waiting for " + strings.Join(queued, ", ") + " to start", nil
	case len(starting) > 0:
		return "Waiting for the SSH step of " + strings.Join(starting, ", "), nil
	}
	return "", nil
}

// sshStep finds an execution's SSH step.
func sshStep(steps []apiclient.JobV3Step) (apiclient.JobV3Step, bool) {
	for _, s := range steps {
		if s.Type == StepType || (s.Type == "" && s.Name == stepName) {
			return s, true
		}
	}
	return apiclient.JobV3Step{}, false
}

// timeoutOr turns an error caused by the wait's own deadline into a
// TimeoutError. Anything else, including the caller's cancellation, is
// returned as is.
func timeoutOr(ctx context.Context, err error, timeout time.Duration, targets []*target) error {
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return err
	}
	var pending []string
	for _, t := range targets {
		if !t.done() {
			pending = append(pending, t.name)
		}
	}
	return &TimeoutError{Timeout: timeout, Pending: pending}
}

func collect(targets []*target) []Conn {
	var out []Conn
	for _, t := range targets {
		for i := 0; i < t.execs; i++ {
			if c, ok := t.conns[i]; ok {
				out = append(out, c)
			}
		}
	}
	return out
}
