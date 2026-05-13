// Copyright (c) 2026 Circle Internet Services, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// SPDX-License-Identifier: MIT

package apiclient_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// recorded captures what the handler observed from the request.
type recorded struct {
	Method string
	Path   string
	Query  string
	Body   map[string]any
	Token  string
}

// newClientWithMux starts an httptest.Server, lets the test register handlers
// on the returned mux, and returns an apiclient.Client pointed at it.
func newClientWithMux(t *testing.T) (*apiclient.Client, *http.ServeMux, *httptest.Server) {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return apiclient.New(srv.URL, "test-token", nil), mux, srv
}

// recordRequest reads + decodes the body and returns a *recorded.
func recordRequest(t *testing.T, r *http.Request) *recorded {
	t.Helper()
	rec := &recorded{
		Method: r.Method,
		Path:   r.URL.Path,
		Query:  r.URL.RawQuery,
		Token:  r.Header.Get("Circle-Token"),
	}
	if r.Body != nil {
		raw, err := io.ReadAll(r.Body)
		assert.NilError(t, err)
		if len(raw) > 0 {
			assert.NilError(t, json.Unmarshal(raw, &rec.Body))
		}
	}
	return rec
}

// --- UploadIOSCertificate ---

func TestUploadIOSCertificate_SendsCorrectRequest(t *testing.T) {
	client, mux, _ := newClientWithMux(t)

	var got *recorded
	mux.HandleFunc("/api/v2/certificates", func(w http.ResponseWriter, r *http.Request) {
		got = recordRequest(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cert-abc"}`))
	})

	id, err := client.UploadIOSCertificate(iostream.Testing(context.Background()),
		"org-uuid", "MyCert.p12", "base64blob", "secret")
	assert.NilError(t, err)
	assert.Equal(t, id, "cert-abc")

	assert.Assert(t, got != nil)
	assert.Equal(t, got.Method, http.MethodPost)
	assert.Equal(t, got.Path, "/api/v2/certificates")
	assert.Equal(t, got.Token, "test-token")
	assert.Check(t, cmp.Equal(got.Body["org_id"], "org-uuid"))
	assert.Check(t, cmp.Equal(got.Body["cert_file_name"], "MyCert.p12"))
	assert.Check(t, cmp.Equal(got.Body["cert_blob"], "base64blob"))
	assert.Check(t, cmp.Equal(got.Body["cert_password"], "secret"))
}

func TestUploadIOSCertificate_PropagatesServerError(t *testing.T) {
	client, mux, _ := newClientWithMux(t)

	mux.HandleFunc("/api/v2/certificates", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"invalid blob"}`))
	})

	_, err := client.UploadIOSCertificate(iostream.Testing(context.Background()),
		"org-uuid", "MyCert.p12", "blob", "secret")
	assert.Assert(t, err != nil)
}

// --- ListIOSCertificates ---

func TestListIOSCertificates_ParsesItems(t *testing.T) {
	client, mux, _ := newClientWithMux(t)

	var got *recorded
	mux.HandleFunc("/api/v2/certificates", func(w http.ResponseWriter, r *http.Request) {
		got = recordRequest(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[
			{"id":"cert-1","file_name":"a.p12","cert_type":"distribution"},
			{"id":"cert-2","file_name":"b.p12","cert_type":"development"}
		]}`))
	})

	certs, err := client.ListIOSCertificates(iostream.Testing(context.Background()), "org-uuid")
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(len(certs), 2))
	assert.Check(t, cmp.Equal(certs[0].ID, "cert-1"))
	assert.Check(t, cmp.Equal(certs[0].FileName, "a.p12"))
	assert.Check(t, cmp.Equal(certs[0].CertType, "distribution"))
	assert.Check(t, cmp.Equal(certs[1].ID, "cert-2"))

	assert.Assert(t, got != nil)
	assert.Equal(t, got.Method, http.MethodGet)
	assert.Equal(t, got.Query, "org-id=org-uuid", "list endpoint must use hyphenated org-id query param")
}

func TestListIOSCertificates_EmptyItems(t *testing.T) {
	client, mux, _ := newClientWithMux(t)

	mux.HandleFunc("/api/v2/certificates", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	})

	certs, err := client.ListIOSCertificates(iostream.Testing(context.Background()), "org-uuid")
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(len(certs), 0))
}

// --- DeleteIOSCertificate ---

func TestDeleteIOSCertificate_HitsCorrectRoute(t *testing.T) {
	client, mux, _ := newClientWithMux(t)

	var got *recorded
	mux.HandleFunc("/api/v2/certificates/cert-abc", func(w http.ResponseWriter, r *http.Request) {
		got = recordRequest(t, r)
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.DeleteIOSCertificate(iostream.Testing(context.Background()), "cert-abc")
	assert.NilError(t, err)
	assert.Assert(t, got != nil)
	assert.Equal(t, got.Method, http.MethodDelete)
	assert.Equal(t, got.Path, "/api/v2/certificates/cert-abc")
}

func TestDeleteIOSCertificate_PropagatesConflict(t *testing.T) {
	client, mux, _ := newClientWithMux(t)

	mux.HandleFunc("/api/v2/certificates/cert-abc", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"certificate is in use"}`))
	})

	err := client.DeleteIOSCertificate(iostream.Testing(context.Background()), "cert-abc")
	assert.Assert(t, err != nil)
}

// --- CreateIOSSigningConfig ---

func TestCreateIOSSigningConfig_SendsCorrectRequest(t *testing.T) {
	client, mux, _ := newClientWithMux(t)

	var got *recorded
	mux.HandleFunc("/api/v2/signing-configs", func(w http.ResponseWriter, r *http.Request) {
		got = recordRequest(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"sc-1"}`))
	})

	profiles := []apiclient.IOSProvisioningProfile{
		{FileName: "MyApp.mobileprovision", Blob: "base64-1"},
		{FileName: "MyAppExt.mobileprovision", Blob: "base64-2"},
	}
	id, err := client.CreateIOSSigningConfig(iostream.Testing(context.Background()),
		"org-uuid", "prod", "cert-abc", profiles)
	assert.NilError(t, err)
	assert.Equal(t, id, "sc-1")

	assert.Assert(t, got != nil)
	assert.Equal(t, got.Method, http.MethodPost)
	assert.Check(t, cmp.Equal(got.Body["org_id"], "org-uuid"))
	assert.Check(t, cmp.Equal(got.Body["name"], "prod"))
	assert.Check(t, cmp.Equal(got.Body["cert_id"], "cert-abc"))

	gotProfiles, _ := got.Body["provisioning_profiles"].([]any)
	assert.Check(t, cmp.Equal(len(gotProfiles), 2))
	first, _ := gotProfiles[0].(map[string]any)
	assert.Check(t, cmp.Equal(first["file_name"], "MyApp.mobileprovision"))
	assert.Check(t, cmp.Equal(first["blob"], "base64-1"))
}

// --- ListIOSSigningConfigs ---

func TestListIOSSigningConfigs_ParsesItems(t *testing.T) {
	client, mux, _ := newClientWithMux(t)

	var got *recorded
	mux.HandleFunc("/api/v2/signing-configs", func(w http.ResponseWriter, r *http.Request) {
		got = recordRequest(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[
			{"id":"sc-1","name":"prod","certificate":{"file_name":"a.p12","cert_type":"distribution"},"provisioning_profiles":[{"file_name":"p.mobileprovision"}]}
		]}`))
	})

	configs, err := client.ListIOSSigningConfigs(iostream.Testing(context.Background()), "org-uuid")
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(len(configs), 1))
	assert.Check(t, cmp.Equal(configs[0].ID, "sc-1"))
	assert.Check(t, cmp.Equal(configs[0].Name, "prod"))
	assert.Assert(t, configs[0].Certificate != nil)
	assert.Check(t, cmp.Equal(configs[0].Certificate.FileName, "a.p12"))
	assert.Check(t, cmp.Equal(configs[0].Certificate.CertType, "distribution"))
	assert.Check(t, cmp.Equal(len(configs[0].ProvisioningProfiles), 1))

	assert.Assert(t, got != nil)
	assert.Equal(t, got.Query, "org-id=org-uuid")
}

// --- DeleteIOSSigningConfig ---

func TestDeleteIOSSigningConfig_HitsCorrectRoute(t *testing.T) {
	client, mux, _ := newClientWithMux(t)

	var got *recorded
	mux.HandleFunc("/api/v2/signing-configs/sc-1", func(w http.ResponseWriter, r *http.Request) {
		got = recordRequest(t, r)
		w.WriteHeader(http.StatusOK)
	})

	err := client.DeleteIOSSigningConfig(iostream.Testing(context.Background()), "sc-1")
	assert.NilError(t, err)
	assert.Assert(t, got != nil)
	assert.Equal(t, got.Method, http.MethodDelete)
	assert.Equal(t, got.Path, "/api/v2/signing-configs/sc-1")
}
