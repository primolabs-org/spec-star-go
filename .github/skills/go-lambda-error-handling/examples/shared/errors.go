package shared

import "errors"

var (
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrRetryable    = errors.New("retryable failure")
	ErrTerminal     = errors.New("terminal failure")
)
