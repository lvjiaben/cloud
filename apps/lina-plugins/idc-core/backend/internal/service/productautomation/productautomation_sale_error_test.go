// These tests preserve reviewed sale-gate explanations through transactional
// automation admission while keeping unknown technical failures private.

package productautomation

import (
	"errors"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"testing"
)

func TestSaleReadinessErrorsSurviveAutomationAdmission(t *testing.T) {
	for _, code := range []*bizerr.Code{catalog.CodeSaleCopyRequired, catalog.CodeSalePricingRequired, catalog.CodeSaleTaxRequired, codeOfferDetachUnsupported, codeOfferMappingRequired, codeOfferOptionsRequired, codeChoiceRelationsRequired} {
		failure := bizerr.NewCode(code)
		if actual := normalizeManagementWriteError(failure); actual != failure || !bizerr.Is(actual, code) {
			t.Fatal("sale prerequisite was masked as infrastructure failure")
		}
	}
	if actual := normalizeManagementWriteError(errors.New("private database detail")); !bizerr.Is(actual, catalog.CodeManagementUnavailable) {
		t.Fatal("unreviewed technical failure escaped normalization")
	}
}
