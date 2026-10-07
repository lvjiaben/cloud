package backend

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestContactVerificationMigrationPostgreSQLIsIdempotentAndTenantBound(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(notificationIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL notification migration tests", notificationIntegrationDSNEnv)
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	schema := newNotificationSchema(t)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteNotificationIdentifier(schema)); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	defer func() {
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+quoteNotificationIdentifier(schema)+" CASCADE")
		_ = admin.Close(ctx)
	}()
	database, err := gdb.New(gdb.ConfigNode{Link: appendNotificationDSN(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatalf("open scoped PostgreSQL: %v", err)
	}
	defer database.Close(ctx)
	for _, name := range []string{"001-notification-foundation.sql", "002-notification-event-policy-materialization.sql"} {
		if _, err = database.Exec(ctx, readNotificationSQL(t, filepath.Join("manifest", "sql", name))); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	install := readNotificationSQL(t, filepath.Join("manifest", "sql", "004-notification-contact-verification.sql"))
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = database.Exec(ctx, install); err != nil {
			t.Fatalf("apply contact verification migration attempt %d: %v", attempt, err)
		}
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_notify_contact_verification
 (tenant_id,verification_no,purpose_code,operation_code,destination_kind,destination_digest,browser_digest,ticket_digest,
 locale_code,template_no,template_version,request_shape_digest,idempotency_digest,code_digest,masked_destination,status_code,
 delivery_state,retry_after_at,expires_at)
 VALUES (1,'nver_cross','support.presales','create','email',repeat('a',64),repeat('b',64),repeat('c',64),
 'zh-CN','ntpl_presales_verification_email_zh',1,repeat('d',64),repeat('e',64),repeat('f',64),'x***@example.com',
 'sent','accepted',CURRENT_TIMESTAMP+INTERVAL '1 minute',CURRENT_TIMESTAMP+INTERVAL '10 minutes')`); err == nil {
		t.Fatal("contact verification accepted a non-default tenant")
	}
	for _, table := range []string{"plugin_idc_notify_contact_verification", "plugin_idc_notify_rate_window", "plugin_idc_notify_presales_operation"} {
		value, queryErr := database.GetValue(ctx, "SELECT to_regclass(?)", table)
		if queryErr != nil || value.String() == "" {
			t.Fatalf("missing %s: value=%v err=%v", table, value, queryErr)
		}
	}
}
