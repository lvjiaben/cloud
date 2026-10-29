// This file defines the bounded staff-only catalog product detail graph.

package catalog

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const (
	maximumProductManagementDetailRows  = 20_000
	maximumProductManagementDetailBytes = 4 * 1024 * 1024
)

type ProductManagementDetail struct {
	Product           ProductManagementDetailProduct
	Version           ProductManagementDetailVersion
	BillingRules      []ProductManagementDetailBillingRule
	Prices            []ProductManagementDetailPrice
	TaxPolicies       []ProductManagementDetailTaxPolicy
	FulfillmentPolicy *FulfillmentPolicyEvidence
	Inventory         ProductVersionInventory
}

type ProductManagementDetailProduct struct {
	CurrentVersionNo                                                 string
	DirectoryNo                                                      string
	ProductNo, CategoryNo, CategoryNameZhCN, CategoryNameEnUS        string
	Status, PublicSlug, NameZhCN, NameEnUS, SummaryZhCN, SummaryEnUS string
	SortOrder                                                        int
	DescriptionVersion                                               int64
	CreatedAt, UpdatedAt                                             time.Time
}

type ProductManagementDetailVersion struct {
	VersionNo     string
	VersionNumber int
	Status        string `orm:"version_status"`
	Source        string
	SourceVersion string
	ChangeSummary string

	PublishedAt *time.Time
	RetiredAt   *time.Time
	CreatedAt   time.Time `orm:"version_created_at"`
}

type ProductManagementDetailBillingRule struct {
	BillingRuleNo, RuleCode, ChargeType, PeriodUnit, PricePhase, QuantityMode, Status string
	PeriodCount, MinimumPurchaseCycles, TrialQuantity                                 int
	MaximumRenewalCycles                                                              *int
	MeterUnit, TierMode, UsageCollectionUnit, UsageRoundingMode, UsageMissingPolicy   *string
	UsageCollectionCount                                                              *int
	UsageRoundingQuantity, UsageMinimumQuantity, UsageMaximumQuantity                 *int64
}

type ProductManagementDetailPrice struct {
	PriceNo, BillingRuleNo, CurrencyCode, Status                     string
	ConfigurationPriceDimensionNo                                    string
	ConfigurationPricingMode                                         ConfigurationPriceMode
	ConfigurationFieldLabelZhCN, ConfigurationFieldLabelEnUS         string
	ConfigurationCandidateLabelZhCN, ConfigurationCandidateLabelEnUS string
	AmountMinor, SetupFeeMinor, MinimumQuantity                      int64
	RenewalAmountMinor, MaximumQuantity                              *int64
}

type ProductManagementDetailTaxPolicy struct {
	TaxPolicyNo, CurrencyCode, TaxTreatment, TaxCategoryCode string
	PriceIncludesTax                                         bool
	TaxRateBasisPoints                                       *int
	RoundingScope, RoundingMode                              string
	RoundingQuantumMinor                                     int64
}

func (s *productManagementReaderImpl) GetProductDetail(
	ctx context.Context,
	productNo string,
) (*ProductManagementDetail, error) {
	if _, err := s.authorizer.AuthorizeProductQuery(ctx, productNo); err != nil {
		return nil, err
	}
	detail, err := loadProductManagementDetail(ctx, productNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	if detail == nil {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	if err = validateProductManagementDetail(detail); err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	return detail, nil
}

func validateProductManagementDetail(detail *ProductManagementDetail) error {
	if detail == nil || !existingCatalogReferencePattern.MatchString(detail.Product.ProductNo) ||
		!existingCatalogReferencePattern.MatchString(detail.Version.VersionNo) ||
		detail.Version.VersionNumber < 1 || detail.Product.CreatedAt.IsZero() ||
		detail.Product.UpdatedAt.IsZero() || detail.Version.CreatedAt.IsZero() {
		return gerror.New("catalog product management detail is inconsistent")
	}
	if detail.Inventory.Version < 1 || detail.Inventory.SellableQuantity < 0 {
		return gerror.New("catalog product inventory is inconsistent")
	}
	rowCount := len(detail.BillingRules) + len(detail.Prices) + len(detail.TaxPolicies)
	if rowCount > maximumProductManagementDetailRows {
		return gerror.New("catalog product management detail exceeds row bound")
	}
	if detail.FulfillmentPolicy != nil &&
		!fulfillmentPolicyDigestPattern.MatchString(detail.FulfillmentPolicy.PolicyDigest) {
		return gerror.New("catalog fulfillment policy is inconsistent")
	}
	encoded, err := json.Marshal(detail)
	if err != nil {
		return gerror.Wrap(err, "measure catalog product management detail")
	}
	if len(encoded) > maximumProductManagementDetailBytes {
		return gerror.New("catalog product management detail exceeds response bound")
	}
	return nil
}
