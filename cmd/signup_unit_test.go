package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/CircleCI-Public/circleci-cli/clitest"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

var _ = Describe("Signup", func() {
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

		It("should set CORS headers on a GET request", func() {
			wrapped := corsMiddleware(dummyHandler)
			req := httptest.NewRequest("GET", "/token", nil)
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			Expect(rec.Header().Get("Access-Control-Allow-Origin")).To(Equal("https://app.circleci.com"))
			Expect(rec.Header().Get("Access-Control-Allow-Methods")).To(Equal("GET, OPTIONS"))
			Expect(rec.Header().Get("Access-Control-Allow-Headers")).To(Equal("Content-Type"))
			Expect(handlerCalled).To(BeTrue())
			Expect(rec.Code).To(Equal(http.StatusOK))
		})

		It("should return 204 on OPTIONS preflight without calling the handler", func() {
			wrapped := corsMiddleware(dummyHandler)
			req := httptest.NewRequest("OPTIONS", "/token", nil)
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusNoContent))
			Expect(rec.Header().Get("Access-Control-Allow-Origin")).To(Equal("https://app.circleci.com"))
			Expect(handlerCalled).To(BeFalse())
		})

		It("should pin origin to app.circleci.com not wildcard", func() {
			wrapped := corsMiddleware(dummyHandler)
			req := httptest.NewRequest("GET", "/token", nil)
			rec := httptest.NewRecorder()

			wrapped.ServeHTTP(rec, req)

			origin := rec.Header().Get("Access-Control-Allow-Origin")
			Expect(origin).To(Equal("https://app.circleci.com"))
			Expect(origin).ToNot(Equal("*"))
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

		It("should accept a valid token and cli_state", func() {
			handler := handleToken(state, tokenCh, errCh)
			req := httptest.NewRequest("GET", "/token?token=mytoken&cli_state=abc123", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Body.String()).To(ContainSubstring("You may close this window"))
			Eventually(tokenCh).Should(Receive(Equal("mytoken")))
		})

		It("should reject a cli_state mismatch", func() {
			handler := handleToken(state, tokenCh, errCh)
			req := httptest.NewRequest("GET", "/token?token=mytoken&cli_state=wrong", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(http.StatusBadRequest))
			Expect(rec.Body.String()).To(ContainSubstring("State mismatch"))
			Eventually(errCh).Should(Receive())
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
				Expect(rec.Body.String()).To(ContainSubstring("Something went wrong"))
				var received error
				Eventually(errCh).Should(Receive(&received))
				Expect(received.Error()).To(ContainSubstring("token delivery failed"))
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

				Expect(rec.Code).To(Equal(http.StatusBadRequest))
				Expect(rec.Body.String()).To(ContainSubstring("State mismatch"))
				Eventually(errCh).Should(Receive())
			})
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
