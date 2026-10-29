// Accessory renewal seed tests exercise isolated PostgreSQL migration replay,
// administrator preservation, bilingual rendering and customer materialization.
// They never start a delivery worker or contact an external provider.
package notification

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/notificationcap"
)

// accessoryRenewalContactResolver supplies a deterministic locale and host owner.
type accessoryRenewalContactResolver struct{ locale string }

// ResolveBatch returns only test-owned contacts without an external lookup.
func (r accessoryRenewalContactResolver) ResolveBatch(_ context.Context, requests []notificationcap.ContactRequest, consume notificationcap.ContactConsumer) error {
	for index, request := range requests {
		if err := consume(index, notificationcap.ContactProjection{
			CustomerNo: request.CustomerNo, ContactNo: request.ContactNo, Channel: request.Channel,
			Topic: request.Topic, PreferredLocale: r.locale, HostUserID: 101, ContactVersion: 1,
			AddressOwner: notificationcap.AddressOwnerCoreContact,
		}, []byte("renewal@example.invalid")); err != nil {
			return err
		}
	}
	return nil
}

// accessoryRenewalDatabase installs only prerequisites in a fresh test schema.
func accessoryRenewalDatabase(t *testing.T) (gdb.DB, string) {
	t.Helper()
	database, cleanup := openNotificationServicePostgreSQL(t)
	t.Cleanup(cleanup)
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve renewal migration path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	var migration string
	for _, name := range []string{"002-notification-event-policy-materialization.sql", "006-multi-provider-sms.sql", "012-accessory-renewal-notifications.sql"} {
		content, err := os.ReadFile(filepath.Join(root, "manifest", "sql", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = database.Exec(t.Context(), string(content)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
		migration = string(content)
	}
	return database, migration
}

// TestAccessoryRenewalSeedPostgreSQL verifies replay preserves administrator edits.
func TestAccessoryRenewalSeedPostgreSQL(t *testing.T) {
	database, migration := accessoryRenewalDatabase(t)
	if _, err := database.Exec(t.Context(), migration); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		query string
		want  int
	}{
		{`SELECT count(*) FROM plugin_idc_notify_event_catalog WHERE event_code LIKE 'billing.accessory_renewal_%' AND topic_code='billing' AND object_type='billing' AND enabled AND NOT security_required`, 3},
		{`SELECT count(*) FROM plugin_idc_notify_event_policy WHERE event_code LIKE 'billing.accessory_renewal_%'`, 15},
		{`SELECT count(*) FROM plugin_idc_notify_event_policy WHERE event_code LIKE 'billing.accessory_renewal_%' AND enabled`, 5},
		{`SELECT count(*) FROM plugin_idc_notify_event_policy WHERE event_code='billing.accessory_renewal_attention' AND enabled AND recipient_kind='staff_user' AND channel_code='in_app'`, 1},
		{`SELECT count(*) FROM plugin_idc_notify_template_version WHERE event_code LIKE 'billing.accessory_renewal_%' AND status_code='active'`, 10},
	} {
		count, err := database.GetValue(t.Context(), check.query)
		if err != nil || count.Int() != check.want {
			t.Fatalf("seed invariant: want %d, got %v, error %v", check.want, count, err)
		}
	}
	// A disabled merchant template with its own identifier must remain authoritative.
	for _, statement := range []string{
		`UPDATE plugin_idc_notify_template_version SET template_no='merchant_renewal_email', body_template='Merchant renewal content', status_code='disabled' WHERE event_code='billing.accessory_renewal_due' AND channel_code='email' AND locale_code='en-US'`,
		`UPDATE plugin_idc_notify_event_policy SET enabled=FALSE,version=7 WHERE event_code='billing.accessory_renewal_due' AND channel_code='email'`,
		`UPDATE plugin_idc_notify_event_catalog SET enabled=FALSE,version=9 WHERE event_code='billing.accessory_renewal_failed'`,
	} {
		if _, err := database.Exec(t.Context(), statement); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if _, err := database.Exec(t.Context(), migration); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{
		`SELECT count(*) FROM plugin_idc_notify_template_version WHERE event_code='billing.accessory_renewal_due' AND channel_code='email' AND locale_code='en-US' AND template_no='merchant_renewal_email' AND body_template='Merchant renewal content' AND status_code='disabled'`,
		`SELECT count(*) FROM plugin_idc_notify_event_policy WHERE event_code='billing.accessory_renewal_due' AND channel_code='email' AND NOT enabled AND version=7`,
		`SELECT count(*) FROM plugin_idc_notify_event_catalog WHERE event_code='billing.accessory_renewal_failed' AND NOT enabled AND version=9`,
	} {
		count, err := database.GetValue(t.Context(), query)
		if err != nil || count.Int() != 1 {
			t.Fatalf("administrator change overwritten: count=%v error=%v", count, err)
		}
	}
	count, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_template_version WHERE event_code LIKE 'billing.accessory_renewal_%'`)
	if err != nil || count.Int() != 10 {
		t.Fatalf("replay recreated merchant-disabled template: count=%v error=%v", count, err)
	}
}

// TestAccessoryRenewalTemplatesPostgreSQL renders actual seed rows for both locales.
func TestAccessoryRenewalTemplatesPostgreSQL(t *testing.T) {
	database, _ := accessoryRenewalDatabase(t)
	var rows []templateRow
	if err := database.Model("plugin_idc_notify_template_version").Ctx(t.Context()).WhereLike("event_code", "billing.accessory_renewal_%").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 10 {
		t.Fatalf("template count: %d", len(rows))
	}
	service := &serviceImpl{renewalPeriods: testRenewalPeriodLabels(t), database: database}
	for _, row := range rows {
		t.Run(row.TemplateNo, func(t *testing.T) {
			event, err := service.loadEvent(t.Context(), row.EventCode)
			if err != nil || event == nil {
				t.Fatalf("event: %v", err)
			}
			if err = validateTemplateVariables(row.SubjectTemplate+"\n"+row.BodyTemplate, event); err != nil {
				t.Fatal(err)
			}
			for _, period := range []string{"day", "month", "year"} {
				values := map[string]string{"invoice_no": "INV-120", "amount": "123.45", "currency": "USD", "tax_amount": "23.45", "period_unit": period, "period_count": "12", "pending_count": "3"}
				// Supply only the declared values, then prove every required value is required.
				allowed := eventVariableSet(event)
				for key := range values {
					if _, ok := allowed[key]; !ok {
						delete(values, key)
					}
				}
				if err = validateEventValues(values, event); err != nil {
					t.Fatal(err)
				}
				renderedValues, renderErr := renewalTemplateVariables(row.EventCode, row.LocaleCode, values, testRenewalPeriodLabels(t))
				if renderErr != nil {
					t.Fatal(renderErr)
				}
				subject, renderErr := renderTemplate(row.SubjectTemplate, renderedValues)
				if renderErr != nil || strings.Contains(subject, "{{") {
					t.Fatalf("subject render: %q error=%v", subject, renderErr)
				}
				body, renderErr := renderTemplate(row.BodyTemplate, renderedValues)
				if renderErr != nil || strings.Contains(body, "{{") || !strings.Contains(body, "INV-120") {
					t.Fatalf("render: %q error=%v", body, renderErr)
				}
				var fragments []string
				switch row.EventCode {
				case "billing.accessory_renewal_due":
					fragments = []string{"123.45 USD", "23.45 USD"}
					if row.LocaleCode == "zh-CN" {
						fragments = append(fragments, "含税总金额", "待付款", "12"+map[string]string{"day": "天", "month": "个月", "year": "年"}[period])
					} else {
						fragments = append(fragments, "tax-inclusive total", "awaits payment", "12 "+period)
					}
				case "billing.accessory_renewal_failed":
					if row.LocaleCode == "zh-CN" {
						fragments = []string{"尚未付款", "仍保留", "已暂停", "删除", "付款成功当天"}
					} else {
						fragments = []string{"unpaid", "retained", "paused", "delete", "payment date"}
					}
				case "billing.accessory_renewal_attention":
					if row.LocaleCode == "zh-CN" {
						fragments = []string{"3 项", "最近账单号", "后台续费待办"}
					} else {
						fragments = []string{"3 accessory", "latest invoice", "renewal worklist"}
					}
				}
				for _, fragment := range fragments {
					if !strings.Contains(body, fragment) {
						t.Fatalf("missing %q in %q", fragment, body)
					}
				}
				for key := range values {
					missing := cloneVariables(values)
					delete(missing, key)
					if err = validateEventValues(missing, event); err == nil {
						t.Fatalf("missing required variable accepted: %s", key)
					}
				}
			}
		})
	}
}

// TestAccessoryRenewalCustomerMaterializationPostgreSQL proves enabled channels
// persist frozen content once, disabled channels stay skipped, and bad input fails.
func TestAccessoryRenewalCustomerMaterializationPostgreSQL(t *testing.T) {
	for _, locale := range []string{"zh-CN", "en-US"} {
		t.Run(locale, func(t *testing.T) {
			database, _ := accessoryRenewalDatabase(t)
			if _, err := database.Exec(t.Context(), `UPDATE plugin_idc_notify_channel SET status_code='enabled',balance_state='sufficient' WHERE channel_code='email'`); err != nil {
				t.Fatal(err)
			}
			service := &serviceImpl{renewalPeriods: testRenewalPeriodLabels(t), database: database, sources: notificationcap.Runtime{Contacts: accessoryRenewalContactResolver{locale: locale}}}
			claims := []notificationcap.ClaimedEvent{}
			for index, code := range []string{"billing.accessory_renewal_due", "billing.accessory_renewal_failed"} {
				values := map[string]string{"invoice_no": "INV-120"}
				if index == 0 {
					values = map[string]string{"invoice_no": "INV-120", "amount": "123.45", "currency": "USD", "tax_amount": "23.45", "period_unit": "month", "period_count": "12"}
				}
				claims = append(claims, notificationcap.ClaimedEvent{EventReceipt: notificationcap.EventReceipt{EventNo: fmt.Sprintf("renewal-%d", index), SourcePlugin: "idc-core"}, Request: notificationcap.EventRequest{
					EventCode: code, Recipient: notificationcap.Recipient{Kind: notificationcap.RecipientCustomerContact, CustomerNo: "customer-1", ContactNo: "contact-1"},
					BusinessObjectType: notificationcap.BusinessObjectBilling, BusinessObjectNo: "INV-120", BusinessObjectVersion: 1, Variables: values, IdempotencyKey: fmt.Sprintf("renewal-%d", index),
				}})
			}
			for range 2 {
				done, err := service.materializeEventBatch(t.Context(), claims)
				if err != nil || len(done) != 2 || !done[0] || !done[1] {
					t.Fatalf("materialization: %v %v", done, err)
				}
			}
			for _, check := range []struct {
				query string
				want  int
			}{
				{`SELECT count(*) FROM plugin_idc_notify_delivery_task WHERE locale_code=? AND status_code='queued' AND NOT external_boundary_crossed`, 2},
				{`SELECT count(*) FROM plugin_idc_notify_delivery_task WHERE locale_code=? AND channel_code='in_app' AND owner_user_id=101`, 0},
				{`SELECT count(*) FROM plugin_idc_notify_delivery_task WHERE locale_code=? AND event_code='billing.accessory_renewal_due' AND body_content LIKE '%123.45 USD%' AND body_content LIKE '%23.45 USD%' AND provider_variables->>'period_unit'='month' AND provider_variables->>'period_count'='12'`, 1},
			} {
				count, err := database.GetValue(t.Context(), check.query, locale)
				if err != nil || count.Int() != check.want {
					t.Fatalf("delivery invariant: count=%v want=%d error=%v", count, check.want, err)
				}
			}
			periodText := "12 month"
			if locale == "zh-CN" {
				periodText = "12个月"
			}
			localized, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_delivery_task WHERE event_code='billing.accessory_renewal_due' AND body_content LIKE ?`, "%"+periodText+"%")
			if err != nil || localized.Int() != 1 {
				t.Fatalf("localized period: count=%v error=%v", localized, err)
			}
			skipped, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_event_materialization WHERE status_code='skipped' AND reason_code='policy_disabled'`)
			if err != nil || skipped.Int() != 2 {
				t.Fatalf("disabled channels: %v %v", skipped, err)
			}
			bad := claims[0]
			bad.EventNo = "renewal-missing-tax"
			bad.Request.Variables = cloneVariables(bad.Request.Variables)
			delete(bad.Request.Variables, "tax_amount")
			if _, err = service.materializeEventBatch(t.Context(), []notificationcap.ClaimedEvent{bad}); err != nil {
				t.Fatal(err)
			}
			failed, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_event_materialization WHERE source_event_no='renewal-missing-tax' AND status_code='failed' AND reason_code='event_variables_invalid'`)
			if err != nil || failed.Int() != 1 {
				t.Fatalf("missing tax failure: %v %v", failed, err)
			}
			assertNotificationCount(t, database, deliveryTable, 2)
		})
	}
}

// TestAccessoryRenewalStaffMaterializationPostgreSQL proves the narrow staff
// inbox route cannot enable external staff delivery or bypass source validation.
func TestAccessoryRenewalStaffMaterializationPostgreSQL(t *testing.T) {
	for _, test := range []struct {
		name, source, event, locale, status, reason string
		user, wantTasks                             int
	}{
		{name: "Chinese current staff", source: "idc-core", event: "billing.accessory_renewal_attention", locale: "zh-CN", user: 301, wantTasks: 1, status: "materialized", reason: "delivery_queued"},
		{name: "English current staff", source: "idc-core", event: "billing.accessory_renewal_attention", locale: "en-US", user: 302, wantTasks: 1, status: "materialized", reason: "delivery_queued"},
		{name: "Default locale", source: "idc-core", event: "billing.accessory_renewal_attention", user: 303, wantTasks: 1, status: "materialized", reason: "delivery_queued"},
		{name: "Other source", source: "idc-provider", event: "billing.accessory_renewal_attention", user: 301, status: "skipped", reason: "channel_removed"},
		{name: "Other staff event", source: "idc-core", event: "support.ticket_unassigned", user: 301, status: "skipped", reason: "channel_removed"},
		{name: "Missing staff", source: "idc-core", event: "billing.accessory_renewal_attention", status: "failed", reason: "recipient_unavailable"},
		{name: "Invalid staff", source: "idc-core", event: "billing.accessory_renewal_attention", user: -1, status: "failed", reason: "recipient_unavailable"},
		{name: "Invalid locale", source: "idc-core", event: "billing.accessory_renewal_attention", locale: "not-a-locale", user: 301, status: "failed", reason: "locale_unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			database, _ := accessoryRenewalDatabase(t)
			// Deliberately enable all event policies and channels to exercise the recipient
			// boundary rather than relying only on the safe default-disabled policies.
			if _, err := database.Exec(t.Context(), `UPDATE plugin_idc_notify_event_policy SET enabled=TRUE WHERE event_code=?`, test.event); err != nil {
				t.Fatal(err)
			}
			if _, err := database.Exec(t.Context(), `UPDATE plugin_idc_notify_channel SET status_code='enabled',balance_state='sufficient' WHERE channel_code IN ('email','sms','wechat_official','signed_webhook')`); err != nil {
				t.Fatal(err)
			}
			variables := map[string]string{"pending_count": "3", "invoice_no": "INV-STAFF-120"}
			objectType := notificationcap.BusinessObjectBilling
			if test.locale != "" {
				variables["locale_code"] = test.locale
			}
			if test.event == "support.ticket_unassigned" {
				variables = map[string]string{"ticket_no": "T-STAFF", "title": "Unassigned"}
				objectType = notificationcap.BusinessObjectSupportTicket
			}
			claimed := []notificationcap.ClaimedEvent{{EventReceipt: notificationcap.EventReceipt{SourcePlugin: test.source, EventNo: "staff-renewal-1"}, Request: notificationcap.EventRequest{
				EventCode: test.event, Recipient: notificationcap.Recipient{Kind: notificationcap.RecipientStaffUser, StaffUserID: test.user},
				BusinessObjectType: objectType, BusinessObjectNo: "INV-STAFF-120", BusinessObjectVersion: 1, Variables: variables, IdempotencyKey: "staff-renewal-1",
			}}}
			// Staff inbox resolution must not require a customer-contact resolver.
			service := &serviceImpl{renewalPeriods: testRenewalPeriodLabels(t), database: database}
			for range 2 {
				done, err := service.materializeEventBatch(t.Context(), claimed)
				if err != nil || len(done) != 1 || !done[0] {
					t.Fatalf("staff materialization: %v %v", done, err)
				}
			}
			assertNotificationCount(t, database, deliveryTable, test.wantTasks)
			outcome, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_event_materialization WHERE channel_code='in_app' AND status_code=? AND reason_code=?`, test.status, test.reason)
			if err != nil || outcome.Int() != 1 {
				t.Fatalf("staff in-app outcome: %v %v", outcome, err)
			}
			blocked, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_event_materialization WHERE channel_code<>'in_app' AND status_code='skipped' AND reason_code='staff_resolver_unavailable'`)
			if err != nil || blocked.Int() != 2 {
				t.Fatalf("external staff channels escaped gate: %v %v", blocked, err)
			}
			retired, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_event_policy WHERE channel_code='wechat_work'`)
			if err != nil || retired.Int() != 0 {
				t.Fatalf("retired WeCom policy restored: %v %v", retired, err)
			}
			if test.wantTasks == 0 {
				return
			}
			var task struct {
				OwnerUserID                                                 int
				ChannelCode, LocaleCode, BodyContent, CustomerNo, ContactNo string
				ExternalBoundaryCrossed                                     bool
			}
			if err = database.Model(deliveryTable).Ctx(t.Context()).Scan(&task); err != nil {
				t.Fatal(err)
			}
			locale := test.locale
			if locale == "" {
				locale = "en-US"
			}
			expected := "3 accessory renewals need attention"
			if locale == "zh-CN" {
				expected = "3 项附属资源续费待处理"
			}
			if task.OwnerUserID != test.user || task.ChannelCode != "in_app" || task.LocaleCode != locale || task.CustomerNo != "" || task.ContactNo != "" || task.ExternalBoundaryCrossed || !strings.Contains(task.BodyContent, expected) || !strings.Contains(task.BodyContent, "INV-STAFF-120") {
				t.Fatalf("staff task lost recipient or localized summary: %+v", task)
			}
		})
	}
}
