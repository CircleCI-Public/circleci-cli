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

package main

import (
	"bytes"
	"mime"
	"mime/multipart"
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/assert/cmp"
)

func TestNeedsUploadURL(t *testing.T) {
	assert.Check(t, !needsUploadURL(0))
	assert.Check(t, !needsUploadURL(directUploadLimit))  // exactly at the limit → direct
	assert.Check(t, needsUploadURL(directUploadLimit+1)) // one byte over → upload URL
	assert.Check(t, needsUploadURL(100<<20))             // comfortably over
}

func TestBuildMultipart(t *testing.T) {
	payload := []byte("circleci binary bytes")
	body, contentType, err := buildMultipart("circleci_1.0.0_linux_amd64.tar.gz", payload)
	assert.NilError(t, err)

	mediaType, params, err := mime.ParseMediaType(contentType)
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(mediaType, "multipart/form-data"))
	assert.Check(t, params["boundary"] != "")

	r := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	part, err := r.NextPart()
	assert.NilError(t, err)
	assert.Check(t, cmp.Equal(part.FormName(), "file"))
	assert.Check(t, cmp.Equal(part.FileName(), "circleci_1.0.0_linux_amd64.tar.gz"))

	got := new(bytes.Buffer)
	_, err = got.ReadFrom(part)
	assert.NilError(t, err)
	assert.Check(t, cmp.DeepEqual(got.Bytes(), payload))

	// Exactly one part.
	_, err = r.NextPart()
	assert.Check(t, err != nil)
}
