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

package acceptance_test

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/binary"
	testenv "github.com/CircleCI-Public/circleci-cli/internal/testing/env"
	"github.com/CircleCI-Public/circleci-cli/internal/testing/fakes"
)

const (
	slackOrgID        = "a0000000-0000-4000-8000-0000000c0002"
	slackAlertsID     = "C0000ALERTS"
	slackPrivateID    = "G0000SECRET"
	slackLockedID     = "C0000LOCKED"
	slackOtherProject = "gh/otherorg/gamma"
	slackConfigID     = "00000000-0000-4000-8000-000000000001"
)

func setupSlackFake(t *testing.T) (*fakes.CircleCI, *testenv.TestEnv) {
	t.Helper()
	fake, env := setupProjectFake(t)
	fake.AddSlackWorkspace(fakes.SlackWorkspace{
		OrgID:         slackOrgID,
		IntegrationID: "b0000000-0000-4000-8000-0000000000a1",
		Name:          "My Org",
		Channels: []fakes.SlackChannel{
			{ID: slackAlertsID, Name: "ci-alerts"},
			{ID: slackPrivateID, Name: "secret", Private: true},
			{ID: slackLockedID, Name: "locked", CannotPost: true},
		},
	})
	fake.SetNotificationPreferences(alphaProjectID,
		fakes.NotificationPreferenceV3{ID: "c0000000-0000-4000-8000-000000000001", Group: "Workflows", Name: "Workflow failed", Channel: "slack", Enabled: true},
		fakes.NotificationPreferenceV3{ID: "c0000000-0000-4000-8000-000000000002", Group: "Workflows", Name: "Workflow succeeded", Channel: "slack", Enabled: false},
		fakes.NotificationPreferenceV3{ID: "c0000000-0000-4000-8000-000000000003", Group: "Workflows", Name: "Workflow failed", Channel: "email", Enabled: true},
	)
	return fake, env
}

func addAlphaSlackConfig(fake *fakes.CircleCI, target string, enabled bool) {
	fake.AddChannelConfig(fakes.ChannelConfigV3{
		ID:          slackConfigID,
		ProjectID:   alphaProjectID,
		OrgID:       slackOrgID,
		ChannelType: "slack",
		Target:      target,
		Enabled:     enabled,
	})
}

func runSlack(t *testing.T, env *testenv.TestEnv, args ...string) binary.CLIResult {
	t.Helper()
	return binary.RunCLI(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    append([]string{"project", "slack"}, args...),
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})
}

func TestProjectSlackSet_ChannelByName(t *testing.T) {
	fake, env := setupSlackFake(t)

	result := runSlack(t, env, "set", "--channel", "#ci-alerts", "--project", "gh/myorg/alpha")

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))

	t.Run("stores the resolved channel ID", func(t *testing.T) {
		cfg, ok := fake.ChannelConfig(alphaProjectID, "slack")
		assert.Assert(t, ok)
		assert.Check(t, cmp.Equal(cfg.Target, slackAlertsID))
		assert.Check(t, cfg.Enabled)
	})
	t.Run("posts the welcome message", func(t *testing.T) {
		assert.Check(t, cmp.DeepEqual(fake.SlackWelcomeMessages(), []string{slackAlertsID}))
	})
}

func TestProjectSlackSet_ChannelAgain_KeepsConfig(t *testing.T) {
	fake, env := setupSlackFake(t)

	first := runSlack(t, env, "set", "--channel", "#ci-alerts", "--project", "gh/myorg/alpha", "--json")
	assert.Assert(t, cmp.Equal(first.ExitCode, 0), "stderr: %s", first.Stderr)
	before, _ := fake.ChannelConfig(alphaProjectID, "slack")

	second := runSlack(t, env, "set", "--channel", slackAlertsID, "--project", "gh/myorg/alpha", "--json")
	assert.Assert(t, cmp.Equal(second.ExitCode, 0), "stderr: %s", second.Stderr)
	after, _ := fake.ChannelConfig(alphaProjectID, "slack")

	assert.Check(t, cmp.Equal(after.ID, before.ID))
	assert.Check(t, cmp.DeepEqual(fake.SlackWelcomeMessages(), []string{slackAlertsID, slackAlertsID}))
}

func TestProjectSlackSet_JSON(t *testing.T) {
	_, env := setupSlackFake(t)

	result := runSlack(t, env, "set", "--channel", "#ci-alerts", "--project", "gh/myorg/alpha", "--json")

	assert.Assert(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, cmp.Equal(result.Stderr, ""))
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.DeepEqual(out, map[string]any{
		"channel_id":   slackAlertsID,
		"channel_name": "ci-alerts",
		"enabled":      true,
	}))
}

func TestProjectSlackSet_PrivateChannelByID(t *testing.T) {
	fake, env := setupSlackFake(t)

	result := runSlack(t, env, "set", "--channel", slackPrivateID, "--project", "gh/myorg/alpha")

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	cfg, _ := fake.ChannelConfig(alphaProjectID, "slack")
	assert.Check(t, cmp.Equal(cfg.Target, slackPrivateID))
}

func TestProjectSlackSet_Disable_KeepsChannel(t *testing.T) {
	fake, env := setupSlackFake(t)
	addAlphaSlackConfig(fake, slackAlertsID, true)

	result := runSlack(t, env, "set", "--disable", "--project", "gh/myorg/alpha")

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	cfg, _ := fake.ChannelConfig(alphaProjectID, "slack")
	assert.Check(t, !cfg.Enabled)
	assert.Check(t, cmp.Equal(cfg.Target, slackAlertsID))
	assert.Check(t, cmp.Len(fake.SlackWelcomeMessages(), 0))
}

func TestProjectSlackSet_Enable_PostsWelcomeAgain(t *testing.T) {
	fake, env := setupSlackFake(t)
	addAlphaSlackConfig(fake, slackAlertsID, false)

	result := runSlack(t, env, "set", "--enable", "--project", "gh/myorg/alpha")

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	cfg, _ := fake.ChannelConfig(alphaProjectID, "slack")
	assert.Check(t, cfg.Enabled)
	assert.Check(t, cmp.DeepEqual(fake.SlackWelcomeMessages(), []string{slackAlertsID}))
}

func TestProjectSlackSet_DisableNotConfigured(t *testing.T) {
	_, env := setupSlackFake(t)

	result := runSlack(t, env, "set", "--disable", "--project", "gh/myorg/alpha")

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitNotFound))
	assert.Check(t, cmp.Contains(result.Stderr, `Project "gh/myorg/alpha" has no Slack channel configured.`))
}

func TestProjectSlackSet_FlagRules(t *testing.T) {
	_, env := setupSlackFake(t)

	t.Run("one of the flags is required", func(t *testing.T) {
		result := runSlack(t, env, "set", "--project", "gh/myorg/alpha")
		assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitBadArguments))
	})
	t.Run("the flags are mutually exclusive", func(t *testing.T) {
		result := runSlack(t, env, "set", "--enable", "--disable", "--project", "gh/myorg/alpha")
		assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitBadArguments))
	})
	t.Run("the channel must not be empty", func(t *testing.T) {
		result := runSlack(t, env, "set", "--channel", " ", "--project", "gh/myorg/alpha")
		assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitBadArguments))
		assert.Check(t, cmp.Contains(result.Stderr, "--channel must be a Slack channel name or ID"))
	})
}

func TestProjectSlackGet(t *testing.T) {
	fake, env := setupSlackFake(t)
	addAlphaSlackConfig(fake, slackAlertsID, true)

	result := runSlack(t, env, "get", "--project", "gh/myorg/alpha")

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stdout, t.Name()+".txt"))
}

func TestProjectSlackGet_JSON(t *testing.T) {
	fake, env := setupSlackFake(t)
	addAlphaSlackConfig(fake, slackAlertsID, false)

	result := runSlack(t, env, "get", "--project", "gh/myorg/alpha", "--json")

	assert.Assert(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	var out map[string]any
	assert.NilError(t, json.Unmarshal([]byte(result.Stdout), &out))
	assert.Check(t, cmp.DeepEqual(out, map[string]any{
		"channel_id":   slackAlertsID,
		"channel_name": "ci-alerts",
		"enabled":      false,
		"preferences": []any{
			map[string]any{"group": "Workflows", "name": "Workflow failed", "enabled": true},
			map[string]any{"group": "Workflows", "name": "Workflow succeeded", "enabled": false},
		},
	}))
}

func TestProjectSlackGet_NotConfigured(t *testing.T) {
	_, env := setupSlackFake(t)

	result := runSlack(t, env, "get", "--project", "gh/myorg/alpha")

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitNotFound))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestProjectSlackDelete_Force(t *testing.T) {
	fake, env := setupSlackFake(t)
	addAlphaSlackConfig(fake, slackAlertsID, true)

	result := runSlack(t, env, "delete", "--project", "gh/myorg/alpha", "--force")

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	_, ok := fake.ChannelConfig(alphaProjectID, "slack")
	assert.Check(t, !ok)
}

func TestProjectSlackDelete_DryRun(t *testing.T) {
	fake, env := setupSlackFake(t)
	addAlphaSlackConfig(fake, slackAlertsID, true)

	result := runSlack(t, env, "delete", "--project", "gh/myorg/alpha", "--dry-run")

	assert.Check(t, cmp.Equal(result.ExitCode, 0), "stderr: %s", result.Stderr)
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
	_, ok := fake.ChannelConfig(alphaProjectID, "slack")
	assert.Check(t, ok)
}

func TestProjectSlackDelete_RequiresForceWhenNotInteractive(t *testing.T) {
	fake, env := setupSlackFake(t)
	addAlphaSlackConfig(fake, slackAlertsID, true)

	result := runSlack(t, env, "delete", "--project", "gh/myorg/alpha")

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitCancelled))
	assert.Check(t, cmp.Contains(result.Stderr, "stops its Slack notifications"))
	_, ok := fake.ChannelConfig(alphaProjectID, "slack")
	assert.Check(t, ok)
}

func TestProjectSlackDelete_Interactive(t *testing.T) {
	fake, env := setupSlackFake(t)
	addAlphaSlackConfig(fake, slackAlertsID, true)
	console := binary.RunCLIInteractive(t, binary.RunOpts{
		Binary:  binaryPath,
		Args:    []string{"project", "slack", "delete", "--project", "gh/myorg/alpha"},
		Env:     env.Environ(),
		WorkDir: t.TempDir(),
	})

	assert.Assert(t, t.Run("deletes when confirmed", func(t *testing.T) {
		_, err := console.ExpectString("[y/N]")
		assert.NilError(t, err)
		_, err = console.Send("y")
		assert.NilError(t, err)
		_, err = console.ExpectString("Removed Slack channel #ci-alerts (C0000ALERTS) from gh/myorg/alpha")
		assert.NilError(t, err)
	}))
}

func TestProjectSlackSet_PrivateChannelByName(t *testing.T) {
	_, env := setupSlackFake(t)

	result := runSlack(t, env, "set", "--channel", "#secret", "--project", "gh/myorg/alpha")

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitNotFound))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestProjectSlackSet_CannotPost(t *testing.T) {
	_, env := setupSlackFake(t)

	result := runSlack(t, env, "set", "--channel", "#locked", "--project", "gh/myorg/alpha")

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitAPIError))
	assert.Check(t, cmp.Contains(result.Stderr, "The CircleCI Slack app is not allowed to post in this channel."))
}

func TestProjectSlackSet_SlackNotInstalled(t *testing.T) {
	_, env := setupSlackFake(t)

	result := runSlack(t, env, "set", "--channel", "#ci-alerts", "--project", slackOtherProject)

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitNotFound))
	assert.Check(t, golden.String(result.Stderr, t.Name()+".stderr.txt"))
}

func TestProjectSlackSet_ConnectionInvalid(t *testing.T) {
	fake, env := setupSlackFake(t)
	fake.AddSlackWorkspace(fakes.SlackWorkspace{
		OrgID:             slackOrgID,
		IntegrationID:     "b0000000-0000-4000-8000-0000000000a1",
		Name:              "My Org",
		ConnectionInvalid: true,
		Channels:          []fakes.SlackChannel{{ID: slackAlertsID, Name: "ci-alerts"}},
	})

	result := runSlack(t, env, "set", "--channel", "#ci-alerts", "--project", "gh/myorg/alpha")

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitAPIError))
	assert.Check(t, cmp.Contains(result.Stderr, "The organization's connection to Slack is no longer valid"))
	assert.Check(t, !cmp.Contains(result.Stderr, "circleci auth login")().Success(), "must not blame the CLI token")
}

func TestProjectSlackSet_PermissionDenied(t *testing.T) {
	fake, env := setupSlackFake(t)
	fake.DenyNotificationAccess(alphaProjectID)

	result := runSlack(t, env, "set", "--channel", "#ci-alerts", "--project", "gh/myorg/alpha")

	assert.Check(t, cmp.Equal(result.ExitCode, clierrors.ExitNotFound))
	assert.Check(t, cmp.Contains(result.Stderr, `You do not have permission to manage notifications for project "gh/myorg/alpha".`))
}
