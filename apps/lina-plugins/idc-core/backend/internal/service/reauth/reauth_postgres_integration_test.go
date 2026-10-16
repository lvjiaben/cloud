// This file proves recent-authentication replay fencing against real PostgreSQL.

package reauth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-core/backend/internal/dao"
)

const reauthIntegrationDSNEnv = "LINAPRO_IDC_CORE_TEST_DSN"

func TestDAOReplayStorePostgreSQLConcurrentClaim(t *testing.T) {
	ctx := context.Background()
	configureReauthIntegrationDB(t, ctx)

	now := time.Now().UTC().Truncate(time.Millisecond)
	claim := ReplayClaim{
		NonceDigest:         digestNonce("database-concurrency-nonce"),
		Purpose:             PurposeBulkResource,
		TargetDigest:        DigestTarget([]byte("immutable-batch-preview")),
		SessionDigest:       digestSessionID("opaque-host-session"),
		UserID:              42,
		TenantID:            0,
		LoginAt:             now.Add(-30 * 24 * time.Hour),
		SessionConfirmation: true,
		IssuedAt:            now,
		ExpiresAt:           now.Add(2 * time.Minute),
	}

	const workers = 32
	var successes atomic.Int32
	errorsOut := make(chan error, workers)
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			// Separate store values model independent plugin service instances.
			claimed, err := NewDAOReplayStore().Claim(context.Background(), claim)
			if err != nil {
				errorsOut <- err
				return
			}
			if claimed {
				successes.Add(1)
			}
		}()
	}
	wait.Wait()
	close(errorsOut)
	for err := range errorsOut {
		t.Errorf("concurrent database claim failed: %v", err)
	}
	if got := successes.Load(); got != 1 {
		t.Fatalf("concurrent successful claims=%d, want exactly 1", got)
	}
	count, err := dao.RecentAuthConsumption.Ctx(ctx).
		Where(dao.RecentAuthConsumption.Columns().NonceDigest, claim.NonceDigest).
		Count()
	if err != nil || count != 1 {
		t.Fatalf("persisted replay rows=%d err=%v, want 1", count, err)
	}
	if _, err = NewDAOReplayStore().Claim(ctx, ReplayClaim{}); err == nil {
		t.Fatal("invalid replay claim must fail before database access")
	}
}

func configureReauthIntegrationDB(t *testing.T, ctx context.Context) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(reauthIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL recent-authentication integration tests", reauthIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create recent-authentication integration admin database: %v", err)
	}
	schema := newReauthIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteReauthPostgreSQLIdentifier(schema)); err != nil {
		_ = adminDB.Close(ctx)
		t.Fatalf("create isolated recent-authentication schema: %v", err)
	}
	originalConfig := gdb.GetAllConfig()
	var scopedDB gdb.DB
	t.Cleanup(func() {
		if scopedDB != nil {
			if closeErr := scopedDB.Close(ctx); closeErr != nil {
				t.Errorf("close scoped recent-authentication database: %v", closeErr)
			}
		}
		if restoreErr := gdb.SetConfig(originalConfig); restoreErr != nil {
			t.Errorf("restore recent-authentication database config: %v", restoreErr)
		}
		if !strings.HasPrefix(schema, "linapro_reauth_test_") {
			t.Errorf("refusing to drop unexpected recent-authentication schema %q", schema)
		} else if _, dropErr := adminDB.Exec(
			ctx,
			"DROP SCHEMA "+quoteReauthPostgreSQLIdentifier(schema)+" CASCADE",
		); dropErr != nil {
			t.Errorf("drop isolated recent-authentication schema: %v", dropErr)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close recent-authentication admin database: %v", closeErr)
		}
	})
	if err = gdb.SetConfig(gdb.Config{
		gdb.DefaultGroupName: gdb.ConfigGroup{{Link: appendReauthDSNParameter(dsn, "search_path="+url.QueryEscape(schema))}},
	}); err != nil {
		t.Fatalf("configure recent-authentication integration database: %v", err)
	}
	scopedDB = g.DB()
	migration, err := os.ReadFile(reauthMigrationPath(t))
	if err != nil {
		t.Fatalf("read recent-authentication migration: %v", err)
	}
	if _, err = scopedDB.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("execute recent-authentication migration: %v", err)
	}
	if _, err = scopedDB.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("repeat recent-authentication migration idempotently: %v", err)
	}
	policy, err := os.ReadFile(filepath.Join(filepath.Dir(reauthMigrationPath(t)), "128-interface-delete-session-operations.sql"))
	if err != nil {
		t.Fatal(err)
	}
	policySQL := strings.Split(string(policy), "-- Interface configuration cleanup.")[0]
	for range 2 {
		if _, err = scopedDB.Exec(ctx, policySQL); err != nil {
			t.Fatal(err)
		}
	}
	currentSchema, err := scopedDB.GetValue(ctx, "SELECT current_schema()")
	if err != nil || currentSchema.String() != schema {
		t.Fatalf("recent-authentication search path=%q want=%q err=%v", currentSchema.String(), schema, err)
	}
}

func newReauthIntegrationSchema(t *testing.T) string {
	t.Helper()
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatalf("generate recent-authentication schema suffix: %v", err)
	}
	return "linapro_reauth_test_" + hex.EncodeToString(suffix[:])
}

func quoteReauthPostgreSQLIdentifier(value string) string {
	for _, character := range value {
		if character == '_' || character >= 'a' && character <= 'z' ||
			character >= '0' && character <= '9' {
			continue
		}
		panic(fmt.Sprintf("unsafe PostgreSQL identifier %q", value))
	}
	return `"` + value + `"`
}

func appendReauthDSNParameter(dsn string, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

func reauthMigrationPath(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve recent-authentication integration test path")
	}
	return filepath.Clean(filepath.Join(
		filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql", "004-security-context.sql",
	))
}
