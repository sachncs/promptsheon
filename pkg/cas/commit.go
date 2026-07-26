package cas

import (
	"errors"
	"fmt"
)

// CommitResult is the value returned by Commit. The Hash is the
// SHA-256 of the newly written commit object. The Ref is the name
// of the branch that was updated, or the empty string for a
// detached HEAD.
type CommitResult struct {
	Hash string
	Ref  string
}

// Commit writes a new commit object to the CAS, links it to its
// parents, and advances the current branch (or leaves HEAD
// detached if HEAD is detached). The arguments mirror the fields
// of a commit object; see NewCommitObject for the canonicalisation
// rules.
//
// Commit is the single mutation point for the commit DAG: every
// other operation that wants to record a state change goes through
// here so HEAD/ref updates stay atomic. The commit write, ref/HEAD
// write, and rename are all performed under a repository-wide
// flock so concurrent writers cannot corrupt each other or leave
// orphan commits.
func Commit(treeHash string, parents []string, author, message string, telemetry map[string]any) (*CommitResult, error) {
	if !IsInitialized() {
		return nil, ErrRepoNotInitialized
	}

	// Validate the tree hash: a malformed or missing tree makes the
	// resulting commit impossible to interpret, and the CAS layer
	// would happily write a commit that nothing can reach.
	cleanedTree := sanitizeHash(treeHash)
	if err := validateHash(cleanedTree); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	if _, err := ReadObject(cleanedTree); err != nil {
		return nil, fmt.Errorf("commit: tree object: %w", err)
	}

	// Validate every parent hash. A missing parent is a serious
	// error: a commit with a dangling parent is unreachable from
	// any ref and would only show up as a Verify failure.
	cleanedParents := make([]string, 0, len(parents))
	for _, p := range parents {
		cleaned := sanitizeHash(p)
		if err := validateHash(cleaned); err != nil {
			return nil, fmt.Errorf("commit: parent %q: %w", p, err)
		}
		if _, err := ReadObject(cleaned); err != nil {
			return nil, fmt.Errorf("commit: parent object: %w", err)
		}
		cleanedParents = append(cleanedParents, cleaned)
	}

	obj := NewCommitObject(cleanedTree, cleanedParents, author, message, telemetry)
	newHash, err := canonicalHash(obj)
	if err != nil {
		return nil, fmt.Errorf("commit: hash: %w", err)
	}

	var result CommitResult

	err = withRepoLock(func() error {
		hash, werr := WriteObject(obj)
		if werr != nil {
			return fmt.Errorf("commit: write: %w", werr)
		}
		result.Hash = hash

		// Update the ref HEAD points at (if any). If HEAD is detached,
		// the ref name stays empty and the caller is expected to manage
		// reachability some other way.
		ref, _, err := readHEADRef()
		if err != nil {
			return err
		}
		if ref != "" {
			if werr := WriteRef(ref, hash); werr != nil {
				return fmt.Errorf("commit: update ref: %w", werr)
			}
			result.Ref = ref
		} else {
			// Detached HEAD: rewrite HEAD with the new commit hash so
			// the working state is consistent.
			if werr := WriteHEAD(hash); werr != nil {
				return fmt.Errorf("commit: update HEAD: %w", werr)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	logger().Debug("commit created", "hash", shortHash(newHash), "ref", result.Ref)
	return &result, nil
}

// CurrentCommitHash returns the hash of the commit HEAD points at. The
// returned string is empty (without error) when the repository has no commits
// yet, matching how the CLI renders "no commits yet".
func CurrentCommitHash() (string, error) {
	if !IsInitialized() {
		return "", ErrRepoNotInitialized
	}
	content, err := ReadHEAD()
	if err != nil {
		return "", err
	}
	if IsHEADDetached(content) {
		return sanitizeHash(content), nil
	}
	ref := HEADRefName(content)
	if ref == "" {
		return "", nil
	}
	hash, err := ReadRef(ref)
	if err != nil {
		// A ref that points at a missing commit hash is the same
		// as "no commits yet" from the caller's perspective.
		if errors.Is(err, ErrRefNotFound) {
			return "", nil
		}
		return "", err
	}
	return hash, nil
}

// GetCurrentCommitHash is a deprecated alias for CurrentCommitHash. New
// callers should use CurrentCommitHash; this wrapper remains so existing
// consumers keep compiling during the migration window.
func GetCurrentCommitHash() (string, error) { return CurrentCommitHash() }

// readHEADRef resolves HEAD to a (ref, head) pair. ref is the branch name HEAD
// points at, or the empty string for a detached HEAD. head is the raw HEAD
// file content (a ref: line or a raw hash). Both are useful for error
// messages and audit logging.
func readHEADRef() (string, string, error) {
	head, err := ReadHEAD()
	if err != nil {
		return "", "", fmt.Errorf("read HEAD: %w", err)
	}
	return HEADRefName(head), head, nil
}
