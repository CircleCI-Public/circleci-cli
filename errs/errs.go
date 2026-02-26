package errs

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrAuthRequired = errors.New("auth required")
)

type NotFoundError struct{ err error }

func (e *NotFoundError) Error() string        { return e.err.Error() }
func (e *NotFoundError) Unwrap() error        { return e.err }
func (e *NotFoundError) Is(target error) bool { return target == ErrNotFound }

func NotFound(err error) error {
	if err == nil {
		return nil
	}
	return &NotFoundError{err: err}
}

func NotFoundf(format string, args ...any) error {
	return &NotFoundError{err: fmt.Errorf(format, args...)}
}

type AuthRequiredError struct{ err error }

func (e *AuthRequiredError) Error() string        { return e.err.Error() }
func (e *AuthRequiredError) Unwrap() error        { return e.err }
func (e *AuthRequiredError) Is(target error) bool { return target == ErrAuthRequired }

func AuthRequired(err error) error {
	if err == nil {
		return nil
	}
	return &AuthRequiredError{err: err}
}

// CheckAuthRequired wraps err as AuthRequired if the error message contains
// "must log in". This consolidates the GQL auth-check pattern used across
// both api/api.go and api/graphql/client.go.
func CheckAuthRequired(err error) error {
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "must log in") {
		return AuthRequired(err)
	}
	return err
}
