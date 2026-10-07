// This file verifies fail-closed tax-policy validation and authorization order.

package catalog

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestValidateTaxPolicyRequiresExactTreatmentAndRoundingSemantics(t *testing.T) {
	t.Parallel()
	rate := 600
	definition := TaxPolicyDefinition{
		PriceIncludesTax: true, TaxTreatment: TaxTreatmentTaxable,
		TaxRateBasisPoints: &rate, TaxCategoryCode: "cloud_service",
		RoundingScope: TaxRoundingScopeLine, RoundingMode: TaxRoundingHalfUp,
		RoundingQuantumMinor: 1,
	}
	if err := ValidateTaxPolicy(definition); err != nil {
		t.Fatalf("valid taxable policy rejected: %v", err)
	}
	definition.TaxTreatment = TaxTreatmentExempt
	if err := ValidateTaxPolicy(definition); err == nil {
		t.Fatal("exempt policy with a tax rate accepted")
	}
	definition.TaxRateBasisPoints = nil
	if err := ValidateTaxPolicy(definition); err != nil {
		t.Fatalf("valid exempt policy rejected: %v", err)
	}
	definition.RoundingQuantumMinor = 0
	if err := ValidateTaxPolicy(definition); err == nil {
		t.Fatal("zero rounding quantum accepted")
	}
}

func TestCreateTaxPolicyAuthorizesBeforeInputAndPersistence(t *testing.T) {
	t.Parallel()
	authorizationError := errors.New("tax authorization unavailable")
	authorizer := &managementTestAuthorizer{err: authorizationError}
	service, err := newManagementService(
		&managementTestAudit{}, authorizer, strings.NewReader(strings.Repeat("t", 16)),
	)
	if err != nil {
		t.Fatalf("create tax management service: %v", err)
	}
	if _, err = service.CreateTaxPolicy(context.Background(), TaxPolicyCreateInput{
		VersionNo: "invalid target", CurrencyCode: "cny",
	}); !errors.Is(err, authorizationError) {
		t.Fatalf("authorization failure returned %v", err)
	}
	if authorizer.calls.Load() != 1 {
		t.Fatalf("authorization calls=%d want=1", authorizer.calls.Load())
	}
}
