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

// Package preflight runs uncommitted local changes through CircleCI: it
// snapshots the working tree into a commit on a throwaway branch, pushes that
// branch so the project's push trigger starts a run, watches the run and
// reports failures as they happen, then deletes the branch.
package preflight

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	git "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/filemode"
	gitconfig "github.com/go-git/go-git/v6/plumbing/format/config"
	"github.com/go-git/go-git/v6/plumbing/format/gitignore"
	"github.com/go-git/go-git/v6/plumbing/format/index"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/storer"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/storage"
	"github.com/google/uuid"
)

// BranchPrefix is the namespace every preflight branch is pushed under. The
// UUID that follows it identifies one preflight, so cleanup can target it.
const BranchPrefix = "cci/preflight/"

// DefaultRemote is the remote preflight branches are pushed to.
const DefaultRemote = "origin"

// fallbackIdent is used for the snapshot commit when the repository has no
// user.name / user.email configured, so a fresh clone in CI or a container can
// still run a preflight. The commit never lands on a real branch.
const (
	fallbackName  = "CircleCI Preflight"
	fallbackEmail = "preflight@circleci.invalid"
)

// BranchName returns the remote branch name for the preflight with the given ID.
func BranchName(id uuid.UUID) string {
	return BranchPrefix + id.String()
}

// ParseBranchName extracts the preflight ID from a branch name produced by
// BranchName. ok is false for any other branch.
func ParseBranchName(name string) (id uuid.UUID, ok bool) {
	rest, found := strings.CutPrefix(name, BranchPrefix)
	if !found {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(rest)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// GitError is a failed git operation. Op names what was being done ("push to
// origin", "snapshot the working tree"); Err is go-git's explanation.
type GitError struct {
	Op  string
	Err error
}

func (e *GitError) Error() string {
	return fmt.Sprintf("could not %s: %s", e.Op, e.Err)
}

func (e *GitError) Unwrap() error { return e.Err }

// ErrNoCommits is returned by Snapshot when HEAD does not point at a commit
// yet. The snapshot is a child of HEAD, so a repository with no history has
// nothing to build it on.
var ErrNoCommits = errors.New("the repository has no commits yet")

// ErrRemoteAuth wraps a remote operation that failed to authenticate. go-git
// authenticates SSH remotes through ssh-agent and does not run git credential
// helpers, so an HTTPS remote that relies on one cannot be pushed to.
var ErrRemoteAuth = errors.New("authentication with the remote failed")

// Repo is a git checkout preflight operates on.
type Repo struct {
	// Dir is the top level of the working tree.
	Dir string
	// Remote is the remote branches are pushed to and deleted from.
	Remote string

	repo *git.Repository
}

// OpenRepo returns the repository containing dir, pushing to remote. Callers
// must Close it to release its file handles.
func OpenRepo(dir, remote string) (*Repo, error) {
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	if errors.Is(err, git.ErrRepositoryNotExists) {
		return nil, fmt.Errorf("%s is not inside a git repository", dir)
	}
	if err != nil {
		return nil, &GitError{Op: "open the repository", Err: err}
	}
	wt, err := repo.Worktree()
	if err != nil {
		_ = repo.Close()
		return nil, err
	}
	return &Repo{Dir: wt.Filesystem().Root(), Remote: remote, repo: repo}, nil
}

// Close releases the repository's file handles.
func (r *Repo) Close() error {
	return r.repo.Close()
}

// Snapshot is the commit a preflight pushes.
type Snapshot struct {
	// Commit is the SHA to push.
	Commit string
	// Base is Commit's parent: where HEAD forked from the remote's default
	// branch, or HEAD itself when that cannot be determined.
	Base string
	// BaseBranch is the default branch Base was found on, or "" when Base
	// fell back to HEAD.
	BaseBranch string
	// Changed is false when the working tree matched Base, in which case
	// Commit is Base itself.
	Changed bool
}

// Snapshot records the whole working tree — staged, unstaged and untracked
// files, minus anything .gitignore excludes — as one commit.
//
// The commit's parent is where HEAD forked from the remote's default branch,
// not HEAD, so its diff shows everything being changed: the branch's own
// commits and the uncommitted work together. The tree built is the working tree
// either way. When the working tree matches the base there is nothing to show,
// and the base itself is returned rather than an empty commit.
//
// Nothing the user can see changes: the working tree is staged into an
// in-memory copy of the index, and the snapshot's objects are written without
// moving any ref, so HEAD, the real index, the working tree and the stash are
// all left as they were.
func (r *Repo) Snapshot(ctx context.Context, message string) (Snapshot, error) {
	headRef, err := r.repo.Head()
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return Snapshot{}, ErrNoCommits
	}
	if err != nil {
		return Snapshot{}, &GitError{Op: "read HEAD", Err: err}
	}
	head, err := r.repo.CommitObject(headRef.Hash())
	if err != nil {
		return Snapshot{}, &GitError{Op: "read HEAD", Err: err}
	}

	tree, err := r.workingTree()
	if err != nil {
		return Snapshot{}, &GitError{Op: "snapshot the working tree", Err: err}
	}

	base, snap := head, Snapshot{Base: head.Hash.String()}
	if fork, branch, ok := r.forkPoint(ctx, head); ok {
		base, snap.Base, snap.BaseBranch = fork, fork.Hash.String(), branch
	}
	if tree == base.TreeHash {
		snap.Commit = snap.Base
		return snap, nil
	}

	commit, err := r.writeCommit(tree, base.Hash, message)
	if err != nil {
		return Snapshot{}, &GitError{Op: "write the snapshot commit", Err: err}
	}
	snap.Commit = commit.String()
	snap.Changed = true
	return snap, nil
}

// workingTree stages the whole working tree into a copy of the index and writes
// it out as a tree, returning the tree's hash. Starting from the real index,
// rather than HEAD's tree, keeps its stat cache, so only files that actually
// changed are re-hashed.
func (r *Repo) workingTree() (plumbing.Hash, error) {
	idx, err := r.repo.Storer.Index()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	scratch := &scratchIndex{Storer: r.repo.Storer, idx: idx}

	wt, err := r.repo.Worktree()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	view, err := git.Open(scratch, wt.Filesystem())
	if err != nil {
		return plumbing.ZeroHash, err
	}
	vwt, err := view.Worktree()
	if err != nil {
		return plumbing.ZeroHash, err
	}
	vwt.Excludes = r.userExcludes()
	if err := vwt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return plumbing.ZeroHash, err
	}
	return writeTree(r.repo.Storer, scratch.idx)
}

// scratchIndex is the repository's storage with the index held in memory: what
// go-git stages through it reaches the object database but never the index
// file the user's own git commands read.
type scratchIndex struct {
	storage.Storer
	idx *index.Index
}

func (s *scratchIndex) Index() (*index.Index, error) { return s.idx, nil }

func (s *scratchIndex) SetIndex(idx *index.Index) error {
	s.idx = idx
	return nil
}

// userExcludes returns the patterns in git's core.excludesFile: the last value
// set across the system, global and repository config or, when none sets it,
// $XDG_CONFIG_HOME/git/ignore (default ~/.config/git/ignore). go-git reads
// .gitignore files and .git/info/exclude itself, but its own lookup of this
// setting reads only ~/.gitconfig and /etc/gitconfig and has no default.
// Unreadable config or ignore files mean no patterns.
func (r *Repo) userExcludes() []gitignore.Pattern {
	file := ""
	for _, p := range globalConfigPaths() {
		if v := excludesFileIn(p); v != "" {
			file = v
		}
	}
	if local, err := r.repo.Config(); err == nil && local.Raw != nil {
		if v := local.Raw.Section("core").Options.Get("excludesfile"); v != "" {
			file = v
		}
	}
	if file == "" {
		file = filepath.Join(xdgConfigHome(), "git", "ignore")
	}
	return readExcludes(expandHome(file))
}

// globalConfigPaths lists the config files git reads before the repository's
// own, lowest precedence first, honouring GIT_CONFIG_SYSTEM, GIT_CONFIG_NOSYSTEM
// and GIT_CONFIG_GLOBAL.
func globalConfigPaths() []string {
	var paths []string
	if os.Getenv("GIT_CONFIG_NOSYSTEM") == "" {
		if p := os.Getenv("GIT_CONFIG_SYSTEM"); p != "" {
			paths = append(paths, p)
		} else {
			paths = append(paths, "/etc/gitconfig")
		}
	}
	if p := os.Getenv("GIT_CONFIG_GLOBAL"); p != "" {
		return append(paths, p)
	}
	paths = append(paths, filepath.Join(xdgConfigHome(), "git", "config"))
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".gitconfig"))
	}
	return paths
}

// excludesFileIn returns core.excludesFile from the config file at path, or ""
// when it is unset or the file cannot be read.
func excludesFileIn(path string) string {
	f, err := os.Open(path) //#nosec:G304 // path is one of git's own config file locations
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	raw := gitconfig.New()
	if err := gitconfig.NewDecoder(f).Decode(raw); err != nil {
		return ""
	}
	return raw.Section("core").Options.Get("excludesfile")
}

// readExcludes parses the ignore file at path. Its patterns apply from the
// repository root, as git applies them.
func readExcludes(path string) []gitignore.Pattern {
	f, err := os.Open(path) //#nosec:G304 // path is the user's own core.excludesFile
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	var ps []gitignore.Pattern
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "#") && strings.TrimSpace(line) != "" {
			ps = append(ps, gitignore.ParsePattern(line, nil))
		}
	}
	return ps
}

func xdgConfigHome() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

// expandHome expands a leading ~/ to the home directory, as git does for
// core.excludesFile.
func expandHome(path string) string {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	return path
}

// writeTree writes idx out as tree objects and returns the root tree's hash, as
// git write-tree does.
func writeTree(s storer.EncodedObjectStorer, idx *index.Index) (plumbing.Hash, error) {
	merged := make(map[string]bool, len(idx.Entries))
	for _, e := range idx.Entries {
		if e.Stage == 0 {
			merged[e.Name] = true
		}
	}
	root := &treeNode{}
	for _, e := range idx.Entries {
		if e.Stage != 0 {
			if merged[e.Name] {
				continue
			}
			return plumbing.ZeroHash, fmt.Errorf("%s has unresolved merge conflicts", e.Name)
		}
		if e.Hash.IsZero() {
			continue
		}
		dir, name := path.Split(e.Name)
		node := root
		for part := range strings.SplitSeq(strings.TrimSuffix(dir, "/"), "/") {
			if part != "" {
				node = node.dir(part)
			}
		}
		node.entries = append(node.entries, object.TreeEntry{Name: name, Mode: e.Mode, Hash: e.Hash})
	}
	return root.write(s)
}

// treeNode is one directory of a tree being written.
type treeNode struct {
	entries []object.TreeEntry
	dirs    map[string]*treeNode
}

func (n *treeNode) dir(name string) *treeNode {
	if n.dirs == nil {
		n.dirs = map[string]*treeNode{}
	}
	d, ok := n.dirs[name]
	if !ok {
		d = &treeNode{}
		n.dirs[name] = d
	}
	return d
}

func (n *treeNode) write(s storer.EncodedObjectStorer) (plumbing.Hash, error) {
	entries := n.entries
	for name, d := range n.dirs {
		h, err := d.write(s)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		entries = append(entries, object.TreeEntry{Name: name, Mode: filemode.Dir, Hash: h})
	}
	// git orders tree entries by name, comparing a directory as if its name
	// ended in "/".
	sortName := func(e object.TreeEntry) string {
		if e.Mode == filemode.Dir {
			return e.Name + "/"
		}
		return e.Name
	}
	sort.Slice(entries, func(i, j int) bool { return sortName(entries[i]) < sortName(entries[j]) })
	return storeObject(s, &object.Tree{Entries: entries})
}

// writeCommit writes a commit of tree with the given parent and message.
func (r *Repo) writeCommit(tree, parent plumbing.Hash, message string) (plumbing.Hash, error) {
	ident := r.identity()
	return storeObject(r.repo.Storer, &object.Commit{
		Author:       ident,
		Committer:    ident,
		Message:      message,
		TreeHash:     tree,
		ParentHashes: []plumbing.Hash{parent},
	})
}

// identity is the author and committer of the snapshot commit: the user's
// configured identity, or a generic one when git has none configured.
func (r *Repo) identity() object.Signature {
	sig := object.Signature{Name: fallbackName, Email: fallbackEmail, When: time.Now()}
	cfg, err := r.repo.ConfigScoped(config.SystemScope)
	if err != nil {
		return sig
	}
	switch {
	case cfg.Committer.Name != "" && cfg.Committer.Email != "":
		sig.Name, sig.Email = cfg.Committer.Name, cfg.Committer.Email
	case cfg.User.Name != "" && cfg.User.Email != "":
		sig.Name, sig.Email = cfg.User.Name, cfg.User.Email
	}
	return sig
}

type encodable interface {
	Encode(plumbing.EncodedObject) error
}

func storeObject(s storer.EncodedObjectStorer, o encodable) (plumbing.Hash, error) {
	obj := s.NewEncodedObject()
	if err := o.Encode(obj); err != nil {
		return plumbing.ZeroHash, err
	}
	return s.SetEncodedObject(obj)
}

// forkPoint returns where head forked from the remote's default branch, and
// that branch's name. ok is false when the default branch is unknown or has no
// remote-tracking ref locally; the caller then parents the snapshot on HEAD.
// The tracking ref is used as last fetched, never refreshed: a stale one still
// gives an ancestor of HEAD, just an older one.
func (r *Repo) forkPoint(ctx context.Context, head *object.Commit) (fork *object.Commit, branch string, ok bool) {
	branch = r.defaultBranch(ctx)
	if branch == "" {
		return nil, "", false
	}
	ref, err := r.repo.Reference(plumbing.NewRemoteReferenceName(r.Remote, branch), true)
	if err != nil {
		return nil, "", false
	}
	tip, err := r.repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, "", false
	}
	bases, err := head.MergeBase(tip)
	if err != nil || len(bases) == 0 {
		return nil, "", false
	}
	return bases[0], branch, true
}

// defaultBranch returns the remote's default branch: from the local
// <remote>/HEAD when it is set (as clone sets it), otherwise by asking the
// remote. It returns "" when neither works.
func (r *Repo) defaultBranch(ctx context.Context) string {
	prefix := "refs/remotes/" + r.Remote + "/"
	if ref, err := r.repo.Reference(plumbing.NewRemoteHEADReferenceName(r.Remote), false); err == nil {
		if name, ok := strings.CutPrefix(ref.Target().String(), prefix); ok {
			return name
		}
	}
	refs, err := r.listRemote(ctx)
	if err != nil {
		return ""
	}
	for _, ref := range refs {
		if ref.Name() == plumbing.HEAD && ref.Type() == plumbing.SymbolicReference && ref.Target().IsBranch() {
			return ref.Target().Short()
		}
	}
	return ""
}

// Push pushes sha to branch on the remote. Hooks are never run: go-git does not
// run them, and the point of a preflight is to let CI judge work in progress
// that a pre-push hook might reject.
func (r *Repo) Push(ctx context.Context, sha, branch string) error {
	return r.push(ctx, "push to "+r.Remote, config.RefSpec(sha+":"+plumbing.NewBranchReferenceName(branch).String()))
}

// DeleteBranch deletes branch from the remote.
func (r *Repo) DeleteBranch(ctx context.Context, branch string) error {
	return r.push(ctx, "delete "+branch+" from "+r.Remote, config.RefSpec(":"+plumbing.NewBranchReferenceName(branch).String()))
}

func (r *Repo) push(ctx context.Context, op string, spec config.RefSpec) error {
	err := r.repo.PushContext(ctx, &git.PushOptions{
		RemoteName: r.Remote,
		RefSpecs:   []config.RefSpec{spec},
		Quiet:      true,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return remoteErr(op, err)
	}
	return nil
}

// RemoteBranch is a preflight branch that exists on the remote.
type RemoteBranch struct {
	ID     uuid.UUID
	Name   string
	Commit string
}

// ListBranches returns every preflight branch on the remote. Branches under
// BranchPrefix whose name is not a UUID were not made by preflight and are
// left out.
func (r *Repo) ListBranches(ctx context.Context) ([]RemoteBranch, error) {
	refs, err := r.listRemote(ctx)
	if err != nil {
		return nil, err
	}
	var branches []RemoteBranch
	for _, ref := range refs {
		if !ref.Name().IsBranch() || ref.Type() != plumbing.HashReference {
			continue
		}
		name := ref.Name().Short()
		id, ok := ParseBranchName(name)
		if !ok {
			continue
		}
		branches = append(branches, RemoteBranch{ID: id, Name: name, Commit: ref.Hash().String()})
	}
	sort.Slice(branches, func(i, j int) bool { return branches[i].Name < branches[j].Name })
	return branches, nil
}

// CommitTime returns when the commit sha was made, and false when it is not in
// the local repository — as for a preflight pushed from another clone.
func (r *Repo) CommitTime(sha string) (time.Time, bool) {
	c, err := r.repo.CommitObject(plumbing.NewHash(sha))
	if err != nil {
		return time.Time{}, false
	}
	return c.Committer.When, true
}

// listRemote returns the references the remote advertises, as git ls-remote
// does.
func (r *Repo) listRemote(ctx context.Context) ([]*plumbing.Reference, error) {
	op := "list the branches on " + r.Remote
	remote, err := r.repo.Remote(r.Remote)
	if err != nil {
		return nil, &GitError{Op: op, Err: err}
	}
	refs, err := remote.ListContext(ctx, &git.ListOptions{})
	if err != nil {
		return nil, remoteErr(op, err)
	}
	return refs, nil
}

// remoteErr wraps a failed remote operation, marking authentication failures
// with ErrRemoteAuth. HTTP transports report those as typed errors; SSH ones
// only as text from golang.org/x/crypto/ssh or go-git's agent lookup.
func remoteErr(op string, err error) error {
	msg := err.Error()
	if errors.Is(err, transport.ErrAuthenticationRequired) ||
		errors.Is(err, transport.ErrAuthorizationFailed) ||
		strings.Contains(msg, "unable to authenticate") ||
		strings.Contains(msg, "SSH_AUTH_SOCK") {
		err = fmt.Errorf("%w: %w", ErrRemoteAuth, err)
	}
	return &GitError{Op: op, Err: err}
}
