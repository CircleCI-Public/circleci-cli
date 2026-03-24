package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
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
	signupURL := fmt.Sprintf("https://circleci.com/signup?source=cli&state=%s", state)
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
	mux.HandleFunc("/callback", handleCallback)
	mux.HandleFunc("/complete", handleComplete(state, tokenCh, errCh))

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

	signupURL := fmt.Sprintf(
		"https://circleci.com/signup?source=cli&state=%s&return-to=http://127.0.0.1:%d/callback",
		state, port,
	)

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

func handleCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// The fragment (#token=...&state=...) is never sent to the server by the
	// browser. So we serve a small page whose script reads the fragment and
	// sends the values back to /complete.
	//
	// The frontend may also redirect with #error=token_creation_failed&state=...
	// when PAT creation fails. The script detects this and forwards the error
	// to /complete so the CLI can exit immediately instead of timing out.
	fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head><title>CircleCI CLI</title></head>
<body>
<p>Authenticating...</p>
<script>
(function() {
	var params = new URLSearchParams(window.location.hash.substring(1));
	var token = params.get("token");
	var state = params.get("state");
	var error = params.get("error");

	if (error) {
		var qs = "error=" + encodeURIComponent(error);
		if (state) { qs += "&state=" + encodeURIComponent(state); }
		fetch("/complete?" + qs)
			.then(function() {
				document.body.innerText = "Something went wrong. Please try again or use 'circleci setup' to configure your CLI manually.";
			})
			.catch(function() {
				document.body.innerText = "Something went wrong. Please try again or use 'circleci setup' to configure your CLI manually.";
			});
		return;
	}

	if (!token || !state) {
		var fallbackQs = "error=no_token";
		if (state) { fallbackQs += "&state=" + encodeURIComponent(state); }
		fetch("/complete?" + fallbackQs)
			.then(function() {
				document.body.innerText = "Something went wrong. Please try again or use 'circleci setup' to configure your CLI manually.";
			})
			.catch(function() {
				document.body.innerText = "Something went wrong. Please try again or use 'circleci setup' to configure your CLI manually.";
			});
		return;
	}

	fetch("/complete?token=" + encodeURIComponent(token) + "&state=" + encodeURIComponent(state))
		.then(function(resp) { return resp.text(); })
		.then(function(msg) { document.body.innerText = msg; })
		.catch(function(err) { document.body.innerText = "Error: " + err; });
})();
</script>
</body>
</html>`)
}

func handleComplete(expectedState string, tokenCh chan<- string, errCh chan<- error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		token := query.Get("token")
		state := query.Get("state")
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
