// This file verifies saved-version previews in a disposable PostgreSQL schema,
// including hidden defaults, imported whole prices, ownership, and no writes.
package catalog

import (
	"context"
	"lina-core/pkg/bizerr"
	"os"
	"strconv"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func TestProductEditorPricePreviewPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	database := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := database.Exec(ctx, productEditorPricePreviewFixtureSQL); err != nil {
		t.Fatal(err)
	}
	service := &productManagementReaderImpl{authorizer: &managementTestAuthorizer{actor: 7}}
	input := ProductEditorPricePreviewInput{ProductNo: "prd_11111111111111111111111111111111", VersionNo: "ver_11111111111111111111111111111111", CurrencyCode: "CNY", ConfigurationVersion: 1, Values: []configurationcontract.ConfigurationValue{{FieldKey: "plan", OptionTokens: []string{"basic"}}}}
	result, err := service.PreviewProductPrice(ctx, input)
	if err != nil || len(result.Terms) != 1 {
		t.Fatalf("saved preview=%+v err=%v", result, err)
	}
	term := result.Terms[0]
	if term.BaseAmountMinor != "1000" || term.ConfigurationAmountMinor != "200" || term.SetupAmountMinor != "110" || term.TotalAmountMinor == nil || *term.TotalAmountMinor != "1441" {
		t.Fatalf("hidden default did not control price: %+v", term)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_import_term(tenant_id,version_no,billing_rule_no,currency_code,whole_item_rule) VALUES(0,?,'monthly','CNY','{"kind":"fixed","fixedAmountMinor":1500,"discountBasisPoints":null}')`, input.VersionNo); err != nil {
		t.Fatal(err)
	}
	result, err = service.PreviewProductPrice(ctx, input)
	if err != nil || *result.Terms[0].TotalAmountMinor != "1771" {
		t.Fatalf("imported whole price was not reused: %+v err=%v", result, err)
	}
	input.ProductNo = "prd_22222222222222222222222222222222"
	if _, err = service.PreviewProductPrice(ctx, input); err == nil {
		t.Fatal("another product's version was accepted")
	}
	input.ProductNo = "prd_11111111111111111111111111111111"
	input.ConfigurationVersion = 2
	if _, err = service.PreviewProductPrice(ctx, input); err == nil {
		t.Fatal("changed configuration revision was accepted")
	}
	stored, err := database.GetValue(ctx, `SELECT SUM(amount_minor) FROM plugin_idc_core_catalog_price`)
	if err != nil || stored.Int64() != 1200 {
		t.Fatalf("preview changed prices: %v err=%v", stored, err)
	}
}

const productEditorPricePreviewFixtureSQL = `
CREATE TABLE plugin_idc_core_catalog_import_contract(tenant_id INT DEFAULT 0,version_no TEXT,snapshot JSONB,snapshot_digest TEXT,read_evidence JSONB);
CREATE TABLE plugin_idc_core_catalog_product_version(tenant_id INT,version_no TEXT,product_no TEXT);
CREATE TABLE plugin_idc_core_catalog_product_version_configuration(tenant_id INT,version_no TEXT,version BIGINT,customer_declaration_snapshot JSONB,customer_presentation_snapshot JSONB);
CREATE TABLE plugin_idc_core_catalog_configuration_price_dimension(id BIGSERIAL,tenant_id INT,version_no TEXT,dimension_no TEXT,field_key TEXT,candidate_value TEXT,pricing_mode_code TEXT,status_code TEXT);
CREATE TABLE plugin_idc_core_catalog_billing_rule(id BIGSERIAL,tenant_id INT,version_no TEXT,billing_rule_no TEXT,charge_type_code TEXT,period_unit_code TEXT,period_count INT,price_phase_code TEXT,quantity_mode_code TEXT,trial_quantity INT,minimum_purchase_cycles INT,maximum_renewal_cycles INT,meter_unit_code TEXT,tier_mode_code TEXT,usage_collection_unit_code TEXT,usage_collection_count INT,usage_rounding_mode_code TEXT,usage_rounding_quantity BIGINT,usage_minimum_quantity BIGINT,usage_maximum_quantity BIGINT,usage_missing_policy_code TEXT,status_code TEXT);
CREATE TABLE plugin_idc_core_catalog_price(id BIGSERIAL,tenant_id INT,version_no TEXT,billing_rule_no TEXT,configuration_price_dimension_no TEXT,option_no TEXT,currency_code TEXT,amount_minor BIGINT,setup_fee_minor BIGINT,renewal_amount_minor BIGINT,minimum_quantity BIGINT,maximum_quantity BIGINT,status_code TEXT);
CREATE TABLE plugin_idc_core_catalog_tax_policy(id BIGSERIAL,tenant_id INT,version_no TEXT,currency_code TEXT,price_includes_tax BOOL,tax_treatment_code TEXT,tax_rate_basis_points INT,tax_category_code TEXT,rounding_scope_code TEXT,rounding_mode_code TEXT,rounding_quantum_minor BIGINT);
CREATE TABLE plugin_idc_core_catalog_import_term(tenant_id INT,version_no TEXT,billing_rule_no TEXT,currency_code TEXT,whole_item_rule TEXT);
INSERT INTO plugin_idc_core_catalog_product_version VALUES(0,'ver_11111111111111111111111111111111','prd_11111111111111111111111111111111');
INSERT INTO plugin_idc_core_catalog_product_version_configuration VALUES(0,'ver_11111111111111111111111111111111',1,'{"version":"v1","fields":[{"key":"plan","type":"single_select","required":true,"label":{"zhCN":"档位","enUS":"Plan"},"staticCandidates":[{"value":"basic","label":{"zhCN":"基础","enUS":"Basic"}},{"value":"fast","label":{"zhCN":"高性能","enUS":"Fast"}}]}]}','{"plan":{"customerVisible":false,"defaultValue":"fast"}}');
INSERT INTO plugin_idc_core_catalog_configuration_price_dimension(tenant_id,version_no,dimension_no,field_key,candidate_value,pricing_mode_code,status_code) VALUES(0,'ver_11111111111111111111111111111111','cpd_11111111111111111111111111111111','plan','fast','fixed','active');
INSERT INTO plugin_idc_core_catalog_billing_rule(tenant_id,version_no,billing_rule_no,charge_type_code,period_unit_code,period_count,price_phase_code,quantity_mode_code,minimum_purchase_cycles,status_code) VALUES(0,'ver_11111111111111111111111111111111','monthly','recurring','month',1,'new_purchase','fixed',1,'active');
INSERT INTO plugin_idc_core_catalog_price(tenant_id,version_no,billing_rule_no,currency_code,amount_minor,setup_fee_minor,minimum_quantity,status_code) VALUES(0,'ver_11111111111111111111111111111111','monthly','CNY',1000,100,0,'active');
INSERT INTO plugin_idc_core_catalog_price(tenant_id,version_no,billing_rule_no,configuration_price_dimension_no,currency_code,amount_minor,setup_fee_minor,minimum_quantity,status_code) VALUES(0,'ver_11111111111111111111111111111111','monthly','cpd_11111111111111111111111111111111','CNY',200,10,0,'active');
INSERT INTO plugin_idc_core_catalog_tax_policy(tenant_id,version_no,currency_code,price_includes_tax,tax_treatment_code,tax_rate_basis_points,tax_category_code,rounding_scope_code,rounding_mode_code,rounding_quantum_minor) VALUES(0,'ver_11111111111111111111111111111111','CNY',false,'taxable',1000,'cloud_service','total','half_up',1);
`

// A daily option gap must not hide a complete monthly quote. Explicit zero
// prices remain valid; null amounts are reserved for unavailable schedules.
func TestProductEditorPreviewPeriodAvailabilityPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	database := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := database.Exec(ctx, productEditorPricePreviewFixtureSQL); err != nil {
		t.Fatal(err)
	}
	const fixture = `
 UPDATE plugin_idc_core_catalog_product_version_configuration SET customer_presentation_snapshot='{}',customer_declaration_snapshot='{"version":"v1","fields":[{"key":"cpu","type":"integer","required":true,"minimum":"0","maximum":"100","label":{"zhCN":"CPU","enUS":"CPU"}},{"key":"memory","type":"integer","required":true,"minimum":"0","maximum":"100","label":{"zhCN":"内存","enUS":"Memory"}},{"key":"bandwidth","type":"integer","required":true,"minimum":"0","maximum":"100","label":{"zhCN":"带宽","enUS":"Bandwidth"}}]}';
 DELETE FROM plugin_idc_core_catalog_configuration_price_dimension;
 DELETE FROM plugin_idc_core_catalog_price;
 INSERT INTO plugin_idc_core_catalog_configuration_price_dimension(tenant_id,version_no,dimension_no,field_key,candidate_value,pricing_mode_code,status_code) VALUES
 (0,'ver_11111111111111111111111111111111','cpd_11111111111111111111111111111111','cpu','quantity','per_unit','active'),
 (0,'ver_11111111111111111111111111111111','cpd_22222222222222222222222222222222','memory','quantity','per_unit','active'),
 (0,'ver_11111111111111111111111111111111','cpd_33333333333333333333333333333333','bandwidth','quantity','per_unit','active');
 UPDATE plugin_idc_core_catalog_tax_policy SET price_includes_tax=true,tax_rate_basis_points=100,rounding_scope_code='line';
 INSERT INTO plugin_idc_core_catalog_billing_rule(tenant_id,version_no,billing_rule_no,charge_type_code,period_unit_code,period_count,price_phase_code,quantity_mode_code,minimum_purchase_cycles,status_code) VALUES(0,'ver_11111111111111111111111111111111','daily','recurring','day',1,'new_purchase','fixed',1,'active');
 INSERT INTO plugin_idc_core_catalog_price(tenant_id,version_no,billing_rule_no,configuration_price_dimension_no,currency_code,amount_minor,setup_fee_minor,minimum_quantity,maximum_quantity,status_code) VALUES
 (0,'ver_11111111111111111111111111111111','monthly',NULL,'CNY',100,1000,0,NULL,'active'),
 (0,'ver_11111111111111111111111111111111','daily',NULL,'CNY',10000,10000,0,NULL,'active'),
 (0,'ver_11111111111111111111111111111111','monthly','cpd_11111111111111111111111111111111','CNY',500,0,0,2,'active'),
 (0,'ver_11111111111111111111111111111111','monthly','cpd_11111111111111111111111111111111','CNY',500,0,2,NULL,'active'),
 (0,'ver_11111111111111111111111111111111','monthly','cpd_22222222222222222222222222222222','CNY',0,0,0,NULL,'active'),
 (0,'ver_11111111111111111111111111111111','monthly','cpd_33333333333333333333333333333333','CNY',300,300,0,NULL,'active');`
	if _, err := database.Exec(ctx, fixture); err != nil {
		t.Fatal(err)
	}
	two, one := "2", "1"
	service := &productManagementReaderImpl{authorizer: &managementTestAuthorizer{actor: 7}}
	input := ProductEditorPricePreviewInput{ProductNo: "prd_11111111111111111111111111111111", VersionNo: "ver_11111111111111111111111111111111", CurrencyCode: "CNY", ConfigurationVersion: 1, Values: []configurationcontract.ConfigurationValue{{FieldKey: "cpu", IntegerTextValue: &two}, {FieldKey: "memory", IntegerTextValue: &two}, {FieldKey: "bandwidth", IntegerTextValue: &one}}}
	result, err := service.PreviewProductPrice(ctx, input)
	if err != nil || len(result.Terms) != 2 {
		t.Fatalf("preview=%+v err=%v", result, err)
	}
	daily, monthly := result.Terms[0], result.Terms[1]
	if daily.BillingRuleNo != "daily" || daily.Status != productEditorPriceUnavailable || daily.BaseAmountMinor != "" || daily.SetupAmountMinor != "" || daily.ConfigurationAmountMinor != "" || daily.TotalAmountMinor != nil || daily.TaxAmountMinor != nil || len(daily.Issues) != 3 {
		t.Fatalf("daily gap invented a price: %+v", daily)
	}
	labels := make(map[string]string)
	for _, issue := range daily.Issues {
		if issue.Kind != CurrencyPriceDimensionOption || issue.Reason != CurrencyPriceIssueMissing {
			t.Fatalf("unexpected issue %+v", issue)
		}
		labels[issue.FieldKey] = issue.NameZhCN
	}
	if labels["cpu"] != "CPU" || labels["memory"] != "内存" || labels["bandwidth"] != "带宽" {
		t.Fatalf("missing safe price labels: %+v", labels)
	}
	if monthly.Status != PublicQuoteFinal || monthly.BaseAmountMinor != "100" || monthly.ConfigurationAmountMinor != "1300" || monthly.SetupAmountMinor != "1300" || monthly.TotalAmountMinor == nil || *monthly.TotalAmountMinor != "2700" || len(monthly.Issues) != 0 {
		t.Fatalf("complete monthly quote changed: %+v", monthly)
	}
	source, err := loadPublicQuoteSource(ctx, database, input.VersionNo, "CNY")
	if err != nil {
		t.Fatal(err)
	}
	// The public calculator must keep the same complete monthly result even
	// when the source also contains an unavailable daily period.
	checkout, err := buildPublicConfigurationQuote("", "CNY", source, map[string]int{"cpd_11111111111111111111111111111111": 2, "cpd_22222222222222222222222222222222": 2, "cpd_33333333333333333333333333333333": 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(checkout.Terms) != 1 || checkout.Terms[0].TermNo != "monthly" || checkout.Terms[0].TotalAmountMinor == nil || monthly.TotalAmountMinor == nil || *checkout.Terms[0].TotalAmountMinor != 2700 || monthly.TaxAmountMinor == nil || checkout.Terms[0].TaxAmountMinor == nil || *monthly.TaxAmountMinor != strconv.FormatInt(*checkout.Terms[0].TaxAmountMinor, 10) {
		t.Fatal("preview differs from checkout")
	}
	// Invalid schedules are diagnosed explicitly, never interpreted as zero.
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_price SET amount_minor=-1 WHERE configuration_price_dimension_no='cpd_22222222222222222222222222222222'`); err != nil {
		t.Fatal(err)
	}
	invalid, err := service.PreviewProductPrice(ctx, input)
	if err != nil || invalid.Terms[1].Status != productEditorPriceUnavailable || invalid.Terms[1].TotalAmountMinor != nil || len(invalid.Terms[1].Issues) != 1 || invalid.Terms[1].Issues[0].Reason != CurrencyPriceIssueInvalid {
		t.Fatalf("invalid schedule was not isolated: %+v %v", invalid, err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_price SET amount_minor=0 WHERE configuration_price_dimension_no='cpd_22222222222222222222222222222222'`); err != nil {
		t.Fatal(err)
	}
	count, err := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_catalog_price`)
	if err != nil || count.Int() != 6 {
		t.Fatalf("preview mutated prices: %v %v", count, err)
	}
	if _, err = database.Exec(ctx, `DELETE FROM plugin_idc_core_catalog_tax_policy`); err != nil {
		t.Fatal(err)
	}
	if unavailable, previewErr := service.PreviewProductPrice(ctx, input); unavailable != nil || !bizerr.Is(previewErr, CodePublicQuoteUnavailable) {
		t.Fatalf("tax failure became an unavailable price term: %v", previewErr)
	}
}
