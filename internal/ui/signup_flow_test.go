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

package ui

import (
	"context"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/internal/oauth"
)

func TestSignupFlowOpensSignupURL(t *testing.T) {
	var opened []string
	model := NewSignupFlow(context.Background(), SignupFlowOptions{
		Host:     "https://circleci.com",
		OpenURL:  func(url string) error { opened = append(opened, url); return nil },
		DeviceID: "test-device",
		OSInfo:   "test-os",
	})

	updated, cmd := model.onSignupStarted(signupOAuthStartedMsg{
		flow: &oauth.Flow{AuthorizeURL: "https://circleci.com/oauth/authorize?signup=true"},
	})
	assert.Assert(t, cmd != nil)
	cmd()

	assert.DeepEqual(t, opened, []string{"https://circleci.com/oauth/authorize?signup=true"})
	assert.Equal(t, updated.(SignupFlowModel).stage, signupStagePrompt)
}

func TestSignupFlowNoBrowserSkipsSignupURL(t *testing.T) {
	var opened []string
	model := NewSignupFlow(context.Background(), SignupFlowOptions{
		Host:      "https://circleci.com",
		NoBrowser: true,
		OpenURL:   func(url string) error { opened = append(opened, url); return nil },
		DeviceID:  "test-device",
		OSInfo:    "test-os",
	})

	updated, cmd := model.onSignupStarted(signupOAuthStartedMsg{
		flow: &oauth.Flow{AuthorizeURL: "https://circleci.com/oauth/authorize?signup=true"},
	})

	assert.Assert(t, cmd == nil)
	assert.DeepEqual(t, opened, []string(nil))
	assert.Equal(t, updated.(SignupFlowModel).stage, signupStagePrompt)
}
