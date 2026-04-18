package domain_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/primolabs-org/spec-star-go/internal/domain"
)

func TestValidationError_ClassifiableViaErrorsAs(t *testing.T) {
	err := &domain.ValidationError{Message: "field is required"}

	var target *domain.ValidationError
	if !errors.As(err, &target) {
		t.Fatal("expected errors.As to match *ValidationError")
	}
	if target.Message != "field is required" {
		t.Fatalf("expected message %q, got %q", "field is required", target.Message)
	}
}

func TestValidationError_MessagePreservedThroughWrapping(t *testing.T) {
	original := &domain.ValidationError{Message: "amount must be non-negative"}
	wrapped := fmt.Errorf("position creation: %w", original)

	var target *domain.ValidationError
	if !errors.As(wrapped, &target) {
		t.Fatal("expected errors.As to match *ValidationError after wrapping")
	}
	if target.Message != "amount must be non-negative" {
		t.Fatalf("expected message %q, got %q", "amount must be non-negative", target.Message)
	}
}

func TestErrNotFound_ClassifiableAfterWrapping(t *testing.T) {
	wrapped := fmt.Errorf("context: %w", domain.ErrNotFound)
	if !errors.Is(wrapped, domain.ErrNotFound) {
		t.Fatal("expected errors.Is to match ErrNotFound after wrapping")
	}
}

func TestErrConcurrencyConflict_ClassifiableAfterWrapping(t *testing.T) {
	wrapped := fmt.Errorf("context: %w", domain.ErrConcurrencyConflict)
	if !errors.Is(wrapped, domain.ErrConcurrencyConflict) {
		t.Fatal("expected errors.Is to match ErrConcurrencyConflict after wrapping")
	}
}

func TestErrDuplicate_ClassifiableAfterWrapping(t *testing.T) {
	wrapped := fmt.Errorf("context: %w", domain.ErrDuplicate)
	if !errors.Is(wrapped, domain.ErrDuplicate) {
		t.Fatal("expected errors.Is to match ErrDuplicate after wrapping")
	}
}
