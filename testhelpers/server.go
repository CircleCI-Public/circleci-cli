// server.go provides HTTP test server utilities that replace ghttp from
// the legacy clitest/ package.
//
// Migration path: replace ghttp.NewServer() with testhelpers.NewTestServer()
// and ghttp.CombineHandlers with sequential AppendHandler calls.
package testhelpers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// TestServer wraps httptest.Server with sequential handler dispatch.
type TestServer struct {
	URL    string
	Server *httptest.Server

	mu       sync.Mutex
	handlers []http.HandlerFunc
	callIdx  int
}

// NewTestServer creates a new test HTTP server that dispatches requests to
// handlers in the order they were appended. The server is closed
// automatically via t.Cleanup.
func NewTestServer(t testing.TB, handlers ...http.HandlerFunc) *TestServer {
	t.Helper()

	ts := &TestServer{
		handlers: handlers,
	}

	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.mu.Lock()
		idx := ts.callIdx
		ts.callIdx++
		h := ts.handlers
		ts.mu.Unlock()

		if idx >= len(h) {
			t.Errorf("TestServer: unexpected request #%d %s %s (only %d handlers registered)", idx+1, r.Method, r.URL.Path, len(h))
			http.Error(w, "unexpected request", http.StatusInternalServerError)
			return
		}
		h[idx](w, r)
	}))
	ts.URL = ts.Server.URL

	t.Cleanup(func() {
		ts.Server.Close()
	})

	return ts
}

// AppendHandler adds an HTTP handler to the sequential dispatch queue.
func (ts *TestServer) AppendHandler(h http.HandlerFunc) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.handlers = append(ts.handlers, h)
}

// VerifyGQLRequest returns a handler that asserts the request is a POST to
// /graphql-unstable with the correct Authorization header and JSON content
// type, then reads and verifies the request body contains the expected query.
func VerifyGQLRequest(t testing.TB, token string, expectedBody string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("VerifyGQLRequest: expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/graphql-unstable" {
			t.Errorf("VerifyGQLRequest: expected path /graphql-unstable, got %s", r.URL.Path)
		}
		if token != "" {
			got := r.Header["Authorization"]
			want := []string{token}
			if len(got) != len(want) || got[0] != want[0] {
				t.Errorf("VerifyGQLRequest: expected Authorization header %v, got %v", want, got)
			}
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("VerifyGQLRequest: reading body: %v", err)
		}
		defer r.Body.Close() //nolint:errcheck

		if expectedBody != "" {
			// Compare as JSON to ignore whitespace differences.
			var expected, actual interface{}
			if err := json.Unmarshal([]byte(expectedBody), &expected); err != nil {
				t.Fatalf("VerifyGQLRequest: unmarshalling expected body: %v", err)
			}
			if err := json.Unmarshal(body, &actual); err != nil {
				t.Fatalf("VerifyGQLRequest: unmarshalling actual body: %v", err)
			}
			expectedJSON, _ := json.Marshal(expected)
			actualJSON, _ := json.Marshal(actual)
			if string(expectedJSON) != string(actualJSON) {
				t.Errorf("VerifyGQLRequest: body mismatch\nexpected: %s\nactual:   %s", expectedJSON, actualJSON)
			}
		}
	}
}

// RespondJSON returns a handler that writes a JSON-encoded response with
// the given status code.
func RespondJSON(t testing.TB, status int, body interface{}) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("RespondJSON: encoding response: %v", err)
		}
	}
}

// ChainHandlers combines multiple handler functions into a single handler
// that calls each in sequence. This replaces ghttp.CombineHandlers.
func ChainHandlers(handlers ...http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for _, h := range handlers {
			h(w, r)
		}
	}
}

// RespondGQLData returns a handler that writes a GraphQL-style JSON response
// with the given status code, wrapping jsonBody in a {"data": ...} envelope.
// This replaces the response portion of clitest.AppendPostHandler.
func RespondGQLData(t testing.TB, status int, jsonBody string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, `{ "data": %s}`, jsonBody)
	}
}

// AppendGQLPostHandler adds a handler to ts that verifies a POST to
// /graphql-unstable with the given auth token and expected request JSON,
// then responds with the given response JSON wrapped in a data envelope.
// If errorResponse is non-empty, the response is wrapped as
// {"data": <response>, "errors": <errorResponse>} instead.
// This is a direct replacement for clitest.AppendPostHandler.
func AppendGQLPostHandler(t testing.TB, ts *TestServer, token, expectedBody, response, errorResponse string) {
	t.Helper()
	responseBody := fmt.Sprintf(`{ "data": %s}`, response)
	if errorResponse != "" {
		responseBody = fmt.Sprintf(`{ "data": %s, "errors": %s}`, response, errorResponse)
	}
	ts.AppendHandler(ChainHandlers(
		VerifyGQLRequest(t, token, expectedBody),
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(responseBody))
		},
	))
}
