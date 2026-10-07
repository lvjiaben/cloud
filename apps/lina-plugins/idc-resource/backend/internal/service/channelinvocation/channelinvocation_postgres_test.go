package channelinvocation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

func TestChannelInvocationLeaseSerializesAcrossInstancesAndRecoversExpiry(t *testing.T) {
	ctx, database := newInvocationDatabase(t)
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_channel
    (channel_no,provider_id,provider_plugin_id,display_name,endpoint,status_code)
VALUES ('CHN_lease000000000001','server-api','server-connector','Lease','https://supplier.example','enabled')`); err != nil {
		t.Fatal(err)
	}
	first, _ := New(database)
	second, _ := New(database)
	lease, err := first.Acquire(ctx, "CHN_lease000000000001")
	if err != nil || lease == nil {
		t.Fatalf("first acquire: %v", err)
	}
	if competing, competingErr := second.Acquire(ctx, "CHN_lease000000000001"); competingErr == nil || competing != nil {
		t.Fatalf("competing acquire escaped serialization: lease=%+v err=%v", competing, competingErr)
	}
	if err = lease.Release(ctx); err != nil {
		t.Fatal(err)
	}
	released, err := second.Acquire(ctx, "CHN_lease000000000001")
	if err != nil || released == nil {
		t.Fatalf("acquire after release: %v", err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_channel_invocation_lease
SET lease_until=CURRENT_TIMESTAMP-INTERVAL '1 second'
WHERE channel_no='CHN_lease000000000001'`); err != nil {
		t.Fatal(err)
	}
	recovered, err := first.Acquire(ctx, "CHN_lease000000000001")
	if err != nil || recovered == nil {
		t.Fatalf("expired lease recovery: %v", err)
	}
	if err = released.Release(ctx); err != nil {
		t.Fatal(err)
	}
	var row struct {
		LeaseToken string `orm:"lease_token"`
	}
	if err = database.GetScan(ctx, &row, `SELECT lease_token FROM plugin_idc_resource_channel_invocation_lease WHERE channel_no='CHN_lease000000000001'`); err != nil || row.LeaseToken != recovered.token {
		t.Fatalf("stale release removed recovered lease: token=%q err=%v", row.LeaseToken, err)
	}
}

func newInvocationDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_RESOURCE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_RESOURCE_TEST_DSN to run PostgreSQL tests")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var suffix [8]byte
	_, _ = rand.Read(suffix[:])
	schema := "linapro_channel_invocation_test_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + map[bool]string{true: "&", false: "?"}[strings.Contains(dsn, "?")] + "search_path=" + schema})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = database.Close(ctx)
		_, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
		_ = admin.Close(ctx)
	})
	_, file, _, _ := runtime.Caller(0)
	paths, _ := filepath.Glob(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "[0-9][0-9][0-9]-*.sql"))
	for _, path := range paths {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, err = database.Exec(ctx, string(content)); err != nil {
			t.Fatalf("migration %s: %v", filepath.Base(path), err)
		}
	}
	return ctx, database
}
