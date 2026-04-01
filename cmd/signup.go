package cmd

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
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

	cmd.Flags().BoolVar(&opts.noBrowser, "no-browser", false, "Don't open a browser; print the signup URL and prompt for a token instead")
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

func runSignup(cmd *cobra.Command, opts signupOptions) error {
	if !opts.force && opts.cfg.Token != "" {
		fmt.Println("You're already authenticated. Your CLI is configured with a personal API token.")
		fmt.Println("If you want to reconfigure, run `circleci setup`.")
		return nil
	}

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

	// Generate a unique PAT label to avoid 422 duplicate errors when the
	// user runs signup multiple times or from different machines.
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	label := fmt.Sprintf("circleci-cli-%s-%d", sanitizeHostname(hostname), time.Now().Unix())

	// Build the signup URL. Go directly to /cli-auth (Magic Path).
	// The frontend checks for an existing session: if authenticated, it creates
	// the PAT immediately; if not, it redirects to signup first.
	params := url.Values{}
	params.Set("cli_port", fmt.Sprintf("%d", port))
	params.Set("cli_state", state)
	params.Set("cli_label", label)
	signupURL := "https://app.circleci.com/cli-auth?" + params.Encode()

	trackSignupStep(cmd, "browser_opening", nil)
	fmt.Println("Opening your browser to sign up for CircleCI...")
	decodedURL, _ := url.QueryUnescape(signupURL)
	fmt.Printf("  %s\n", decodedURL)

	if err := browser.OpenURL(signupURL); err != nil {
		fmt.Printf("⚠️  Could not open browser automatically: %v\n", err)
		fmt.Printf("   Please manually visit: %s\n", decodedURL)
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

const allowedOrigin = "https://app.circleci.com"

// corsMiddleware validates the Origin header and adds CORS headers allowing
// the CircleCI frontend to make cross-origin requests to the CLI's local server.
// Requests with missing or non-matching Origin are rejected with 403.
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != allowedOrigin {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Private-Network", "true")
		w.Header().Set("Access-Control-Max-Age", "300")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

func stateMatches(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func handleToken(expectedState string, tokenCh chan<- string, errCh chan<- error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		query := r.URL.Query()
		token := query.Get("token")
		state := query.Get("cli_state")
		callbackErr := query.Get("error")

		// When an error is present, state validation is best-effort: if state
		// is provided it must match, but a missing state is tolerated because
		// the frontend may not have had access to it when the failure occurred.
		if callbackErr != "" {
			if state != "" && !stateMatches(state, expectedState) {
				http.Error(w, "Invalid state", http.StatusForbidden)
				errCh <- errors.New("state mismatch — possible CSRF attempt")
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error"})
			errCh <- fmt.Errorf("authentication failed (%s). Run `circleci setup` to manually configure your CLI with a personal API token", callbackErr)
			return
		}

		if !stateMatches(state, expectedState) {
			http.Error(w, "Invalid state", http.StatusForbidden)
			errCh <- errors.New("state mismatch — possible CSRF attempt")
			return
		}

		if token == "" {
			http.Error(w, "Missing token", http.StatusBadRequest)
			errCh <- errors.New("callback returned an empty token. Run `circleci setup` to manually configure your CLI with a personal API token")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		tokenCh <- token
	}
}

func sanitizeHostname(h string) string {
	var b strings.Builder
	for _, r := range h {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if s == "" {
		return "unknown"
	}
	return s
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
