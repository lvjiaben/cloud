// This file exercises portable import preparation, atomic binding, detail read,
// immutable configuration and publication admission with real PostgreSQL.

package productautomation

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func TestPortableBindingPublicationPostgreSQL(t *testing.T) {
	for _, scenario := range []struct{ ongoing, emptyCustomer bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		ongoing := scenario.ongoing
		name := map[bool]string{false: "one_time_delivery", true: "renewable_service"}[ongoing] + map[bool]string{false: "_customer_fields", true: "_empty_customer"}[scenario.emptyCustomer]
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			db := portableBindingDatabase(t, ctx)
			exact := automationEvidence([]string{"provision", "query.status", "renew"})
			exact.PluginID = portableImportProviderID
			declaration := testFulfillmentDeclaration()
			declaration.OngoingManagement = true
			declaration.EntitlementOwnerID = "idc-core"
			declaration.RenewalSupport = productfulfillmentcap.RenewalSupportSupported
			declaration.ExpiryManagement = productfulfillmentcap.ExpiryManagementProvider
			declaration.MethodRoles.Renewal = "renew"
			exact.ProviderMetadata = mustTestProviderMetadata(t, declaration)
			label := &capmodel.LocalizedText{ZhCN: "上游商品", EnUS: "Supplier product"}
			exact.ProductConfiguration = &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "upstream_public_slug", Type: capmodel.ConfigurationFieldSingleLine, Label: label, Required: true}, {Key: "upstream_configuration_revision", Type: capmodel.ConfigurationFieldSingleLine, Label: label, Required: true}}}
			exts := &automationTestExtensions{evidence: []*capmodel.BusinessExtensionProviderEvidence{exact}}
			svc, err := newWithRandom(db, exts, &automationTestAuthorizer{}, &automationTestAudit{}, &automationTestCodec{evidence: exact}, rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			created, err := svc.CreateInterface(ctx, CreateInterfaceInput{DisplayName: "Peer supplier", SelectionToken: "opaque"})
			if err != nil {
				t.Fatal(err)
			}
			ownerRevision := "rci_" + strings.Repeat("1", 32)
			if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_product_automation_interface_instance SET interface_origin_code='resource',resource_owner_id='idc-resource',resource_channel_ref='CHN_import',display_name='Imported peer supplier',version=version+1`); err != nil {
				t.Fatal(err)
			}
			ownerDeclaration := `{"version":"resource-owner-v1"}`
			if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_product_automation_interface_configuration_revision(tenant_id,interface_instance_no,revision,schema_version,declaration_snapshot,public_value_snapshot,declaration_digest,value_digest,secret_field_count,created_by,resource_owner_configuration_revision) VALUES(0,?,2,'resource-owner-v1',CAST(? AS JSONB),'{}',?,?,0,101,?)`, created.InterfaceInstanceNo, ownerDeclaration, configurationDigest([]byte(ownerDeclaration)), configurationDigest([]byte(`{}`)), ownerRevision); err != nil {
				t.Fatal(err)
			}
			read := &resourceinterfaceprojectioncap.CatalogReadEvidence{InterfaceInstanceNo: created.InterfaceInstanceNo, OwnerID: resourceinterfaceprojectioncap.OwnerID, ResourceChannelRef: "CHN_import", InterfaceConfigurationRevision: ownerRevision, PluginID: exact.PluginID, PluginReleaseVersion: exact.ReleaseVersion, ContractVersion: exact.ContractVersion, AuthorizedMethods: exact.AuthorizedMethods, ReleaseDigest: exact.ReleaseDigest, DeclarationDigest: exact.DeclarationDigest}
			snapshot := portableAutomationSnapshot()
			if scenario.emptyCustomer {
				snapshot.Schema = nil
				snapshot.Mappings = nil
			}
			snapshot.Fulfillment.OngoingService = ongoing
			if ongoing {
				snapshot.Fulfillment.RenewalMode = productfulfillmentcap.RenewalModeLocalEntitlement
			}
			supplied := []resourceinterfaceprojectioncap.CatalogConfigurationValue{{FieldKey: "upstream_public_slug", TextValue: &snapshot.Product.PublicSlug}, {FieldKey: "upstream_configuration_revision", TextValue: &snapshot.ConfigurationRevision}}
			prepared, err := svc.PrepareImportBinding(ctx, created.InterfaceInstanceNo, read, supplied, snapshot)
			if err != nil {
				t.Fatalf("prepare: %v", err)
			}
			version := "ver_" + strings.Repeat("a", 32)
			encoded, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			readJSON, err := json.Marshal(read)
			if err != nil {
				t.Fatal(err)
			}
			err = svc.WithImportBinding(ctx, prepared, func(txCtx context.Context) (string, error) {
				if _, e := db.Exec(txCtx, `INSERT INTO plugin_idc_core_catalog_product_version(tenant_id,version_no,status_code) VALUES(0,?,'draft')`, version); e != nil {
					return "", e
				}
				_, e := db.Exec(txCtx, `INSERT INTO plugin_idc_core_catalog_import_contract(tenant_id,version_no,interface_instance_no,snapshot,snapshot_digest,read_evidence) VALUES(0,?,?,CAST(? AS JSONB),?,CAST(? AS JSONB))`, version, created.InterfaceInstanceNo, string(encoded), configurationDigest(encoded), string(readJSON))
				return version, e
			})
			if err != nil {
				t.Fatalf("commit binding: %v", err)
			}
			configuration, err := svc.GetVersionConfiguration(ctx, version)
			if err != nil || configuration == nil || !configuration.ImportedConfiguration || (!scenario.emptyCustomer && configuration.CustomerDeclaration.Fields[0].Key != "region") {
				t.Fatalf("read imported declaration: %+v %v", configuration, err)
			}
			if err = svc.SaveVersionConfiguration(ctx, SaveVersionConfigurationInput{VersionNo: version, ExpectedVersion: 1}); err == nil {
				t.Fatal("import configuration mutated")
			}
			contract, err := svc.loadFrozenFulfillmentContract(ctx, version)
			if err != nil || contract.OngoingManagement != ongoing {
				t.Fatalf("contract: %+v %v", contract, err)
			}
			wantMode := productfulfillmentcap.RenewalModeUnavailable
			if ongoing {
				wantMode = productfulfillmentcap.RenewalModeProviderConfirmed
			}
			if contract.RenewalMode != wantMode {
				t.Fatalf("renewal mode: %s", contract.RenewalMode)
			}
			published := false
			if err = svc.WithPublicationAdmission(ctx, version, func(txCtx context.Context) error {
				published = true
				_, e := db.Exec(txCtx, `UPDATE plugin_idc_core_catalog_product_version SET status_code='published' WHERE version_no=?`, version)
				return e
			}); err != nil || !published {
				t.Fatalf("publish: %v", err)
			}
			if available, e := svc.CheckPublicPurchaseAvailability(ctx, version); e != nil || !available {
				t.Fatalf("published purchase readiness: %t %v", available, e)
			}
			verifyPortablePublicFreeze(t, ctx, db, version, exact, scenario.emptyCustomer)
		})
	}
}

type portablePublicRuntime struct{}

func (portablePublicRuntime) CheckPublicPurchaseAvailability(context.Context, string) (bool, error) {
	return true, nil
}
func (portablePublicRuntime) ResolvePublicPurchaseCandidates(context.Context, string, *capmodel.ConfigurationSchemaDeclaration, []configurationcontract.ConfigurationValue) (configurationcontract.ResolvedCandidates, error) {
	return configurationcontract.ResolvedCandidates{}, nil
}

func verifyPortablePublicFreeze(t *testing.T, ctx context.Context, db gdb.DB, version string, exact *capmodel.BusinessExtensionProviderEvidence, emptyCustomer bool) {
	t.Helper()
	if _, err := db.Exec(ctx, `ALTER TABLE plugin_idc_core_catalog_product_version ADD COLUMN product_no TEXT DEFAULT 'product-test';
CREATE TABLE plugin_idc_core_catalog_category(tenant_id INT,category_no TEXT,deleted_at TIMESTAMPTZ,status_code TEXT);
INSERT INTO plugin_idc_core_catalog_category VALUES(0,'category-test',NULL,'active');
CREATE TABLE plugin_idc_core_catalog_product(tenant_id INT,product_no TEXT,current_version_no TEXT,category_no TEXT,deleted_at TIMESTAMPTZ,status_code TEXT,public_slug TEXT);
CREATE TABLE plugin_idc_core_catalog_configuration_price_dimension(id BIGSERIAL,tenant_id INT,version_no TEXT,dimension_no TEXT,field_key TEXT,candidate_value TEXT,pricing_mode_code TEXT NOT NULL DEFAULT 'fixed',status_code TEXT);`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product VALUES(0,'product-test',?,'category-test',NULL,'active','portable-test')`, version); err != nil {
		t.Fatal(err)
	}
	files := &frozenCustomerFileVerifierStub{}
	reader, err := catalog.NewPublicCatalogReaderConfigured(db, []byte("0123456789abcdef0123456789abcdef"), portablePublicRuntime{}, files)
	if err != nil {
		t.Fatal(err)
	}
	view, err := reader.GetPurchaseConfiguration(ctx, "portable-test")
	if err != nil {
		t.Fatal(err)
	}
	input := catalog.PublicPurchaseConfigurationInput{PublicSlug: "portable-test", ConfigurationRevision: view.ConfigurationRevision}
	if !emptyCustomer {
		text := "test-region"
		input.Values = []catalog.PublicPurchaseConfigurationValue{{FieldToken: view.Fields[0].FieldToken, Type: view.Fields[0].Type, TextValue: &text}}
	}
	freezer := reader.(catalog.PurchaseConfigurationFreezer)
	frozen, err := freezer.FreezePurchaseConfiguration(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	var configuration entity.CatalogProductVersionConfiguration
	if err = db.Model("plugin_idc_core_catalog_product_version_configuration").Ctx(ctx).Where("version_no", version).Scan(&configuration); err != nil {
		t.Fatal(err)
	}
	if frozen.CustomerDeclarationDigest == configuration.DeclarationDigest {
		t.Fatal("customer digest incorrectly includes product declaration")
	}
	schema, err := configurationcontract.DecodeDeclaration(configuration.CustomerDeclarationSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	values, err := configurationcontract.DecodeCompiledValues(schema, frozen.TypedValueSnapshotJSON)
	if err != nil {
		t.Fatal(err)
	}
	var candidates configurationcontract.ResolvedCandidates
	if err = json.Unmarshal(frozen.CandidateSnapshotJSON, &candidates); err != nil {
		t.Fatal(err)
	}
	validator, err := NewFrozenCustomerConfigurationValidator(db, &automationTestExtensions{evidence: []*capmodel.BusinessExtensionProviderEvidence{exact}}, files)
	if err != nil {
		t.Fatal(err)
	}
	validation := FrozenCustomerConfigurationInput{OrderNo: "order-public-freeze", CustomerNo: "customer-public-freeze", UploaderUserID: 101, VersionNo: version, InterfaceInstanceNo: frozen.InterfaceInstanceNo, InterfaceConfigurationRevision: frozen.InterfaceConfigurationRevision, CustomerDeclarationDigest: frozen.CustomerDeclarationDigest, CandidateSnapshotDigest: frozen.CandidateSnapshotDigest, CustomerValueDigest: frozen.CustomerValueDigest, ConfigurationRevision: frozen.ConfigurationRevision, Values: values, Candidates: candidates}
	if result, e := validator.ValidateBatch(ctx, []FrozenCustomerConfigurationInput{validation}); e != nil || len(result) != 1 {
		t.Fatalf("public freeze rejected during fulfillment: %v", e)
	}
	validation.CustomerDeclarationDigest = configuration.DeclarationDigest
	if _, err = validator.ValidateBatch(ctx, []FrozenCustomerConfigurationInput{validation}); err == nil {
		t.Fatal("combined declaration digest bypassed fulfillment validation")
	}
	current, err := reader.GetPurchaseConfiguration(ctx, "portable-test")
	if err != nil || current.ConfigurationRevision != input.ConfigurationRevision {
		t.Fatalf("freezing changed combined declaration revision: %v", err)
	}
}

func portableBindingDatabase(t *testing.T, ctx context.Context) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if e := admin.Close(ctx); e != nil {
			t.Error(e)
		}
	})
	schema := newAutomationSchema(t)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteAutomationIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, e := admin.Exec(ctx, `DROP SCHEMA `+quoteAutomationIdentifier(schema)+` CASCADE`); e != nil {
			t.Error(e)
		}
	})
	db := openAutomationDatabase(t, ctx, dsn, schema)
	if _, err = db.Exec(ctx, `CREATE TABLE plugin_idc_core_catalog_product_version(tenant_id INT NOT NULL DEFAULT 0,version_no VARCHAR(40) PRIMARY KEY,status_code VARCHAR(16),UNIQUE(tenant_id,version_no))`); err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	for _, name := range []string{"074-product-automation-binding.sql", "075-structured-product-configuration.sql"} {
		raw, e := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../../manifest/sql", name))
		if e != nil {
			t.Fatal(e)
		}
		if i := bytes.Index(raw, []byte("-- Generic transaction evidence is a sidecar")); i >= 0 {
			raw = raw[:i]
		}
		if _, e = db.Exec(ctx, string(raw)); e != nil {
			t.Fatal(e)
		}
	}
	installResourceProjectionTestColumns(t, ctx, db)
	if _, err = db.Exec(ctx, `ALTER TABLE plugin_idc_core_product_automation_interface_configuration_revision ADD COLUMN resource_owner_configuration_revision VARCHAR(40) NOT NULL DEFAULT '';
CREATE TABLE plugin_idc_core_catalog_import_contract(id BIGSERIAL PRIMARY KEY,tenant_id INT,version_no VARCHAR(40),interface_instance_no VARCHAR(40),snapshot JSONB,snapshot_digest CHAR(64),read_evidence JSONB);
CREATE TABLE plugin_idc_core_catalog_billing_rule(tenant_id INT,version_no VARCHAR(40),charge_type_code VARCHAR(16),status_code VARCHAR(16));
CREATE TABLE plugin_idc_core_catalog_product_version_fulfillment_policy(tenant_id INT,version_no VARCHAR(40),strategy_code TEXT,granularity_code TEXT,ongoing_management BOOLEAN,renewal_mode_code TEXT,renewal_method TEXT,entitlement_owner_id TEXT,provision_method TEXT,query_method TEXT,maximum_attempts INT,retry_base_seconds INT,policy_digest CHAR(64),created_by INT,created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP);`); err != nil {
		t.Fatal(err)
	}
	return db
}
