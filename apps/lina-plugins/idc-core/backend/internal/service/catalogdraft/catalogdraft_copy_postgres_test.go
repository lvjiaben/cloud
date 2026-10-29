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
func (a *copyTestAuthorizer) AuthorizeProductDelete(ctx context.Context, no string) (int, error) {
	return a.AuthorizeProductUpdate(ctx, no)
}

func (a *copyTestAuthorizer) AuthorizeProductUpdate(context.Context, string) (int, error) {
	return a.allow()
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
	// Trigger-only migration is repeatable and changes no table/DAO shape.
	migration, readErr := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql/132-local-product-revision-binding.sql"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if _, err = g.DB().Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	audit := &catalogDraftTestAuditWriter{}
	auth := &copyTestAuthorizer{actor: 17}
	writer, err := catalog.NewManagement(audit, auth)
	if err != nil {
		t.Fatal(err)
	}
	source, err := writer.CreateProductDraft(ctx, catalog.ProductCreateInput{CategoryNo: "ctg_" + strings.Repeat("1", 32), PublicSlug: "original-product", NameZhCN: "原商品", NameEnUS: "Original", InventoryTrackingEnabled: true, SellableQuantity: 9})
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
	// Editing creates a new version on the original product without touching its sale pointer.
	revisionInput := catalog.ProductCopyInput{Revision: true, SourceProductNo: source.ProductNo, SourceVersionNo: source.VersionNo}
	// Editing must remain possible even when the provider is stopped/upgraded
	// and the original release cannot receive new bindings. No remote gate runs.
	extensions.setAdmissionError(gerror.New("original provider release unavailable"))
	if err = automation.StopNewBusiness(ctx, interfaceRow.InterfaceInstanceNo); err != nil {
		t.Fatal(err)
	}
	upgradedEvidence := automationPublicationEvidence()
	upgradedEvidence.ReleaseVersion = "v99.0.0"
	upgradedEvidence.DeclarationDigest = strings.Repeat("9", 64)
	upgradedEvidence.ReleaseDigest = strings.Repeat("9", 64)
	extensions.setEvidence(upgradedEvidence)
	input.PublicSlug = "unavailable-new-copy"
	if _, err = copier.Copy(ctx, input); err == nil {
		t.Fatal("new-product copy bypassed new-business admission")
	}
	// The SQL exception must stay restricted to the same product's exact copy.
	foreignDraft, createErr := writer.CreateProductDraft(ctx, catalog.ProductCreateInput{CategoryNo: source.CategoryNo, PublicSlug: "stopped-interface-unrelated", NameZhCN: "其他商品", NameEnUS: "Other"})
	if createErr != nil {
		t.Fatal(createErr)
	}
	_, bindErr := g.DB().Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding(tenant_id,version_no,interface_instance_no,interface_display_name_snapshot,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,bound_by)
 SELECT tenant_id,?,interface_instance_no,interface_display_name_snapshot,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,bound_by FROM plugin_idc_core_catalog_product_version_fulfillment_binding WHERE version_no=?`, foreignDraft.VersionNo, source.VersionNo)
	if bindErr == nil {
		t.Fatal("unrelated product inherited stopped interface admission")
	}
	admissionCalls := extensions.admissionCount()
	auth.denied = true
	if _, err = copier.Copy(ctx, revisionInput); !bizerr.Is(err, catalog.CodeManagementNotAccessible) {
		t.Fatalf("unavailable provider bypassed staff revision permission: %v", err)
	}
	auth.denied = false
	audit.err = gerror.New("revision audit unavailable")
	if _, err = copier.Copy(ctx, revisionInput); err == nil {
		t.Fatal("revision survived audit failure")
	}
	if count, err = g.DB().GetCount(ctx, `SELECT 1 FROM plugin_idc_core_catalog_product_version WHERE product_no=?`, source.ProductNo); err != nil || count != 1 {
		t.Fatalf("failed revision left partial rows: %d %v", count, err)
	}
	audit.err = nil
	revised, err := copier.Copy(ctx, revisionInput)
	if err != nil || revised.ProductNo != source.ProductNo || revised.VersionNo == source.VersionNo || revised.VersionNumber != 2 || revised.VersionStatus != "draft" {
		t.Fatalf("revision: %+v %v", revised, err)
	}
	if extensions.admissionCount() != admissionCalls {
		t.Fatal("local editing revision invoked provider admission")
	}
	// Saving the existing selection must be a local no-op even if its original
	// provider cannot be resolved. It must not claim verified execution evidence.
	for attempt := 0; attempt < 2; attempt++ {
		retained, retainErr := automation.Bind(ctx, productautomation.BindInput{VersionNo: revised.VersionNo, InterfaceInstanceNo: interfaceRow.InterfaceInstanceNo})
		if retainErr != nil || retained == nil || retained.InterfaceInstanceNo != interfaceRow.InterfaceInstanceNo || retained.EvidenceVerified || retained.InterfaceNewBusinessState != productautomation.InterfaceStateStopped {
			t.Fatalf("retain existing interface: %+v %v", retained, retainErr)
		}
	}
	if extensions.admissionCount() != admissionCalls {
		t.Fatal("unchanged interface selection invoked new-business admission")
	}
	if _, retainErr := automation.Bind(ctx, productautomation.BindInput{VersionNo: source.VersionNo, InterfaceInstanceNo: interfaceRow.InterfaceInstanceNo}); retainErr == nil {
		t.Fatal("unchanged selection admitted a published version write")
	}
	if _, newBindErr := automation.Bind(ctx, productautomation.BindInput{VersionNo: foreignDraft.VersionNo, InterfaceInstanceNo: interfaceRow.InterfaceInstanceNo}); newBindErr == nil {
		t.Fatal("new binding bypassed unavailable provider evidence")
	}
	frozen, freezeErr := g.DB().GetOne(ctx, `SELECT a.release_digest AS original,b.release_digest AS copied FROM plugin_idc_core_catalog_product_version_fulfillment_binding a JOIN plugin_idc_core_catalog_product_version_fulfillment_binding b ON b.version_no=? WHERE a.version_no=?`, revised.VersionNo, source.VersionNo)
	if freezeErr != nil || frozen["original"].String() != frozen["copied"].String() {
		t.Fatalf("revision rewrote historical provider evidence: %v %v", frozen, freezeErr)
	}
	readiness, readinessErr := automation.CheckPublicationReadiness(ctx, revised.VersionNo)
	if readinessErr == nil || readiness.Passed {
		t.Fatalf("current host rejection must remain an explicit availability error: %+v %v", readiness, readinessErr)
	}
	if err = automation.WithPublicationAdmission(ctx, revised.VersionNo, func(context.Context) error {
		t.Fatal("unavailable provider reached publication callback")
		return nil
	}); err == nil {
		t.Fatal("local revision implicitly granted publication")
	}
	if contract, contractErr := automation.GetFrozenFulfillmentContract(ctx, revised.VersionNo); contractErr != nil || contract == nil || contract.DeclarationDigest != automationPublicationEvidence().DeclarationDigest {
		t.Fatalf("old package digest blocked the current declaration: %v", contractErr)
	}
	extensions.setAdmissionError(nil)
	row, err = g.DB().GetOne(ctx, `SELECT status_code,current_version_no FROM plugin_idc_core_catalog_product WHERE product_no=?`, source.ProductNo)
	if err != nil || row["status_code"].String() != "active" || row["current_version_no"].String() != source.VersionNo {
		t.Fatalf("revision changed sale: %v %v", row, err)
	}
	for _, table := range []string{"plugin_idc_core_catalog_product_version_configuration", "plugin_idc_core_catalog_product_version_fulfillment_binding", "plugin_idc_core_catalog_product_version_fulfillment_policy"} {
		if count, err = g.DB().GetCount(ctx, `SELECT 1 FROM `+table+` WHERE version_no=?`, revised.VersionNo); err != nil || count != 1 {
			t.Fatalf("revision missing %s: %d %v", table, count, err)
		}
	}
	if _, err = g.DB().Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_fulfillment_binding SET interface_display_name_snapshot='tampered' WHERE version_no=?`, revised.VersionNo); err == nil {
		t.Fatal("revision exception admitted a different binding snapshot")
	}
	revisionPrices, err := g.DB().GetAll(ctx, `SELECT price_no,billing_rule_no,currency_code,amount_minor,configuration_price_dimension_no FROM plugin_idc_core_catalog_price WHERE version_no=? ORDER BY amount_minor`, revised.VersionNo)
	if err != nil || len(revisionPrices) != 2 {
		t.Fatalf("revision prices: %v %v", revisionPrices, err)
	}
	revisionRenewal := int64(500)
	if _, err = writer.UpdatePriceSchedule(ctx, revisionPrices[0]["price_no"].String(), catalog.PriceScheduleCreateInput{VersionNo: revised.VersionNo, BillingRuleNo: revisionPrices[0]["billing_rule_no"].String(), ConfigurationPriceDimensionNo: revisionPrices[0]["configuration_price_dimension_no"].String(), CurrencyCode: "CNY", Active: true, Tiers: []catalog.PriceTierDefinition{{MinimumQuantity: 0, AmountMinor: 500, RenewalAmountMinor: &revisionRenewal}}}); err != nil {
		t.Fatalf("edit revision price: %v", err)
	}
	if err = writer.DeletePriceSchedule(ctx, revised.VersionNo, revisionPrices[1]["price_no"].String()); err != nil {
		t.Fatalf("remove revision price: %v", err)
	}
	if count, err = g.DB().GetCount(ctx, `SELECT 1 FROM plugin_idc_core_catalog_price WHERE version_no=?`, source.VersionNo); err != nil || count != 2 {
		t.Fatalf("revision mutated old prices: %d %v", count, err)
	}
	if _, err = copier.Copy(ctx, revisionInput); !bizerr.Is(err, catalog.CodeManagementConflict) {
		t.Fatalf("duplicate editing revision: %v", err)
	}
	extensions.setEvidence(automationPublicationEvidence())
	if err = automation.ResumeNewBusiness(ctx, interfaceRow.InterfaceInstanceNo); err != nil {
		t.Fatal(err)
	}
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
