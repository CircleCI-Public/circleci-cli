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

package extension

import "fmt"

// ErrInvalidName is returned when the extension name contains invalid characters.
type ErrInvalidName struct {
	Name string
}

func (e *ErrInvalidName) Error() string {
	return fmt.Sprintf("%q is not a valid extension name: use only letters, digits, hyphens, and underscores", e.Name)
}

// ErrExtensionNotFound is returned when the extension does not exist in the registry.
type ErrExtensionNotFound struct {
	Name string
}

func (e *ErrExtensionNotFound) Error() string {
	return fmt.Sprintf("extension %q not found in the registry", e.Name)
}

// ErrNoBinaryForPlatform is returned when the registry has no binary for the current OS/arch.
type ErrNoBinaryForPlatform struct {
	Name string
	OS   string
	Arch string
}

func (e *ErrNoBinaryForPlatform) Error() string {
	return fmt.Sprintf("extension %q has no binary for %s/%s", e.Name, e.OS, e.Arch)
}

// ErrDownloadFailed is returned when the binary download returns a non-200 status.
type ErrDownloadFailed struct {
	StatusCode int
}

func (e *ErrDownloadFailed) Error() string {
	return fmt.Sprintf("extension download failed with status %d", e.StatusCode)
}

// ErrChecksumMismatch is returned when the downloaded binary does not match the expected SHA256.
type ErrChecksumMismatch struct {
	Expected string
	Got      string
}

func (e *ErrChecksumMismatch) Error() string {
	return fmt.Sprintf("checksum mismatch: expected %s, got %s", e.Expected, e.Got)
}

type ErrCorruptExtensionManifest struct {
	ManifestPath string
	Err          error
}

func (e *ErrCorruptExtensionManifest) Error() string {
	return fmt.Sprintf("corrupt extension manifest: %s", e.ManifestPath)
}

// ErrExtensionBinaryNotFound is returned when the extension binary named in a
// manifest does not exist on disk
type ErrExtensionBinaryNotFound struct {
	Name string
	Path string
}

func (e *ErrExtensionBinaryNotFound) Error() string {
	return fmt.Sprintf("could not run extension %q - no binary found at %q", e.Name, e.Path)
}

// ErrExited is returned by Run when the extension process exits with a
// non-zero status code. The caller should exit with Code rather than printing
// an error message — the extension is responsible for its own error output.
type ErrExited struct {
	Code int
}

func (e *ErrExited) Error() string {
	return fmt.Sprintf("extension exited with code %d", e.Code)
}

// ErrExtensionTooLarge is returned when the decompressed extension binary
// exceeds the maximum allowed size.
type ErrExtensionTooLarge struct {
	Name    string
	MaxSize int64
}

func (e *ErrExtensionTooLarge) Error() string {
	return fmt.Sprintf("extension %q exceeds maximum allowed size of %d bytes", e.Name, e.MaxSize)
}

// ErrExtensionNotInstalled is returned when attempting to remove an extension
// that is not installed.
type ErrExtensionNotInstalled struct {
	Name string
}

func (e *ErrExtensionNotInstalled) Error() string {
	return fmt.Sprintf("extension %q is not installed", e.Name)
}
