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

	"github.com/google/uuid"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"

	"github.com/CircleCI-Public/circleci-cli/internal/apiclient"
	"github.com/CircleCI-Public/circleci-cli/internal/iostream"
)

// Shared UUIDs for the iOS signing client tests. Real IDs are UUIDs, so the
// client types them as uuid.UUID; the literal strings in the JSON payloads
// below are the canonical (lowercase) forms of these values.
var (
	testOrgID  = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	testCertID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	testSCID   = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	testCert1  = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	testCert2  = uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
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
	return apiclient.New(apiclient.Config{
		BaseURL: srv.URL,
		Token:   "test-token",
		Version: "1.2.3",
	}), mux, srv
}

// recordRequest reads + decodes the body and returns a *recorded.
func recordRequest(t *testing.T, r *http.Request) *recorded {
	t.Helper()
	rec := &recorded{
		Method: r.Method,
		Path:   r.URL.Path,
		Query:  r.URL.RawQuery,
		Token:  r.Header.Get("Authorization"),
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
	mux.HandleFunc("/api/v3/signing/certificates", func(w http.ResponseWriter, r *http.Request) {
		got = recordRequest(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"22222222-2222-2222-2222-222222222222"}}`))
	})

	id, err := client.UploadIOSCertificate(iostream.Testing(context.Background()),
		testOrgID, "MyCert.p12", "base64blob", "secret")
	assert.NilError(t, err)
	assert.Equal(t, id, testCertID)

	assert.Assert(t, got != nil)
	assert.Equal(t, got.Method, http.MethodPost)
	assert.Equal(t, got.Path, "/api/v3/signing/certificates")
	assert.Equal(t, got.Token, "Bearer test-token")

	// Body uses the V3 data envelope: attributes + an org reference.
	data, _ := got.Body["data"].(map[string]any)
	attrs, _ := data["attributes"].(map[string]any)
	assert.Check(t, cmp.Equal(attrs["file_name"], "MyCert.p12"))
	assert.Check(t, cmp.Equal(attrs["cert_blob"], "base64blob"))
	assert.Check(t, cmp.Equal(attrs["cert_password"], "secret"))
	refs, _ := data["references"].(map[string]any)
	org, _ := refs["org"].(map[string]any)
	assert.Check(t, cmp.Equal(org["id"], testOrgID.String()))
}

func TestUploadIOSCertificate_PropagatesServerError(t *testing.T) {
	client, mux, _ := newClientWithMux(t)

	mux.HandleFunc("/api/v3/signing/certificates", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"invalid blob"}`))
	})

	_, err := client.UploadIOSCertificate(iostream.Testing(context.Background()),
		testOrgID, "MyCert.p12", "blob", "secret")
	assert.Assert(t, err != nil)
}

// --- ListIOSCertificates ---

func TestListIOSCertificates_ParsesItems(t *testing.T) {
	client, mux, _ := newClientWithMux(t)

	var got *recorded
	mux.HandleFunc("/api/v3/signing/certificates", func(w http.ResponseWriter, r *http.Request) {
		got = recordRequest(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
			{"id":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","attributes":{"file_name":"a.p12","cert_type":"distribution"}},
			{"id":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","attributes":{"file_name":"b.p12","cert_type":"development"}}
		]}`))
	})

	certs, err := client.ListIOSCertificates(iostream.Testing(context.Background()), testOrgID)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(len(certs), 2))
	assert.Check(t, cmp.Equal(certs[0].ID, testCert1))
	assert.Check(t, cmp.Equal(certs[0].FileName, "a.p12"))
	assert.Check(t, cmp.Equal(certs[0].CertType, "distribution"))
	assert.Check(t, cmp.Equal(certs[1].ID, testCert2))

	assert.Assert(t, got != nil)
	assert.Equal(t, got.Method, http.MethodGet)
	assert.Equal(t, got.Query, "filter%5Borg_id%5D=11111111-1111-1111-1111-111111111111", "list endpoint must filter on org_id")
}

func TestListIOSCertificates_EmptyItems(t *testing.T) {
	client, mux, _ := newClientWithMux(t)

	mux.HandleFunc("/api/v3/signing/certificates", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	})

	certs, err := client.ListIOSCertificates(iostream.Testing(context.Background()), testOrgID)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(len(certs), 0))
}

// --- DeleteIOSCertificate ---

func TestDeleteIOSCertificate_HitsCorrectRoute(t *testing.T) {
	client, mux, _ := newClientWithMux(t)

	var got *recorded
	mux.HandleFunc("/api/v3/signing/certificates/"+testCertID.String(), func(w http.ResponseWriter, r *http.Request) {
		got = recordRequest(t, r)
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.DeleteIOSCertificate(iostream.Testing(context.Background()), testCertID)
	assert.NilError(t, err)
	assert.Assert(t, got != nil)
	assert.Equal(t, got.Method, http.MethodDelete)
	assert.Equal(t, got.Path, "/api/v3/signing/certificates/"+testCertID.String())
}

func TestDeleteIOSCertificate_PropagatesConflict(t *testing.T) {
	client, mux, _ := newClientWithMux(t)

	mux.HandleFunc("/api/v3/signing/certificates/"+testCertID.String(), func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"certificate is in use"}`))
	})

	err := client.DeleteIOSCertificate(iostream.Testing(context.Background()), testCertID)
	assert.Assert(t, err != nil)
}

// --- CreateIOSSigningConfig ---

func TestCreateIOSSigningConfig_SendsCorrectRequest(t *testing.T) {
	client, mux, _ := newClientWithMux(t)

	var got *recorded
	mux.HandleFunc("/api/v3/signing/configs", func(w http.ResponseWriter, r *http.Request) {
		got = recordRequest(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"33333333-3333-3333-3333-333333333333"}}`))
	})

	profiles := []apiclient.IOSProvisioningProfile{
		{FileName: "MyApp.mobileprovision", Blob: "base64-1"},
		{FileName: "MyAppExt.mobileprovision", Blob: "base64-2"},
	}
	id, err := client.CreateIOSSigningConfig(iostream.Testing(context.Background()),
		testOrgID, "prod", testCertID, profiles)
	assert.NilError(t, err)
	assert.Equal(t, id, testSCID)

	assert.Assert(t, got != nil)
	assert.Equal(t, got.Method, http.MethodPost)
	assert.Equal(t, got.Path, "/api/v3/signing/configs")

	// Body uses the V3 data envelope: name + profiles in attributes, org and
	// signing_certificate as references.
	data, _ := got.Body["data"].(map[string]any)
	attrs, _ := data["attributes"].(map[string]any)
	assert.Check(t, cmp.Equal(attrs["name"], "prod"))
	refs, _ := data["references"].(map[string]any)
	org, _ := refs["org"].(map[string]any)
	assert.Check(t, cmp.Equal(org["id"], testOrgID.String()))
	cert, _ := refs["signing_certificate"].(map[string]any)
	assert.Check(t, cmp.Equal(cert["id"], testCertID.String()))

	gotProfiles, _ := attrs["provisioning_profiles"].([]any)
	assert.Check(t, cmp.Equal(len(gotProfiles), 2))
	first, _ := gotProfiles[0].(map[string]any)
	assert.Check(t, cmp.Equal(first["file_name"], "MyApp.mobileprovision"))
	assert.Check(t, cmp.Equal(first["blob"], "base64-1"))
}

// --- ListIOSSigningConfigs ---

func TestListIOSSigningConfigs_ParsesItems(t *testing.T) {
	client, mux, _ := newClientWithMux(t)

	var got *recorded
	mux.HandleFunc("/api/v3/signing/configs", func(w http.ResponseWriter, r *http.Request) {
		got = recordRequest(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
			{"id":"33333333-3333-3333-3333-333333333333","attributes":{"name":"prod","provisioning_profiles":[{"file_name":"p.mobileprovision"}]},"references":{"signing_certificate":{"id":"22222222-2222-2222-2222-222222222222","attributes":{"file_name":"a.p12","cert_type":"distribution"}}}}
		]}`))
	})

	configs, err := client.ListIOSSigningConfigs(iostream.Testing(context.Background()), testOrgID)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(len(configs), 1))
	assert.Check(t, cmp.Equal(configs[0].ID, testSCID))
	assert.Check(t, cmp.Equal(configs[0].Name, "prod"))
	assert.Assert(t, configs[0].Certificate != nil)
	assert.Check(t, cmp.Equal(configs[0].Certificate.FileName, "a.p12"))
	assert.Check(t, cmp.Equal(configs[0].Certificate.CertType, "distribution"))
	assert.Check(t, cmp.Equal(len(configs[0].ProvisioningProfiles), 1))

	assert.Assert(t, got != nil)
	assert.Equal(t, got.Query, "filter%5Borg_id%5D=11111111-1111-1111-1111-111111111111")
}

// --- DeleteIOSSigningConfig ---

func TestDeleteIOSSigningConfig_HitsCorrectRoute(t *testing.T) {
	client, mux, _ := newClientWithMux(t)

	var got *recorded
	mux.HandleFunc("/api/v3/signing/configs/"+testSCID.String(), func(w http.ResponseWriter, r *http.Request) {
		got = recordRequest(t, r)
		w.WriteHeader(http.StatusNoContent)
	})

	err := client.DeleteIOSSigningConfig(iostream.Testing(context.Background()), testSCID)
	assert.NilError(t, err)
	assert.Assert(t, got != nil)
	assert.Equal(t, got.Method, http.MethodDelete)
	assert.Equal(t, got.Path, "/api/v3/signing/configs/"+testSCID.String())
}
