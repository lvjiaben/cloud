// This file exercises merchant selection boundaries before any database writes:
// opt-in stays closed, sale ranges narrow technical bounds, and frozen schema
// tampering is rejected while original releases keep their exact declarations.

package productautomation

import (
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/database/gdb"
	"os"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
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
		{{FieldKey: "operating_system", CandidateValues: []string{"linux"}}},
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
