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

package resourceclass

// Version is the algorithm's semantic version, emitted in every finding's
// keys so a result can be traced to the algorithm that made it.
//   - MAJOR: the criterion that makes a downsize actionable changes, or a
//     contract breaks (an input no longer read, stored evidence no longer
//     matched).
//   - MINOR: estimates, evidence, thresholds, codes or matching change within
//     that criterion; a bug fix that changes findings is MINOR.
//   - PATCH: the boundary changed, the semantic output on the golden corpus
//     did not.
//
// The version covers findings and candidates under --modules resourceclass,
// not the text report's wording, the JSON schema (schema_version) or the
// other modules.
const Version = "2.12.0"

// FingerprintVersion names how fingerprint identifies a job: fp1 was
// 16 hex digits of yaml.Marshal (2.5.0), fp2 is the full SHA-256 of
// canonical JSON (2.6.0 on). A record matches only a fingerprint of its own
// version. A version that changes the fingerprint emits the previous one as
// well, as "fingerprint_prev", so stored records can be re-keyed.
const FingerprintVersion = "fp2"
