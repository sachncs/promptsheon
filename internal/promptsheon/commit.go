package promptsheon

import (
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
// here so HEAD/ref updates stay atomic.
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
	hash, err := WriteObject(obj)
	if err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// Update the ref HEAD points at (if any). If HEAD is detached,
	// the ref name stays empty and the caller is expected to manage
	// reachability some other way.
	ref, head, err := readHEADRef()
	if err != nil {
		return nil, err
	}
	if ref != "" {
		if err := WriteRef(ref, hash); err != nil {
			return nil, fmt.Errorf("commit: update ref: %w", err)
		}
	} else {
		// Detached HEAD: rewrite HEAD with the new commit hash so
		// the working state is consistent.
		if err := WriteHEAD(hash); err != nil {
			return nil, fmt.Errorf("commit: update HEAD: %w", err)
		}
	}
	logger.Debug("commit created", "hash", shortHash(hash), "ref", ref, "head", head)

	return &CommitResult{Hash: hash, Ref: ref}, nil
}

// GetCurrentCommitHash returns the hash of the commit HEAD points
// at. The returned string is empty (without error) when the
// repository has no commits yet, matching how the CLI renders
// "no commits yet".
func GetCurrentCommitHash() (string, error) {
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
		if errIs(err, ErrRefNotFound) {
			return "", nil
		}
		return "", err
	}
	return hash, nil
}

// errIs is a tiny wrapper that returns true when target is in err's
// chain. We avoid importing "errors" here so the commit module
// stays import-light; the rest of the package uses errors.Is
// directly.
func errIs(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}

// readHEADRef resolves HEAD to a (ref, head) pair. ref is the
// branch name HEAD points at, or the empty string for a detached
// HEAD. head is the raw HEAD file content (a ref: line or a raw
// hash). Both are useful for error messages.
func readHEADRef() (string, string, error) {
	head, err := ReadHEAD()
	if err != nil {
		return "", "", fmt.Errorf("read HEAD: %w", err)
	}
	return HEADRefName(head), head, nil
}
