// This file proves current connection changes do not rewrite product terms or
// invalidate orders frozen against another revision of the same interface.
package productautomation

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func TestLiveResourceConnectionAndHistoricalOrdersRemainIndependentPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.WithoutCancel(t.Context())
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := newAutomationSchema(t)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteAutomationIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	database := openAutomationDatabase(t, ctx, dsn, schema)
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
		if _, dropErr := admin.Exec(ctx, "DROP SCHEMA "+quoteAutomationIdentifier(schema)+" CASCADE"); dropErr != nil {
			t.Error(dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
	})
	if _, err = database.Exec(ctx, liveConnectionFixtureSQL); err != nil {
		t.Fatal(err)
	}
	const versionNo = "ver_11111111111111111111111111111111"
	const interfaceNo = "pfi_11111111111111111111111111111111"
	exact := automationEvidence(nil)
	owner := &capmodel.ConfigurationSchemaDeclaration{Version: "resource-owner-v1"}
	ownerJSON, err := json.Marshal(owner)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_product_automation_interface_instance VALUES(0,?,'resource','idc-resource')`, interfaceNo); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding VALUES(0,?,?,?,?,?,?,?,ARRAY[?],?,?)`, versionNo, interfaceNo, "Cloud", exact.PluginID, exact.Point, exact.ReleaseVersion, exact.ContractVersion, exact.AuthorizedMethods, exact.ReleaseDigest, exact.DeclarationDigest); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version_configuration VALUES(0,?,?,1,'none','{}','none','{}','{}',?)`, versionNo, interfaceNo, configurationDigest([]byte("{}{}"))); err != nil {
		t.Fatal(err)
	}
	for _, revision := range []int64{1, 2} {
		if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_product_automation_interface_configuration_revision VALUES(0,?,?,?,?,'{}',?,?,?)`, interfaceNo, revision, owner.Version, string(ownerJSON), configurationDigest(ownerJSON), configurationDigest([]byte("{}")), "rci_"+strings.Repeat(string(rune('0'+revision)), 32)); err != nil {
			t.Fatal(err)
		}
	}
	service := &serviceImpl{database: database}
	saved := &entity.CatalogProductVersionConfiguration{InterfaceInstanceNo: interfaceNo, InterfaceConfigurationRevision: 1}
	current := &entity.ProductAutomationInterfaceConfigurationRevision{InterfaceInstanceNo: interfaceNo, Revision: 2, SchemaVersion: owner.Version, ResourceOwnerConfigurationRevision: "rci_" + strings.Repeat("2", 32)}
	if err = service.validateConfigurationInterfaceRevision(ctx, interfaceNo, saved, current); err != nil {
		t.Fatal("connection maintenance disabled the unchanged product", err)
	}
	validator := &frozenCustomerConfigurationValidator{database: database, extensions: &automationTestExtensions{evidence: []*capmodel.BusinessExtensionProviderEvidence{exact}}}
	old := FrozenCustomerConfigurationInput{
		OrderNo: "order-before-address-correction", CustomerNo: "CUS_" + strings.Repeat("1", 32), UploaderUserID: 101,
		VersionNo: versionNo, InterfaceInstanceNo: interfaceNo, InterfaceConfigurationRevision: 1,
		CustomerDeclarationDigest: configurationDigest([]byte("{}")), CandidateSnapshotDigest: configurationDigest([]byte("{}")), CustomerValueDigest: configurationDigest([]byte("{}")), Candidates: configurationcontract.ResolvedCandidates{},
	}
	newOrder := old
	newOrder.OrderNo = "order-after-address-correction"
	newOrder.InterfaceConfigurationRevision = 2
	result, err := validator.ValidateBatch(ctx, []FrozenCustomerConfigurationInput{old, newOrder})
	if err != nil || len(result) != 2 {
		t.Fatalf("orders using two connections of the same product were mixed: results=%d err=%v", len(result), err)
	}
	invalid := newOrder
	invalid.InterfaceConfigurationRevision = 999
	if _, err = validator.ValidateBatch(ctx, []FrozenCustomerConfigurationInput{invalid}); err == nil {
		t.Fatal("nonexistent connection revision accepted")
	}
	invalid = newOrder
	invalid.InterfaceInstanceNo = "pfi_" + strings.Repeat("2", 32)
	if _, err = validator.ValidateBatch(ctx, []FrozenCustomerConfigurationInput{invalid}); err == nil {
		t.Fatal("foreign interface connection accepted")
	}
	stored, err := database.GetValue(ctx, `SELECT interface_configuration_revision FROM plugin_idc_core_catalog_product_version_configuration WHERE version_no=?`, versionNo)
	if err != nil || stored.Int64() != 1 || old.InterfaceConfigurationRevision != 1 {
		t.Fatal("historical product or order facts were rewritten")
	}
}

const liveConnectionFixtureSQL = `
CREATE TABLE plugin_idc_core_product_automation_interface_instance(tenant_id INT,interface_instance_no TEXT,interface_origin_code TEXT,resource_owner_id TEXT);
CREATE TABLE plugin_idc_core_catalog_product_version_fulfillment_binding(tenant_id INT,version_no TEXT,interface_instance_no TEXT,interface_display_name_snapshot TEXT,plugin_id TEXT,extension_point_code TEXT,plugin_release_version TEXT,contract_version TEXT,authorized_methods TEXT[],release_digest TEXT,declaration_digest TEXT);
CREATE TABLE plugin_idc_core_catalog_product_version_configuration(tenant_id INT,version_no TEXT,interface_instance_no TEXT,interface_configuration_revision BIGINT,product_schema_version TEXT,product_declaration_snapshot TEXT,customer_schema_version TEXT,customer_declaration_snapshot TEXT,customer_presentation_snapshot TEXT,declaration_digest TEXT);
CREATE TABLE plugin_idc_core_product_automation_interface_configuration_revision(tenant_id INT,interface_instance_no TEXT,revision BIGINT,schema_version TEXT,declaration_snapshot TEXT,public_value_snapshot TEXT,declaration_digest TEXT,value_digest TEXT,resource_owner_configuration_revision TEXT);
`
