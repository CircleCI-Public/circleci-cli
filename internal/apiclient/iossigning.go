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

package apiclient

import "context"

// IOSCertificate is an Apple .p12 code signing certificate stored in CircleCI's
// secure storage. Shape matches the ciam-gateway list response.
type IOSCertificate struct {
	ID       string `json:"id,omitempty"`
	FileName string `json:"file_name,omitempty"`
	CertType string `json:"cert_type,omitempty"`
}

// IOSSigningConfig is an iOS signing config: a named pairing of a certificate
// and one or more provisioning profiles, referenced by name in pipeline config.
// Shape matches the ciam-gateway list response — the certificate is returned
// as a nested object (file_name + cert_type only); the cert UUID is not echoed
// back in list output.
type IOSSigningConfig struct {
	ID                   string                   `json:"id,omitempty"`
	Name                 string                   `json:"name,omitempty"`
	Certificate          *IOSCertificateRef       `json:"certificate,omitempty"`
	ProvisioningProfiles []IOSProvisioningProfile `json:"provisioning_profiles,omitempty"`
}

// IOSCertificateRef is the embedded certificate descriptor returned by the
// signing-config list endpoint. Holds only display fields — no UUID.
type IOSCertificateRef struct {
	FileName string `json:"file_name,omitempty"`
	CertType string `json:"cert_type,omitempty"`
}

// IOSProvisioningProfile is a base64-encoded Apple provisioning profile. Blob
// is populated on create; list responses only echo the file name.
type IOSProvisioningProfile struct {
	FileName string `json:"file_name"`
	Blob     string `json:"blob,omitempty"`
}

// UploadIOSCertificate uploads a .p12 certificate to the org's secure storage.
// blob must be base64-encoded. Returns the new certificate ID.
func (c *Client) UploadIOSCertificate(ctx context.Context, orgID, fileName, blob, password string) (string, error) {
	body := map[string]any{
		"org_id":         orgID,
		"cert_file_name": fileName,
		"cert_blob":      blob,
		"cert_password":  password,
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := c.post(ctx, "/certificates", body, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

// ListIOSCertificates returns the certificates stored for the given org.
func (c *Client) ListIOSCertificates(ctx context.Context, orgID string) ([]IOSCertificate, error) {
	var resp struct {
		Items []IOSCertificate `json:"items"`
	}
	if err := c.get(ctx, "/certificates", &resp, queryParam("org-id", orgID)); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// DeleteIOSCertificate deletes a certificate by ID. The server returns 409
// Conflict if the certificate is referenced by one or more signing configs.
func (c *Client) DeleteIOSCertificate(ctx context.Context, certID string) error {
	return c.deleteV2(ctx, "/certificates/%s", routeParams(certID))
}

// CreateIOSSigningConfig creates a signing config linking a certificate to one
// or more base64-encoded provisioning profiles. Returns the new config ID.
func (c *Client) CreateIOSSigningConfig(ctx context.Context, orgID, name, certID string, profiles []IOSProvisioningProfile) (string, error) {
	body := map[string]any{
		"name":                  name,
		"org_id":                orgID,
		"cert_id":               certID,
		"provisioning_profiles": profiles,
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := c.post(ctx, "/signing-configs", body, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

// ListIOSSigningConfigs returns the signing configs stored for the given org.
func (c *Client) ListIOSSigningConfigs(ctx context.Context, orgID string) ([]IOSSigningConfig, error) {
	var resp struct {
		Items []IOSSigningConfig `json:"items"`
	}
	if err := c.get(ctx, "/signing-configs", &resp, queryParam("org-id", orgID)); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// DeleteIOSSigningConfig deletes a signing config by ID. The server returns
// 204 No Content on success.
func (c *Client) DeleteIOSSigningConfig(ctx context.Context, id string) error {
	return c.deleteV2(ctx, "/signing-configs/%s", routeParams(id))
}
