package resourceinterfaceprojection

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

func TestProjectionUpsertRenamesAndStopsWithAuthenticatedActorPostgreSQL(t *testing.T) {
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
	sink, err := New(database)
	if err != nil {
		t.Fatal(err)
	}
	projection := resourceinterfaceprojectioncap.Projection{
		InterfaceInstanceNo:            "pfi_11111111111111111111111111111111",
		InterfaceConfigurationRevision: "rci_11111111111111111111111111111111",
		DisplayName:                    "魔方云 A",
		ActorUserID:                    101,
		OwnerID:                        resourceinterfaceprojectioncap.OwnerID,
		ResourceChannelRef:             "CHN_A",
		PluginID:                       "idc-zjmfcloud",
		PluginReleaseVersion:           "1.0.0",
		ContractVersion:                "v1",
		AuthorizedMethods:              []string{"query", "provision"},
		ReleaseDigest:                  strings.Repeat("a", 64),
		DeclarationDigest:              strings.Repeat("b", 64),
		NewBusinessState:               resourceinterfaceprojectioncap.StateAccepting,
	}
	if err = sink.Upsert(ctx, projection); err != nil {
		t.Fatalf("create projection: %v", err)
	}
	projection.DisplayName = "魔方云正式接口"
	projection.NewBusinessState = resourceinterfaceprojectioncap.StateStopped
	projection.ActorUserID = 202
	if err = sink.Upsert(ctx, projection); err != nil {
		t.Fatalf("rename and stop projection: %v", err)
	}
	row, err := database.GetOne(ctx, `SELECT display_name,new_business_state,version,created_by,changed_by,resource_channel_ref,authorized_methods=ARRAY['provision','query']::TEXT[] AS methods_match FROM plugin_idc_core_product_automation_interface_instance WHERE interface_instance_no=?`, projection.InterfaceInstanceNo)
	if err != nil {
		t.Fatal(err)
	}
	if row["display_name"].String() != projection.DisplayName || row["new_business_state"].String() != resourceinterfaceprojectioncap.StateStopped ||
		row["version"].Int64() != 3 || row["created_by"].Int() != 101 || row["changed_by"].Int() != 202 || row["resource_channel_ref"].String() != "CHN_A" {
		t.Fatalf("unexpected projection row: %#v", row.Map())
	}
	if !row["methods_match"].Bool() {
		t.Fatalf("authorized methods were not stored as one two-element TEXT[]: %#v", row.Map())
	}
	if err = sink.Upsert(ctx, projection); err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	projection.InterfaceConfigurationRevision = "rci_22222222222222222222222222222222"
	if err = sink.Upsert(ctx, projection); err != nil {
		t.Fatalf("append resource configuration revision: %v", err)
	}
	var revisionRows []struct {
		ResourceOwnerConfigurationRevision string
	}
	if err = database.GetScan(ctx, &revisionRows, `SELECT resource_owner_configuration_revision FROM plugin_idc_core_product_automation_interface_configuration_revision WHERE interface_instance_no=? ORDER BY revision`, projection.InterfaceInstanceNo); err != nil {
		t.Fatal(err)
	}
	if len(revisionRows) != 2 || revisionRows[0].ResourceOwnerConfigurationRevision != "rci_11111111111111111111111111111111" || revisionRows[1].ResourceOwnerConfigurationRevision != projection.InterfaceConfigurationRevision {
		t.Fatalf("resource configuration history was not preserved: %#v", revisionRows)
	}
}

func quoteIdentifier(value string) string { return `"` + strings.ReplaceAll(value, `"`, `""`) + `"` }

func appendSearchPath(dsn, schema string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + "search_path=" + url.QueryEscape(schema)
}
