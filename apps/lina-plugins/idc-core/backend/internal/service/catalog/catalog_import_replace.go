// This file replaces the selected upstream definition only on an authorized,
// locked edit version. The enclosing import transaction also writes binding,
// source evidence, pricing and application, or rolls back everything.

package catalog

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
)

func (s *managementServiceImpl) replaceImportedDraft(ctx context.Context, input ImportInput, actor int) (*ProductDraft, error) {
	target, product, err := lockCatalogVersionTarget(ctx, input.TargetVersionNo, false)
	if err != nil {
		return nil, err
	}
	if target.StatusCode != catalogStatusDraft || product.CategoryNo != input.CategoryNo {
		return nil, bizerr.NewCode(CodeManagementConflict)
	}
	for _, authorize := range []func(context.Context, string) (int, error){s.authorizer.AuthorizeBillingWrite, s.authorizer.AuthorizePriceWrite, s.authorizer.AuthorizeTaxWrite} {
		current, authErr := authorize(ctx, target.VersionNo)
		if authErr != nil {
			return nil, authErr
		}
		if current != actor {
			return nil, bizerr.NewCode(CodeManagementNotAccessible)
		}
	}
	current, err := s.authorizer.AuthorizeProductUpdate(ctx, product.ProductNo)
	if err != nil {
		return nil, err
	}
	if current != actor {
		return nil, bizerr.NewCode(CodeManagementNotAccessible)
	}
	// Imported specifications are immutable; their existing local price editor
	// remains available. Replacing them must never delete protected evidence.
	imported, err := dao.CatalogImportContract.Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0, VersionNo: target.VersionNo}).Exist()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	if imported {
		return nil, bizerr.NewCode(CodeUpstreamConfigurationLocked)
	}
	// A fixed set of dependent draft tables is cleared in foreign-key order.
	// No row-count-dependent lookups or writes are performed.
	for _, table := range []any{do.CatalogImportCandidate{}, do.CatalogImportTerm{}, do.CatalogPrice{}, "plugin_idc_core_catalog_configuration_price_dimension", do.CatalogBillingRule{}, do.CatalogTaxPolicy{}, do.CatalogImportContract{}, do.CatalogProductVersionConfiguration{}, "plugin_idc_core_catalog_product_version_fulfillment_policy", do.CatalogProductVersionFulfillmentBinding{}} {
		if _, err = dao.CatalogProduct.DB().Model(table).Ctx(ctx).Where("tenant_id", 0).Where("version_no", target.VersionNo).Delete(); err != nil {
			return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
		}
	}
	return &ProductDraft{ProductNo: product.ProductNo, VersionNo: target.VersionNo, CategoryNo: product.CategoryNo, VersionNumber: target.VersionNumber, Status: product.StatusCode, VersionStatus: target.StatusCode, Source: target.SourceCode}, nil
}

// requireLocalConfigurationPrices protects supplier-authored surcharges. Local
// base prices remain editable; imported choices never gain a separate editor or
// a price-write bypass through the schedule APIs.
func requireLocalConfigurationPrices(ctx context.Context, versionNo string) error {
	imported, err := dao.CatalogImportContract.Ctx(ctx).Where(do.CatalogImportContract{TenantId: 0, VersionNo: versionNo}).Exist()
	if err != nil {
		return bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	if imported {
		return bizerr.NewCode(CodeUpstreamConfigurationLocked)
	}
	return nil
}
