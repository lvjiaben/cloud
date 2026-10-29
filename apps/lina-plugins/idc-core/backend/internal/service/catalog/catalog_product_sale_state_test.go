// These tests retain all sale gates while reporting the actual missing copy,
// price or tax requirement without changing prices or supplying tax defaults.

package catalog

import (
	"lina-core/pkg/bizerr"
	"testing"
)

func TestSaleReadinessReturnsTheFirstMissingBusinessRequirement(t *testing.T) {
	for _, scenario := range []struct {
		name   string
		checks []PublicationReadinessCheck
		want   *bizerr.Code
	}{
		{"copy", []PublicationReadinessCheck{{Code: PublicationCheckSalesCopy}, {Code: PublicationCheckPricing}, {Code: PublicationCheckTax}}, CodeSaleCopyRequired},
		{"price", []PublicationReadinessCheck{{Code: PublicationCheckSalesCopy, Passed: true}, {Code: PublicationCheckPricing}, {Code: PublicationCheckTax}}, CodeSalePricingRequired},
		{"tax", []PublicationReadinessCheck{{Code: PublicationCheckSalesCopy, Passed: true}, {Code: PublicationCheckPricing, Passed: true}, {Code: PublicationCheckTax}}, CodeSaleTaxRequired},
		{"complete", []PublicationReadinessCheck{{Code: PublicationCheckSalesCopy, Passed: true}, {Code: PublicationCheckPricing, Passed: true}, {Code: PublicationCheckTax, Passed: true}}, nil},
		{"missing result", nil, CodePublicationIncomplete},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			err := saleReadinessError(scenario.checks)
			if scenario.want == nil {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if !bizerr.Is(err, scenario.want) {
				t.Fatalf("missing requirement lost its specific cause: %v", err)
			}
		})
	}
}
