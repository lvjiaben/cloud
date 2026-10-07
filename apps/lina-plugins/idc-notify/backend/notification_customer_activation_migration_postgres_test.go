package backend

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestCustomerActivationNotificationMigrationPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(notificationIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL notification migration tests", notificationIntegrationDSNEnv)
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := newNotificationSchema(t)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteNotificationIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+quoteNotificationIdentifier(schema)+" CASCADE")
		_ = admin.Close(ctx)
	}()
	database, err := gdb.New(gdb.ConfigNode{Link: appendNotificationDSN(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(ctx)
	for _, name := range []string{"001-notification-foundation.sql", "002-notification-event-policy-materialization.sql", "006-multi-provider-sms.sql"} {
		if _, err = database.Exec(ctx, readNotificationSQL(t, filepath.Join("manifest", "sql", name))); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	migration := readNotificationSQL(t, filepath.Join("manifest", "sql", "007-customer-activation-invitation.sql"))
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = database.Exec(ctx, migration); err != nil {
			t.Fatalf("activation migration attempt %d: %v", attempt, err)
		}
	}
	event, err := database.GetValue(ctx, `SELECT COUNT(1) FROM plugin_idc_notify_event_catalog WHERE tenant_id=0 AND event_code='customer.activation_invitation' AND topic_code='security' AND security_required=TRUE`)
	if err != nil || event.Int() != 1 {
		t.Fatalf("event count=%d err=%v", event.Int(), err)
	}
	policies, err := database.GetValue(ctx, `SELECT COUNT(1) FROM plugin_idc_notify_event_policy WHERE tenant_id=0 AND event_code='customer.activation_invitation' AND recipient_kind='customer_contact' AND channel_code IN ('email','sms') AND enabled=TRUE`)
	if err != nil || policies.Int() != 2 {
		t.Fatalf("policy count=%d err=%v", policies.Int(), err)
	}
	templates, err := database.GetValue(ctx, `SELECT COUNT(1) FROM plugin_idc_notify_template_version WHERE tenant_id=0 AND event_code='customer.activation_invitation' AND channel_code='email' AND status_code='active'`)
	if err != nil || templates.Int() != 2 {
		t.Fatalf("template count=%d err=%v", templates.Int(), err)
	}
}
