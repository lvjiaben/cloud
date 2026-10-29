// This file verifies retirement of customer inbox and removed external channels.
// Isolated PostgreSQL schemas preserve production data and provider evidence.
package notification

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestEmailSMSMigrationPostgreSQL checks replay, uncertain outcomes and the retained staff flow.
func TestEmailSMSMigrationPostgreSQL(t *testing.T) {
	database, _ := accessoryRenewalDatabase(t)
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	migration, err := os.ReadFile(filepath.Join(root, "manifest", "sql", "013-email-sms-notifications.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range []struct {
		no, channel, event string
		crossed            bool
		want               string
	}{
		{"retire-customer-inbox", "in_app", "support.ticket_updated", false, "cancelled"},
		{"retire-wechat-called", "wechat_official", "support.ticket_updated", true, "manual_review"},
		{"retire-webhook-queued", "signed_webhook", "support.ticket_updated", false, "cancelled"},
		{"retain-email", "email", "support.ticket_updated", false, "sending"},
		{"retain-sms", "sms", "support.ticket_updated", false, "sending"},
		{"retain-staff-inbox", "in_app", "billing.accessory_renewal_attention", false, "sending"},
	} {
		insertInAppDeliveryFixture(t, database, task.no, 101)
		if _, err = database.Exec(t.Context(), `UPDATE plugin_idc_notify_delivery_task SET channel_code=?,event_code=?,external_boundary_crossed=? WHERE task_no=?`, task.channel, task.event, task.crossed, task.no); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if _, err = database.Exec(t.Context(), string(migration)); err != nil {
			t.Fatalf("replay retirement: %v", err)
		}
	}
	for _, check := range []struct {
		query string
		want  int
	}{
		{`SELECT count(*) FROM plugin_idc_notify_event_policy WHERE channel_code NOT IN ('email','sms') AND NOT (channel_code='in_app' AND event_code='billing.accessory_renewal_attention' AND recipient_kind='staff_user')`, 0},
		{`SELECT count(*) FROM plugin_idc_notify_event_policy WHERE channel_code='in_app' AND event_code='billing.accessory_renewal_attention' AND recipient_kind='staff_user' AND enabled`, 1},
		{`SELECT count(*) FROM plugin_idc_notify_channel WHERE channel_code NOT IN ('in_app','email','sms') AND status_code<>'disabled'`, 0},
		{`SELECT count(*) FROM plugin_idc_notify_delivery_task WHERE task_no IN ('retire-customer-inbox','retire-webhook-queued') AND status_code='cancelled'`, 2},
		{`SELECT count(*) FROM plugin_idc_notify_delivery_task WHERE task_no='retire-wechat-called' AND status_code='manual_review' AND external_boundary_crossed AND completed_at IS NULL`, 1},
		{`SELECT count(*) FROM plugin_idc_notify_delivery_task WHERE task_no IN ('retain-email','retain-sms','retain-staff-inbox') AND status_code='sending'`, 3},
		{`SELECT count(*) FROM plugin_idc_notify_delivery_audit WHERE reason_code='channel_removed'`, 3},
	} {
		count, err := database.GetValue(t.Context(), check.query)
		if err != nil || count.Int() != check.want {
			t.Fatalf("migration invariant: want %d got %v error %v", check.want, count, err)
		}
	}
}
