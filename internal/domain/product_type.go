package domain

// ProductType represents a fixed-income instrument family.
type ProductType string

const (
	ProductTypeCDB ProductType = "CDB"
	ProductTypeLF  ProductType = "LF"
	ProductTypeLCI ProductType = "LCI"
	ProductTypeLCA ProductType = "LCA"
	ProductTypeCRI ProductType = "CRI"
	ProductTypeCRA ProductType = "CRA"
	ProductTypeLFT ProductType = "LFT"
)

var validProductTypes = map[ProductType]struct{}{
	ProductTypeCDB: {},
	ProductTypeLF:  {},
	ProductTypeLCI: {},
	ProductTypeLCA: {},
	ProductTypeCRI: {},
	ProductTypeCRA: {},
	ProductTypeLFT: {},
}

// ValidateProductType rejects any value outside the closed set of supported product types.
func ValidateProductType(pt ProductType) error {
	if _, ok := validProductTypes[pt]; !ok {
		return &ValidationError{Message: "invalid product type: " + string(pt)}
	}
	return nil
}
