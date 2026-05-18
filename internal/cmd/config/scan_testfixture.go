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

//go:build testfixtures

package cmdconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/CircleCI-Public/circleci-cli/internal/reposcan"
)

const scanFixtureEnv = "CIRCLECI_SCAN_FIXTURE"

func init() {
	realScan := scan
	scan = func(ctx context.Context, dir string) (*reposcan.Result, error) {
		if path := os.Getenv(scanFixtureEnv); path != "" {
			return loadScanFixture(path)
		}
		return realScan(ctx, dir)
	}
}

func loadScanFixture(path string) (*reposcan.Result, error) {
	data, err := os.ReadFile(path) //#nosec:G304,G703 // test-only hook; path comes from CIRCLECI_SCAN_FIXTURE
	if err != nil {
		return nil, err
	}
	var probe struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &probe); err == nil && probe.Error != "" {
		return nil, errors.New(probe.Error)
	}
	var r reposcan.Result
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("decode scan fixture: %w", err)
	}
	return &r, nil
}
