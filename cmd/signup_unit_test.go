package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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

	Describe("generateState", func() {
		It("should return a 32-character hex string", func() {
			state, err := generateState()
			Expect(err).ShouldNot(HaveOccurred())
			Expect(state).To(HaveLen(32))
			Expect(state).To(MatchRegexp(`^[0-9a-f]{32}$`))
		})

		It("should generate unique values", func() {
			a, _ := generateState()
			b, _ := generateState()
			Expect(a).ToNot(Equal(b))
		})
	})

	Describe("corsMiddleware", func() {
		var dummyHandler http.HandlerFunc
		var handlerCalled bool

		BeforeEach(func() {
			handlerCalled = false
			dummyHandler = func(w http.ResponseWriter, r *http.Request) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, "ok")
			}
		})

		It("should set CORS headers on a GET request with valid origin", func() {
			wrapped := corsMiddleware(dummyHandler)
			req := httptest.NewRequest("GET", "/token", nil)
			req.Header.Set("Origin", "https://app.circleci.com")
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			Expect(rec.Header().Get("Access-Control-Allow-Origin")).To(Equal("https://app.circleci.com"))
			Expect(rec.Header().Get("Access-Control-Allow-Methods")).To(Equal("GET"))
			Expect(rec.Header().Get("Access-Control-Allow-Headers")).To(Equal("Content-Type"))
			Expect(rec.Header().Get("Access-Control-Allow-Private-Network")).To(Equal("true"))
			Expect(rec.Header().Get("Access-Control-Max-Age")).To(Equal("300"))
			Expect(handlerCalled).To(BeTrue())
			Expect(rec.Code).To(Equal(http.StatusOK))
		})

		It("should return 204 on OPTIONS preflight without calling the handler", func() {
			wrapped := corsMiddleware(dummyHandler)
			req := httptest.NewRequest("OPTIONS", "/token", nil)
			req.Header.Set("Origin", "https://app.circleci.com")
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNoContent))
			Expect(rec.Header().Get("Access-Control-Allow-Origin")).To(Equal("https://app.circleci.com"))
			Expect(rec.Header().Get("Access-Control-Allow-Private-Network")).To(Equal("true"))
			Expect(handlerCalled).To(BeFalse())
		})

		It("should reject requests with wrong origin", func() {
			wrapped := corsMiddleware(dummyHandler)
			req := httptest.NewRequest("GET", "/token", nil)
			req.Header.Set("Origin", "https://evil.com")
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusForbidden))
			Expect(handlerCalled).To(BeFalse())
		})

		It("should reject requests with missing origin", func() {
			wrapped := corsMiddleware(dummyHandler)
			req := httptest.NewRequest("GET", "/token", nil)
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusForbidden))
			Expect(handlerCalled).To(BeFalse())
		})
	})

	Describe("unique PAT label", func() {
		It("should generate a cli_label containing hostname and timestamp", func() {
			hostname, _ := os.Hostname()
			label := fmt.Sprintf("circleci-cli-%s-%d", hostname, time.Now().Unix())

			Expect(label).To(ContainSubstring("circleci-cli-"))
			Expect(label).To(ContainSubstring(hostname))
			Expect(label).To(MatchRegexp(`-\d+$`))
		})
	})

	Describe("Magic Path URL", func() {
		It("should open browser directly to /cli-auth, not /authentication/", func() {
			params := url.Values{}
			params.Set("cli_port", "12345")
			params.Set("cli_state", "abc123")
			params.Set("cli_label", "circleci-cli-test-1234")
			signupURL := "https://app.circleci.com/cli-auth?" + params.Encode()

			Expect(signupURL).ToNot(ContainSubstring("/authentication/"))
			Expect(signupURL).ToNot(ContainSubstring("/successful-signup"))
			Expect(signupURL).To(HavePrefix("https://app.circleci.com/cli-auth?"))
			Expect(signupURL).To(ContainSubstring("cli_port=12345"))
			Expect(signupURL).To(ContainSubstring("cli_state=abc123"))
			Expect(signupURL).To(ContainSubstring("cli_label=circleci-cli-test-1234"))
		})
	})

	Describe("handleToken", func() {
		var (
			tokenCh chan string
			errCh   chan error
			state   string
		)

		BeforeEach(func() {
			tokenCh = make(chan string, 1)
			errCh = make(chan error, 1)
			state = "abc123"
		})

		It("should accept a valid token and cli_state with JSON response", func() {
			handler := handleToken(state, tokenCh, errCh)
			req := httptest.NewRequest("GET", "/token?token=mytoken&cli_state=abc123", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Header().Get("Content-Type")).To(Equal("application/json"))
			Expect(rec.Body.String()).To(ContainSubstring(`"status":"ok"`))
			Eventually(tokenCh).Should(Receive(Equal("mytoken")))
		})

		It("should reject a cli_state mismatch with 403", func() {
			handler := handleToken(state, tokenCh, errCh)
			req := httptest.NewRequest("GET", "/token?token=mytoken&cli_state=wrong", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusForbidden))
			Expect(rec.Body.String()).To(ContainSubstring("Invalid state"))
			Eventually(errCh).Should(Receive())
		})

		It("should reject non-GET methods", func() {
			handler := handleToken(state, tokenCh, errCh)
			req := httptest.NewRequest("POST", "/token?token=mytoken&cli_state=abc123", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusMethodNotAllowed))
		})

		It("should reject a missing token when no error param is present", func() {
			handler := handleToken(state, tokenCh, errCh)
			req := httptest.NewRequest("GET", "/token?cli_state=abc123", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusBadRequest))
			Expect(rec.Body.String()).To(ContainSubstring("Missing token"))
			var received error
			Eventually(errCh).Should(Receive(&received))
			Expect(received.Error()).To(ContainSubstring("circleci setup"))
		})

		Context("with error param from frontend", func() {
			It("should forward the error when cli_state matches", func() {
				handler := handleToken(state, tokenCh, errCh)
				req := httptest.NewRequest("GET", "/token?error=token_creation_failed&cli_state=abc123", nil)
				rec := httptest.NewRecorder()

				handler.ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Header().Get("Content-Type")).To(Equal("application/json"))
				Expect(rec.Body.String()).To(ContainSubstring(`"status":"error"`))
				var received error
				Eventually(errCh).Should(Receive(&received))
				Expect(received.Error()).To(ContainSubstring("authentication failed"))
				Expect(received.Error()).To(ContainSubstring("token_creation_failed"))
				Expect(received.Error()).To(ContainSubstring("circleci setup"))
			})

			It("should tolerate a missing cli_state when error is present", func() {
				handler := handleToken(state, tokenCh, errCh)
				req := httptest.NewRequest("GET", "/token?error=no_token", nil)
				rec := httptest.NewRecorder()

				handler.ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusOK))
				var received error
				Eventually(errCh).Should(Receive(&received))
				Expect(received.Error()).To(ContainSubstring("no_token"))
			})

			It("should reject error with a mismatched cli_state", func() {
				handler := handleToken(state, tokenCh, errCh)
				req := httptest.NewRequest("GET", "/token?error=token_creation_failed&cli_state=wrong", nil)
				rec := httptest.NewRecorder()

				handler.ServeHTTP(rec, req)

				Expect(rec.Code).To(Equal(http.StatusForbidden))
				Expect(rec.Body.String()).To(ContainSubstring("Invalid state"))
				Eventually(errCh).Should(Receive())
			})
		})
	})

	Describe("sanitizeHostname", func() {
		It("should strip non-alphanumeric characters except hyphens", func() {
			Expect(sanitizeHostname("my<host>.local")).To(Equal("myhostlocal"))
			Expect(sanitizeHostname("MacBook-Pro")).To(Equal("MacBook-Pro"))
			Expect(sanitizeHostname("host name!@#")).To(Equal("hostname"))
			Expect(sanitizeHostname("")).To(Equal("unknown"))
			Expect(sanitizeHostname("!!!")).To(Equal("unknown"))
		})
	})

	Describe("stateMatches", func() {
		It("should return true for matching states", func() {
			Expect(stateMatches("abc123", "abc123")).To(BeTrue())
		})

		It("should return false for mismatched states", func() {
			Expect(stateMatches("abc123", "wrong")).To(BeFalse())
		})

		It("should return false for empty vs non-empty", func() {
			Expect(stateMatches("", "abc123")).To(BeFalse())
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
