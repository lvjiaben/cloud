// This file verifies portable imports against PostgreSQL, including explicit
// prices, immutable terms, full source evidence and atomic rollback.

package catalog

import (
	"context"
	"crypto/rand"
	"errors"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPortableImportPersistencePostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	db := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := db.Exec(ctx, catalogImportMigrationFixtureSQL+catalogImportPersistenceFixtureSQL+portableImportFixtureSQL); err != nil {
		t.Fatal(err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test path")
	}
	for _, name := range []string{"110-catalog-product-import.sql", "120-portable-catalog-import.sql", "120-portable-catalog-import.sql"} {
		raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(ctx, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	svc, err := newManagementService(&managementTestAudit{}, &managementTestAuthorizer{actor: 7}, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	input := ImportInput{RequestKey: "portable-import-test-001", InterfaceInstanceNo: "pfi_" + strings.Repeat("1", 32), Reference: "demo", CategoryNo: "ctg_" + strings.Repeat("1", 32), PricingMode: ImportPricingMarkup, MarkupPercent: "10"}
	source := portableImportTestSource()
	var result *ImportResult
	write := func(rollback bool) error {
		return db.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
			var e error
			result, _, e = svc.CommitImport(txCtx, input, source, nil)
			if e != nil {
				return e
			}
			if rollback {
				return errors.New("rollback after complete price write")
			}
			return nil
		})
	}
	if err = write(true); err == nil {
		t.Fatal("rollback unexpectedly succeeded")
	}
	count, err := db.GetCount(ctx, "SELECT 1 FROM plugin_idc_core_catalog_import_contract")
	if err != nil || count != 0 {
		t.Fatalf("rollback leaked contract: %d %v", count, err)
	}
	if err = write(false); err != nil {
		t.Fatal(err)
	}
	if result.PricingIncomplete {
		t.Fatal("complete snapshot marked incomplete")
	}
	prices, err := db.GetAll(ctx, `SELECT amount_minor,setup_fee_minor,renewal_amount_minor FROM plugin_idc_core_catalog_price WHERE version_no=? ORDER BY amount_minor`, result.Item.VersionNo)
	if err != nil || len(prices) != 2 || prices[0]["amount_minor"].Int64() != 22 || prices[1]["amount_minor"].Int64() != 110 || prices[1]["setup_fee_minor"].Int64() != 11 || prices[1]["renewal_amount_minor"].Int64() != 99 {
		t.Fatalf("price preservation: %#v %v", prices, err)
	}
	amount, err := db.GetValue(ctx, `SELECT whole_item_rule->>'fixedAmountMinor' FROM plugin_idc_core_catalog_import_term WHERE version_no=?`, result.Item.VersionNo)
	if err != nil || amount.Int64() != 220 {
		t.Fatalf("fixed price above ordinary lost: %v %v", amount, err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_import_contract SET snapshot_digest=repeat('a',64) WHERE version_no=?`, result.Item.VersionNo); err == nil {
		t.Fatal("mutable contract")
	}
	for _, mode := range []channelcommercecap.ImportConfigurationPriceMode{channelcommercecap.ImportConfigurationPricePerUnit, channelcommercecap.ImportConfigurationPriceBandTotal} {
		t.Run(string(mode), func(t *testing.T) {
			quantitySource := portableImportTestSource()
			quantitySource.ImportSnapshot.Schema.Fields = []*capmodel.ConfigurationFieldDeclaration{{Key: "disk", Type: capmodel.ConfigurationFieldInteger, Label: &capmodel.LocalizedText{ZhCN: "数据盘", EnUS: "Data disk"}}}
			quantitySource.ImportSnapshot.Mappings = []channelcommercecap.ImportFieldMapping{{FieldKey: "disk", FieldToken: "field-disk"}}
			maximum, renewal := int64(100), int64(1)
			quantitySource.ImportSnapshot.Prices[0].Options = []channelcommercecap.ImportOptionPrice{{FieldKey: "disk", PricingMode: mode, Bands: []channelcommercecap.ImportPriceBand{{MaximumQuantity: &maximum, AmountMinor: 2, SetupAmountMinor: 3, RenewalAmountMinor: &renewal}, {MinimumQuantity: 100, AmountMinor: 1, SetupAmountMinor: 5, RenewalAmountMinor: &renewal}}}}
			quantityInput := input
			quantityInput.RequestKey = "quantity-import-" + string(mode)
			quantityInput.MarkupPercent = "0"
			var imported *ImportResult
			err := db.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
				var writeErr error
				imported, _, writeErr = svc.CommitImport(txCtx, quantityInput, quantitySource, nil)
				return writeErr
			})
			if err != nil {
				t.Fatal(err)
			}
			stored, err := db.GetAll(ctx, `SELECT dimension.pricing_mode_code,price.minimum_quantity,price.maximum_quantity,price.amount_minor,price.setup_fee_minor FROM plugin_idc_core_catalog_price price JOIN plugin_idc_core_catalog_configuration_price_dimension dimension ON dimension.dimension_no=price.configuration_price_dimension_no WHERE price.version_no=? ORDER BY price.minimum_quantity`, imported.Item.VersionNo)
			if err != nil || len(stored) != 2 || stored[0]["pricing_mode_code"].String() != string(mode) || stored[0]["maximum_quantity"].Int64() != 100 || stored[1]["minimum_quantity"].Int64() != 100 || stored[1]["amount_minor"].Int64() != 1 || stored[1]["setup_fee_minor"].Int64() != 5 {
				t.Fatalf("quantity import changed: %+v err=%v", stored, err)
			}
		})
	}

}

func portableImportTestSource() *resourceinterfaceprojectioncap.CatalogDetail {
	renewal := int64(90)
	optionRenewal := int64(18)
	fixed := int64(200)
	label := &capmodel.LocalizedText{ZhCN: "区域", EnUS: "Region"}
	snapshot := &channelcommercecap.ImportSnapshot{ProtocolVersion: "1", Product: channelcommercecap.CatalogProduct{PublicSlug: "demo"}, ConfigurationRevision: "revision-demo", Schema: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "region", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "us", Label: label}}}}}, Mappings: []channelcommercecap.ImportFieldMapping{{FieldKey: "region", FieldToken: "field-demo", Options: []channelcommercecap.ImportOptionMapping{{Value: "us", OptionToken: "option-demo"}}}}, Currencies: []channelcommercecap.CatalogCurrency{{CurrencyCode: "USD", Precision: 2}}, PricingComplete: true, Fulfillment: channelcommercecap.ImportFulfillment{Granularity: "per_order_item", RenewalMode: "unavailable"}, Prices: []channelcommercecap.ImportPriceSchedule{{CurrencyCode: "USD", TermReference: "term-demo", ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 1, PricePhase: "both", QuantityMode: "fixed", MinimumPurchaseCycles: 1, Base: []channelcommercecap.ImportPriceBand{{AmountMinor: 100, SetupAmountMinor: 10, RenewalAmountMinor: &renewal}}, Options: []channelcommercecap.ImportOptionPrice{{FieldKey: "region", Value: "us", Bands: []channelcommercecap.ImportPriceBand{{AmountMinor: 20, RenewalAmountMinor: &optionRenewal}}}}, Tax: channelcommercecap.ImportTaxPolicy{Treatment: "exempt", CategoryCode: "service", RoundingScope: "line", RoundingMode: "half_up", RoundingQuantumMinor: 1}, WholeItemRule: &channelcommercecap.ImportWholeItemRule{Kind: "fixed", FixedAmountMinor: &fixed}}}}
	return &resourceinterfaceprojectioncap.CatalogDetail{Product: resourceinterfaceprojectioncap.CatalogProduct{Reference: "demo", Name: "测试商品"}, BasePrices: []resourceinterfaceprojectioncap.CatalogPrice{}, Options: []resourceinterfaceprojectioncap.CatalogOption{}, ProductConfiguration: []resourceinterfaceprojectioncap.CatalogConfigurationValue{}, PriceScheduleComplete: true, ImportSnapshot: snapshot, ReadEvidence: &resourceinterfaceprojectioncap.CatalogReadEvidence{InterfaceInstanceNo: "pfi_" + strings.Repeat("1", 32), OwnerID: "idc-resource", ResourceChannelRef: "CHN_demo", InterfaceConfigurationRevision: "rci_" + strings.Repeat("1", 32), PluginID: "idc-peer", PluginReleaseVersion: "v0.1.0", ContractVersion: "v1", AuthorizedMethods: []string{"provision", "query"}, ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64)}}
}

const portableImportFixtureSQL = `
ALTER TABLE plugin_idc_core_catalog_billing_rule ADD COLUMN maximum_renewal_cycles INT,ADD COLUMN meter_unit_code TEXT,ADD COLUMN tier_mode_code TEXT,ADD COLUMN usage_collection_unit_code TEXT,ADD COLUMN usage_collection_count INT,ADD COLUMN usage_rounding_mode_code TEXT,ADD COLUMN usage_rounding_quantity BIGINT,ADD COLUMN usage_minimum_quantity BIGINT,ADD COLUMN usage_maximum_quantity BIGINT,ADD COLUMN usage_missing_policy_code TEXT;
ALTER TABLE plugin_idc_core_catalog_price ADD COLUMN configuration_price_dimension_no TEXT,ADD COLUMN maximum_quantity BIGINT;
CREATE TABLE plugin_idc_core_catalog_tax_policy(tenant_id INT,tax_policy_no TEXT,version_no TEXT,currency_code TEXT,price_includes_tax BOOL,tax_treatment_code TEXT,tax_rate_basis_points INT,tax_category_code TEXT,rounding_scope_code TEXT,rounding_mode_code TEXT,rounding_quantum_minor BIGINT,created_at TIMESTAMPTZ);
CREATE TABLE plugin_idc_core_catalog_configuration_price_dimension(tenant_id INT,dimension_no TEXT,version_no TEXT,configuration_version BIGINT,field_key TEXT,candidate_value TEXT,field_label_zh_cn TEXT,field_label_en_us TEXT,candidate_label_zh_cn TEXT,candidate_label_en_us TEXT,pricing_mode_code TEXT DEFAULT 'fixed',status_code TEXT);
`
