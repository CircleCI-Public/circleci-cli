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

import (
	"context"

	"github.com/google/uuid"
)

// The iOS code signing endpoints live under /api/v3/signing. Requests use the
// V3 data envelope ({"data":{"attributes":...,"references":...}}) and responses
// come back as V3 entities/collections. The public types below are the
// flattened shapes the command layer consumes; the wire envelopes are private.

// IOSCertificate is an Apple .p12 code signing certificate stored in CircleCI's
// secure storage.
type IOSCertificate struct {
	ID       uuid.UUID `json:"id,omitempty"`
	FileName string    `json:"file_name,omitempty"`
	CertType string    `json:"cert_type,omitempty"`
}

// IOSSigningConfig is an iOS signing config: a named pairing of a certificate
// and one or more provisioning profiles, referenced by name in pipeline config.
type IOSSigningConfig struct {
	ID                   uuid.UUID                `json:"id,omitempty"`
	Name                 string                   `json:"name,omitempty"`
	Certificate          *IOSCertificateRef       `json:"certificate,omitempty"`
	ProvisioningProfiles []IOSProvisioningProfile `json:"provisioning_profiles,omitempty"`
}

// IOSCertificateRef is the embedded certificate descriptor returned by the
// signing-config list endpoint. Holds only display fields.
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

// idEntity decodes a V3 create response, which echoes only the new entity's ID.
type idEntity struct {
	ID uuid.UUID `json:"id"`
}

// certEntity is the V3 wire shape for a certificate list/get item.
type certEntity struct {
	ID         uuid.UUID `json:"id"`
	Attributes struct {
		FileName string `json:"file_name"`
		CertType string `json:"cert_type"`
	} `json:"attributes"`
}

// signingConfigEntity is the V3 wire shape for a signing config list item. The
// certificate is carried as a reference whose attributes hold the display fields.
type signingConfigEntity struct {
	ID         uuid.UUID `json:"id"`
	Attributes struct {
		Name                 string                   `json:"name"`
		ProvisioningProfiles []IOSProvisioningProfile `json:"provisioning_profiles"`
	} `json:"attributes"`
	References struct {
		Certificate struct {
			Attributes IOSCertificateRef `json:"attributes"`
		} `json:"signing_certificate"`
	} `json:"references"`
}

// UploadIOSCertificate uploads a .p12 certificate to the org's secure storage.
// blob must be base64-encoded. Returns the new certificate ID.
func (c *Client) UploadIOSCertificate(ctx context.Context, orgID uuid.UUID, fileName, blob, password string) (uuid.UUID, error) {
	body := map[string]any{
		"data": map[string]any{
			"attributes": map[string]any{
				"file_name":     fileName,
				"cert_blob":     blob,
				"cert_password": password,
			},
			"references": map[string]any{
				"org": map[string]any{"id": orgID},
			},
		},
	}
	var env v3Entity[idEntity]
	if err := c.postV3(ctx, "/signing/certificates", body, &env); err != nil {
		return uuid.Nil, err
	}
	return env.Data.ID, nil
}

// ListIOSCertificates returns the certificates stored for the given org.
func (c *Client) ListIOSCertificates(ctx context.Context, orgID uuid.UUID) ([]IOSCertificate, error) {
	var env v3List[certEntity]
	if err := c.getV3(ctx, "/signing/certificates", &env, filterParam("org_id", orgID.String())); err != nil {
		return nil, err
	}
	certs := make([]IOSCertificate, len(env.Data))
	for i, e := range env.Data {
		certs[i] = IOSCertificate{
			ID:       e.ID,
			FileName: e.Attributes.FileName,
			CertType: e.Attributes.CertType,
		}
	}
	return certs, nil
}

// DeleteIOSCertificate deletes a certificate by ID. The server returns 409
// Conflict if the certificate is referenced by one or more signing configs.
func (c *Client) DeleteIOSCertificate(ctx context.Context, certID uuid.UUID) error {
	return c.deleteV3(ctx, "/signing/certificates/%s", routeParams(certID))
}

// CreateIOSSigningConfig creates a signing config linking a certificate to one
// or more base64-encoded provisioning profiles. Returns the new config ID.
func (c *Client) CreateIOSSigningConfig(ctx context.Context, orgID uuid.UUID, name string, certID uuid.UUID, profiles []IOSProvisioningProfile) (uuid.UUID, error) {
	body := map[string]any{
		"data": map[string]any{
			"attributes": map[string]any{
				"name":                  name,
				"provisioning_profiles": profiles,
			},
			"references": map[string]any{
				"org":                 map[string]any{"id": orgID},
				"signing_certificate": map[string]any{"id": certID},
			},
		},
	}
	var env v3Entity[idEntity]
	if err := c.postV3(ctx, "/signing/configs", body, &env); err != nil {
		return uuid.Nil, err
	}
	return env.Data.ID, nil
}

// ListIOSSigningConfigs returns the signing configs stored for the given org.
func (c *Client) ListIOSSigningConfigs(ctx context.Context, orgID uuid.UUID) ([]IOSSigningConfig, error) {
	var env v3List[signingConfigEntity]
	if err := c.getV3(ctx, "/signing/configs", &env, filterParam("org_id", orgID.String())); err != nil {
		return nil, err
	}
	configs := make([]IOSSigningConfig, len(env.Data))
	for i, e := range env.Data {
		cert := e.References.Certificate.Attributes
		configs[i] = IOSSigningConfig{
			ID:                   e.ID,
			Name:                 e.Attributes.Name,
			Certificate:          &cert,
			ProvisioningProfiles: e.Attributes.ProvisioningProfiles,
		}
	}
	return configs, nil
}

// DeleteIOSSigningConfig deletes a signing config by ID. The server returns
// 204 No Content on success.
func (c *Client) DeleteIOSSigningConfig(ctx context.Context, id uuid.UUID) error {
	return c.deleteV3(ctx, "/signing/configs/%s", routeParams(id))
}
