package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/browser"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/CircleCI-Public/circleci-cli/prompt"
	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/telemetry"
)

type signupOptions struct {
	cfg       *settings.Config
	noBrowser bool
}

func newSignupCommand(config *settings.Config) *cobra.Command {
	opts := signupOptions{
		cfg: config,
	}

	cmd := &cobra.Command{
		Use:   "signup",
		Short: "Sign up for a CircleCI account and authenticate the CLI",
		RunE: func(cmd *cobra.Command, _ []string) error {
			err := runSignup(cmd, opts)

			telemetryClient, ok := telemetry.FromContext(cmd.Context())
			if ok {
				_ = telemetryClient.Track(createSignupEvent(opts.noBrowser, err))
			}

			return err
		},
	}

	cmd.Flags().BoolVar(&opts.noBrowser, "no-browser", false, "Don't open a browser; print the signup URL and prompt for a token instead")

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

func runSignup(cmd *cobra.Command, opts signupOptions) error {
	state, err := generateState()
	if err != nil {
		return errors.Wrap(err, "failed to generate cryptographic state")
	}

	if opts.noBrowser {
		return signupNoBrowser(opts, state)
	}

	return signupWithBrowser(cmd, opts, state)
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func signupNoBrowser(opts signupOptions, state string) error {
	signupURL := "https://app.circleci.com/authentication/login?f=gho&return-to=/settings/user/tokens"
	fmt.Printf("Open this URL in your browser to sign up:\n\n  %s\n\n", signupURL)

	token, err := prompt.ReadSecretStringFromUser("Paste your CircleCI API token here")
	if err != nil {
		return errors.Wrap(err, "failed to read token")
	}

	if token == "" {
		return errors.New("no token provided")
	}

	return saveToken(opts.cfg, token)
}

func signupWithBrowser(cmd *cobra.Command, opts signupOptions, state string) error {
	// Start an ephemeral HTTP server on a random available port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return errors.Wrap(err, "failed to start local server")
	}
	port := listener.Addr().(*net.TCPAddr).Port

	tokenCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/token", corsMiddleware(handleToken(state, tokenCh, errCh)))

	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
			errCh <- serveErr
		}
	}()

	// Build the signup URL. The return-to is a relative path so it passes
	// the existing domain whitelist. The CLI port and state are embedded as
	// query params that the successful-signup page will read.
	returnTo := fmt.Sprintf("/successful-signup?source=cli&cli_port=%d&cli_state=%s", port, state)
	params := url.Values{}
	params.Set("f", "gho")
	params.Set("return-to", returnTo)
	signupURL := "https://app.circleci.com/authentication/login?" + params.Encode()

	trackSignupStep(cmd, "browser_opening", nil)
	fmt.Println("Opening your browser to sign up for CircleCI...")

	if err := browser.OpenURL(signupURL); err != nil {
		fmt.Printf("⚠️  Could not open browser automatically: %v\n", err)
		fmt.Printf("   Please manually visit: %s\n", signupURL)
	}

	fmt.Println("Waiting for authentication...")

	// Wait for the token or an error, with a timeout.
	select {
	case token := <-tokenCh:
		_ = server.Shutdown(context.Background())
		trackSignupStep(cmd, "token_received", nil)
		return saveToken(opts.cfg, token)
	case err := <-errCh:
		_ = server.Shutdown(context.Background())
		trackSignupStep(cmd, "failed", nil)
		return errors.Wrap(err, "signup failed")
	case <-time.After(5 * time.Minute):
		_ = server.Shutdown(context.Background())
		trackSignupStep(cmd, "timeout", nil)
		return errors.New("timed out waiting for signup to complete. Run `circleci setup` to manually configure your CLI with a personal API token")
	}
}

// corsMiddleware adds CORS headers allowing the CircleCI frontend to make
// cross-origin requests to the CLI's local server.
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "https://app.circleci.com")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

func handleToken(expectedState string, tokenCh chan<- string, errCh chan<- error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		token := query.Get("token")
		state := query.Get("cli_state")
		callbackErr := query.Get("error")

		// When an error is present, state validation is best-effort: if state
		// is provided it must match, but a missing state is tolerated because
		// the frontend may not have had access to it when the failure occurred.
		if callbackErr != "" {
			if state != "" && state != expectedState {
				http.Error(w, "State mismatch — possible CSRF. Please try again.", http.StatusBadRequest)
				errCh <- errors.New("state mismatch — possible CSRF attempt")
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, "Something went wrong. Please return to your terminal.")
			errCh <- fmt.Errorf("account created but token delivery failed (%s). Run `circleci setup` to manually configure your CLI with a personal API token", callbackErr)
			return
		}

		if state != expectedState {
			http.Error(w, "State mismatch — possible CSRF. Please try again.", http.StatusBadRequest)
			errCh <- errors.New("state mismatch — possible CSRF attempt")
			return
		}

		if token == "" {
			http.Error(w, "Missing token.", http.StatusBadRequest)
			errCh <- errors.New("callback returned an empty token. Run `circleci setup` to manually configure your CLI with a personal API token")
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "You may close this window and return to your terminal.")
		tokenCh <- token
	}
}

func saveToken(cfg *settings.Config, token string) error {
	cfg.Token = token
	if err := cfg.WriteToDisk(); err != nil {
		return errors.Wrap(err, "failed to save token to config")
	}
	fmt.Println("\n✅ Welcome to CircleCI! Your CLI is now authenticated.")
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
