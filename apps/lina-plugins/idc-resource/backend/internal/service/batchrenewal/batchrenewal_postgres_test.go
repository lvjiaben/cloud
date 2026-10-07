package batchrenewal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

const batchRenewalTestDSNEnv = "LINAPRO_IDC_RESOURCE_TEST_DSN"

func TestPostgreSQLStoreListsAndLoadsOnlyResourceOwnedProjection(t *testing.T) {
	ctx, database := newBatchRenewalDatabase(t)
	for index, serverNo := range []string{serverA, serverB, "SVR-CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC"} {
		_, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_server_entitlement_projection
(tenant_id,server_no,entitlement_no,customer_no,product_name_zh_cn,product_name_en_us,version)
VALUES(0,?,?,?,?,?,?)`, serverNo, "ent_"+strings.Repeat(string(rune('a'+index)), 32),
			"CUS_"+string(rune('A'+index)), "服务器"+string(rune('一'+index)), "Server", index+1)
		if err != nil {
			t.Fatal(err)
		}
	}
	store := &sqlStore{database: database}
	first, err := store.List(ctx, ListInput{Limit: 2})
	if err != nil || len(first.Items) != 2 || first.NextCursor != serverB {
		t.Fatalf("first page=%+v err=%v", first, err)
	}
	second, err := store.List(ctx, ListInput{Cursor: first.NextCursor, Limit: 2})
	if err != nil || len(second.Items) != 1 || second.NextCursor != "" {
		t.Fatalf("second page=%+v err=%v", second, err)
	}
	loaded, err := store.Load(ctx, []string{serverB, serverA})
	if err != nil || len(loaded) != 2 {
		t.Fatalf("loaded=%+v err=%v", loaded, err)
	}
	for _, item := range loaded {
		if item.EntitlementNo == "" || item.CustomerNo == "" || item.ProjectionVersion <= 0 {
			t.Fatalf("incomplete owned projection: %+v", item)
		}
	}
}

func TestRenewalNumericPagesPostgreSQL(t *testing.T) {
	ctx, database := newBatchRenewalDatabase(t)
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_server_entitlement_projection
(tenant_id,server_no,entitlement_no,customer_no,product_name_zh_cn,product_name_en_us,version)
SELECT 0,'SVR-'||lpad(n::text,32,'0'),'ent_'||n,'CUS_test','测试服务器','Test server',1 FROM generate_series(1,121) n`); err != nil {
		t.Fatal(err)
	}
	store := &sqlStore{database: database}
	for _, size := range []int{10, 20, 50, 100} {
		page, err := store.List(ctx, ListInput{Page: 2, Limit: size})
		if err != nil || page.Total != 121 || len(page.Items) != min(size, 121-size) || page.Items[0].ServerNo != fmt.Sprintf("SVR-%032d", size+1) {
			t.Fatalf("size %d: %+v / %v", size, page, err)
		}
	}
	tail, err := store.List(ctx, ListInput{Page: 99, Limit: 10})
	if err != nil || tail.Total != 121 || len(tail.Items) != 0 {
		t.Fatalf("tail %+v / %v", tail, err)
	}
}

func newBatchRenewalDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(batchRenewalTestDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL batch renewal tests", batchRenewalTestDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_batch_renewal_test_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = database.Close(ctx)
		_, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
		_ = admin.Close(ctx)
	})
	_, err = database.Exec(ctx, `CREATE TABLE plugin_idc_resource_server_entitlement_projection (
tenant_id INT NOT NULL, server_no VARCHAR(36) NOT NULL, entitlement_no VARCHAR(40) NOT NULL,
customer_no VARCHAR(40) NOT NULL, product_name_zh_cn VARCHAR(256), product_name_en_us VARCHAR(256), version BIGINT NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, database
}
