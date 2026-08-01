// Package promptsheon — error-construction helpers.
//
// This file is the single source of fmt usage in the entire
// repository. Every other Go file calls promptsheon.Errorf
// (or promptsheon.New) instead of fmt.Errorf so the
// dependency on the "fmt" package is contained.
//
// The %w verb is preserved so errors.Is / errors.As continue
// to walk the chain. The wrappers accept the same arguments
// as their fmt counterparts and return the same types.
package promptsheon

import (
	"errors"
	"fmt"
)

// Errorf formats according to a format specifier and returns
// the error as a value. The %w verb is honoured.
//
// Equivalent to fmt.Errorf; provided so the rest of the
// repository can import only "github.com/sachncs/promptsheon/promptsheon"
// and never "fmt" directly.
func Errorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

// NewError returns an error that formats as the given text. Each
// call returns a distinct error value even if the text is
// identical.
//
// Equivalent to errors.New; provided for the same reason as
// Errorf. The name avoids collision with any other New
// function in the promptsheon package.
func NewError(text string) error {
	return errors.New(text)
}
