package domain_test

import (
	"errors"
	"testing"

	"github.com/primolabs-org/spec-star-go/internal/domain"
)

func TestValidateProductType_AcceptsAllValidTypes(t *testing.T) {
	valid := []domain.ProductType{
		domain.ProductTypeCDB,
		domain.ProductTypeLF,
		domain.ProductTypeLCI,
		domain.ProductTypeLCA,
		domain.ProductTypeCRI,
		domain.ProductTypeCRA,
		domain.ProductTypeLFT,
	}
	for _, pt := range valid {
		if err := domain.ValidateProductType(pt); err != nil {
			t.Errorf("expected %q to be accepted, got error: %v", pt, err)
		}
	}
}

func TestValidateProductType_RejectsEmptyString(t *testing.T) {
	err := domain.ValidateProductType("")
	if err == nil {
		t.Fatal("expected error for empty string")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatal("expected ValidationError for empty string")
	}
}

func TestValidateProductType_RejectsInvalidString(t *testing.T) {
	err := domain.ValidateProductType("INVALID")
	if err == nil {
		t.Fatal("expected error for invalid product type")
	}
	var ve *domain.ValidationError
	if !errors.As(err, &ve) {
		t.Fatal("expected ValidationError for invalid product type")
	}
}

func TestValidateProductType_RejectsLowercaseVariants(t *testing.T) {
	lowercase := []domain.ProductType{"cdb", "lf", "lci", "lca", "cri", "cra", "lft"}
	for _, pt := range lowercase {
		err := domain.ValidateProductType(pt)
		if err == nil {
			t.Errorf("expected %q to be rejected", pt)
			continue
		}
		var ve *domain.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("expected ValidationError for %q, got %T", pt, err)
		}
	}
}

func TestValidateProductType_RejectsMixedCaseVariants(t *testing.T) {
	mixed := []domain.ProductType{"Cdb", "Lf", "Lci", "Lca", "Cri", "Cra", "Lft"}
	for _, pt := range mixed {
		err := domain.ValidateProductType(pt)
		if err == nil {
			t.Errorf("expected %q to be rejected", pt)
			continue
		}
		var ve *domain.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("expected ValidationError for %q, got %T", pt, err)
		}
	}
}
