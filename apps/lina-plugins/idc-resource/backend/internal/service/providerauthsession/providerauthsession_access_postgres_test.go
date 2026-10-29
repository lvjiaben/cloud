// Failed refresh cleanup uses a bounded independent context and releases only
// its exact generation; the fixture is isolated in a disposable test schema.
package providerauthsession

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

func TestCanceledAuthenticationRefreshReleasesOnlyItsOwnClaimPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_RESOURCE_TEST_DSN"))
	if dsn == "" {
		t.Skip("requires explicit resource test database")
	}
	ctx := t.Context()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal("test database setup failed")
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_auth_cleanup_test_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal("test schema creation failed")
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatal("test scoped database failed")
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		if closeErr := database.Close(cleanupCtx); closeErr != nil {
			t.Error("test database close failed")
		}
		if _, dropErr := admin.Exec(cleanupCtx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Error("test schema cleanup failed")
		}
		if closeErr := admin.Close(cleanupCtx); closeErr != nil {
			t.Error("test admin database close failed")
		}
	})
	if _, err = database.Exec(ctx, `CREATE TABLE plugin_idc_resource_channel(channel_no VARCHAR(64) PRIMARY KEY)`); err != nil {
		t.Fatal("parent fixture failed")
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("migration location unavailable")
	}
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "018-provider-authentication-session.sql"))
	if err != nil {
		t.Fatal("migration read failed")
	}
	if _, err = database.Exec(ctx, string(migration)); err != nil {
		t.Fatal("authentication migration failed")
	}
	channel := "auth-cleanup-test"
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_resource_channel(channel_no) VALUES (?)`, channel); err != nil {
		t.Fatal("channel fixture failed")
	}
	oldDigest, newDigest := strings.Repeat("a", 64), strings.Repeat("b", 64)
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_resource_provider_auth_session(channel_no,status_code,generation,lease_token_digest,lease_expires_at) VALUES (?,'refreshing',2,?,CURRENT_TIMESTAMP+INTERVAL '150 seconds')`, channel, oldDigest); err != nil {
		t.Fatal("claim fixture failed")
	}
	service := &serviceImpl{database: database, channelRef: channel}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	old := claim{generation: 2, leaseDigest: oldDigest}
	if err = service.releaseFailed(canceled, old); err != nil {
		t.Fatal("canceled login stranded its refresh claim")
	}
	status, err := database.GetValue(ctx, `SELECT status_code FROM plugin_idc_resource_provider_auth_session WHERE channel_no=?`, channel)
	if err != nil || status.String() != "idle" {
		t.Fatal("owned refresh claim was not released")
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_provider_auth_session SET status_code='refreshing',generation=3,lease_token_digest=?,lease_expires_at=CURRENT_TIMESTAMP+INTERVAL '150 seconds' WHERE channel_no=?`, newDigest, channel); err != nil {
		t.Fatal("successor fixture failed")
	}
	if err = service.releaseFailed(canceled, old); err != nil {
		t.Fatal("stale cleanup failed")
	}
	digest, err := database.GetValue(ctx, `SELECT lease_token_digest FROM plugin_idc_resource_provider_auth_session WHERE channel_no=? AND status_code='refreshing' AND generation=3`, channel)
	if err != nil || digest.String() != newDigest {
		t.Fatal("stale cleanup cleared another worker generation")
	}
}
