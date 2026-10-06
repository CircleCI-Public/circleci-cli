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

//go:build !unix

package publish

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// ToFile writes data to a new file at path through a temporary file and a
// rename. inputPath is the analysed config ("" for stdin); -o may not
// name it, since replacing the input is --in-place's job.
func ToFile(path string, data []byte, inputPath string, force bool) error {
	if err := PreflightToFile(path, inputPath, force); err != nil {
		return err
	}
	tmp, err := writeTemp(filepath.Dir(path), data, defaultMode)
	if err != nil {
		return err
	}
	if !force {
		// The rename below replaces a file, so look once more for one created
		// since the preflight.
		if _, err := os.Lstat(path); err == nil {
			_ = os.Remove(tmp)
			return refuseHint("output.exists", "Pass --force to overwrite it", "%s already exists", path)
		}
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// Snapshot is what PreflightInPlace saw of the file. InPlace takes it, so the
// refusals run once, and re-verifies only that it is the same file, still
// holding the analysed content.
type Snapshot struct {
	info fs.FileInfo
}

// PreflightInPlace runs InPlace's refusals without writing, so a file that
// cannot be replaced fails before any analysis.
func PreflightInPlace(path string, force bool) (Snapshot, error) {
	var none Snapshot
	info, err := os.Lstat(path)
	if err != nil {
		return none, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return none, refuseHint("output.symlink", "Pass the path the link points to", "%s is a symlink", path)
	}
	if !info.Mode().IsRegular() {
		return none, refuseHint("output.not_regular", "Pass a regular file", "%s is not a regular file", path)
	}
	if !force {
		if err := CleanTree(path); err != nil {
			return none, err
		}
	}
	return Snapshot{info: info}, nil
}

// InPlace replaces path, which held original when it was analysed. before is
// what PreflightInPlace returned for path. It keeps the file's permission
// bits. force is unused here; it matches the unix API.
func InPlace(path string, before Snapshot, original, data []byte, _ bool) error {
	if before.info == nil {
		return errors.New("publish: InPlace needs the Snapshot from PreflightInPlace")
	}
	tmp, err := writeTemp(filepath.Dir(path), data, before.info.Mode().Perm())
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp) }()
	now, err := os.Lstat(path)
	if err != nil {
		return err
	}
	same := now.Mode().IsRegular() && os.SameFile(before.info, now)
	if same {
		if same, err = sameContent(path, original); err != nil {
			return err
		}
	}
	if !same {
		return refuseHint("output.changed", "Run the command again", "%s changed while circleci was running; nothing was written", path)
	}
	return os.Rename(tmp, path)
}

// writeTemp creates a temporary file in dir holding data, synced to disk.
func writeTemp(dir string, data []byte, perm fs.FileMode) (string, error) {
	f, err := createTemp(dir, perm)
	if err != nil {
		return "", err
	}
	name := f.Name()
	_, werr := f.Write(data)
	if err := errors.Join(werr, f.Sync(), f.Close()); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}
