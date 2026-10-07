package serverdirectory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

const directoryTestDSNEnv = "LINAPRO_IDC_RESOURCE_TEST_DSN"

func TestServerDirectoryPostgreSQLCustomerIsolationPaginationAndSafeProjection(t *testing.T) {
	ctx, database := newDirectoryDatabase(t)
	seedDirectoryFoundation(t, ctx, database)
	seedDirectoryServer(t, ctx, database, "customer-a", "1", "active", "succeeded")
	seedDirectoryServer(t, ctx, database, "customer-a", "2", "released", "running")
	seedDirectoryServer(t, ctx, database, "customer-a", "3", "quarantined", "result_unknown")
	seedDirectoryServer(t, ctx, database, "customer-b", "4", "active", "succeeded")

	counter := &countingScanner{DB: database}
	service, err := newWithScanner(counter)
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.List(ctx, ListRequest{
		Scope: TrustedCustomerScope{CustomerNo: "customer-a"}, Limit: 2,
	})
	if err != nil {
		t.Fatalf("list first server page: %v", err)
	}
	if counter.calls.Load() != 1 || len(first.Items) != 2 || first.NextCursor == "" {
		t.Fatalf("first page=%+v database calls=%d", first, counter.calls.Load())
	}
	counter.calls.Store(0)
	second, err := service.List(ctx, ListRequest{
		Scope: TrustedCustomerScope{CustomerNo: "customer-a"}, Cursor: first.NextCursor, Limit: 2,
	})
	if err != nil {
		t.Fatalf("list second server page: %v", err)
	}
	if counter.calls.Load() != 1 || len(second.Items) != 1 || second.NextCursor != "" {
		t.Fatalf("second page=%+v database calls=%d", second, counter.calls.Load())
	}
	seen := make(map[string]struct{}, 3)
	all := append(append([]Item{}, first.Items...), second.Items...)
	var released bool
	for _, item := range all {
		if _, duplicated := seen[item.ServerNo]; duplicated {
			t.Fatalf("stable cursor returned duplicate item %q", item.ServerNo)
		}
		seen[item.ServerNo] = struct{}{}
		if !serverNoPattern.MatchString(item.ServerNo) || !actionPattern.MatchString(item.ActionReference) ||
			item.ServerNo == item.ActionReference ||
			!strings.HasPrefix(item.ProductNameZhCN, "客户商品") ||
			!strings.HasPrefix(item.ProductNameEnUS, "Customer product") ||
			item.ChannelName != "主供应商渠道" || item.PoolName != "云服务器资源池" ||
			len(item.AvailableActions) != 2 || item.AvailableActions[0] != ActionReboot ||
			item.AvailableActions[1] != ActionStart {
			t.Fatalf("unsafe or incomplete server projection: %+v", item)
		}
		if strings.Contains(fmt.Sprintf("%+v", item), "ent_") ||
			strings.Contains(fmt.Sprintf("%+v", item), "mapping-") ||
			strings.Contains(fmt.Sprintf("%+v", item), "provider-") {
			t.Fatalf("raw owner/provider reference escaped directory projection: %+v", item)
		}
		if item.State == ResourceStateReleased {
			released = true
		}
	}
	if len(seen) != 3 || !released {
		t.Fatalf("directory items=%+v released mapping was not safely projected", all)
	}

	target := first.Items[0].ServerNo
	counter.calls.Store(0)
	detail, err := service.Detail(ctx, DetailRequest{
		Scope: TrustedCustomerScope{CustomerNo: "customer-a"}, ServerNo: target,
	})
	if err != nil || detail.ServerNo != target || counter.calls.Load() != 1 {
		t.Fatalf("owned detail=%+v err=%v calls=%d", detail, err, counter.calls.Load())
	}
	_, foreignErr := service.Detail(ctx, DetailRequest{
		Scope: TrustedCustomerScope{CustomerNo: "customer-b"}, ServerNo: target,
	})
	_, absentErr := service.Detail(ctx, DetailRequest{
		Scope:    TrustedCustomerScope{CustomerNo: "customer-b"},
		ServerNo: "SVR-FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF",
	})
	if !errors.Is(foreignErr, ErrNotFound) || !errors.Is(absentErr, ErrNotFound) ||
		foreignErr.Error() != absentErr.Error() {
		t.Fatalf("cross-customer existence leaked: foreign=%v absent=%v", foreignErr, absentErr)
	}
	if _, err = service.List(ctx, ListRequest{
		Scope: TrustedCustomerScope{CustomerNo: "customer-a"}, Limit: MaximumPageSize + 1,
	}); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("oversized page returned %v", err)
	}
}

func TestServerDirectoryPostgreSQLUnknownStatesMigrationReplayAndIndexPlan(t *testing.T) {
	ctx, database := newDirectoryDatabase(t)
	seedDirectoryFoundation(t, ctx, database)
	seedDirectoryServer(t, ctx, database, "customer-a", "9", "active", "succeeded")

	before, err := database.GetValue(ctx, `SELECT action_reference
FROM plugin_idc_resource_server_entitlement_projection
WHERE customer_no='customer-a'`)
	if err != nil || !actionPattern.MatchString(before.String()) {
		t.Fatalf("generated action reference=%q err=%v", before.String(), err)
	}
	migration := readDirectoryMigration(t, "027-server-entitlement-action-reference.sql")
	for attempt := 0; attempt < 2; attempt++ {
		if _, err = database.Exec(ctx, migration); err != nil {
			t.Fatalf("replay action-reference migration %d: %v", attempt+1, err)
		}
	}
	after, err := database.GetValue(ctx, `SELECT action_reference
FROM plugin_idc_resource_server_entitlement_projection
WHERE customer_no='customer-a'`)
	if err != nil || before.String() != after.String() {
		t.Fatalf("action reference changed across migration replay: before=%q after=%q err=%v",
			before.String(), after.String(), err)
	}
	publicBefore, err := database.GetValue(ctx, `SELECT server_no
FROM plugin_idc_resource_server_entitlement_projection
WHERE customer_no='customer-a'`)
	if err != nil || !serverNoPattern.MatchString(publicBefore.String()) || publicBefore.String() == before.String() {
		t.Fatalf("public server number=%q action reference=%q err=%v", publicBefore.String(), before.String(), err)
	}
	publicMigration := readDirectoryMigration(t, "028-server-public-number.sql")
	for attempt := 0; attempt < 2; attempt++ {
		if _, err = database.Exec(ctx, publicMigration); err != nil {
			t.Fatalf("replay public-server-number migration %d: %v", attempt+1, err)
		}
	}
	publicAfter, err := database.GetValue(ctx, `SELECT server_no
FROM plugin_idc_resource_server_entitlement_projection
WHERE customer_no='customer-a'`)
	if err != nil || publicBefore.String() != publicAfter.String() {
		t.Fatalf("public server number changed across migration replay: before=%q after=%q err=%v",
			publicBefore.String(), publicAfter.String(), err)
	}

	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_channel SET status_code='disabled';
UPDATE plugin_idc_resource_pool SET status_code='disabled'`); err != nil {
		t.Fatalf("stop new sales for resource source: %v", err)
	}
	service, err := New(database)
	if err != nil {
		t.Fatal(err)
	}
	stillVisible, err := service.List(ctx, ListRequest{
		Scope: TrustedCustomerScope{CustomerNo: "customer-a"}, Limit: 1,
	})
	if err != nil || len(stillVisible.Items) != 1 || stillVisible.Items[0].ServerNo != publicBefore.String() {
		t.Fatalf("existing server disappeared after new-sales source was disabled: page=%+v err=%v", stillVisible, err)
	}

	if _, err = database.Exec(ctx, `
ALTER TABLE plugin_idc_resource_mapping
    DROP CONSTRAINT ck_plugin_idc_resource_mapping_status;
ALTER TABLE plugin_idc_resource_task
    DROP CONSTRAINT ck_plugin_idc_resource_task_status,
    DROP CONSTRAINT ck_plugin_idc_resource_task_unknown,
    DROP CONSTRAINT ck_plugin_idc_resource_task_finish;
UPDATE plugin_idc_resource_mapping SET status_code='future_mapping' WHERE mapping_no='mapping-9';
UPDATE plugin_idc_resource_task SET status_code='future_operation' WHERE task_no='task-9';`); err != nil {
		t.Fatalf("stage future provider states: %v", err)
	}
	service, err = New(database)
	page, err := service.List(ctx, ListRequest{
		Scope: TrustedCustomerScope{CustomerNo: "customer-a"}, Limit: 1,
	})
	if err != nil || len(page.Items) != 1 || page.Items[0].State != ResourceStateUnknown ||
		page.Items[0].LatestOperation != OperationStateUnknown {
		t.Fatalf("future states were not safely projected: page=%+v err=%v", page, err)
	}

	if _, err = database.Exec(ctx, `SET enable_seqscan=off`); err != nil {
		t.Fatal(err)
	}
	planRows, err := database.GetAll(ctx, `EXPLAIN (FORMAT TEXT)
SELECT server_no
FROM plugin_idc_resource_server_entitlement_projection
WHERE tenant_id=0 AND customer_no=? AND server_no=?
LIMIT 2`, "customer-a", publicBefore.String())
	if err != nil {
		t.Fatalf("explain customer ownership lookup: %v", err)
	}
	planParts := make([]string, 0, len(planRows))
	for _, row := range planRows {
		planParts = append(planParts, row["QUERY PLAN"].String())
	}
	plan := strings.Join(planParts, "\n")
	if strings.Contains(plan, "Seq Scan") ||
		!strings.Contains(plan, "idx_plugin_idc_resource_server_customer_public_no") {
		t.Fatalf("customer ownership lookup does not use bounded index: %s", plan)
	}
}

type countingScanner struct {
	gdb.DB
	calls atomic.Int64
}

func (scanner *countingScanner) GetScan(
	ctx context.Context,
	pointer any,
	sql string,
	args ...any,
) error {
	scanner.calls.Add(1)
	return scanner.DB.GetScan(ctx, pointer, sql, args...)
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
		"mapping-"+suffix, entitlementNo, "external-"+suffix, mappingStatus, releasedAt); err != nil {
		t.Fatalf("seed server directory mapping %s: %v", suffix, err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_task
    (task_no,request_id_digest,capability_code,local_resource_no,channel_no,pool_no,
     status_code,finished_at,result_unknown)
VALUES (?,?,'server.provision',?,'channel-main','pool-main',?,?,?)`,
		"task-"+suffix, strings.Repeat(suffix, 64), entitlementNo, taskStatus, finishedAt, resultUnknown); err != nil {
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
		"客户商品 "+suffix, "Customer product "+suffix, entitlementNo, "mapping-"+suffix,
		"task-"+suffix, "interface-"+suffix, strings.Repeat("a", 64), strings.Repeat("b", 64)); err != nil {
		t.Fatalf("seed server directory ownership %s: %v", suffix, err)
	}
}

func newDirectoryDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(directoryTestDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL server directory tests", directoryTestDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_resource_directory_test_" + hex.EncodeToString(random[:])
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
	for _, path := range directoryMigrationPaths(t) {
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

func directoryMigrationPaths(t *testing.T) []string {
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

func readDirectoryMigration(t *testing.T, name string) string {
	t.Helper()
	for _, path := range directoryMigrationPaths(t) {
		if filepath.Base(path) == name {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			return string(content)
		}
	}
	t.Fatalf("migration %s is missing", name)
	return ""
}
