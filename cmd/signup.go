package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
)

const (
	// App base URL override for enterprise / testing. Falls back to
	// defaultAppBaseURL when unset.
	appBaseURLEnv     = "CIRCLECI_APP_URL"
	defaultAppBaseURL = "https://app.circleci.com"

	handshakeTimeout = 10 * time.Minute
	handshakeHTTPTO  = 10 * time.Second
	// Consecutive network errors tolerated before giving up.
	handshakeMaxNetErrs = 3
)

// handshakePollWait is the delay between polls. It's a var so tests can
// shorten it; production code should treat it as constant.
var handshakePollWait = 3 * time.Second

type signupOptions struct {
	cfg       *settings.Config
	noBrowser bool
	force     bool
}

func newSignupCommand(config *settings.Config) *cobra.Command {
	opts := signupOptions{
		cfg: config,
	}

	cmd := &cobra.Command{
		Use:   "signup",
		Short: "Sign up for a CircleCI account or authenticate an existing account",
		RunE: func(cmd *cobra.Command, _ []string) error {
			err := runSignup(cmd, opts)

			telemetryClient, ok := telemetry.FromContext(cmd.Context())
			if ok {
				_ = telemetryClient.Track(createSignupEvent(opts.noBrowser, err))
			}

			return err
		},
	}

	cmd.Flags().BoolVar(&opts.noBrowser, "no-browser", false, "Don't open a browser — print the signup URL so you can visit it from any device")
	cmd.Flags().BoolVar(&opts.force, "force", false, "Run signup even if already authenticated")

	return cmd
}

func createSignupEvent(noBrowser bool, err error) telemetry.Event {
	properties := map[string]interface{}{
		"no_browser":        noBrowser,
		"has_been_executed": true,
	}
	if err != nil {
		properties["error"] = err.Error()
	}
	return telemetry.Event{
		Object:     "cli-signup",
		Action:     "signup",
		Properties: properties,
	}
}

func appBaseURL() string {
	if v := os.Getenv(appBaseURLEnv); v != "" {
		return v
	}
	return defaultAppBaseURL
}

func runSignup(cmd *cobra.Command, opts signupOptions) error {
	if !opts.force && opts.cfg.Token != "" {
		fmt.Println("You're already authenticated. Your CLI is configured with a personal API token.")
		fmt.Println("If you want to reconfigure, run `circleci setup`.")
		return nil
	}

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	handshakeID := uuid.NewString()
	baseURL := appBaseURL()
	signupURL := fmt.Sprintf("%s/cli-auth?handshake_id=%s", baseURL, handshakeID)

	if opts.noBrowser {
		fmt.Printf("To complete signup, open this URL on any device:\n\n  %s\n\n", signupURL)
	} else {
		trackSignupStep(cmd, "browser_opening", nil)
		fmt.Println("Opening your browser to sign up for CircleCI...")
		fmt.Printf("  %s\n", signupURL)
		if err := browser.OpenURL(signupURL); err != nil {
			fmt.Printf("Could not open browser automatically: %v\n", err)
			fmt.Println("Please visit the URL above from any device.")
		}
	}

	fmt.Println("Waiting for browser authentication...")

	token, err := pollHandshake(ctx, baseURL, handshakeID, handshakeTimeout)
	if err != nil {
		if ctx.Err() != nil {
			trackSignupStep(cmd, "canceled", nil)
			fmt.Println("\nAuthentication canceled.")
			return nil
		}
		trackSignupStep(cmd, "failed", nil)
		return fmt.Errorf("signup failed: %w", err)
	}

	trackSignupStep(cmd, "token_received", nil)
	return saveToken(opts.cfg, token)
}

// pollHandshake polls the server-side handshake endpoint until a token appears
// (200), the handshake expires (404), the context is cancelled, or the overall
// timeout elapses. 202 responses mean "still pending"; transient network errors
// are retried up to handshakeMaxNetErrs consecutive times.
func pollHandshake(ctx context.Context, baseURL, handshakeID string, timeout time.Duration) (string, error) {
	client := &http.Client{Timeout: handshakeHTTPTO}
	endpoint := fmt.Sprintf("%s/api/v1/cli-handshake/%s", baseURL, handshakeID)

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	var netErrs int
	for {
		token, status, err := handshakePoll(ctx, client, endpoint)
		switch {
		case err == nil && status == http.StatusOK:
			return token, nil
		case err == nil && status == http.StatusAccepted:
			netErrs = 0
		case err == nil:
			return "", fmt.Errorf("unexpected response from handshake endpoint: %d", status)
		case ctx.Err() != nil:
			// Parent context was canceled or hit its deadline — surface it so
			// the caller can distinguish from transport-level timeouts.
			return "", ctx.Err()
		default:
			netErrs++
			if netErrs > handshakeMaxNetErrs {
				return "", fmt.Errorf("repeated network errors while polling for authentication: %w", err)
			}
		}

		fmt.Print(".")

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline.C:
			return "", fmt.Errorf("timed out waiting for browser authentication (%s) — run `circleci signup` to try again", timeout)
		case <-time.After(handshakePollWait):
		}
	}
}

// handshakePoll performs a single GET against the handshake endpoint.
// On 200 it decodes and returns the token; on any other status it returns the
// status code for the caller to dispatch on. Network / transport errors surface
// via the error return so the caller can decide whether to retry.
func handshakePoll(ctx context.Context, client *http.Client, endpoint string) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", resp.StatusCode, nil
	}

	var body struct {
		Token     string `json:"token"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", resp.StatusCode, fmt.Errorf("failed to parse handshake response: %w", err)
	}
	if body.Token == "" {
		return "", resp.StatusCode, errors.New("handshake response contained no token")
	}
	return body.Token, resp.StatusCode, nil
}

func saveToken(cfg *settings.Config, token string) error {
	cfg.Token = token
	if err := cfg.WriteToDisk(); err != nil {
		return fmt.Errorf("failed to save token to config: %w", err)
	}
	fmt.Println("\n✅ Welcome to CircleCI! Your CLI is now authenticated.")
	fmt.Println("\nNext steps:")
	fmt.Println("  circleci init    — set up a project in the current directory")
	fmt.Println("  circleci help    — see all available commands")
	return nil
}

func trackSignupStep(cmd *cobra.Command, step string, extra map[string]interface{}) {
	client, ok := telemetry.FromContext(cmd.Context())
	if !ok {
		return
	}
	invID, _ := telemetry.InvocationIDFromContext(cmd.Context())
	telemetry.TrackWorkflowStep(client, "signup", step, invID, extra)
}
