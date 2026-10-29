// This file exercises merchant selection boundaries before any database writes:
// opt-in stays closed, sale ranges narrow technical bounds, and frozen schema
// tampering is rejected while original releases keep their exact declarations.

package productautomation

import (
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/database/gdb"
	"os"
	"strconv"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func customerSelectionEvidence(t *testing.T) *capmodel.BusinessExtensionProviderEvidence {
	t.Helper()
	declaration := testFulfillmentDeclaration()
	declaration.MerchantCustomerFields = []string{"cpu", "snapshot_count", "snapshot_unlimited"}
	metadata, err := productfulfillmentcap.NewProviderMetadata(declaration)
	if err != nil {
		t.Fatal(err)
	}
	label := &capmodel.LocalizedText{EnUS: "Choice", ZhCN: "配置"}
	return &capmodel.BusinessExtensionProviderEvidence{ProviderMetadata: metadata, AuthorizedMethods: []string{"provision", "query.status"}, CustomerConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "operating_system", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, Required: true, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "linux", Label: label}}},
		{Key: "cpu", Type: capmodel.ConfigurationFieldInteger, Label: label, Minimum: "1", Maximum: "1024"},
		{Key: "snapshot_count", Type: capmodel.ConfigurationFieldInteger, Label: label, Minimum: "0", Maximum: "1000000"},
		{Key: "snapshot_unlimited", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "unlimited", Label: label}}},
	}, Conditions: []*capmodel.ConfigurationRuleDeclaration{{When: &capmodel.ConfigurationConditionDeclaration{Operator: "equal", Field: "snapshot_unlimited", Value: "unlimited"}, Actions: []*capmodel.ConfigurationActionDeclaration{{Action: "hide", Field: "snapshot_count"}}}}}}
}

func TestMerchantCustomerOptionsDefaultClosedAndFrozen(t *testing.T) {
	exact := customerSelectionEvidence(t)
	frozen, err := selectCustomerSchema(exact, nil)
	if err != nil || len(frozen.Fields) != 1 || len(frozen.Conditions) != 0 {
		t.Fatalf("opt-in exposed by default: %+v %v", frozen, err)
	}
	selected := []CustomerFieldSelection{{FieldKey: "cpu", Minimum: "2", Maximum: "16"}, {FieldKey: "snapshot_count", Minimum: "0", Maximum: "5"}, {FieldKey: "snapshot_unlimited", CandidateValues: []string{"unlimited"}}}
	frozen, err = selectCustomerSchema(exact, selected)
	if err != nil || len(frozen.Fields) != 4 || len(frozen.Conditions) != 1 {
		t.Fatalf("valid selection failed: %+v %v", frozen, err)
	}
	if exact.CustomerConfiguration.Fields[1].Maximum != "1024" {
		t.Fatal("provider declaration mutated")
	}
	encoded, err := json.Marshal(frozen)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = verifiedCustomerSchema(exact, string(encoded)); err != nil {
		t.Fatal(err)
	}
	frozen.Fields[1].Required = true
	encoded, err = json.Marshal(frozen)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = verifiedCustomerSchema(exact, string(encoded)); err == nil {
		t.Fatal("merchant changed field requirement outside authorized narrowing")
	}
}

func TestMerchantCustomerOptionsRejectInvalidBoundsAndCandidates(t *testing.T) {
	cases := [][]CustomerFieldSelection{
		{{FieldKey: "cpu", Minimum: "0", Maximum: "16"}},
		{{FieldKey: "cpu", Minimum: "1", Maximum: "1025"}},
		{{FieldKey: "cpu", Minimum: "16", Maximum: "2"}},
		{{FieldKey: "cpu", Minimum: "1"}},
		{{FieldKey: "cpu", Minimum: "1", Maximum: "2"}, {FieldKey: "cpu", Minimum: "1", Maximum: "2"}},
		{{FieldKey: "operating_system", CandidateValues: []string{"unknown"}}},
		{{FieldKey: "snapshot_unlimited", CandidateValues: []string{"free"}}},
		{{FieldKey: "snapshot_unlimited", CandidateValues: []string{"unlimited", "unlimited"}}},
	}
	for index, selections := range cases {
		if _, err := selectCustomerSchema(customerSelectionEvidence(t), selections); err == nil {
			t.Fatalf("invalid case %d accepted", index)
		}
	}
}

func TestMerchantCustomerOptionsLegacyDeclarationUnchanged(t *testing.T) {
	exact := customerSelectionEvidence(t)
	declaration := testFulfillmentDeclaration()
	metadata, err := productfulfillmentcap.NewProviderMetadata(declaration)
	if err != nil {
		t.Fatal(err)
	}
	exact.ProviderMetadata = metadata
	frozen, err := selectCustomerSchema(exact, nil)
	if err != nil || !sameExactConfiguration(frozen, exact.CustomerConfiguration) {
		t.Fatal("legacy declaration changed")
	}
	if _, err = frozenCustomerSelections(exact, frozen); err != nil {
		t.Fatal(err)
	}
	if _, err = selectCustomerSchema(exact, []CustomerFieldSelection{{FieldKey: "cpu", Minimum: "1", Maximum: "4"}}); err == nil {
		t.Fatal("non opt-in release allowed configuration changes")
	}
}

func TestMerchantCustomerOptionPricesRequireExplicitZeroPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	schemaName := newAutomationSchema(t)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteAutomationIdentifier(schemaName)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, cleanupErr := admin.Exec(ctx, `DROP SCHEMA `+quoteAutomationIdentifier(schemaName)+` CASCADE`); cleanupErr != nil {
			t.Error(cleanupErr)
		}
	})
	database := openAutomationDatabase(t, ctx, dsn, schemaName)
	_, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_catalog_price(tenant_id INT,version_no TEXT,option_no TEXT,configuration_price_dimension_no TEXT,currency_code TEXT,billing_rule_no TEXT,status_code TEXT,price_no TEXT,amount_minor BIGINT);
 CREATE TABLE plugin_idc_core_catalog_billing_rule(tenant_id INT,version_no TEXT,billing_rule_no TEXT,status_code TEXT);
 CREATE TABLE plugin_idc_core_catalog_configuration_price_dimension(tenant_id INT,version_no TEXT,dimension_no TEXT,status_code TEXT,field_key TEXT,candidate_value TEXT);
 INSERT INTO plugin_idc_core_catalog_billing_rule VALUES(0,'version','month','active');
 INSERT INTO plugin_idc_core_catalog_price VALUES(0,'version',NULL,NULL,'CNY','month','active','base',10000);`)
	if err != nil {
		t.Fatal(err)
	}
	exact := customerSelectionEvidence(t)
	schema, err := selectCustomerSchema(exact, []CustomerFieldSelection{{FieldKey: "cpu", Minimum: "1", Maximum: "16"}})
	if err != nil {
		t.Fatal(err)
	}
	svc := &serviceImpl{database: database}
	if err = svc.validateCustomerSelectionPrices(ctx, "version", exact, schema); err == nil {
		t.Fatal("no dimension silently became free")
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_configuration_price_dimension VALUES(0,'version','dimension','active','cpu','quantity')`); err != nil {
		t.Fatal(err)
	}
	if err = svc.validateCustomerSelectionPrices(ctx, "version", exact, schema); err == nil {
		t.Fatal("dimension without price silently became free")
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_price VALUES(0,'version',NULL,'dimension','CNY','month','active','free',0)`); err != nil {
		t.Fatal(err)
	}
	if err = svc.validateCustomerSelectionPrices(ctx, "version", exact, schema); err != nil {
		t.Fatalf("explicit zero rejected: %v", err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_price VALUES(0,'version',NULL,NULL,'USD','month','active','usd-base',1000)`); err != nil {
		t.Fatal(err)
	}
	if err = svc.validateCustomerSelectionPrices(ctx, "version", exact, schema); err == nil {
		t.Fatal("missing second currency price silently became free")
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_price VALUES(0,'version',NULL,'dimension','USD','month','active','usd-paid',10)`); err != nil {
		t.Fatal(err)
	}
	if err = svc.validateCustomerSelectionPrices(ctx, "version", exact, schema); err != nil {
		t.Fatal(err)
	}
}

func TestMerchantCustomerPresentationPreservesMappingAndCoreFields(t *testing.T) {
	exact := customerSelectionEvidence(t)
	selections := []CustomerFieldSelection{
		{FieldKey: "cpu", Minimum: "2", Maximum: "16", DisplayNameZhCN: "性能核心", DisplayNameEnUS: "Performance cores", SortOrder: 20},
		{FieldKey: "snapshot_count", Minimum: "0", Maximum: "5", DisplayNameZhCN: "快照额度", DisplayNameEnUS: "Snapshot quota", SortOrder: 10},
	}
	frozen, err := selectCustomerSchema(exact, selections)
	if err != nil {
		t.Fatal(err)
	}
	if len(frozen.Fields) != 3 || frozen.Fields[0].Key != "operating_system" || !frozen.Fields[0].Required || frozen.Fields[1].Key != "snapshot_count" || frozen.Fields[2].Key != "cpu" || frozen.Fields[2].Label.ZhCN != "性能核心" {
		t.Fatalf("core mapping or custom presentation changed: %+v", frozen)
	}
	encoded, err := json.Marshal(frozen)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = verifiedCustomerSchema(exact, string(encoded)); err != nil {
		t.Fatal(err)
	}
	selections[0].FieldKey = "unsupported_supplier_parameter"
	if _, err = selectCustomerSchema(exact, selections); err == nil {
		t.Fatal("accepted undeclared supplier parameter")
	}
	selections[0].FieldKey = "cpu"
	selections[0].Maximum = "2048"
	if _, err = selectCustomerSchema(exact, selections); err == nil {
		t.Fatal("renaming bypassed supplier technical bounds")
	}
}

func TestLocalCustomerFieldsFreezeIndependentlyOfProvider(t *testing.T) {
	exact := customerSelectionEvidence(t)
	field := &capmodel.ConfigurationFieldDeclaration{Key: "local_support", Type: capmodel.ConfigurationFieldSingleSelect, Label: &capmodel.LocalizedText{ZhCN: "支持等级", EnUS: "Support level"}, Required: true, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "basic", Label: &capmodel.LocalizedText{ZhCN: "基础", EnUS: "Basic"}}}}
	selections := []CustomerFieldSelection{{FieldKey: field.Key, LocalField: field}, {FieldKey: "operating_system", DisplayControl: "radio"}}
	schema, err := selectCustomerSchema(exact, selections)
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Fields) != 2 || schema.Fields[0].Key != "operating_system" || schema.Fields[1].Key != field.Key {
		t.Fatalf("core or local field lost: %+v", schema.Fields)
	}
	frozen, err := frozenCustomerSelections(exact, schema)
	if err != nil || len(frozen) != 1 || frozen[0].LocalField == nil {
		t.Fatalf("local definition not restored: %+v %v", frozen, err)
	}
	schema.Fields[1].DynamicCandidates = &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "query.status"}
	if _, err = frozenCustomerSelections(exact, schema); err == nil {
		t.Fatal("merchant field acquired provider capability")
	}
}

func TestLocalCustomerFieldTypePalette(t *testing.T) {
	types := []string{capmodel.ConfigurationFieldSingleLine, capmodel.ConfigurationFieldMultiLine, capmodel.ConfigurationFieldInteger, capmodel.ConfigurationFieldDecimal, capmodel.ConfigurationFieldBoolean, capmodel.ConfigurationFieldSingleSelect, capmodel.ConfigurationFieldMultiSelect, capmodel.ConfigurationFieldDate, capmodel.ConfigurationFieldDateTime, capmodel.ConfigurationFieldTreeSelect, capmodel.ConfigurationFieldAddress}
	for _, kind := range types {
		t.Run(kind, func(t *testing.T) {
			field := &capmodel.ConfigurationFieldDeclaration{Key: "local_extra", Type: kind, Label: &capmodel.LocalizedText{ZhCN: "本地配置", EnUS: "Local option"}}
			if kind == capmodel.ConfigurationFieldSingleSelect || kind == capmodel.ConfigurationFieldMultiSelect || kind == capmodel.ConfigurationFieldTreeSelect || kind == capmodel.ConfigurationFieldAddress {
				field.StaticCandidates = []*capmodel.ConfigurationCandidate{{Value: "first", Label: &capmodel.LocalizedText{ZhCN: "子项", EnUS: "Choice"}}}
			}
			selection := CustomerFieldSelection{FieldKey: field.Key, LocalField: field, DisplayControl: "auto"}
			if kind == capmodel.ConfigurationFieldInteger {
				selection.Minimum = "0"
				selection.Maximum = "16"
			}
			schema, err := selectCustomerSchema(customerSelectionEvidence(t), []CustomerFieldSelection{selection})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = compileCustomerPresentations(schema, []CustomerFieldSelection{selection}); err != nil {
				t.Fatal(err)
			}
			restored, err := frozenCustomerSelections(customerSelectionEvidence(t), schema)
			if err != nil || len(restored) != 1 || restored[0].LocalField.Type != kind {
				t.Fatalf("local type not frozen: %+v %v", restored, err)
			}
		})
	}
}

func TestCustomerPresentationRejectsOversizedMetadataBeforePersistence(t *testing.T) {
	selection := CustomerFieldSelection{FieldKey: "cpu", Minimum: "1", Maximum: "512", DisplayControl: configurationcontract.ControlSelect, NumericLabels: map[string]*capmodel.LocalizedText{}}
	for number := 1; number <= 512; number++ {
		value := strconv.Itoa(number)
		selection.NumericChoices = append(selection.NumericChoices, value)
		selection.NumericLabels[value] = &capmodel.LocalizedText{ZhCN: strings.Repeat("x", 128), EnUS: strings.Repeat("y", 128)}
	}
	schema, err := selectCustomerSchema(customerSelectionEvidence(t), []CustomerFieldSelection{selection})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = compileCustomerPresentations(schema, []CustomerFieldSelection{selection}); err == nil {
		t.Fatal("oversized metadata was passed to persistence")
	}
}

func TestRequiredCoreIntegerRetainsBoundsWithLocalPresentation(t *testing.T) {
	exact := customerSelectionEvidence(t)
	declaration := testFulfillmentDeclaration()
	metadata, err := productfulfillmentcap.NewProviderMetadata(declaration)
	if err != nil {
		t.Fatal(err)
	}
	exact.ProviderMetadata = metadata
	exact.CustomerConfiguration.Conditions = nil
	exact.CustomerConfiguration.Fields = []*capmodel.ConfigurationFieldDeclaration{{Key: "required_cpu", Type: capmodel.ConfigurationFieldInteger, Label: &capmodel.LocalizedText{ZhCN: "处理器", EnUS: "CPU"}, Required: true, Minimum: "1", Maximum: "16"}}
	selection := CustomerFieldSelection{FieldKey: "required_cpu", DisplayControl: configurationcontract.ControlSlider, QuantityStep: "2", NumericPricing: configurationcontract.NumericPricingQuantity}
	schema, err := selectCustomerSchema(exact, []CustomerFieldSelection{selection})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = configurationcontract.ValidateSchema(schema, configurationSurfaceCustomer, exact.AuthorizedMethods); err != nil {
		t.Fatal(err)
	}
	if !schema.Fields[0].Required || schema.Fields[0].Minimum != "1" || schema.Fields[0].Maximum != "16" {
		t.Fatal("core requirement or technical bounds changed")
	}
	if _, err = compileCustomerPresentations(schema, []CustomerFieldSelection{selection}); err != nil {
		t.Fatal(err)
	}
	if _, err = frozenCustomerSelections(exact, schema); err != nil {
		t.Fatal(err)
	}
}

// TestOptionalTextFieldDoesNotBreakExistingSelections covers advanced connector
// fields such as an optional SSH port without widening existing numeric bounds.
func TestOptionalTextFieldDoesNotBreakExistingSelections(t *testing.T) {
	exact := customerSelectionEvidence(t)
	declaration := testFulfillmentDeclaration()
	declaration.MerchantCustomerFields = []string{"cpu", "snapshot_count", "snapshot_unlimited", "port"}
	metadata, err := productfulfillmentcap.NewProviderMetadata(declaration)
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := selectCustomerSchema(exact, []CustomerFieldSelection{{FieldKey: "cpu", Minimum: "1", Maximum: "4"}})
	if err != nil {
		t.Fatal(err)
	}
	exact.ProviderMetadata = metadata
	exact.CustomerConfiguration.Fields = append(exact.CustomerConfiguration.Fields, &capmodel.ConfigurationFieldDeclaration{Key: "port", Type: capmodel.ConfigurationFieldSingleLine, Label: &capmodel.LocalizedText{ZhCN: "登录端口", EnUS: "Login port"}, MaximumLength: 5})
	if _, err = frozenCustomerSelections(exact, frozen); err != nil {
		t.Fatalf("new optional text field broke existing product: %v", err)
	}
	selected, err := selectCustomerSchema(exact, []CustomerFieldSelection{{FieldKey: "port"}})
	if err != nil || len(selected.Fields) != 2 || selected.Fields[1].MaximumLength != 5 {
		t.Fatalf("optional text selection: %+v %v", selected, err)
	}
	for _, invalid := range []CustomerFieldSelection{{FieldKey: "port", Minimum: "1", Maximum: "2"}, {FieldKey: "port", CandidateValues: []string{"22"}}} {
		if _, err = selectCustomerSchema(exact, []CustomerFieldSelection{invalid}); err == nil {
			t.Fatal("text parameter accepted invented quantity or choices")
		}
	}
}

// Required fields remain technically required but are editable display content.
func TestRequiredCustomerFieldCanBeRenamedWithoutChangingProvider(t *testing.T) {
	exact := customerSelectionEvidence(t)
	schema, err := selectCustomerSchema(exact, []CustomerFieldSelection{{FieldKey: "operating_system", DisplayNameZhCN: "镜像系统", DisplayNameEnUS: "Server image"}})
	if err != nil {
		t.Fatal(err)
	}
	if schema.Fields[0].Label.ZhCN != "镜像系统" || !schema.Fields[0].Required || exact.CustomerConfiguration.Fields[0].Label.ZhCN != "配置" {
		t.Fatal("renaming changed technical requirement or provider declaration")
	}
	restored, err := frozenCustomerSelections(exact, schema)
	if err != nil || len(restored) != 1 || restored[0].DisplayNameZhCN != "镜像系统" {
		t.Fatalf("renamed required field not restored: %+v %v", restored, err)
	}
	for _, selection := range []CustomerFieldSelection{{FieldKey: "operating_system", DisplayNameZhCN: "镜像系统"}, {FieldKey: "operating_system", DisplayNameZhCN: strings.Repeat("界", 129), DisplayNameEnUS: "Image"}, {FieldKey: "operating_system", Minimum: "0"}} {
		if _, err = selectCustomerSchema(exact, []CustomerFieldSelection{selection}); err == nil {
			t.Fatal("invalid required field override accepted")
		}
	}
}

func TestSameLabelsHaveDistinctFrozenPriceFieldTokens(t *testing.T) {
	label := &capmodel.LocalizedText{ZhCN: "规格", EnUS: "Specification"}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "cpu", Label: label}, {Key: "memory", Label: label}}}
	tokens := configurationPriceFieldTokens("ver_fixture", 2, schema)
	if tokens["cpu"] == "" || tokens["cpu"] == tokens["memory"] || tokens["cpu"] == configurationPriceFieldTokens("ver_fixture", 3, schema)["cpu"] {
		t.Fatal("price field identity used display label or ignored the configuration version")
	}
}
