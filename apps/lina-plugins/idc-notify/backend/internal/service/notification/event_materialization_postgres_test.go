package notification

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
)

type materializationContactResolverStub struct{}

func (materializationContactResolverStub) ResolveBatch(
	_ context.Context,
	requests []notificationcap.ContactRequest,
	consume notificationcap.ContactConsumer,
) error {
	for index := range requests {
		if err := consume(index, notificationcap.ContactProjection{
			CustomerNo: requests[index].CustomerNo, ContactNo: requests[index].ContactNo,
			Channel: requests[index].Channel, Topic: requests[index].Topic,
			PreferredLocale: "en-US", AddressOwner: notificationcap.AddressOwnerCoreContact,
			HostUserID: 101, ContactVersion: 1,
		}, []byte("temporary@example.invalid")); err != nil {
			return err
		}
	}
	return nil
}

func TestEventMaterializationBatchIsConcurrentAndIdempotentPostgreSQL(t *testing.T) {
	database, cleanup := openNotificationServicePostgreSQL(t)
	defer cleanup()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve event materialization test path")
	}
	pluginRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	migration, err := os.ReadFile(filepath.Join(pluginRoot, "manifest", "sql", "002-notification-event-policy-materialization.sql"))
	if err != nil {
		t.Fatalf("read event materialization migration: %v", err)
	}
	if _, err = database.Exec(t.Context(), string(migration)); err != nil {
		t.Fatalf("apply event materialization migration: %v", err)
	}
	if _, err = database.Exec(t.Context(), string(migration)); err != nil {
		t.Fatalf("reapply idempotent event materialization migration: %v", err)
	}
	providerMigration, err := os.ReadFile(filepath.Join(pluginRoot, "manifest", "sql", "006-multi-provider-sms.sql"))
	if err != nil {
		t.Fatalf("read multi-provider SMS migration: %v", err)
	}
	if _, err = database.Exec(t.Context(), string(providerMigration)); err != nil {
		t.Fatalf("apply multi-provider SMS migration: %v", err)
	}
	if _, err = database.Exec(t.Context(), `UPDATE plugin_idc_notify_channel
        SET status_code='enabled',balance_state='sufficient'
		WHERE tenant_id=0 AND channel_code IN ('email','signed_webhook')`); err != nil {
		t.Fatalf("enable test channels: %v", err)
	}
	if _, err = database.Exec(t.Context(), `UPDATE plugin_idc_notify_event_policy
		SET enabled=TRUE,version=version+1,updated_at=CURRENT_TIMESTAMP
		WHERE tenant_id=0 AND event_code='support.ticket_created'
		  AND recipient_kind='customer_contact' AND channel_code='signed_webhook'`); err != nil {
		t.Fatalf("enable signed webhook policy: %v", err)
	}
	audit := &channelIntentAuditStub{}
	policyService := &serviceImpl{database: database, management: managementcap.Runtime{
		Authorizer: resendAuthorizerStub{actor: 101}, Audit: audit,
	}}
	policies, err := policyService.ListEventPolicies(t.Context(), MaximumPageSize)
	if err != nil || len(policies) == 0 || len(policies) > MaximumPageSize {
		t.Fatalf("list bounded event policies: count=%d err=%v", len(policies), err)
	}
	var marketingEmail EventPolicy
	for _, item := range policies {
		if item.EventCode == "marketing.campaign" && item.ChannelCode == "email" {
			marketingEmail = item
			break
		}
	}
	if marketingEmail.EventCode == "" || marketingEmail.Enabled {
		t.Fatalf("missing default-disabled marketing email policy: %+v", marketingEmail)
	}
	updatedPolicy, err := policyService.UpdateEventPolicy(t.Context(), EventPolicyWriteInput{
		EventCode: marketingEmail.EventCode, RecipientKind: marketingEmail.RecipientKind,
		ChannelCode: marketingEmail.ChannelCode, Enabled: true, ExpectedVersion: marketingEmail.Version,
		IdempotencyKey: "event-policy-marketing-email-1",
	})
	if err != nil || !updatedPolicy.Enabled || updatedPolicy.Version != marketingEmail.Version+1 {
		t.Fatalf("update event policy: item=%+v err=%v", updatedPolicy, err)
	}
	replayedPolicy, err := policyService.UpdateEventPolicy(t.Context(), EventPolicyWriteInput{
		EventCode: marketingEmail.EventCode, RecipientKind: marketingEmail.RecipientKind,
		ChannelCode: marketingEmail.ChannelCode, Enabled: true, ExpectedVersion: marketingEmail.Version,
		IdempotencyKey: "event-policy-marketing-email-1",
	})
	if err != nil || replayedPolicy.Version != updatedPolicy.Version || audit.count() != 1 {
		t.Fatalf("event policy replay was not idempotent: item=%+v audits=%d err=%v", replayedPolicy, audit.count(), err)
	}
	if _, err = policyService.UpdateEventPolicy(t.Context(), EventPolicyWriteInput{
		EventCode: marketingEmail.EventCode, RecipientKind: marketingEmail.RecipientKind,
		ChannelCode: marketingEmail.ChannelCode, Enabled: false, ExpectedVersion: marketingEmail.Version,
		IdempotencyKey: "event-policy-marketing-email-stale",
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("stale event policy update must conflict: %v", err)
	}
	if _, err = database.Exec(t.Context(), `UPDATE plugin_idc_notify_event_policy SET enabled=TRUE WHERE event_code='account.password_reset' AND channel_code='sms'`); err != nil {
		t.Fatal(err)
	}
	policies, err = policyService.ListEventPolicies(t.Context(), MaximumPageSize)
	if err != nil {
		t.Fatal(err)
	}
	var securityEmail, securityInbox EventPolicy
	for _, item := range policies {
		if item.EventCode != "account.password_reset" || item.RecipientKind != notificationcap.RecipientCustomerContact {
			continue
		}
		switch item.ChannelCode {
		case "email":
			securityEmail = item
		case "sms":
			securityInbox = item
		}
	}
	if securityEmail.EventCode == "" || securityInbox.EventCode == "" {
		t.Fatalf("missing default security policies: email=%+v inbox=%+v", securityEmail, securityInbox)
	}
	securityUpdates := []EventPolicyWriteInput{
		{
			EventCode: securityEmail.EventCode, RecipientKind: securityEmail.RecipientKind,
			ChannelCode: securityEmail.ChannelCode, Enabled: false, ExpectedVersion: securityEmail.Version,
			IdempotencyKey: "event-policy-security-email-off",
		},
		{
			EventCode: securityInbox.EventCode, RecipientKind: securityInbox.RecipientKind,
			ChannelCode: securityInbox.ChannelCode, Enabled: false, ExpectedVersion: securityInbox.Version,
			IdempotencyKey: "event-policy-security-inbox-off",
		},
	}
	securityResults := make(chan error, len(securityUpdates))
	var securityWait sync.WaitGroup
	for _, input := range securityUpdates {
		securityWait.Add(1)
		go func() {
			defer securityWait.Done()
			_, updateErr := policyService.UpdateEventPolicy(context.Background(), input)
			securityResults <- updateErr
		}()
	}
	securityWait.Wait()
	close(securityResults)
	var succeeded, conflicted int
	for updateErr := range securityResults {
		switch {
		case updateErr == nil:
			succeeded++
		case bizerr.Is(updateErr, CodeConflict):
			conflicted++
		default:
			t.Fatalf("concurrent security policy update returned an unexpected error: %v", updateErr)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("concurrent security policy updates must preserve one channel: succeeded=%d conflicted=%d", succeeded, conflicted)
	}
	enabledSecurityChannels, countErr := database.Model(eventPolicyTable).Ctx(t.Context()).
		Where(tenantIDColumn, notificationTenantID).Where("event_code", securityEmail.EventCode).
		Where("recipient_kind", string(notificationcap.RecipientCustomerContact)).Where("enabled", true).
		WhereIn("channel_code", supportedChannelCodes).Count()
	if countErr != nil || enabledSecurityChannels != 1 {
		t.Fatalf("security event must keep exactly one customer delivery channel: count=%d err=%v", enabledSecurityChannels, countErr)
	}
	service := &serviceImpl{database: database, sources: notificationcap.Runtime{Contacts: materializationContactResolverStub{}}}
	claimed := []notificationcap.ClaimedEvent{{
		EventReceipt: notificationcap.EventReceipt{EventNo: "event-materialization-1", SourcePlugin: "idc-core"},
		Request: notificationcap.EventRequest{
			EventCode: "support.ticket_created",
			Recipient: notificationcap.Recipient{Kind: notificationcap.RecipientCustomerContact,
				CustomerNo: "customer-1", ContactNo: "contact-1"},
			BusinessObjectType: notificationcap.BusinessObjectSupportTicket,
			BusinessObjectNo:   "ticket-1", BusinessObjectVersion: 1,
			Variables:      map[string]string{"ticket_no": "T-1", "title": "Need help"},
			IdempotencyKey: "support-ticket-created-1",
		},
	}}

	var wait sync.WaitGroup
	errors := make(chan error, 2)
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			completed, materializeErr := service.materializeEventBatch(context.Background(), claimed)
			if materializeErr == nil && (len(completed) != 1 || !completed[0]) {
				materializeErr = context.Canceled
			}
			errors <- materializeErr
		}()
	}
	wait.Wait()
	close(errors)
	for materializeErr := range errors {
		if materializeErr != nil {
			t.Fatalf("concurrent materialization failed: %v", materializeErr)
		}
	}
	assertNotificationCount(t, database, eventMaterializationTable, 5)
	assertNotificationCount(t, database, deliveryTable, 1)
	assertNotificationCount(t, database, deliveryChainTable, 1)
	assertNotificationCount(t, database, "plugin_idc_notify_delivery_audit", 1)
	var emailStatus struct {
		Status string `orm:"status_code"`
		Reason string `orm:"reason_code"`
	}
	if err = database.Model(eventMaterializationTable).Ctx(t.Context()).
		Where("channel_code", "email").Fields("status_code,reason_code").Scan(&emailStatus); err != nil {
		t.Fatalf("read email materialization: %v", err)
	}
	if emailStatus.Status != "materialized" || emailStatus.Reason != "delivery_queued" {
		t.Fatalf("unexpected email materialization: %+v", emailStatus)
	}
	retired, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_delivery_task WHERE channel_code NOT IN ('email','sms')`)
	if err != nil || retired.Int() != 0 {
		t.Fatalf("retired channel materialized: %v %v", retired, err)
	}
	for _, name := range []string{"000-notification-event-policy-materialization.sql", "001-notification-foundation.sql"} {
		uninstall, readErr := os.ReadFile(filepath.Join(pluginRoot, "manifest", "sql", "uninstall", name))
		if readErr != nil {
			t.Fatalf("read uninstall migration %s: %v", name, readErr)
		}
		if _, execErr := database.Exec(t.Context(), string(uninstall)); execErr != nil {
			t.Fatalf("apply uninstall migration %s: %v", name, execErr)
		}
	}
	remaining, err := database.GetValue(t.Context(), `SELECT to_regclass('plugin_idc_notify_event_materialization')`)
	if err != nil || !remaining.IsNil() {
		t.Fatalf("event materialization table survived uninstall: value=%v err=%v", remaining, err)
	}
}

func assertNotificationCount(t *testing.T, database gdb.DB, table string, want int) {
	t.Helper()
	if !stableCodePattern.MatchString(table) {
		t.Fatalf("unsafe test table name %q", table)
	}
	value, err := database.GetValue(t.Context(), "SELECT count(*) FROM "+table)
	if err != nil || value.Int() != want {
		t.Fatalf("table %s count=%d want=%d err=%v", table, value.Int(), want, err)
	}
}
