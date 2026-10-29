// This file creates an isolated resource schema from its real migrations.
package customeroperations

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func newNativeDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_RESOURCE_TEST_DSN"))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL server directory tests", "LINAPRO_IDC_RESOURCE_TEST_DSN")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_native_customer_test_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		_ = admin.Close(ctx)
		t.Fatal(err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		_ = admin.Close(ctx)
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = database.Close(ctx)
		_, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
		_ = admin.Close(ctx)
	})
	for _, path := range nativeMigrationPaths(t) {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, err = database.Exec(ctx, string(content)); err != nil {
			t.Fatalf("apply migration %s: %v", filepath.Base(path), err)
		}
	}
	return ctx, database
}

func nativeMigrationPaths(t *testing.T) []string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve server directory test file")
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "[0-9][0-9][0-9]-*.sql"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("resolve resource migrations: %v", err)
	}
	return paths
}

func seedDirectoryFoundation(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_channel
    (channel_no,provider_id,provider_plugin_id,display_name,endpoint,status_code)
VALUES ('channel-main','provider-main','server-connector','主供应商渠道',
        'https://provider.invalid','enabled');
INSERT INTO plugin_idc_resource_pool
    (pool_no,channel_no,display_name,resource_kind,status_code)
VALUES ('pool-main','channel-main','云服务器资源池','virtual_server','enabled');
INSERT INTO plugin_idc_resource_capability_snapshot
    (snapshot_no,channel_no,adapter_version,status_code,item_count,observed_at,completed_at)
VALUES ('capability-main','channel-main','1.0.0','complete',3,
        CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_resource_capability_snapshot_item
    (snapshot_no,capability_code,support_code,unsupported_reason)
VALUES ('capability-main','server.start','supported',''),
       ('capability-main','server.reboot','conditional',''),
       ('capability-main','provider.secret.inspect','unsupported','not exposed');`); err != nil {
		t.Fatalf("seed server directory foundation: %v", err)
	}
}

func seedDirectoryServer(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	customerNo string,
	suffix string,
	mappingStatus string,
	taskStatus string,
) {
	t.Helper()
	entitlementNo := "ent_" + strings.Repeat(suffix, 32)
	resourceNo := "SRV_" + strings.Repeat(suffix, 32)
	releasedAt := any(nil)
	if mappingStatus == "released" {
		releasedAt = "2026-01-01T00:00:00Z"
	}
	finishedAt := any(nil)
	resultUnknown := false
	if taskStatus == "succeeded" || taskStatus == "failed" || taskStatus == "manual_takeover" || taskStatus == "canceled" {
		finishedAt = "2026-01-01T00:00:00Z"
	}
	if taskStatus == "result_unknown" {
		resultUnknown = true
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_mapping
    (mapping_no,local_resource_no,channel_no,pool_no,external_resource_ref,status_code,released_at)
VALUES (?,?, 'channel-main','pool-main',?,?,?)`,
		"mapping-"+suffix, resourceNo, "external-"+suffix, mappingStatus, releasedAt); err != nil {
		t.Fatalf("seed server directory mapping %s: %v", suffix, err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_task
    (task_no,request_id_digest,capability_code,local_resource_no,channel_no,pool_no,
     status_code,finished_at,result_unknown)
VALUES (?,?,'server.provision',?,'channel-main','pool-main',?,?,?)`,
		"task-"+suffix, strings.Repeat(suffix, 64), resourceNo, taskStatus, finishedAt, resultUnknown); err != nil {
		t.Fatalf("seed server directory task %s: %v", suffix, err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_server_entitlement_projection
    (entitlement_no,source_intent_no,customer_no,product_no,product_version_no,
     product_name_zh_cn,product_name_en_us,local_resource_no,mapping_no,provider_task_no,
     interface_instance_no,provider_plugin_id,plugin_release_version,
     fulfillment_contract_version,server_contract_version,release_digest,declaration_digest)
VALUES (?,?,?,?,?,?,?,?,?,?,?,'server-connector','1.0.0','v1','v1',?,?)`,
		entitlementNo, "intent-"+suffix, customerNo, "product-"+suffix, "version-"+suffix,
		"客户商品 "+suffix, "Customer product "+suffix, resourceNo, "mapping-"+suffix,
		"task-"+suffix, "interface-"+suffix, strings.Repeat("a", 64), strings.Repeat("b", 64)); err != nil {
		t.Fatalf("seed server directory ownership %s: %v", suffix, err)
	}
}
