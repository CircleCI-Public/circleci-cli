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

// Package forgejoconn connects a CircleCI organization to a self-hosted Forgejo
// instance from the terminal, for onboarding.
//
// There is no browser install: the user creates a bot user and two integrations in
// Forgejo, gives CircleCI their audiences, and once the connection is stored
// creates the webhook it answers with. Each value comes from a flag, else a
// prompt, else an error naming the flag, so the flow also runs unattended.
package forgejoconn

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	clierrors "github.com/CircleCI-Public/circleci-cli/clikit/errors"
	"github.com/CircleCI-Public/circleci-cli/clikit/iostream"
	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/gitremote"
	"github.com/CircleCI-Public/circleci-cli/internal/ui"
)

const (
	// issuerBase prefixes an organization ID to form the issuer of the OIDC
	// tokens CircleCI presents to Forgejo.
	issuerBase = "https://oidc.circleci.com/org/"
	// subjectBase prefixes the integration's purpose to form the sub claim its
	// tokens carry, which the integration's claim rule must require.
	subjectBase = "circleci/soc-integrations/forgejo/"
	// audiencePlaceholder is the shape of an audience Forgejo generates.
	audiencePlaceholder = "e.g. u:1:f92855…"

	urlPrompt   = "Forgejo instance URL"
	ownerPrompt = "Owner: the org or user that owns your repos"
)

// Options are the values passed by flag. Any left empty is prompted for when the
// session is interactive.
type Options struct {
	// URL is the instance URL. Defaults to https://<the remote's host>.
	URL string
	// Owner is the Forgejo organization or user. Defaults to the remote's owner.
	Owner              string
	ConnectionAudience string
	CheckoutAudience   string
	// Headers are "Name: Value" request headers. Optional, and asked about only
	// when interactive and none were passed.
	Headers []string
}

// integration is one of the two Generic JWT integrations the bot user creates.
type integration struct {
	field        setupField
	name         string
	purpose      string
	capabilities string
	prompt       string
}

var (
	connectionIntegration = integration{
		field:        fieldConnectionAudience,
		name:         "CircleCI connection",
		purpose:      "connection",
		capabilities: "user Read, organization Read, repository Read and write",
		prompt:       "Connection integration audience",
	}
	checkoutIntegration = integration{
		field:        fieldCheckoutAudience,
		name:         "CircleCI checkout",
		purpose:      "checkout",
		capabilities: "repository Read",
		prompt:       "Checkout integration audience",
	}
)

// Connect connects the organization orgID to the Forgejo instance the checkout's
// remote is on, prints the webhook the user has to create, and, when
// interactive, waits for them to create it.
//
// A value the API rejects is asked for again when interactive; otherwise the
// error names the flag that supplies it.
func Connect(
	ctx context.Context,
	client *apiclient.Client,
	orgID string,
	remote gitremote.RemoteRef,
	opts Options,
) (*apiclient.ForgejoConnection, error) {
	headers, err := ParseHeaders(opts.Headers)
	if err != nil {
		return nil, err
	}

	in, err := collect(ctx, orgID, remote, opts)
	if err != nil {
		return nil, err
	}
	if headers != nil {
		in.RequestHeaders = headers
	} else if iostream.IsInteractive(ctx) {
		if in.RequestHeaders, err = promptHeaders(ctx); err != nil {
			return nil, err
		}
	}

	conn, err := setup(ctx, client, orgID, in)
	if err != nil {
		return nil, err
	}
	iostream.Printf(ctx, "\n%s Connected Forgejo at %s for %s\n", iostream.SymbolOK(ctx), in.URL, conn.Owner.Login)

	if err := printWebhook(ctx, in.URL, conn); err != nil {
		return nil, err
	}
	return conn, waitForWebhook(ctx)
}

// collect gathers the instance, owner and audiences, printing the Forgejo steps
// that produce an audience before asking for it.
func collect(ctx context.Context, orgID string, remote gitremote.RemoteRef, opts Options) (apiclient.ForgejoSetup, error) {
	var in apiclient.ForgejoSetup
	iostream.Printf(ctx, "\nConnect Forgejo to CircleCI. Needs Forgejo 16 or later.\n")

	var err error
	if in.URL, err = valueWithDefault(ctx, opts.URL, urlPrompt, defaultURL(remote.Host)); err != nil {
		return in, err
	}
	if in.Owner, err = valueWithDefault(ctx, opts.Owner, ownerPrompt, remote.Owner); err != nil {
		return in, err
	}

	var missing []integration
	for _, ig := range []integration{connectionIntegration, checkoutIntegration} {
		if audienceFlag(opts, ig.field) == "" {
			missing = append(missing, ig)
		}
	}
	if len(missing) > 0 {
		printBotUser(ctx, in.Owner)
	}
	if len(missing) > 0 && !iostream.IsInteractive(ctx) {
		flags := make([]string, len(missing))
		for i, ig := range missing {
			printIntegration(ctx, ig, orgID)
			flags[i] = ig.field.flag()
		}
		return in, clierrors.New("args.missing_flag", "Missing required flag",
			fmt.Sprintf("Connecting Forgejo needs %s, and this session cannot prompt for it.", strings.Join(flags, " and "))).
			WithSuggestions(
				"Create the integrations above, then pass each one's audience: " + strings.Join(flags, " <audience> ") + " <audience>",
			).
			WithExitCode(clierrors.ExitBadArguments)
	}

	in.ConnectionAudience, in.CheckoutAudience = opts.ConnectionAudience, opts.CheckoutAudience
	for _, ig := range missing {
		printIntegration(ctx, ig, orgID)
		v, err := requiredValue(ctx, ig.prompt)
		if err != nil {
			return in, err
		}
		setAudience(&in, ig.field, v)
	}
	return in, nil
}

func audienceFlag(opts Options, f setupField) string {
	if f == fieldConnectionAudience {
		return opts.ConnectionAudience
	}
	return opts.CheckoutAudience
}

func setAudience(in *apiclient.ForgejoSetup, f setupField, v string) {
	if f == fieldConnectionAudience {
		in.ConnectionAudience = v
		return
	}
	in.CheckoutAudience = v
}

// setup calls the API until it accepts the values. When interactive, a value it
// rejects is shown with the API's reason and asked for again.
func setup(ctx context.Context, client *apiclient.Client, orgID string, in apiclient.ForgejoSetup) (*apiclient.ForgejoConnection, error) {
	for {
		sp := iostream.Spinner(ctx, true, "Checking the Forgejo instance")
		conn, err := client.SetupForgejoConnection(ctx, orgID, in)
		sp.Stop()
		if err == nil {
			return conn, nil
		}

		field, detail := fieldForError(err)
		if field == "" || !iostream.IsInteractive(ctx) {
			return nil, setupError(err, in.Owner)
		}
		iostream.ErrPrintf(ctx, "%s Setup failed: %s\n", iostream.SymbolWarn(ctx), sentence(detail))
		if err := reprompt(ctx, &in, field); err != nil {
			return nil, err
		}
	}
}

// reprompt asks again for the field a setup rejected, offering what was sent.
func reprompt(ctx context.Context, in *apiclient.ForgejoSetup, field setupField) error {
	var err error
	switch field {
	case fieldURL:
		in.URL, err = promptWithDefault(ctx, urlPrompt, in.URL)
	case fieldOwner:
		in.Owner, err = promptWithDefault(ctx, ownerPrompt, in.Owner)
	case fieldConnectionAudience:
		in.ConnectionAudience, err = requiredValue(ctx, connectionIntegration.prompt)
	case fieldCheckoutAudience:
		in.CheckoutAudience, err = requiredValue(ctx, checkoutIntegration.prompt)
	case fieldRequestHeaders:
		in.RequestHeaders, err = promptHeaderValues(ctx)
	}
	return err
}

// valueWithDefault returns flagVal when set. Otherwise it asks, offering def,
// when interactive, and takes def when not.
func valueWithDefault(ctx context.Context, flagVal, prompt, def string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	if !iostream.IsInteractive(ctx) {
		return def, nil
	}
	return promptWithDefault(ctx, prompt, def)
}

func promptWithDefault(ctx context.Context, prompt, def string) (string, error) {
	v, err := iostream.PromptText(ctx, prompt, def, def)
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", cancelled(prompt)
	}
	return strings.TrimSpace(v), nil
}

// requiredValue asks for a value that has no default. Empty is a cancel: there is
// nothing to fall back on.
func requiredValue(ctx context.Context, prompt string) (string, error) {
	v, err := iostream.PromptText(ctx, prompt, audiencePlaceholder)
	if err != nil {
		return "", err
	}
	v = strings.TrimSpace(v)
	if v == "" {
		return "", cancelled(prompt)
	}
	return v, nil
}

// promptHeaders asks whether the instance needs request headers, defaulting to
// no, and collects them if it does.
func promptHeaders(ctx context.Context) (map[string]string, error) {
	idx, err := iostream.PromptSelectDefault(ctx,
		"Does your instance need extra request headers (e.g. behind a proxy)?",
		[]string{"No", "Yes"}, 0)
	if err != nil {
		return nil, err
	}
	if idx < 0 {
		return nil, cancelled("request headers")
	}
	if idx == 0 {
		return nil, nil
	}
	return promptHeaderValues(ctx)
}

// promptHeaderValues collects headers until an empty name or the limit. Values
// are masked: a header a gateway demands is usually a credential.
func promptHeaderValues(ctx context.Context) (map[string]string, error) {
	headers := map[string]string{}
	for len(headers) < MaxRequestHeaders {
		name, err := iostream.PromptText(ctx, "Header name (leave empty to finish)", "e.g. X-Gateway-Token")
		if err != nil {
			return nil, err
		}
		name = strings.TrimSpace(name)
		if name == "" {
			break
		}
		value, err := iostream.PromptSecret(ctx, "Value for "+name)
		if err != nil {
			return nil, err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, cancelled("Value for " + name)
		}
		headers[name] = value
	}
	if len(headers) == 0 {
		return nil, nil
	}
	return headers, nil
}

func cancelled(prompt string) error {
	return clierrors.New("onboard.cancelled", "Onboarding cancelled", "No value entered for: "+prompt+".").
		WithExitCode(clierrors.ExitCancelled)
}

func printBotUser(ctx context.Context, owner string) {
	iostream.Printf(ctx, "\nCreate a bot user\n")
	iostream.Printf(ctx, "  As a site admin, create a dedicated user, e.g. CircleCI-bot, and give it access to %s's repos:\n", owner)
	iostream.Printf(ctx, "    An organization: make a team with Specific repositories and Read access,\n")
	iostream.Printf(ctx, "    then add the bot and the repos to it.\n")
	iostream.Printf(ctx, "    A user: add the bot to each repo as a collaborator.\n")
}

func printIntegration(ctx context.Context, ig integration, orgID string) {
	iostream.Printf(ctx, "\nAdd the %s integration\n", ig.purpose)
	iostream.Printf(ctx, "  Signed in as the bot: Settings → Authorized Integrations → Add → Generic JWT.\n")
	iostream.Printf(ctx, "    Name:          %s\n", ig.name)
	iostream.Printf(ctx, "    Issuer:        %s%s\n", issuerBase, orgID)
	iostream.Printf(ctx, "    Claim rules:   %s\n", claimRules(ig.purpose))
	iostream.Printf(ctx, "    Capabilities:  %s\n", ig.capabilities)
	iostream.Printf(ctx, "  Then copy the Audience it generates (%s).\n", ig.field.flag())
}

func claimRules(purpose string) string {
	return fmt.Sprintf(`{"rules":[{"claim":"sub","compare":"eq","value":"%s%s"}]}`, subjectBase, purpose)
}

// printWebhook prints the webhook the user creates in Forgejo, its secret shown
// the one time the API returns it.
func printWebhook(ctx context.Context, instanceURL string, conn *apiclient.ForgejoConnection) error {
	if conn.Webhook.Secret == "" {
		return clierrors.New("onboard.forgejo_no_webhook_secret", "No webhook secret returned",
			"Forgejo is connected, but CircleCI did not return the webhook secret, and it is shown only once.").
			WithSuggestions(
				fmt.Sprintf("Issue a new secret: circleci api -X POST /api/v3/provider/connections/%s/rotate-webhook-secret", conn.ID),
				"Then add the webhook in Forgejo and re-run circleci onboard",
			).
			WithExitCode(clierrors.ExitAPIError)
	}

	iostream.Printf(ctx, "\nAdd the webhook in Forgejo. Pipelines won't start without it.\n")
	iostream.Printf(ctx, "  Open %s → Add webhook → Forgejo:\n", webhookSettings(conn.Owner))
	iostream.Printf(ctx, "    %s\n", webhookSettingsURL(instanceURL, conn.Owner))
	iostream.Printf(ctx, "  Target URL:\n    %s\n", conn.Webhook.URL)
	iostream.Printf(ctx, "  Secret: paste the secret shown below\n")
	iostream.Printf(ctx, "  Trigger on: Custom events…\n")
	iostream.Printf(ctx, "    Repository events → Push\n")
	iostream.Printf(ctx, "    Pull request events → Modification, Synchronized\n")
	iostream.Printf(ctx, "  Then select Add webhook. To check it, push a commit and look under Recent Deliveries.\n")
	iostream.Printf(ctx, "\nSecret (save this — it will not be shown again):\n%s\n\n", conn.Webhook.Secret)
	return nil
}

func webhookSettings(owner apiclient.ConnectionOwner) string {
	if owner.Type == "user" {
		return owner.Login + "'s user Settings → Webhooks"
	}
	return "the " + owner.Login + " organization's Settings → Webhooks"
}

func webhookSettingsURL(instanceURL string, owner apiclient.ConnectionOwner) string {
	base := strings.TrimRight(instanceURL, "/")
	if owner.Type == "user" {
		return base + "/user/settings/hooks"
	}
	return base + "/org/" + owner.Login + "/settings/hooks"
}

// waitForWebhook holds the flow until the user has added the webhook. Nothing
// reports a delivery, so it cannot be polled. A non-interactive session carries
// on: creating the pipeline needs no webhook, only its builds do.
func waitForWebhook(ctx context.Context) error {
	if !iostream.IsInteractive(ctx) {
		return nil
	}
	p := tea.NewProgram(ui.NewPreambleModel("Add the webhook in Forgejo, then continue to set up the pipeline.", "", nil),
		tea.WithContext(ctx),
		tea.WithInput(iostream.In(ctx)),
		tea.WithOutput(iostream.Err(ctx)),
	)
	final, err := p.Run()
	if err != nil {
		return clierrors.New("onboard.webhook_prompt_failed", "Webhook prompt failed", err.Error()).
			WithExitCode(clierrors.ExitGeneralError)
	}
	if m, ok := final.(ui.PreambleModel); !ok || !m.Proceed() {
		return clierrors.New("onboard.cancelled", "Onboarding cancelled",
			"Forgejo is connected, but the pipeline was not set up.").
			WithSuggestions("Add the webhook, then re-run circleci onboard to finish").
			WithExitCode(clierrors.ExitCancelled)
	}
	return nil
}
