// This file creates immutable per-currency tax policies on editable catalog versions.

package catalog

import (
	"context"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
)

const (
	maximumManagedTaxPolicies = 64
	maximumRoundingQuantum    = int64(1_000_000)
)

var taxCategoryPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

type TaxTreatment string

const (
	TaxTreatmentTaxable   TaxTreatment = "taxable"
	TaxTreatmentZeroRated TaxTreatment = "zero_rated"
	TaxTreatmentExempt    TaxTreatment = "exempt"
)

type TaxRoundingScope string

const (
	TaxRoundingScopeLine  TaxRoundingScope = "line"
	TaxRoundingScopeTotal TaxRoundingScope = "total"
)

type TaxRoundingMode string

const (
	TaxRoundingHalfUp TaxRoundingMode = "half_up"
	TaxRoundingDown   TaxRoundingMode = "down"
	TaxRoundingUp     TaxRoundingMode = "up"
)

type TaxPolicyDefinition struct {
	PriceIncludesTax     bool
	TaxTreatment         TaxTreatment
	TaxRateBasisPoints   *int
	TaxCategoryCode      string
	RoundingScope        TaxRoundingScope
	RoundingMode         TaxRoundingMode
	RoundingQuantumMinor int64
}

type TaxPolicyCreateInput struct {
	VersionNo    string
	CurrencyCode string
	Definition   TaxPolicyDefinition
}

type TaxPolicy struct {
	TaxPolicyNo  string
	VersionNo    string
	CurrencyCode string
	Definition   TaxPolicyDefinition
}

type normalizedTaxPolicyCreate struct {
	versionNo, currencyCode string
	definition              TaxPolicyDefinition
}

// ValidateTaxPolicy accepts only explicit, internally consistent tax semantics.
func ValidateTaxPolicy(definition TaxPolicyDefinition) error {
	if !taxCategoryPattern.MatchString(definition.TaxCategoryCode) ||
		(definition.RoundingScope != TaxRoundingScopeLine &&
			definition.RoundingScope != TaxRoundingScopeTotal) ||
		(definition.RoundingMode != TaxRoundingHalfUp &&
			definition.RoundingMode != TaxRoundingDown &&
			definition.RoundingMode != TaxRoundingUp) ||
		definition.RoundingQuantumMinor < 1 ||
		definition.RoundingQuantumMinor > maximumRoundingQuantum {
		return bizerr.NewCode(CodeManagementInvalid)
	}
	switch definition.TaxTreatment {
	case TaxTreatmentTaxable:
		if definition.TaxRateBasisPoints == nil || *definition.TaxRateBasisPoints < 0 ||
			*definition.TaxRateBasisPoints > 10_000 {
			return bizerr.NewCode(CodeManagementInvalid)
		}
	case TaxTreatmentZeroRated:
		if definition.TaxRateBasisPoints == nil || *definition.TaxRateBasisPoints != 0 {
			return bizerr.NewCode(CodeManagementInvalid)
		}
	case TaxTreatmentExempt:
		if definition.TaxRateBasisPoints != nil {
			return bizerr.NewCode(CodeManagementInvalid)
		}
	default:
		return bizerr.NewCode(CodeManagementInvalid)
	}
	return nil
}

func (s *managementServiceImpl) CreateTaxPolicy(
	ctx context.Context,
	input TaxPolicyCreateInput,
) (*TaxPolicy, error) {
	actorUserID, err := s.authorizer.AuthorizeTaxWrite(ctx, input.VersionNo)
	if err != nil {
		return nil, err
	}
	if actorUserID <= 0 {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	normalized, err := normalizeTaxPolicyCreate(input)
	if err != nil {
		return nil, err
	}
	taxPolicyNo, err := newManagedCatalogNumber(s.random, "tax")
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	var result *TaxPolicy
	err = dao.CatalogProductVersion.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, lockErr := lockEditableCatalogVersion(ctx, normalized.versionNo); lockErr != nil {
			return lockErr
		}
		if ensureErr := ensureTaxPolicyCurrencyEligible(
			ctx, normalized.versionNo, normalized.currencyCode,
		); ensureErr != nil {
			return ensureErr
		}
		insert, insertErr := dao.CatalogTaxPolicy.Ctx(ctx).Data(do.CatalogTaxPolicy{
			TenantId: idccap.DefaultTenantID, TaxPolicyNo: taxPolicyNo,
			VersionNo: normalized.versionNo, CurrencyCode: normalized.currencyCode,
			PriceIncludesTax:     normalized.definition.PriceIncludesTax,
			TaxTreatmentCode:     string(normalized.definition.TaxTreatment),
			TaxRateBasisPoints:   normalized.definition.TaxRateBasisPoints,
			TaxCategoryCode:      normalized.definition.TaxCategoryCode,
			RoundingScopeCode:    string(normalized.definition.RoundingScope),
			RoundingModeCode:     string(normalized.definition.RoundingMode),
			RoundingQuantumMinor: normalized.definition.RoundingQuantumMinor,
		}).InsertIgnore()
		if insertErr = requireSingleCatalogInsert(insert, insertErr, "insert catalog tax policy"); insertErr != nil {
			return insertErr
		}
		result = &TaxPolicy{
			TaxPolicyNo: taxPolicyNo, VersionNo: normalized.versionNo,
			CurrencyCode: normalized.currencyCode, Definition: cloneTaxPolicyDefinition(normalized.definition),
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "catalog.tax-policy.create", IdempotencyKey: taxPolicyNo,
			OperationCode: "catalog.tax-policy.create", TargetType: "catalog_product_version",
			TargetNo: normalized.versionNo, ResultCode: "succeeded", ReasonCode: "manual_create",
			RiskCode: "high", AfterSnapshot: map[string]any{
				"tax_policy_no": taxPolicyNo, "currency_code": normalized.currencyCode,
				"price_includes_tax":     normalized.definition.PriceIncludesTax,
				"tax_treatment":          normalized.definition.TaxTreatment,
				"tax_rate_basis_points":  normalized.definition.TaxRateBasisPoints,
				"tax_category":           normalized.definition.TaxCategoryCode,
				"rounding_scope":         normalized.definition.RoundingScope,
				"rounding_mode":          normalized.definition.RoundingMode,
				"rounding_quantum_minor": normalized.definition.RoundingQuantumMinor,
			},
		})
	})
	return managementResult(result, err)
}

func normalizeTaxPolicyCreate(input TaxPolicyCreateInput) (normalizedTaxPolicyCreate, error) {
	versionNo := strings.TrimSpace(input.VersionNo)
	currencyCode := strings.TrimSpace(input.CurrencyCode)
	definition := cloneTaxPolicyDefinition(input.Definition)
	definition.TaxCategoryCode = strings.ToLower(strings.TrimSpace(definition.TaxCategoryCode))
	if !existingCatalogReferencePattern.MatchString(versionNo) ||
		currencyCode != input.CurrencyCode || !currencyCodePattern.MatchString(currencyCode) ||
		ValidateTaxPolicy(definition) != nil {
		return normalizedTaxPolicyCreate{}, bizerr.NewCode(CodeManagementInvalid)
	}
	return normalizedTaxPolicyCreate{
		versionNo: versionNo, currencyCode: currencyCode, definition: definition,
	}, nil
}

func ensureTaxPolicyCurrencyEligible(ctx context.Context, versionNo, currencyCode string) error {
	priceCount, err := dao.CatalogPrice.Ctx(ctx).Where(do.CatalogPrice{
		TenantId: idccap.DefaultTenantID, VersionNo: versionNo,
		CurrencyCode: currencyCode, StatusCode: catalogStatusActive,
	}).Limit(1).Count()
	if err != nil {
		return gerror.Wrap(err, "count tax-policy currency prices")
	}
	if priceCount == 0 {
		return bizerr.NewCode(CodeManagementConflict)
	}
	policyCount, err := dao.CatalogTaxPolicy.Ctx(ctx).Where(do.CatalogTaxPolicy{
		TenantId: idccap.DefaultTenantID, VersionNo: versionNo,
	}).Limit(maximumManagedTaxPolicies + 1).Count()
	if err != nil {
		return gerror.Wrap(err, "count catalog tax policies")
	}
	if policyCount >= maximumManagedTaxPolicies {
		return bizerr.NewCode(CodeManagementConflict)
	}
	return nil
}

func cloneTaxPolicyDefinition(input TaxPolicyDefinition) TaxPolicyDefinition {
	result := input
	result.TaxRateBasisPoints = cloneIntPointer(input.TaxRateBasisPoints)
	return result
}
