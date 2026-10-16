// These tests exercise real core and resource transactions, migration guards,
// historical-reference refusal and immutable revision retention in PostgreSQL.
package resourceinterfaceprojection

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func newRefreshDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_CORE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := "idc_resource_projection_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+quoteIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, `DROP SCHEMA `+quoteIdentifier(schema)+` CASCADE`)
		_ = admin.Close(ctx)
	})
	database, err := gdb.New(gdb.ConfigNode{Link: appendSearchPath(dsn, schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })
	if _, err = database.Exec(ctx, `
CREATE TABLE plugin_idc_core_catalog_product_version (
 version_no VARCHAR(40) PRIMARY KEY, tenant_id INT NOT NULL DEFAULT 0, status_code VARCHAR(16) NOT NULL DEFAULT 'draft'
);
CREATE TABLE plugin_idc_core_product_fulfillment_task (task_no VARCHAR(40) PRIMARY KEY);`); err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	pluginRoot := filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
	for _, name := range []string{"074-product-automation-binding.sql"} {
		migration, readErr := os.ReadFile(filepath.Join(pluginRoot, "manifest", "sql", name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, execErr := database.Exec(ctx, string(migration)); execErr != nil {
			t.Fatalf("apply %s: %v", name, execErr)
		}
	}
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_product_automation_interface_configuration_revision (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id INT NOT NULL DEFAULT 0,
 interface_instance_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_product_automation_interface_instance(interface_instance_no),
 revision BIGINT NOT NULL,
 schema_version VARCHAR(32) NOT NULL,
 declaration_snapshot JSONB NOT NULL,
 public_value_snapshot JSONB NOT NULL,
 declaration_digest CHAR(64) NOT NULL,
 value_digest CHAR(64) NOT NULL,
 secret_field_count INT NOT NULL DEFAULT 0,
 created_by INT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE(tenant_id,interface_instance_no,revision)
);`); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile(filepath.Join(pluginRoot, "manifest", "sql", "096-resource-owned-product-fulfillment.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply 096-resource-owned-product-fulfillment.sql: %v", err)
	}
	migration, err = os.ReadFile(filepath.Join(pluginRoot, "manifest", "sql", "097-resource-interface-configuration-revision.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("apply 097-resource-interface-configuration-revision.sql: %v", err)
	}
	if _, err = database.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("reapply 097-resource-interface-configuration-revision.sql: %v", err)
	}

	return ctx, database
}

func TestRefreshUnusedResourceEvidencePostgreSQL(t *testing.T) {
	ctx, database := newRefreshDatabase(t)
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate migration")
	}
	coreRoot := filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
	resourcePaths, err := filepath.Glob(filepath.Join(coreRoot, "..", "idc-resource", "manifest", "sql", "[0-9][0-9][0-9]-*.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range resourcePaths {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, execErr := database.Exec(ctx, string(migration)); execErr != nil {
			t.Fatalf("resource fixture %s: %v", filepath.Base(path), execErr)
		}
	}
	for _, table := range []string{"catalog_product_version_configuration", "catalog_import_contract", "catalog_product_import", "service_entitlement", "product_fulfillment_intent"} {
		if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_`+table+`(tenant_id INT,interface_instance_no TEXT)`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_core_service_instance(tenant_id INT,resource_owner_code TEXT,channel_no TEXT); INSERT INTO plugin_idc_resource_channel(channel_no,provider_id,provider_plugin_id,display_name,endpoint,status_code) VALUES('CHN_refresh','mofang','idc-zjmfcloud','Probe','https://supplier.example','disabled')`); err != nil {
		t.Fatal(err)
	}
	sink, err := New(database)
	if err != nil {
		t.Fatal(err)
	}
	projection := resourceinterfaceprojectioncap.Projection{InterfaceInstanceNo: "pfi_11111111111111111111111111111111", InterfaceConfigurationRevision: "rci_11111111111111111111111111111111", DisplayName: "Probe", ActorUserID: 101, OwnerID: resourceinterfaceprojectioncap.OwnerID, ResourceChannelRef: "CHN_refresh", PluginID: "idc-zjmfcloud", PluginReleaseVersion: "v1.0.0", ContractVersion: "v1", AuthorizedMethods: []string{"query", "provision"}, ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64), NewBusinessState: resourceinterfaceprojectioncap.StateStopped}
	if err = sink.Upsert(ctx, projection); err != nil {
		t.Fatal(err)
	}
	resourceDB, err := gdb.New(*database.GetConfig())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := resourceDB.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	callbackCalls := 0
	apply := func(txCtx context.Context) error {
		callbackCalls++
		return resourceDB.Transaction(txCtx, func(innerCtx context.Context, _ gdb.TX) error {
			_, writeErr := resourceDB.Exec(innerCtx, `UPDATE plugin_idc_resource_channel SET display_name='Refreshed' WHERE channel_no='CHN_refresh'`)
			return writeErr
		})
	}
	projection.PluginReleaseVersion = "v2.0.0"
	projection.ReleaseDigest = strings.Repeat("c", 64)
	projection.DeclarationDigest = strings.Repeat("d", 64)
	projection.InterfaceConfigurationRevision = "rci_22222222222222222222222222222222"
	// The old immutable-evidence trigger rejects after the actual resource DB
	// callback commits its nested transaction; the outer transaction must undo it.
	if err = sink.RefreshUnused(ctx, projection, apply); err == nil {
		t.Fatal("old trigger unexpectedly accepted evidence update")
	}
	name, err := database.GetValue(ctx, `SELECT display_name FROM plugin_idc_resource_channel WHERE channel_no='CHN_refresh'`)
	if err != nil || name.String() != "Probe" || callbackCalls != 1 {
		t.Fatalf("cross-owner transaction did not roll back: name=%s calls=%d err=%v", name.String(), callbackCalls, err)
	}
	migration, err := os.ReadFile(filepath.Join(coreRoot, "manifest", "sql", "130-unused-resource-interface-refresh.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	if err = sink.RefreshUnused(ctx, projection, apply); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	stored, err := database.GetOne(ctx, `SELECT plugin_release_version,ever_referenced,new_business_state FROM plugin_idc_core_product_automation_interface_instance WHERE interface_instance_no=?`, projection.InterfaceInstanceNo)
	count, countErr := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_product_automation_interface_configuration_revision WHERE interface_instance_no=?`, projection.InterfaceInstanceNo)
	if err != nil || countErr != nil || stored["plugin_release_version"].String() != "v2.0.0" || stored["ever_referenced"].Bool() || stored["new_business_state"].String() != "stopped" || count.Int() != 2 {
		t.Fatalf("refresh/history state wrong: %v %v %v", stored, count, err)
	}
	// Current usage counts can be zero while historical/import/draft rows exist.
	// Every durable core reference independently blocks the owner callback.
	projection.PluginReleaseVersion = "v3.0.0"
	projection.ReleaseDigest = strings.Repeat("e", 64)
	for _, table := range []string{"catalog_product_version_configuration", "catalog_import_contract", "catalog_product_import", "service_entitlement", "product_fulfillment_intent"} {
		if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_`+table+`(tenant_id,interface_instance_no) VALUES(0,?)`, projection.InterfaceInstanceNo); err != nil {
			t.Fatal(err)
		}
		beforeCalls := callbackCalls
		if err = sink.RefreshUnused(ctx, projection, apply); !bizerr.Is(err, resourceinterfaceprojectioncap.CodeRefreshReferenced) || callbackCalls != beforeCalls {
			t.Fatalf("%s reference did not stop owner write: %v", table, err)
		}
		if _, err = database.Exec(ctx, `DELETE FROM plugin_idc_core_`+table); err != nil {
			t.Fatal(err)
		}
	}
	// Historical use is irreversible even when no current product remains.
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_product_automation_interface_instance SET ever_referenced=TRUE,version=version+1,changed_by=101 WHERE interface_instance_no=?`, projection.InterfaceInstanceNo); err != nil {
		t.Fatal(err)
	}
	projection.PluginReleaseVersion = "v3.0.0"
	projection.ReleaseDigest = strings.Repeat("e", 64)
	beforeCalls := callbackCalls
	if err = sink.RefreshUnused(ctx, projection, apply); !bizerr.Is(err, resourceinterfaceprojectioncap.CodeRefreshReferenced) || callbackCalls != beforeCalls {
		t.Fatalf("historical use did not refuse before owner write: %v", err)
	}
}
