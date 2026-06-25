package promptsheon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CreateBranch creates a new branch pointing at targetHash. If
// targetHash is empty, the new branch points at the current HEAD
// (or, when HEAD is detached, at the raw commit hash HEAD
// contains).
//
// CreateBranch refuses to overwrite an existing branch. Callers
// that want a force-create should delete the branch first; this
// matches Git's safety story.
func CreateBranch(name, targetHash string) error {
	if !IsInitialized() {
		return ErrRepoNotInitialized
	}
	if err := validateBranchName(name); err != nil {
		return err
	}
	if name == headFile || strings.Contains(name, "..") {
		return fmt.Errorf("invalid branch name %q", name)
	}

	// Refuse to create a branch with the same name as an existing
	// one. Git does this; doing the same keeps scripts that
	// pre-check branch presence from being surprised.
	existing, err := ReadRef(name)
	if err != nil && !errors.Is(err, ErrRefNotFound) {
		return fmt.Errorf("read ref: %w", err)
	}
	if existing != "" {
		return fmt.Errorf("branch %q already exists", name)
	}

	hash := targetHash
	if hash == "" {
		// Default to current HEAD.
		current, err := GetCurrentCommitHash()
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
		return fmt.Errorf("target hash: %w", err)
	}
	if _, err := ReadObject(cleaned); err != nil {
		return fmt.Errorf("target object: %w", err)
	}
	return WriteRef(name, cleaned)
}

// DeleteBranch removes a branch. It refuses to delete the branch
// HEAD currently points at, mirroring Git's safety story: deleting
// the current branch would leave HEAD in an inconsistent state and
// is almost always a mistake.
func DeleteBranch(name string) error {
	if !IsInitialized() {
		return ErrRepoNotInitialized
	}
	if err := validateBranchName(name); err != nil {
		return err
	}
	ref, _, err := readHEADRef()
	if err != nil {
		return err
	}
	if ref == name {
		return fmt.Errorf("cannot delete the currently checked-out branch %q", name)
	}
	refPath := filepath.Join(PromptsheonDir, headsDir, name)
	if _, err := os.Stat(refPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrRefNotFound, name)
		}
		return fmt.Errorf("stat ref: %w", err)
	}
	if err := os.Remove(refPath); err != nil {
		return fmt.Errorf("remove ref: %w", err)
	}
	logger.Debug("branch deleted", "name", name)
	return nil
}

// Checkout moves HEAD to the given target. The target may be a
// branch name (which must exist) or a commit hash (raw or short
// form is not supported; the full 64-character hash is required).
//
// For a branch checkout, HEAD is rewritten to a symbolic ref. For
// a hash checkout, HEAD is rewritten to the raw hash (detached
// HEAD).
func Checkout(target string) error {
	if !IsInitialized() {
		return ErrRepoNotInitialized
	}

	// A 64-character lowercase hex string is unambiguously a
	// commit hash; anything else is treated as a branch name.
	cleaned := sanitizeHash(target)
	if hashPattern.MatchString(cleaned) {
		if _, err := ReadObject(cleaned); err != nil {
			return fmt.Errorf("checkout: target object: %w", err)
		}
		if err := WriteHEAD(cleaned); err != nil {
			return fmt.Errorf("checkout: write HEAD: %w", err)
		}
		logger.Debug("checkout (detached)", "hash", shortHash(cleaned))
		return nil
	}

	// Otherwise, treat it as a branch name. The ref must exist
	// (even pointing at an empty commit) so that callers can
	// distinguish "branch does not exist" from "branch has no
	// commit yet".
	if err := validateBranchName(target); err != nil {
		return fmt.Errorf("checkout: %w", err)
	}
	if _, err := ReadRef(target); err != nil {
		if errors.Is(err, ErrRefNotFound) {
			return fmt.Errorf("%w: %s", ErrRefNotFound, target)
		}
		return fmt.Errorf("checkout: read ref: %w", err)
	}
	if err := WriteHEAD("ref: refs/heads/" + target); err != nil {
		return fmt.Errorf("checkout: write HEAD: %w", err)
	}
	logger.Debug("checkout (branch)", "branch", target)
	return nil
}

// GetCurrentRef returns the branch name HEAD points at, or the
// empty string for a detached HEAD. The empty string is the
// canonical "no branch" indicator; callers should treat it as
// detached HEAD rather than as an error.
func GetCurrentRef() (string, error) {
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
func ListRefDetails() ([]*RefDetail, error) {
	if !IsInitialized() {
		return nil, ErrRepoNotInitialized
	}
	names, err := ListRefs()
	if err != nil {
		return nil, err
	}
	out := make([]*RefDetail, 0, len(names))
	for _, n := range names {
		hash, err := ReadRef(n)
		if err != nil {
			if errors.Is(err, ErrRefNotFound) {
				// The ref disappeared between the directory
				// listing and the file read; treat it as empty.
				out = append(out, &RefDetail{Name: n})
				continue
			}
			return nil, fmt.Errorf("read ref %q: %w", n, err)
		}
		out = append(out, &RefDetail{Name: n, Hash: hash})
	}
	return out, nil
}
