package settings_test

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/CircleCI-Public/circleci-cli/settings"
)

func TestWithHTTPClient(t *testing.T) {
	table := []struct {
		label   string
		tlsCert string
		fn      func()
		expErr  string
	}{
		{
			label:   "should return an error when path does not point to a file",
			tlsCert: "..",
			expErr:  "provided TLSCert path must be a file",
		},
		{
			label:   "should return an error when files/directories are world-writable",
			tlsCert: "testdata/mockcert.pem",
			fn: func() {
				err := os.Chmod("testdata/mockcert.pem", 0602)
				if err != nil {
					panic(fmt.Sprintf("unable to modify permissions in test: %s", err.Error()))
				}
			},
			expErr: func() string {
				if runtime.GOOS == "windows" {
					return ""
				}
				return "mockcert.pem cannot be world-writable"
			}(),
		},
		{
			label:   "should return an error when certificate contents are invalid",
			tlsCert: "settings_test.go",
			expErr:  "unable to parse certificates",
		},
		{
			label:   "should configure httpclient successfully",
			tlsCert: "testdata/mockcert.pem",
			fn: func() {
				err := os.Chmod("testdata/mockcert.pem", 0600)
				if err != nil {
					panic(fmt.Sprintf("unable to modify permissions in test: %s", err.Error()))
				}
			},
			expErr: "",
		},
	}

	for _, ts := range table {
		t.Run(ts.label, func(t *testing.T) {
			c := settings.Config{
				TLSCert: ts.tlsCert,
			}

			if ts.fn != nil {
				ts.fn()
			}

			err := c.WithHTTPClient()
			if err != nil {
				if ts.expErr == "" || !strings.Contains(err.Error(), ts.expErr) {
					t.Fatalf("unexpected error: %s", err.Error())
				}
				return
			}

			if ts.expErr != "" {
				t.Fatalf("unexpected nil error")
			}
		})
	}
}

func TestServerURL(t *testing.T) {
	config := settings.Config{
		Host:         "/host",
		RestEndpoint: "/restendpoint",
	}

	serverURL, err := config.ServerURL()

	assert.NilError(t, err)
	assert.Equal(t, serverURL.String(), "/restendpoint/")
}
