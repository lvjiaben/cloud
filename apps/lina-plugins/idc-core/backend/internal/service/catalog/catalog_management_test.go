// This file verifies catalog-management validation and dependency gates.

package catalog

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

func newManagementService(
	auditWriter idccap.BusinessAuditWriter,
	authorizer ManagementAuthorizer,
	random io.Reader,
) (*managementServiceImpl, error) {
	return newManagementServiceWithSources(auditWriter, authorizer, nil, random)
}

type managementTestAuthorizer struct {
	actor int
	err   error
	calls atomic.Int32
}

func (a *managementTestAuthorizer) AuthorizeProductList(context.Context) (int, error) {
	a.calls.Add(1)
	return a.actor, a.err
}

func (a *managementTestAuthorizer) AuthorizeProductQuery(context.Context, string) (int, error) {
	a.calls.Add(1)
	return a.actor, a.err
}

func (a *managementTestAuthorizer) AuthorizeCategoryCreate(context.Context) (int, error) {
	a.calls.Add(1)
	return a.actor, a.err
}

func (a *managementTestAuthorizer) AuthorizeCategoryList(context.Context) (int, error) {
	a.calls.Add(1)
	return a.actor, a.err
}

func (a *managementTestAuthorizer) AuthorizeCategoryUpdate(context.Context, string) (int, error) {
	a.calls.Add(1)
	return a.actor, a.err
}

func (a *managementTestAuthorizer) AuthorizeCategoryDelete(context.Context, string) (int, error) {
	a.calls.Add(1)
	return a.actor, a.err
}

func (a *managementTestAuthorizer) AuthorizeCategoryRestore(context.Context, string) (int, error) {
	a.calls.Add(1)
	return a.actor, a.err
}

func (a *managementTestAuthorizer) AuthorizeProductCreate(context.Context) (int, error) {
	a.calls.Add(1)
	return a.actor, a.err
}

func (a *managementTestAuthorizer) AuthorizeBillingWrite(context.Context, string) (int, error) {
	a.calls.Add(1)
	return a.actor, a.err
}

func (a *managementTestAuthorizer) AuthorizePriceWrite(context.Context, string) (int, error) {
	a.calls.Add(1)
	return a.actor, a.err
}

func (a *managementTestAuthorizer) AuthorizeTaxWrite(context.Context, string) (int, error) {
	a.calls.Add(1)
	return a.actor, a.err
}

func (a *managementTestAuthorizer) AuthorizeCostWrite(context.Context, string) (int, error) {
	a.calls.Add(1)
	return a.actor, a.err
}

type managementTestAudit struct{}

func (*managementTestAudit) AppendBusinessTransition(
	context.Context,
	idccap.BusinessAuditTransition,
) error {
	return nil
}

func (*managementTestAudit) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

func TestManagementNormalizationRejectsUnsafeOrInconsistentInput(t *testing.T) {
	t.Parallel()
	category, err := normalizeCategoryCreate(CategoryCreateInput{
		NameZhCN: " 证书服务 ", NameEnUS: " Certificates ",
	})
	if err != nil || category.nameZhCN != "证书服务" || category.nameEnUS != "Certificates" {
		t.Fatalf("normalize category result=%+v err=%v", category, err)
	}
	validCategoryNo := "ctg_0123456789abcdef0123456789abcdef"
	product, err := normalizeProductCreate(ProductCreateInput{
		CategoryNo: validCategoryNo, PublicSlug: " Managed-SSL ",
		NameZhCN: "托管 SSL 证书", NameEnUS: "Managed SSL",
	})
	if err != nil || product.publicSlug != "managed-ssl" {
		t.Fatalf("normalize product result=%+v err=%v", product, err)
	}
	invalidProducts := []ProductCreateInput{
		{CategoryNo: "invalid", PublicSlug: "valid", NameZhCN: "中", NameEnUS: "EN"},
		{CategoryNo: validCategoryNo, PublicSlug: "bad/slug", NameZhCN: "中", NameEnUS: "EN"},
	}
	for index, input := range invalidProducts {
		if _, normalizeErr := normalizeProductCreate(input); !bizerr.Is(normalizeErr, CodeManagementInvalid) {
			t.Fatalf("invalid product %d returned %v", index, normalizeErr)
		}
	}
}

func TestNormalizeProductVersionCostKeepsOnlySimpleCatalogDimensions(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	end := now.Add(30 * 24 * time.Hour)
	result, err := normalizeProductVersionCost(ProductVersionCostCreateInput{
		VersionNo: "ver_0123456789abcdef0123456789abcdef", CurrencyCode: "CNY",
		PeriodUnit: PeriodUnitMonth, PeriodCount: 1, UnitCostMinor: 1800,
		EffectiveFrom: &now, EffectiveUntil: &end,
	}, now)
	if err != nil || result.UnitCostMinor != 1800 || result.PeriodUnit != PeriodUnitMonth {
		t.Fatalf("normalize simple product cost result=%+v err=%v", result, err)
	}
	invalidEnd := now
	if _, err = normalizeProductVersionCost(ProductVersionCostCreateInput{
		VersionNo: result.VersionNo, CurrencyCode: "CNY", PeriodUnit: PeriodUnitMonth,
		PeriodCount: 1, UnitCostMinor: 1, EffectiveFrom: &now, EffectiveUntil: &invalidEnd,
	}, now); err == nil {
		t.Fatal("expected an invalid effective window to be rejected")
	}
}

func TestManagementServiceFailsClosedBeforePersistence(t *testing.T) {
	t.Parallel()
	authorizationError := errors.New("authorization unavailable")
	authorizer := &managementTestAuthorizer{err: authorizationError}
	service, err := newManagementService(
		&managementTestAudit{}, authorizer, strings.NewReader(strings.Repeat("x", 48)),
	)
	if err != nil {
		t.Fatalf("create management service: %v", err)
	}
	if _, err = service.CreateCategory(context.Background(), CategoryCreateInput{}); !errors.Is(err, authorizationError) {
		t.Fatalf("authorization failure returned %v", err)
	}
	if authorizer.calls.Load() != 1 {
		t.Fatalf("authorization calls=%d want=1", authorizer.calls.Load())
	}
}

func TestManagementConstructorAndOpaqueNumbers(t *testing.T) {
	t.Parallel()
	audit := &managementTestAudit{}
	authorizer := &managementTestAuthorizer{actor: 7}
	if _, err := newManagementService(nil, authorizer, strings.NewReader("x")); err == nil {
		t.Fatal("nil audit accepted")
	}
	if _, err := newManagementService(audit, nil, strings.NewReader("x")); err == nil {
		t.Fatal("nil authorizer accepted")
	}
	if _, err := newManagementService(audit, authorizer, nil); err == nil {
		t.Fatal("nil random dependency accepted")
	}
	number, err := newCatalogNumber(strings.NewReader(strings.Repeat("a", 16)), "prd")
	if err != nil || number != "prd_61616161616161616161616161616161" ||
		!productNumberPattern.MatchString(number) || versionNumberPattern.MatchString(number) {
		t.Fatalf("opaque product number=%q err=%v", number, err)
	}
	if _, err = newCatalogNumber(strings.NewReader("short"), "prd"); err == nil {
		t.Fatal("short entropy accepted")
	}
}
