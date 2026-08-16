// Package cas implements a Git-like content-addressable object
// store with commits, branches, refs, and verification.
package cas

import (
	"errors"

	"github.com/sachncs/promptsheon/errf"
)

// CreateBranch creates a new branch pointing at targetHash. If
// targetHash is empty, the new branch points at the current HEAD
// (or, when HEAD is detached, at the raw commit hash HEAD
// contains).
//
// CreateBranch refuses to overwrite an existing branch. Callers
// that want a force-create should delete the branch first; this
// matches Git's safety story.
//
// The ref write is performed under the repository-wide flock so
// concurrent CreateBranch calls cannot both succeed.
func CreateBranch(name, targetHash string) error {
	if !IsInitialized() {
		return ErrRepoNotInitialized
	}
	if err := validateBranchName(name); err != nil {
		return err
	}

	return withRepoLock(func() error {
		existing, err := ReadRef(name)
		if err != nil && !errors.Is(err, ErrRefNotFound) {
			return errf.Errorf("read ref: %w", err)
		}
		if existing != "" {
			return errf.Errorf("branch %q already exists", name)
		}

		hash := targetHash
		if hash == "" {
			current, err := CurrentCommitHash()
			if err != nil {
				return err
			}
			hash = current
		}
		if hash == "" {
			// Empty repo: create the branch with no commit hash yet.
			// The branch file will be empty; ListRefDetails reports it
			// as hash="" and the CLI renders it as "(no commit)".
			return WriteRef(name, "")
		}

		cleaned := sanitizeHash(hash)
		if err := validateHash(cleaned); err != nil {
			return errf.Errorf("target hash: %w", err)
		}
		obj, err := ReadObject(cleaned)
		if err != nil {
			return errf.Errorf("target object: %w", err)
		}
		// Verify the target is actually a commit so the branch points at
		// something usable. The current branch check (HEAD → commit) is
		// not enforced here — a detached HEAD's commit hash is valid.
		if !obj.IsCommit() {
			return errf.Errorf("target object is not a commit")
		}
		return WriteRef(name, cleaned)
	})
}

// DeleteBranch removes a branch. It refuses to delete the branch
// HEAD currently points at, mirroring Git's safety story: deleting
// the current branch would leave HEAD in an inconsistent state and
// is almost always a mistake.
//
// DeleteBranch runs under the repository-wide flock and performs
// the existence check and remove together to defeat the TOCTOU
// window that previously let a concurrent Checkout delete the
// branch the user had just checked out.
func DeleteBranch(name string) error {
	if !IsInitialized() {
		return ErrRepoNotInitialized
	}
	if err := validateBranchName(name); err != nil {
		return err
	}

	return withRepoLock(func() error {
		ref, _, err := readHEADRef()
		if err != nil {
			return err
		}
		if ref == name {
			return errf.Errorf("cannot delete the currently checked-out branch %q", name)
		}

		root, err := openRepoRoot()
		if err != nil {
			return err
		}
		defer func() { _ = root.Close() }()

		rel := branchRefPath(name)
		if err := root.Remove(rel); err != nil {
			if isNotExist(err) {
				return errf.Errorf("%w: %s", ErrRefNotFound, name)
			}
			return errf.Errorf("remove ref: %w", err)
		}
		logger().Debug("branch deleted", "name", name)
		return nil
	})
}

// Checkout moves HEAD to the given target. The target may be a
// branch name (which must exist) or a commit hash (raw or short
// form is not supported; the full 64-character hash is required).
//
// For a branch checkout, HEAD is rewritten to a symbolic ref. For
// a hash checkout, HEAD is rewritten to the raw hash (detached
// HEAD). Checkout refuses to leave HEAD pointing at a non-commit
// object.
//
// The whole operation runs under the repository-wide flock and
// does its own existence check so a concurrent DeleteBranch
// cannot leave HEAD dangling.
func Checkout(target string) error {
	if !IsInitialized() {
		return ErrRepoNotInitialized
	}

	return withRepoLock(func() error {
		// A 64-character lowercase hex string is unambiguously a
		// commit hash; anything else is treated as a branch name.
		cleaned := sanitizeHash(target)
		if hashPattern.MatchString(cleaned) {
			obj, err := ReadObject(cleaned)
			if err != nil {
				return errf.Errorf("checkout: target object: %w", err)
			}
			if !obj.IsCommit() {
				return errf.Errorf("checkout: target is not a commit object")
			}
			if err := WriteHEAD(cleaned); err != nil {
				return errf.Errorf("checkout: write HEAD: %w", err)
			}
			logger().Debug("checkout (detached)", "hash", shortHash(cleaned))
			return nil
		}

		// Otherwise, treat it as a branch name. The ref must exist
		// (even pointing at an empty commit) so that callers can
		// distinguish "branch does not exist" from "branch has no
		// commit yet".
		if err := validateBranchName(target); err != nil {
			return errf.Errorf("checkout: %w", err)
		}
		if _, err := ReadRef(target); err != nil {
			if errors.Is(err, ErrRefNotFound) {
				return errf.Errorf("%w: %s", ErrRefNotFound, target)
			}
			return errf.Errorf("checkout: read ref: %w", err)
		}
		if err := WriteHEAD("ref: refs/heads/" + target); err != nil {
			return errf.Errorf("checkout: write HEAD: %w", err)
		}
		logger().Debug("checkout (branch)", "branch", target)
		return nil
	})
}

// CurrentRef returns the branch name HEAD points at, or the empty string for
// a detached HEAD. The empty string is the canonical "no branch" indicator;
// callers should treat it as detached HEAD rather than as an error.
func CurrentRef() (string, error) {
	if !IsInitialized() {
		return "", ErrRepoNotInitialized
	}
	ref, _, err := readHEADRef()
	return ref, err
}

// RefDetail is one entry returned by ListRefDetails. The Hash is
// the commit the ref currently points at, or the empty string if
// the ref has no commit yet.
type RefDetail struct {
	Name string
	Hash string
}

// ListRefDetails returns one RefDetail per local branch. The
// branches are listed in directory order, which is stable on
// every platform because we read them via os.ReadDir.
func ListRefDetails() ([]RefDetail, error) {
	if !IsInitialized() {
		return nil, ErrRepoNotInitialized
	}
	names, err := ListRefs()
	if err != nil {
		return nil, err
	}
	out := make([]RefDetail, 0, len(names))
	for _, n := range names {
		hash, err := ReadRef(n)
		if err != nil {
			if errors.Is(err, ErrRefNotFound) {
				out = append(out, RefDetail{Name: n})
				continue
			}
			return nil, errf.Errorf("read ref %q: %w", n, err)
		}
		out = append(out, RefDetail{Name: n, Hash: hash})
	}
	return out, nil
}
