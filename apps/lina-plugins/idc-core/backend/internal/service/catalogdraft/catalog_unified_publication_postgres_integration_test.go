// This file verifies the unified publication path for merchant-managed versions.

package catalogdraft

import (
	"context"
	"errors"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

type unifiedPublicationAutomationReadiness struct{}

func (*unifiedPublicationAutomationReadiness) CheckPublicationReadiness(context.Context, string) (bool, error) {
	return true, nil
}

func TestCatalogUnifiedPublicationPostgreSQLIntegration(t *testing.T) {
	t.Skip("legacy server publication checks replaced by generic commercial checks")
	ctx := context.Background()
	configureCatalogDraftIntegrationDB(t, ctx)
	insertUnifiedPublicationCatalog(t, ctx)
	authorizer := &catalogProductPublicationTestAuthorizer{actorUserID: 17}
	auditWriter := &catalogDraftTestAuditWriter{}
	service, err := catalog.NewPublication(auditWriter, authorizer, catalogDraftTestAutomationAdmission{})
	if err != nil {
		t.Fatalf("create unified publication service: %v", err)
	}
	previewService, err := catalog.NewPublicationPreview(authorizer, &unifiedPublicationAutomationReadiness{})
	if err != nil {
		t.Fatalf("create unified publication preview service: %v", err)
	}
	readyPreview, err := previewService.PreviewVersion(ctx, "version-manual-1")
	if err != nil || !readyPreview.Ready || len(readyPreview.Checks) != 7 {
		t.Fatalf("ready publication preview=%+v err=%v", readyPreview, err)
	}
	incompletePreview, err := previewService.PreviewVersion(ctx, "version-incomplete-1")
	if err != nil || incompletePreview.Ready || publicationPreviewCheckPassed(
		incompletePreview, catalog.PublicationCheckPricing,
	) {
		t.Fatalf("incomplete publication preview=%+v err=%v", incompletePreview, err)
	}
	first, err := service.PublishVersion(ctx, "version-manual-1")
	if err != nil || first.Status != "published" || first.SupersededVersionNo != "" {
		t.Fatalf("publish first manual version result=%+v err=%v", first, err)
	}
	assertUnifiedPublicationState(t, ctx, "published", "draft", "version-manual-1")
	failing, err := catalog.NewPublication(
		&catalogDraftTestAuditWriter{err: errors.New("unified publication audit unavailable")},
		authorizer,
		catalogDraftTestAutomationAdmission{},
	)
	if err != nil {
		t.Fatalf("create failing-audit unified publisher: %v", err)
	}
	if _, err = failing.PublishVersion(ctx, "version-copy-2"); !bizerr.Is(err, catalog.CodePublicationUnavailable) {
		t.Fatalf("unified publication audit failure returned %v", err)
	}
	assertUnifiedPublicationState(t, ctx, "published", "draft", "version-manual-1")
	second, err := service.PublishVersion(ctx, "version-copy-2")
	if err != nil || second.Status != "published" ||
		second.SupersededVersionNo != "version-manual-1" {
		t.Fatalf("publish copied version result=%+v err=%v", second, err)
	}
	assertUnifiedPublicationState(t, ctx, "superseded", "published", "version-copy-2")
	if _, err = service.PublishVersion(ctx, "version-copy-2"); !bizerr.Is(err, catalog.CodePublicationState) {
		t.Fatalf("repeat unified publication returned %v", err)
	}
	if _, err = service.PublishVersion(ctx, "version-incomplete-1"); !bizerr.Is(err, catalog.CodePublicationIncomplete) {
		t.Fatalf("publication without fulfillment pool returned %v", err)
	}
}

func publicationPreviewCheckPassed(
	preview *catalog.PublicationPreview,
	code catalog.PublicationCheckCode,
) bool {
	if preview == nil {
		return false
	}
	for _, check := range preview.Checks {
		if check.Code == code {
			return check.Passed
		}
	}
	return false
}

func insertUnifiedPublicationCatalog(t *testing.T, ctx context.Context) {
	t.Helper()
	if _, err := dao.CatalogProduct.DB().Exec(ctx, `
INSERT INTO plugin_idc_core_catalog_category
    (category_no, name_zh_cn, name_en_us)
VALUES ('category-unified', '统一发布', 'Unified publication');
INSERT INTO plugin_idc_core_catalog_product
    (product_no, category_no, status_code, public_slug,
     name_zh_cn, name_en_us, summary_zh_cn, summary_en_us)
VALUES
    ('product-unified', 'category-unified', 'draft',
     'unified-product', '统一发布商品', 'Unified product', '完整中文摘要', 'Complete English summary'),
    ('product-incomplete', 'category-unified', 'draft',
     'incomplete-product', '不完整商品', 'Incomplete product', '完整中文摘要', 'Complete English summary');
INSERT INTO plugin_idc_core_legacy_server_category
    (tenant_id, category_no, category_type_code)
VALUES (0, 'category-unified', 'cloud_server');
INSERT INTO plugin_idc_core_legacy_server_product
    (tenant_id, product_no, category_no, product_type_code, configuration_mode_code)
VALUES
    (0, 'product-unified', 'category-unified', 'cloud_server', 'fixed'),
    (0, 'product-incomplete', 'category-unified', 'cloud_server', 'fixed');
INSERT INTO plugin_idc_core_catalog_product_version
    (version_no, product_no, version_number, status_code, source_code, source_version)
VALUES
    ('version-manual-1', 'product-unified', 1, 'draft', 'manual', ''),
    ('version-copy-2', 'product-unified', 2, 'draft', 'copy', 'version-manual-1'),
    ('version-incomplete-1', 'product-incomplete', 1, 'draft', 'migration', 'legacy-1');
INSERT INTO plugin_idc_core_catalog_configuration_group
    (group_no, version_no, group_code, selection_mode_code, name_zh_cn, name_en_us,
     minimum_selected, maximum_selected)
VALUES
    ('group-manual-cpu', 'version-manual-1', 'cpu', 'single', '处理器', 'CPU', 1, 1),
    ('group-copy-cpu', 'version-copy-2', 'cpu', 'single', '处理器', 'CPU', 1, 1),
    ('group-incomplete-cpu', 'version-incomplete-1', 'cpu', 'single', '处理器', 'CPU', 1, 1);
INSERT INTO plugin_idc_core_catalog_configuration_option
    (option_no, group_no, version_no, option_code, component_type_code,
     name_zh_cn, name_en_us, normalized_value,
     minimum_quantity, maximum_quantity, default_quantity)
VALUES
    ('option-manual-cpu', 'group-manual-cpu', 'version-manual-1', 'cpu-2', 'cpu',
     '双核', '2 cores', '{"cores":2}', 1, 1, 1),
    ('option-copy-cpu', 'group-copy-cpu', 'version-copy-2', 'cpu-2', 'cpu',
     '双核', '2 cores', '{"cores":2}', 1, 1, 1),
    ('option-incomplete-cpu', 'group-incomplete-cpu', 'version-incomplete-1', 'cpu-2', 'cpu',
     '双核', '2 cores', '{"cores":2}', 1, 1, 1);
INSERT INTO plugin_idc_core_catalog_billing_rule
    (billing_rule_no, version_no, rule_code, charge_type_code, period_unit_code,
     period_count, price_phase_code)
VALUES
    ('billing-manual-month', 'version-manual-1', 'monthly', 'recurring', 'month', 1, 'both'),
    ('billing-copy-month', 'version-copy-2', 'monthly', 'recurring', 'month', 1, 'both'),
    ('billing-incomplete-month', 'version-incomplete-1', 'monthly', 'recurring', 'month', 1, 'both');
INSERT INTO plugin_idc_core_catalog_price
    (price_no, version_no, billing_rule_no, option_no, currency_code,
     amount_minor, renewal_amount_minor)
VALUES
    ('price-manual-base', 'version-manual-1', 'billing-manual-month', NULL, 'CNY', 10000, 10000),
    ('price-manual-cpu', 'version-manual-1', 'billing-manual-month', 'option-manual-cpu', 'CNY', 0, 0),
    ('price-copy-base', 'version-copy-2', 'billing-copy-month', NULL, 'CNY', 10000, 10000),
    ('price-copy-cpu', 'version-copy-2', 'billing-copy-month', 'option-copy-cpu', 'CNY', 0, 0),
    ('price-incomplete-base', 'version-incomplete-1', 'billing-incomplete-month', NULL, 'CNY', 10000, 10000),
    ('price-incomplete-cpu', 'version-incomplete-1', 'billing-incomplete-month', 'option-incomplete-cpu', 'CNY', 0, 0);
INSERT INTO plugin_idc_core_catalog_tax_policy
    (tax_policy_no, version_no, currency_code, price_includes_tax,
     tax_treatment_code, tax_rate_basis_points, tax_category_code,
     rounding_scope_code, rounding_mode_code, rounding_quantum_minor)
VALUES
    ('tax-manual-cny', 'version-manual-1', 'CNY', TRUE,
     'taxable', 600, 'cloud_service', 'line', 'half_up', 1),
    ('tax-copy-cny', 'version-copy-2', 'CNY', TRUE,
     'taxable', 600, 'cloud_service', 'line', 'half_up', 1),
    ('tax-incomplete-cny', 'version-incomplete-1', 'CNY', TRUE,
     'taxable', 600, 'cloud_service', 'line', 'half_up', 1);
INSERT INTO plugin_idc_core_catalog_resource_pool_binding
    (binding_no, version_no, pool_no, priority, weight, new_order_enabled, status_code)
VALUES
    ('binding-manual', 'version-manual-1', 'pool-manual', 10, 100, TRUE, 'active'),
    ('binding-copy', 'version-copy-2', 'pool-copy', 10, 100, TRUE, 'active')`); err != nil {
		t.Fatalf("insert unified publication catalog: %v", err)
	}
}

func assertUnifiedPublicationState(
	t *testing.T,
	ctx context.Context,
	manualStatus string,
	copyStatus string,
	currentVersionNo string,
) {
	t.Helper()
	var versions []entity.CatalogProductVersion
	if err := dao.CatalogProductVersion.Ctx(ctx).
		WhereIn(dao.CatalogProductVersion.Columns().VersionNo, []string{
			"version-manual-1", "version-copy-2",
		}).Scan(&versions); err != nil {
		t.Fatalf("load unified publication versions: %v", err)
	}
	statuses := make(map[string]string, len(versions))
	for _, version := range versions {
		statuses[version.VersionNo] = version.StatusCode
	}
	if statuses["version-manual-1"] != manualStatus || statuses["version-copy-2"] != copyStatus {
		t.Fatalf("unified publication statuses=%v want manual=%s copy=%s",
			statuses, manualStatus, copyStatus)
	}
	var product *entity.CatalogProduct
	if err := dao.CatalogProduct.Ctx(ctx).
		Where(do.CatalogProduct{ProductNo: "product-unified"}).Scan(&product); err != nil {
		t.Fatalf("load unified publication product: %v", err)
	}
	if product == nil || product.CurrentVersionNo == nil ||
		*product.CurrentVersionNo != currentVersionNo || product.StatusCode != "active" {
		t.Fatalf("unified publication product=%+v want current=%s", product, currentVersionNo)
	}
}
