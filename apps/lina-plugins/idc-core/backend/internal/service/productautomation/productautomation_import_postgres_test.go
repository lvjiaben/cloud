// This file verifies partial import binding against the real owner migrations,
// including rollback and unchanged ordinary save/publication completeness gates.
package productautomation

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func TestImportedPartialBindingPostgreSQL(t *testing.T) {
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
	defer admin.Exec(ctx, `DROP SCHEMA `+quoteAutomationIdentifier(schemaName)+` CASCADE`)
	database := openAutomationDatabase(t, ctx, dsn, schemaName)
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_catalog_product_version (
 tenant_id INT NOT NULL DEFAULT 0, version_no VARCHAR(40) NOT NULL PRIMARY KEY,
 status_code VARCHAR(16) NOT NULL, UNIQUE(tenant_id, version_no))`); err != nil {
		t.Fatal(err)
	}
	_, filename, _, _ := runtime.Caller(0)
	for _, name := range []string{"074-product-automation-binding.sql", "075-structured-product-configuration.sql"} {
		migration, readErr := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql", name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if name == "075-structured-product-configuration.sql" {
			// This focused fixture does not create unrelated commerce sidecars.
			if sidecar := bytes.Index(migration, []byte("-- Generic transaction evidence is a sidecar")); sidecar >= 0 {
				migration = migration[:sidecar]
			}
		}
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	installResourceProjectionTestColumns(t, ctx, database)
	if _, err = database.Exec(ctx, `ALTER TABLE plugin_idc_core_product_automation_interface_configuration_revision ADD COLUMN resource_owner_configuration_revision VARCHAR(40) NOT NULL DEFAULT ''`); err != nil {
		t.Fatal(err)
	}
	label := &capmodel.LocalizedText{ZhCN: "处理器", EnUS: "CPU"}
	evidence := automationEvidence(nil)
	evidence.ProductConfiguration = &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "cpu", Type: capmodel.ConfigurationFieldInteger, Label: label, Required: true, Minimum: "1"},
		{Key: "upstream_product_id", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, Required: true,
			StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "101", Label: label}}},
	}}
	extensions := &automationTestExtensions{evidence: []*capmodel.BusinessExtensionProviderEvidence{evidence}}
	audit := &automationTestAudit{}
	service, err := newWithRandom(database, extensions, &automationTestAuthorizer{}, audit, &automationTestCodec{evidence: evidence}, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateInterface(ctx, CreateInterfaceInput{DisplayName: "Import supplier", SelectionToken: "opaque"})
	if err != nil {
		t.Fatal(err)
	}
	reference := "101"
	const ownerRevision = "rci_11111111111111111111111111111111"
	// Advance the focused 074/075 fixture to the resource-owned 097 shape.
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_product_automation_interface_instance SET interface_origin_code='resource',resource_owner_id='idc-resource',resource_channel_ref='CHN_import',display_name='Import resource supplier',version=version+1`); err != nil {
		t.Fatal(err)
	}
	declaration := `{"version":"resource-owner-v1"}`
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_product_automation_interface_configuration_revision(tenant_id,interface_instance_no,revision,schema_version,declaration_snapshot,public_value_snapshot,declaration_digest,value_digest,secret_field_count,created_by,resource_owner_configuration_revision) VALUES(0,?,2,'resource-owner-v1',CAST(? AS JSONB),'{}',?,?,0,101,?)`, created.InterfaceInstanceNo, declaration, configurationDigest([]byte(declaration)), configurationDigest([]byte(`{}`)), ownerRevision); err != nil {
		t.Fatal(err)
	}
	readEvidence := &resourceinterfaceprojectioncap.CatalogReadEvidence{InterfaceInstanceNo: created.InterfaceInstanceNo,
		OwnerID: resourceinterfaceprojectioncap.OwnerID, ResourceChannelRef: "CHN_import", InterfaceConfigurationRevision: ownerRevision,
		PluginID: evidence.PluginID, PluginReleaseVersion: evidence.ReleaseVersion, ContractVersion: evidence.ContractVersion,
		AuthorizedMethods: evidence.AuthorizedMethods, ReleaseDigest: evidence.ReleaseDigest, DeclarationDigest: evidence.DeclarationDigest}
	staleRead := *readEvidence
	staleRead.InterfaceConfigurationRevision = "rci_22222222222222222222222222222222"
	if _, err = service.PrepareImportBinding(ctx, created.InterfaceInstanceNo, &staleRead, nil); !bizerr.Is(err, catalog.CodeManagementConflict) {
		t.Fatalf("source read and core projection revision mismatch was accepted: %v", err)
	}
	prepared, err := service.PrepareImportBinding(ctx, created.InterfaceInstanceNo, readEvidence, []resourceinterfaceprojectioncap.CatalogConfigurationValue{{FieldKey: "upstream_product_id", OptionValue: &reference}})
	if err != nil || len(prepared.MissingConfiguration) != 1 || prepared.MissingConfiguration[0].FieldKey != "cpu" {
		t.Fatalf("prepare partial binding: %+v %v", prepared, err)
	}
	const versionNo = "ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	changedPreparation := *prepared
	changedPreparation.readEvidence = staleRead
	called := false
	if err = service.WithImportBinding(ctx, &changedPreparation, func(context.Context) (string, error) { called = true; return "", nil }); !bizerr.Is(err, catalog.CodeManagementConflict) || called {
		t.Fatalf("commit admitted a different read revision: %v", err)
	}
	action := func(txCtx context.Context) (string, error) {
		_, insertErr := database.Exec(txCtx, `INSERT INTO plugin_idc_core_catalog_product_version VALUES (0,?,'draft')`, versionNo)
		return versionNo, insertErr
	}
	audit.err = errors.New("force binding audit rollback")
	if err = service.WithImportBinding(ctx, prepared, action); err == nil {
		t.Fatal("failed binding audit committed import")
	}
	count, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_catalog_product_version`)
	if err != nil || count.Int() != 0 {
		t.Fatalf("binding failure left draft: %v %v", count, err)
	}
	audit.err = nil
	if err = service.WithImportBinding(ctx, prepared, action); err != nil {
		t.Fatal(err)
	}
	stored, err := database.GetValue(ctx, `SELECT merchant_value_snapshot FROM plugin_idc_core_catalog_product_version_configuration WHERE version_no=?`, versionNo)
	if err != nil || strings.ReplaceAll(stored.String(), " ", "") != `{"upstream_product_id":"101"}` {
		t.Fatalf("partial snapshot guessed configuration: %v %v", stored, err)
	}
	values := []ConfigurationValue{{FieldKey: "upstream_product_id", OptionTokens: []string{reference}}}
	if err = service.SaveVersionConfiguration(ctx, SaveVersionConfigurationInput{VersionNo: versionNo, ExpectedVersion: 1, MerchantValues: values}); !bizerr.Is(err, catalog.CodeManagementInvalid) {
		t.Fatalf("ordinary save accepted missing CPU: %v", err)
	}
	published := false
	err = service.WithPublicationAdmission(ctx, versionNo, func(context.Context) error { published = true; return nil })
	if !bizerr.Is(err, catalog.CodeManagementConflict) || published {
		t.Fatalf("publication accepted partial configuration: %v", err)
	}
	if extensions.invokeCalls != 0 {
		t.Fatal("static proven configuration unexpectedly called supplier")
	}
}
