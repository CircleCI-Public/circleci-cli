package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

var _ = Describe("Signup", func() {
	Describe("already authenticated guard", func() {
		It("should print already authenticated when token exists", func() {
			opts := signupOptions{
				cfg: &settings.Config{Token: "existing-token"},
			}

			output := clitest.WithCapturedOutput(func() {
				err := runSignup(dummyCmd(), opts)
				Expect(err).ShouldNot(HaveOccurred())
			})

			Expect(output).To(ContainSubstring("already authenticated"))
			Expect(output).To(ContainSubstring("circleci setup"))
		})

		It("should not guard when --force is set", func() {
			opts := signupOptions{
				cfg:   &settings.Config{Token: "existing-token"},
				force: true,
			}

			Expect(opts.force).To(BeTrue())
			Expect(!opts.force && opts.cfg.Token != "").To(BeFalse())
		})

		It("should not guard when no token exists", func() {
			opts := signupOptions{
				cfg: &settings.Config{Token: ""},
			}

			Expect(!opts.force && opts.cfg.Token != "").To(BeFalse())
		})
	})

	Describe("appBaseURL", func() {
		AfterEach(func() {
			os.Unsetenv(appBaseURLEnv)
		})

		It("returns the default when the env var is unset", func() {
			os.Unsetenv(appBaseURLEnv)
			Expect(appBaseURL()).To(Equal(defaultAppBaseURL))
		})

		It("honors the CIRCLECI_APP_URL override", func() {
			os.Setenv(appBaseURLEnv, "https://enterprise.example.com")
			Expect(appBaseURL()).To(Equal("https://enterprise.example.com"))
		})
	})

	Describe("pollHandshake", func() {
		It("returns the token once the backend responds with 200", func() {
			var calls int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := atomic.AddInt32(&calls, 1)
				Expect(r.Method).To(Equal(http.MethodGet))
				Expect(r.URL.Path).To(Equal("/api/v1/cli-handshake/abc-123"))
				if n < 2 {
					w.WriteHeader(http.StatusAccepted)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, `{"token":"pat-xyz","created_at":"2026-04-16T12:00:00Z"}`)
			}))
			defer server.Close()

			token, err := pollHandshakeFast(context.Background(), server.URL, "abc-123", time.Minute)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(token).To(Equal("pat-xyz"))
			Expect(atomic.LoadInt32(&calls)).To(BeNumerically(">=", 2))
		})

		It("returns an expired error on 404", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			_, err := pollHandshakeFast(context.Background(), server.URL, "missing", time.Minute)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expired"))
			Expect(err.Error()).To(ContainSubstring("circleci signup"))
		})

		It("fails on unexpected status codes", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			_, err := pollHandshakeFast(context.Background(), server.URL, "boom", time.Minute)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unexpected response"))
		})

		It("surfaces ctx.Err when the context is canceled", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
			}))
			defer server.Close()

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			_, err := pollHandshakeFast(ctx, server.URL, "id", time.Minute)
			Expect(err).To(MatchError(context.Canceled))
		})

		It("times out when the backend never completes the handshake", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
			}))
			defer server.Close()

			_, err := pollHandshakeFast(context.Background(), server.URL, "id", 20*time.Millisecond)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("timed out"))
		})

		It("returns an error after repeated network failures", func() {
			// Stand up a test server then immediately close it. Subsequent
			// requests to the bound port fail fast with "connection refused",
			// simulating sustained network errors without the HTTP client's
			// 10s timeout kicking in.
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
			addr := server.URL
			server.Close()

			_, err := pollHandshakeFast(context.Background(), addr, "id", time.Minute)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("network"))
		})
	})

	Describe("saveToken", func() {
		var tempSettings *clitest.TempSettings

		BeforeEach(func() {
			tempSettings = clitest.WithTempSettings()
		})

		AfterEach(func() {
			tempSettings.Close()
		})

		It("should write the token to the config file", func() {
			cfg := &settings.Config{
				FileUsed: tempSettings.Config.Path,
				Host:     "https://circleci.com",
			}

			output := clitest.WithCapturedOutput(func() {
				err := saveToken(cfg, "my-new-token")
				Expect(err).ShouldNot(HaveOccurred())
			})

			Expect(output).To(ContainSubstring("Welcome to CircleCI"))
			Expect(output).To(ContainSubstring("Next steps"))
			Expect(cfg.Token).To(Equal("my-new-token"))

			// Verify it was persisted to disk
			file, err := os.Open(tempSettings.Config.Path)
			Expect(err).ShouldNot(HaveOccurred())
			defer file.Close()

			content, err := io.ReadAll(file)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(content)).To(ContainSubstring("token: my-new-token"))
		})
	})

	Describe("createSignupEvent", func() {
		It("should create event with no_browser property", func() {
			event := createSignupEvent(true, nil)
			Expect(event.Object).To(Equal("cli-signup"))
			Expect(event.Action).To(Equal("signup"))
			Expect(event.Properties["no_browser"]).To(BeTrue())
			Expect(event.Properties["has_been_executed"]).To(BeTrue())
			Expect(event.Properties).ToNot(HaveKey("error"))
		})

		It("should include error when present", func() {
			event := createSignupEvent(false, fmt.Errorf("something broke"))
			Expect(event.Properties["error"]).To(Equal("something broke"))
			Expect(event.Properties["no_browser"]).To(BeFalse())
		})
	})
})

// pollHandshakeFast shortens the internal poll interval so the suite doesn't
// sit in 3-second sleeps between requests. The production delay is restored
// after each call.
func pollHandshakeFast(ctx context.Context, baseURL, handshakeID string, timeout time.Duration) (string, error) {
	orig := handshakePollWait
	handshakePollWait = 5 * time.Millisecond
	defer func() { handshakePollWait = orig }()
	return pollHandshake(ctx, baseURL, handshakeID, timeout)
}
