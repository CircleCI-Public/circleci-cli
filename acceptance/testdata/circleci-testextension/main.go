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

package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"
)

func main() {
	args := os.Args[1:]

	// Printed first so a test can read the variable from this process's stdout.
	fmt.Println("socket", os.Getenv("CIRCLE_TELEMETRY_SOCKET"))

	if len(args) >= 2 && args[0] == "telemetry" {
		payload := ""
		if len(args) >= 3 {
			payload = args[2]
		}
		os.Exit(telemetry(args[1], payload))
	}

	fmt.Println("args", args)
	fmt.Println("token", os.Getenv("CIRCLE_TOKEN"))
	fmt.Println("url", os.Getenv("CIRCLE_HOST"))
	fmt.Println("device_id", os.Getenv("CIRCLE_DEVICE_ID"))
	fmt.Println("user_id", os.Getenv("CIRCLE_USER_ID"))

	if len(args) == 2 && args[0] == "exit" {
		atoi, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Println("exit with default error", 1)
			os.Exit(1)
		}
		fmt.Println("exit with error", atoi)
		os.Exit(atoi)
	}
}

// holdSeconds is how long a held connection stays open. The connection must stay
// open longer than the CLI waits for a payload.
const holdSeconds = 10

// telemetry drives every shape of telemetry handoff, so one binary serves a
// whole table of cases. It returns the exit code.
//
//	send       write payload, close, exit 0
//	send-fail  as send, then exit 3
//	cut        write the first half of payload, close
//	hold       connect and write nothing; a background child keeps the
//	           connection open for holdSeconds while this process exits at once
//	send-kill  as send, then kill itself
//	none       do not connect
func telemetry(mode, payload string) int {
	switch mode {
	case "none":
		return 0
	case "hold-child":
		// The connection arrives as an inherited file; keep it open.
		time.Sleep(holdSeconds * time.Second)
		return 0
	}

	// An extension the CLI gave no socket reports nothing, as a real one would.
	socket := os.Getenv("CIRCLE_TELEMETRY_SOCKET")
	if socket == "" {
		return 0
	}
	conn, err := net.Dial("unix", socket)
	if err != nil {
		fmt.Println("dial failed")
		return 1
	}

	// The CLI reads only after this process exits, so a payload larger than the
	// socket buffer would block the write for ever. Give up after a second.
	_ = conn.SetWriteDeadline(time.Now().Add(time.Second))

	switch mode {
	case "send", "send-fail", "send-kill":
		_, _ = conn.Write([]byte(payload))
		_ = conn.Close()
	case "cut":
		_, _ = conn.Write([]byte(payload[:len(payload)/2]))
		_ = conn.Close()
	case "hold":
		if err := holdInBackground(conn); err != nil {
			fmt.Println("hold failed")
			return 1
		}
	default:
		fmt.Println("unknown telemetry mode", mode)
		return 1
	}

	switch mode {
	case "send-fail":
		return 3
	case "send-kill":
		self, err := os.FindProcess(os.Getpid())
		if err == nil {
			_ = self.Kill()
		}
		time.Sleep(time.Minute) // not reached once the kill lands
	}
	return 0
}

// holdInBackground starts a copy of this program that inherits conn and keeps
// it open, with its output detached so the CLI does not wait for it. Windows
// cannot pass the file on, so the test skips there.
func holdInBackground(conn net.Conn) error {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("not a unix connection")
	}
	f, err := uc.File()
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	child := exec.Command(os.Args[0], "telemetry", "hold-child") //#nosec:G204 // re-runs this test binary
	child.ExtraFiles = []*os.File{f}
	return child.Start()
}
