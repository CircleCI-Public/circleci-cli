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

//go:build unix

package publish

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func notDurable(path string, err error) error {
	return &IncompleteError{Path: path, Code: "output.not_durable",
		Err: fmt.Errorf("the directory could not be synced, so the write may not survive a crash: %w", err)}
}

// preservedBits are the mode bits a replacement keeps.
const preservedBits = fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky

// ToFile writes data to a new file at path. inputPath is the analysed
// config ("" for stdin); -o may not name it, since replacing the input
// is --in-place's job with its own checks. The mode is 0644, narrowed by the
// input's mode and by the umask.
func ToFile(path string, data []byte, inputPath string, force bool) error {
	if err := PreflightToFile(path, inputPath, force); err != nil {
		return err
	}
	mode := defaultMode
	if inputPath != "" {
		in, err := os.Stat(inputPath)
		if err != nil {
			return err
		}
		mode &= in.Mode().Perm()
	}
	dir := filepath.Dir(path)
	tmp, err := writeTemp(dir, data, temp{perm: mode, gid: -1})
	if err != nil {
		return err
	}
	if force {
		err = os.Rename(tmp, path)
	} else {
		// A hard link fails if path exists, so the publish cannot clobber a
		// file created after the check above.
		err = os.Link(tmp, path)
		if errors.Is(err, fs.ErrExist) {
			_ = os.Remove(tmp)
			return refuseHint("output.exists", "Pass --force to overwrite it", "%s already exists", path)
		}
		if err == nil {
			// The output is published: from here a failure is incomplete,
			// not unwritten. One name only, before the directory sync.
			if rerr := os.Remove(tmp); rerr != nil {
				return &IncompleteError{Path: path, Code: "output.temp_left",
					Err: errors.Join(fmt.Errorf("the temporary file %s could not be removed: %w", tmp, rerr), syncDir(dir))}
			}
		}
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := syncDir(dir); err != nil {
		return notDurable(path, err)
	}
	return nil
}

// Snapshot is what PreflightInPlace saw of the file. InPlace takes it, so the
// refusals run once, and re-verifies only that the file still matches it.
type Snapshot struct {
	info  fs.FileInfo
	stat  *syscall.Stat_t
	attrs []string
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
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return none, refuseHint("output.unsupported", "Write to a new file with -o FILE", "cannot read the file's ownership on this platform")
	}
	if st.Nlink > 1 {
		return none, refuseHint("output.hardlink", "Write to a new file with -o FILE", "%s has other hard links, which a replacement would split", path)
	}
	attrs, err := extendedAttributes(path)
	if err != nil {
		return none, refuseHint("output.xattr", "Pass --force to accept", "cannot read %s's extended attributes (%v), so a replacement could drop them", path, err)
	}
	if !force {
		if int(st.Uid) != os.Geteuid() {
			return none, refuseHint("output.owner", "Pass --force to accept", "%s is owned by another user, which a replacement would change", path)
		}
		if len(attrs) > 0 {
			return none, refuseHint("output.xattr", "Pass --force to accept", "%s has extended attributes (%s) that a replacement would drop",
				path, strings.Join(attrs, ", "))
		}
		other, err := hiddenMetadata(path, st)
		if err != nil {
			return none, refuseHint("output.metadata", "Pass --force to accept", "cannot read %s's ACL or file flags (%v), so a replacement could drop them", path, err)
		}
		if len(other) > 0 {
			return none, refuseHint("output.metadata", "Pass --force to accept", "%s has %s that a replacement would drop",
				path, strings.Join(other, " and "))
		}
		if err := CleanTree(path); err != nil {
			return none, err
		}
	}
	return Snapshot{info: info, stat: st, attrs: attrs}, nil
}

// InPlace replaces path, which held original when it was analysed. before is
// what PreflightInPlace returned for path. The replacement keeps the file's
// mode bits and group; --force lets it take a different group if the old one
// cannot be kept.
func InPlace(path string, before Snapshot, original, data []byte, force bool) error {
	if before.info == nil {
		return errors.New("publish: InPlace needs the Snapshot from PreflightInPlace")
	}
	dir := filepath.Dir(path)
	// The group is set explicitly: a setgid directory, or BSD semantics,
	// would give the new file the directory's group. The mode is exact, not
	// narrowed by the umask.
	want := temp{perm: 0o600, exact: true, mode: before.info.Mode() & preservedBits, gid: int(before.stat.Gid)}
	tmp, err := writeTemp(dir, data, want)
	if errors.Is(err, errGroup) && force {
		want.gid = -1
		tmp, err = writeTemp(dir, data, want)
	}
	if errors.Is(err, errGroup) {
		return refuseHint("output.group", "Pass --force to accept a different group", "cannot keep %s's group (%v)", path, err)
	}
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp) }()
	// The last check before replacing: the file must still be the one that
	// was analysed, with the same links, owner and attributes. --force never
	// skips it.
	if err := unchanged(path, before, original); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	if err := syncDir(dir); err != nil {
		return notDurable(path, err)
	}
	return nil
}

func unchanged(path string, before Snapshot, original []byte) error {
	now, err := os.Lstat(path)
	if err != nil {
		return err
	}
	st, ok := now.Sys().(*syscall.Stat_t)
	attrs, attrErr := extendedAttributes(path)
	current, err := os.ReadFile(path) //#nosec:G304 // the user's config
	if err != nil {
		return err
	}
	if !ok || attrErr != nil || !os.SameFile(before.info, now) || now.Size() != before.info.Size() ||
		now.Mode() != before.info.Mode() || st.Nlink != 1 || st.Uid != before.stat.Uid ||
		st.Gid != before.stat.Gid || !slices.Equal(attrs, before.attrs) ||
		sha256.Sum256(current) != sha256.Sum256(original) {
		return refuseHint("output.changed", "Run the command again", "%s changed while circleci was running; nothing was written", path)
	}
	return nil
}

// errGroup marks a temp file whose group could not be set.
var errGroup = errors.New("cannot set the group")

// temp says how writeTemp creates the new file.
type temp struct {
	perm  fs.FileMode // creation mode, narrowed by the umask
	exact bool        // set mode afterwards, regardless of the umask
	mode  fs.FileMode
	gid   int // -1 keeps the default group
}

// writeTemp creates the new file in dir: data, then its group, then its
// exact mode if asked (a chown clears setuid and setgid, so the mode comes
// after), then an fsync, so all of it is durable before it is published.
// Creating with perm lets the kernel apply the umask, which the process
// cannot read without changing it.
func writeTemp(dir string, data []byte, t temp) (string, error) {
	f, err := createTemp(dir, t.perm)
	if err != nil {
		return "", err
	}
	name := f.Name()
	_, werr := f.Write(data)
	var gerr, cerr error
	if t.gid >= 0 {
		if err := f.Chown(-1, t.gid); err != nil {
			gerr = fmt.Errorf("%w: %w", errGroup, err)
		}
	}
	if t.exact {
		cerr = f.Chmod(t.mode)
	}
	serr := f.Sync()
	if err := errors.Join(werr, gerr, cerr, serr, f.Close()); err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

func syncDir(dir string) error {
	d, err := os.Open(dir) //#nosec:G304 // the output's own directory
	if err != nil {
		return err
	}
	return errors.Join(d.Sync(), d.Close())
}

// systemAttributes are attributes macOS sets on ordinary files by itself;
// they are not user metadata. Quarantine is not among them: it is security
// metadata a replacement must not silently drop.
var systemAttributes = []string{"com.apple.provenance"}

// extendedAttributes lists the file's user extended attributes. An error is
// returned, not treated as "none", so a caller never drops attributes it
// could not read.
func extendedAttributes(path string) ([]string, error) {
	size, err := unix.Listxattr(path, nil)
	if errors.Is(err, unix.ENOTSUP) {
		return nil, nil // the filesystem has no extended attributes
	}
	if err != nil {
		return nil, err
	}
	if size <= 0 {
		return nil, nil
	}
	buf := make([]byte, size)
	n, err := unix.Listxattr(path, buf)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, name := range strings.Split(strings.TrimRight(string(buf[:n]), "\x00"), "\x00") {
		if name != "" && !slices.Contains(systemAttributes, name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names, nil
}
