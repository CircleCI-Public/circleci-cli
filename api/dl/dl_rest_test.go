package dl

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/settings"
	"github.com/CircleCI-Public/circleci-cli/version"
)

// getDlRestClient returns a dlRestClient hooked up to the passed server
func getDlRestClient(server *httptest.Server) (*dlRestClient, error) {
	return NewDlRestClient(settings.Config{
		DlHost:     server.URL,
		HTTPClient: http.DefaultClient,
		Token:      "token",
	})
}

func Test_DLCPurge(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr bool
	}{
		{
			name: "Should handle a successful request",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, r.Header.Get("circle-token"), "token")
				assert.Equal(t, r.Header.Get("accept"), "application/json")
				assert.Equal(t, r.Header.Get("user-agent"), version.UserAgent())

				assert.Equal(t, r.Method, "DELETE")
				assert.Equal(t, r.URL.Path, fmt.Sprintf("/private/output/project/%s/dlc", "projectid"))

				// check the request was made with an empty body
				br := r.Body
				b, err := io.ReadAll(br)
				assert.NilError(t, err)
				assert.Equal(t, string(b), "")
				assert.NilError(t, br.Close())

				// send response as empty 200
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, err = w.Write([]byte(``))
				assert.NilError(t, err)
			},
		},
		{
			name: "Should handle an error request",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("content-type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, err := w.Write([]byte(`{"message": "error"}`))
				assert.NilError(t, err)
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			c, err := getDlRestClient(server)
			assert.NilError(t, err)

			err = c.PurgeDLC("projectid")
			if (err != nil) != tt.wantErr {
				t.Errorf("PurgeDLC() error = %#v (%s), wantErr %v", err, err, tt.wantErr)
				return
			}
		})
	}
}
