package cas

import (
	"fmt"

	"regexp"
	"strings"
	"unicode"
)

// hashPattern matches a 64-character lowercase hex SHA-256 hash.
var hashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// sanitizeHash normalises a user-supplied hash. It lowercases the
// input, strips any surrounding whitespace, and returns the
// cleaned string. Validation (and the wrapped ErrInvalidHash) is
// performed by validateHash — callers that want to surface a
// helpful diagnostic should call validateHash after sanitizing.
func sanitizeHash(raw string) string {
	cleaned := strings.ToLower(strings.TrimSpace(raw))
	return cleaned
}

// validateHash returns ErrInvalidHash (wrapped) if cleaned is not a
// 64-character lowercase hex string. sanitizeHash should be called
// first.
func validateHash(cleaned string) error {
	if !hashPattern.MatchString(cleaned) {
		return fmt.Errorf("%w: must be 64 lowercase hex characters", ErrInvalidHash)
	}
	return nil
}

// branchNamePattern matches a valid branch name. Empty, names
// containing whitespace or path separators, and names containing
// backslashes or ASCII control characters are all rejected. This
// matches the validation Git applies in refs.c.
var branchNamePattern = regexp.MustCompile(`^[!-~]+$`)

// validateBranchName returns nil if name is a legal branch name
// and an explanatory error otherwise. The check rejects:
//
//   - empty names
//   - names longer than maxBranchLength characters
//   - names containing whitespace, control characters, slashes, or
//     backslashes
//   - names containing ".." anywhere (path-traversal defence)
//   - names equal to the reserved HEAD sentinel
//   - names starting with a dot (Git reserves those)
func validateBranchName(name string) error {
	if name == "" {
		return fmt.Errorf("branch name is empty")
	}
	if name == headFile {
		return fmt.Errorf("branch name %q is reserved", name)
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("branch name %q contains '..'", name)
	}
	if len(name) > maxBranchLength {
		return fmt.Errorf("branch name is %d characters, max is %d", len(name), maxBranchLength)
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("branch name %q starts with a dot", name)
	}
	for _, r := range name {
		if r == '/' || r == '\\' {
			return fmt.Errorf("branch name %q contains %q", name, string(r))
		}
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return fmt.Errorf("branch name %q contains whitespace or control character", name)
		}
	}
	if !branchNamePattern.MatchString(name) {
		return fmt.Errorf("branch name %q contains non-printable characters", name)
	}
	return nil
}
