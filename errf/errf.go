// Package errf is a leaf helper for error construction.
//
// It exists at the repository root as a leaf package so that
// every other package in the repo — including the sub-packages
// of promptsheon/ and cmd/ — can call errf.Errorf without
// creating an import cycle. Putting the wrapper inside the
// promptsheon package itself would not work: a subdir that
// imports promptsheon to call Errorf creates a cycle
// (promptsheon imports the subdir, the subdir imports promptsheon).
//
// errf is a pure leaf: it imports only "errors" and "fmt",
// exposes Errorf (delegates to fmt.Errorf) and New (delegates
// to errors.New), and is otherwise empty.
//
// Migration rule across the repo: any file that previously
// called fmt.Errorf should now call errf.Errorf. The local
// "fmt" import can be dropped once no other fmt calls remain.
package errf

import (
	"errors"
	"fmt"
)

// Errorf formats according to a format specifier and returns
// the error. The %w verb is honoured.
//
// Equivalent to fmt.Errorf; provided so the rest of the
// repository can avoid importing "fmt" directly for error
// construction.
func Errorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

// New returns an error that formats as the given text. Each
// call to New returns a distinct error value even if the text
// is identical.
//
// Equivalent to errors.New; provided for the same reason as
// Errorf.
func New(text string) error {
	return errors.New(text)
}
