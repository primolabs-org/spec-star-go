package domain

import "errors"

// ValidationError is a typed error for domain invariant violations.
// Classifiable via errors.As.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

var (
	// ErrNotFound indicates a repository lookup returned no result.
	ErrNotFound = errors.New("not found")

	// ErrConcurrencyConflict indicates an optimistic concurrency failure.
	ErrConcurrencyConflict = errors.New("concurrency conflict")

	// ErrDuplicate indicates a uniqueness constraint violation.
	ErrDuplicate = errors.New("duplicate")
)
