// This file verifies the complete local copy transaction preserves source
// inventory and prices while producing an unpublished, zero-stock draft.

package catalogdraft

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/contractpolicy"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

type copyTestAuthorizer struct {
	catalog.ManagementAuthorizer
	actor  int
	denied bool
}

func (a *copyTestAuthorizer) allow() (int, error) {
	if a.denied {
		return 0, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	return a.actor, nil
}
func (a *copyTestAuthorizer) AuthorizeProductCreate(context.Context) (int, error) { return a.allow() }
func (a *copyTestAuthorizer) AuthorizeProductQuery(context.Context, string) (int, error) {
	return a.allow()
}
func (a *copyTestAuthorizer) AuthorizeBillingWrite(context.Context, string) (int, error) {
	return a.allow()
}
func (a *copyTestAuthorizer) AuthorizePriceWrite(context.Context, string) (int, error) {
	return a.allow()
}
func (a *copyTestAuthorizer) AuthorizeTaxWrite(context.Context, string) (int, error) {
	return a.allow()
}
func (a *copyTestAuthorizer) AuthorizeCostWrite(context.Context, string) (int, error) {
	return a.allow()
}

func TestCatalogCopyPostgresAtomicCommercialGraph(t *testing.T) {
	ctx := context.Background()
	configureCatalogDraftIntegrationDB(t, ctx)
	installAutomationPublicationMigration(t, ctx)
	_, file, _, _ := runtime.Caller(0)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/081-simple-product-version-inventory.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = g.DB().Exec(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/014-catalog-inventory-routing.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = g.DB().Exec(ctx, string(raw)); err != nil {
		t.Fatal(err)
	}
	// Only empty owner tables outside the publication fixture are needed. Their
	// copied relations are separately checked below against populated facts.
	if _, err = g.DB().Exec(ctx, `
 CREATE TABLE plugin_idc_core_catalog_product_version_cost(id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT,cost_no TEXT,version_no TEXT,currency_code TEXT,period_unit_code TEXT,period_count INT,unit_cost_minor BIGINT,effective_from TIMESTAMPTZ,effective_until TIMESTAMPTZ,version BIGINT,created_at TIMESTAMPTZ,updated_at TIMESTAMPTZ);
 CREATE TABLE plugin_idc_core_catalog_import_contract(id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT,version_no TEXT,interface_instance_no TEXT,snapshot JSONB,snapshot_digest TEXT,read_evidence JSONB,created_at TIMESTAMPTZ);
 CREATE TABLE plugin_idc_core_catalog_import_term(id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT,version_no TEXT,billing_rule_no TEXT,currency_code TEXT,upstream_term_reference TEXT,whole_item_rule TEXT,markup_percent TEXT,created_at TIMESTAMPTZ);
 CREATE TABLE plugin_idc_core_catalog_import_candidate(id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT,version_no TEXT,field_key TEXT,candidate_value TEXT,option_token TEXT,created_at TIMESTAMPTZ);
 INSERT INTO plugin_idc_core_catalog_category(tenant_id,category_no,name_zh_cn,name_en_us,status_code,sort_order) VALUES(0,'ctg_11111111111111111111111111111111','测试','Test','active',0);`); err != nil {
		t.Fatal(err)
	}
	audit := &catalogDraftTestAuditWriter{}
	auth := &copyTestAuthorizer{actor: 17}
	writer, err := catalog.NewManagement(audit, auth)
	if err != nil {
		t.Fatal(err)
	}
	source, err := writer.CreateProductDraft(ctx, catalog.ProductCreateInput{CategoryNo: "ctg_" + strings.Repeat("1", 32), PublicSlug: "original-product", NameZhCN: "原商品", NameEnUS: "Original", InventoryTrackingEnabled: true, SellableQuantity: 9, ContractPolicyCode: contractpolicy.PolicyInherit})
	if err != nil {
		t.Fatal(err)
	}
	billing, err := writer.CreateBillingRule(ctx, catalog.BillingRuleCreateInput{VersionNo: source.VersionNo, RuleCode: "monthly", Active: true, Definition: catalog.BillingRuleDefinition{ChargeType: catalog.ChargeTypeRecurring, PeriodUnit: catalog.PeriodUnitMonth, PeriodCount: 1, PricePhase: catalog.PricePhaseBoth, QuantityMode: catalog.QuantityModeFixed, MinimumPurchaseCycles: 1}})
	if err != nil {
		t.Fatal(err)
	}
	renewal := int64(10000)
	if _, err = writer.CreatePriceSchedule(ctx, catalog.PriceScheduleCreateInput{VersionNo: source.VersionNo, BillingRuleNo: billing.BillingRuleNo, CurrencyCode: "CNY", Active: true, Tiers: []catalog.PriceTierDefinition{{MinimumQuantity: 0, AmountMinor: 10000, SetupFeeMinor: 100, RenewalAmountMinor: &renewal}}}); err != nil {
		t.Fatal(err)
	}

	fixtures := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO plugin_idc_core_catalog_configuration_price_dimension(tenant_id,dimension_no,version_no,configuration_version,field_key,candidate_value,field_label_zh_cn,field_label_en_us,candidate_label_zh_cn,candidate_label_en_us,pricing_mode_code) VALUES(0,'cpd_11111111111111111111111111111111',?,1,'disk','quantity','数据盘','Data disk','容量','Capacity','per_unit')`, []any{source.VersionNo}},
		{`INSERT INTO plugin_idc_core_catalog_price(tenant_id,price_no,version_no,billing_rule_no,configuration_price_dimension_no,currency_code,amount_minor,minimum_quantity,status_code) VALUES(0,'price-source',?,?,'cpd_11111111111111111111111111111111','CNY',2000,0,'active')`, []any{source.VersionNo, billing.BillingRuleNo}},
	}
	fixtures = append(fixtures, struct {
		sql  string
		args []any
	}{`INSERT INTO plugin_idc_core_catalog_resource_pool_binding(tenant_id,binding_no,version_no,pool_no,priority,weight) VALUES(0,'binding-source',?,'opaque-pool',20,30)`, []any{source.VersionNo}})
	for _, fixture := range fixtures {
		if _, err = g.DB().Exec(ctx, fixture.sql, fixture.args...); err != nil {
			t.Fatal(err)
		}
	}

	extensions := &automationPublicationExtensions{evidence: automationPublicationEvidence()}
	automation, err := productautomation.New(g.DB(), extensions, automationPublicationAuthorizer{}, audit, automationPublicationCodec{evidence: automationPublicationEvidence()})
	if err != nil {
		t.Fatal(err)
	}
	interfaceRow, err := automation.CreateInterface(ctx, productautomation.CreateInterfaceInput{DisplayName: "Copy provider", SelectionToken: "opaque-provider-selection"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = automation.Bind(ctx, productautomation.BindInput{VersionNo: source.VersionNo, InterfaceInstanceNo: interfaceRow.InterfaceInstanceNo}); err != nil {
		t.Fatal(err)
	}
	saveAutomationPublicationConfiguration(t, ctx, automation, source.VersionNo)
	saveAutomationPublicationPolicy(t, ctx, source.VersionNo, automationPublicationEvidence())
	if _, err = g.DB().Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version SET status_code='published',published_at=CURRENT_TIMESTAMP WHERE version_no=?`, source.VersionNo); err != nil {
		t.Fatal(err)
	}
	if _, err = g.DB().Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET status_code='active',current_version_no=? WHERE product_no=?`, source.VersionNo, source.ProductNo); err != nil {
		t.Fatal(err)
	}
	copyWriter, err := catalog.ProductCopyWriterFromManagement(writer)
	if err != nil {
		t.Fatal(err)
	}
	copyAutomation, err := productautomation.ProductCopyServiceFromService(automation)
	if err != nil {
		t.Fatal(err)
	}
	copier, err := NewCopy(copyWriter, copyAutomation, copyTestPrices{})
	if err != nil {
		t.Fatal(err)
	}
	input := catalog.ProductCopyInput{SourceProductNo: source.ProductNo, SourceVersionNo: source.VersionNo, PublicSlug: "copied-product", NameZhCN: "复制商品", NameEnUS: "Copied"}
	result, err := copier.Copy(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if result.ProductNo == source.ProductNo || result.VersionNo == source.VersionNo || result.Status != "draft" || result.VersionStatus != "draft" {
		t.Fatalf("bad copied identities: %+v", result)
	}
	row, err := g.DB().GetOne(ctx, `SELECT p.current_version_no,v.status_code,i.tracking_enabled,i.sellable_quantity FROM plugin_idc_core_catalog_product p JOIN plugin_idc_core_catalog_product_version v USING(product_no) JOIN plugin_idc_core_catalog_product_version_inventory i USING(version_no) WHERE p.product_no=?`, result.ProductNo)
	if err != nil || !row["current_version_no"].IsNil() || row["status_code"].String() != "draft" || !row["tracking_enabled"].Bool() || row["sellable_quantity"].Int64() != 0 {
		t.Fatalf("copied draft unexpectedly saleable: %v %v", row, err)
	}
	prices, err := g.DB().GetAll(ctx, `SELECT p.amount_minor,p.setup_fee_minor,p.configuration_price_dimension_no,p.billing_rule_no FROM plugin_idc_core_catalog_price p WHERE version_no=? ORDER BY amount_minor`, result.VersionNo)
	if err != nil || len(prices) != 2 || prices[0]["amount_minor"].Int64() != 2000 || prices[1]["amount_minor"].Int64() != 10000 || prices[0]["configuration_price_dimension_no"].String() == "cpd_11111111111111111111111111111111" || prices[0]["billing_rule_no"].String() == billing.BillingRuleNo {
		t.Fatalf("copied graph lost remapping or price: %v %v", prices, err)
	}
	var count int
	if count, err = g.DB().GetCount(ctx, `SELECT 1 FROM plugin_idc_core_catalog_configuration_price_dimension WHERE version_no=? AND dimension_no=? AND pricing_mode_code='per_unit'`, result.VersionNo, prices[0]["configuration_price_dimension_no"].String()); err != nil || count != 1 {
		t.Fatalf("price references no copied quantity dimension: %d %v", count, err)
	}
	row, err = g.DB().GetOne(ctx, `SELECT sellable_quantity FROM plugin_idc_core_catalog_product_version_inventory WHERE version_no=?`, source.VersionNo)
	if err != nil || row["sellable_quantity"].Int64() != 9 {
		t.Fatalf("source inventory changed: %v %v", row, err)
	}
	for _, table := range []string{"plugin_idc_core_catalog_product_version_fulfillment_binding", "plugin_idc_core_catalog_product_version_configuration", "plugin_idc_core_catalog_product_version_fulfillment_policy"} {
		if count, err = g.DB().GetCount(ctx, `SELECT 1 FROM `+table+` WHERE version_no=?`, result.VersionNo); err != nil || count != 1 {
			t.Fatalf("copied automation graph missing %s: %d %v", table, count, err)
		}
	}
	row, err = g.DB().GetOne(ctx, `SELECT status_code,current_version_no FROM plugin_idc_core_catalog_product WHERE product_no=?`, source.ProductNo)
	if err != nil || row["status_code"].String() != "active" || row["current_version_no"].String() != source.VersionNo {
		t.Fatalf("source publication changed: %v %v", row, err)
	}
	if count, err = g.DB().GetCount(ctx, `SELECT 1 FROM plugin_idc_core_catalog_resource_pool_binding WHERE version_no=?`, result.VersionNo); err != nil || count != 0 {
		t.Fatalf("retired resource routing was copied: %d %v", count, err)
	}

	audit.err = gerror.New("injected audit failure")
	input.PublicSlug = "failed-copy"
	if _, err = copier.Copy(ctx, input); err == nil {
		t.Fatal("audit failure accepted")
	}
	if count, err = g.DB().GetCount(ctx, `SELECT 1 FROM plugin_idc_core_catalog_product WHERE public_slug='failed-copy'`); err != nil || count != 0 {
		t.Fatalf("partial copy survived: %d %v", count, err)
	}
	audit.err = nil
	auth.denied = true
	input.PublicSlug = "denied-copy"
	if _, err = copier.Copy(ctx, input); !bizerr.Is(err, catalog.CodeManagementNotAccessible) {
		t.Fatalf("denied copy accepted: %v", err)
	}
	if count, err = g.DB().GetCount(ctx, `SELECT 1 FROM plugin_idc_core_catalog_price WHERE version_no=?`, source.VersionNo); err != nil || count != 2 {
		t.Fatalf("source prices changed: %d %v", count, err)
	}
	auth.denied = false
	legacyFixtures := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO plugin_idc_core_catalog_configuration_group(tenant_id,group_no,version_no,group_code,selection_mode_code,name_zh_cn,name_en_us) VALUES(0,'group-source',?,'legacy','single','旧配置','Legacy')`, []any{source.VersionNo}},
		{`INSERT INTO plugin_idc_core_catalog_configuration_option(tenant_id,option_no,group_no,version_no,option_code,component_type_code,name_zh_cn,name_en_us) VALUES(0,'option-source','group-source',?,'ssd','data_disk','固态盘','SSD')`, []any{source.VersionNo}},
		{`INSERT INTO plugin_idc_core_catalog_price(tenant_id,price_no,version_no,billing_rule_no,option_no,currency_code,amount_minor,minimum_quantity,status_code) VALUES(0,'legacy-price',?,?,'option-source','CNY',2000,0,'active')`, []any{source.VersionNo, billing.BillingRuleNo}},
	}
	for _, fixture := range legacyFixtures {
		if _, err = g.DB().Exec(ctx, fixture.sql, fixture.args...); err != nil {
			t.Fatal(err)
		}
	}
	input.PublicSlug = "legacy-copy-refused"
	if _, err = copier.Copy(ctx, input); !bizerr.Is(err, catalog.CodeManagementConflict) {
		t.Fatalf("retired configuration price copied: %v", err)
	}
	if count, err = g.DB().GetCount(ctx, `SELECT 1 FROM plugin_idc_core_catalog_product WHERE public_slug=?`, input.PublicSlug); err != nil || count != 0 {
		t.Fatalf("rejected legacy copy left a draft: %d %v", count, err)
	}
}

// copyTestPrices isolates catalog/automation owner tests from the pricing owner.
type copyTestPrices struct{}

func (copyTestPrices) CopyLevelProductPrices(context.Context, string, string) error { return nil }
