// This file verifies that retired employee channels cannot re-enter configuration
// and that their upgrade cleanup is repeatable while preserving historical evidence.

package notification

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
)

func TestRetiredEmployeeChannelCannotBeConfigured(t *testing.T) {
	if validChannelCode("wechat_work") || validLifecycleInput(ChannelLifecycleInput{
		ChannelCode: "wechat_work", ExpectedVersion: 1,
		IdempotencyKey: "removed-channel-0001", Confirmed: true,
	}) {
		t.Fatal("the removed employee channel still permits channel operations")
	}
	for _, code := range []string{"email", "sms"} {
		if !validChannelCode(code) {
			t.Fatalf("remaining channel %q is unavailable", code)
		}
	}
	service := &serviceImpl{}
	input := ChannelLifecycleInput{ChannelCode: "wechat_work", ExpectedVersion: 1,
		IdempotencyKey: "retired-lifecycle-0001", Confirmed: true}
	for name, operation := range map[string]func() error{
		"enable": func() error { _, err := service.EnableChannel(t.Context(), input); return err },
		"clear":  func() error { _, err := service.ClearChannel(t.Context(), input); return err },
		"configure": func() error {
			_, err := service.ConfigureChannel(t.Context(), ChannelConfigurationInput{ChannelCode: "wechat_work",
				ProviderCode: "old_wecom", ExpectedVersion: 1, IdempotencyKey: "retired-configure-0001",
				Secrets: map[string][]byte{"secret": []byte("retired-test-secret")}})
			return err
		},
		"self test": func() error {
			_, err := service.TestChannel(t.Context(), ChannelTestInput{ChannelCode: "wechat_work",
				ExpectedVersion: 1, TemplateNo: "retired-template", TemplateVersion: 1, Target: []byte("old-staff-id"),
				IdempotencyKey: "retired-test-0001", Confirmed: true})
			return err
		},
	} {
		if err := operation(); !bizerr.Is(err, CodeInvalid) {
			t.Fatalf("retired channel %s reached authorization, database, or provider: %v", name, err)
		}
	}
}

// TestRetiredCrossedWorkCannotReenterWorkersPostgreSQL challenges the upgrade
// with every active provider-boundary state, then runs the surviving workers.
func TestRetiredCrossedWorkCannotReenterWorkersPostgreSQL(t *testing.T) {
	database, cleanup := openNotificationServicePostgreSQL(t)
	defer cleanup()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve retirement SQL")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	policy, err := os.ReadFile(filepath.Join(root, "manifest", "sql", "002-notification-event-policy-materialization.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(t.Context(), string(policy)); err != nil {
		t.Fatal(err)
	}
	retirement, err := os.ReadFile(filepath.Join(root, "manifest", "sql", "009-remove-employee-wecom.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = database.Exec(t.Context(), string(retirement)); err != nil {
			t.Fatalf("retirement on a fresh installation: %v", err)
		}
	}
	freshChannels, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_channel`)
	if err != nil || freshChannels.Int() != 7 {
		t.Fatalf("fresh retirement changed remaining channels: count=%v err=%v", freshChannels, err)
	}
	if _, err = database.Exec(t.Context(), `INSERT INTO plugin_idc_notify_channel
        (channel_code,channel_kind,display_name,status_code,balance_state)
        VALUES ('wechat_work','email','Retired channel','enabled','sufficient')`); err != nil {
		t.Fatal(err)
	}
	states := []struct {
		status DeliveryStatus
		result channelcap.ResultClass
	}{
		{DeliveryQueued, channelcap.ResultUnknown}, {DeliveryRetryWait, channelcap.ResultDefinitiveNotReceivedTemporary},
		{DeliverySending, channelcap.ResultUnknown}, {DeliverySubmitted, channelcap.ResultAccepted},
		{DeliveryResultUnknown, channelcap.ResultUnknown}, {DeliveryDelivered, channelcap.ResultDelivered},
		{DeliveryFailed, channelcap.ResultDefinitiveNotReceivedPermanent}, {DeliveryManualUnknownClosed, "manually_closed_unknown"},
	}
	if err = database.Transaction(t.Context(), func(ctx context.Context, tx gdb.TX) error {
		for _, fixture := range states {
			status := fixture.status
			taskNo := "crossed-" + string(status)
			if _, insertErr := tx.Ctx(ctx).Exec(`INSERT INTO plugin_idc_notify_delivery_chain
                (chain_no,root_task_no,tail_task_no) VALUES (?,?,?)`, taskNo, taskNo, taskNo); insertErr != nil {
				return insertErr
			}
			if _, insertErr := tx.Ctx(ctx).Exec(`INSERT INTO plugin_idc_notify_delivery_task
                (task_no,chain_no,event_code,topic_code,channel_code,locale_code,owner_user_id,
                 business_object_type,business_object_no,body_content,content_digest,status_code,result_class,
                 external_boundary_crossed,provider_message_ref,provider_query_deadline,provider_query_deadline_source,
                 attempt_lease,attempt_lease_until,retry_sequence,version)
                VALUES (?,?,'support.ticket_updated','support','wechat_work','zh-CN',101,
                        'support_ticket','retired-ticket','Historical body',?,?,?,TRUE,
                        'original-provider-reference',CURRENT_TIMESTAMP + INTERVAL '1 day','original-receipt',
                        'expired-lease',CURRENT_TIMESTAMP - INTERVAL '1 hour',2,4)`,
				taskNo, taskNo, digestText("retirement-evidence"), string(status), string(fixture.result)); insertErr != nil {
				return insertErr
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = database.Exec(t.Context(), string(retirement)); err != nil {
			t.Fatal(err)
		}
	}
	// The existing internal staff renewal inbox must still pass through the worker.
	if _, err = database.Exec(t.Context(), `INSERT INTO plugin_idc_notify_event_catalog (event_code,topic_code,object_type,required_variables,optional_variables) VALUES ('billing.accessory_renewal_attention','billing','billing','[]'::jsonb,'[]'::jsonb) ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	insertInAppDeliveryFixture(t, database, "surviving-in-app", 101)
	if _, err = database.Exec(t.Context(), `UPDATE plugin_idc_notify_delivery_task SET status_code='queued',event_code='billing.accessory_renewal_attention'
        WHERE task_no='surviving-in-app'`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	inbox := &marketingInboxStub{}
	service := &serviceImpl{database: database, now: func() time.Time { return now }, inbox: inbox,
		management: managementcap.Runtime{Authorizer: resendAuthorizerStub{actor: 101}},
		sessions:   resendSessionStub{current: &sessioncap.SessionInfo{UserID: "101", LoginAt: &now}},
		sources:    notificationcap.Runtime{BusinessObjects: &resendBusinessObjectStub{allowed: true}},
	}
	for range 2 {
		if err = service.ProcessDue(t.Context(), maximumWorkerBatch); err != nil {
			t.Fatal(err)
		}
		if err = service.ProcessQueries(t.Context(), maximumWorkerBatch); err != nil {
			t.Fatal(err)
		}
	}
	if err = service.finishClaim(t.Context(), &deliveryRow{TaskNo: "crossed-sending", StatusCode: string(DeliverySending),
		Version: 4, RetrySequence: 2, ExternalBoundaryCrossed: true},
		channelcap.Result{Class: channelcap.ResultDelivered, CrossedExternalBorder: true}, DeliveryDelivered, "stale_retired_response"); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("a stale pre-upgrade worker overwrote retirement state: %v", err)
	}
	for index, fixture := range states {
		original := fixture.status
		var row deliveryRow
		if err = database.Model(deliveryTable).Ctx(t.Context()).Where("task_no", "crossed-"+string(original)).Scan(&row); err != nil {
			t.Fatal(err)
		}
		wantStatus, wantVersion := original, int64(4)
		if index < 5 {
			wantStatus, wantVersion = DeliveryManualReview, 5
		}
		if row.StatusCode != string(wantStatus) || row.Version != wantVersion ||
			!row.ExternalBoundaryCrossed || row.ProviderMessageRef == nil || *row.ProviderMessageRef != "original-provider-reference" ||
			row.ResultClass != string(fixture.result) || row.RetrySequence != 2 || row.ProviderQueryDeadlineSource != "original-receipt" ||
			row.ContentDigest != digestText("retirement-evidence") {
			t.Fatalf("worker changed retired %s evidence: %+v", original, row)
		}
	}
	audits, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_delivery_audit WHERE reason_code='channel_removed'`)
	if err != nil || audits.Int() != 5 {
		t.Fatalf("upgrade must record exactly one transition per unknown task: count=%v err=%v", audits, err)
	}
	leased, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_delivery_task
        WHERE channel_code='wechat_work' AND status_code='manual_review'
          AND (attempt_lease IS NOT NULL OR attempt_lease_until IS NOT NULL OR next_attempt_at IS NOT NULL)`)
	if err != nil || leased.Int() != 0 {
		t.Fatalf("retired unknown work retained a lease or retry: count=%v err=%v", leased, err)
	}
	if inbox.calls != 1 {
		t.Fatalf("surviving in-app delivery calls=%d want=1", inbox.calls)
	}
	closed, err := service.ManualCloseUnknown(t.Context(), ManualClosureInput{TaskNo: "crossed-sending", ExpectedVersion: 5,
		OperationNo: "retired-close-operation-0001", Reason: "Retired provider cannot verify outcome", Confirmed: true})
	if err != nil || closed == nil || closed.Status != DeliveryManualUnknownClosed {
		t.Fatalf("historical unknown task cannot be manually closed: closed=%+v err=%v", closed, err)
	}
	if _, err = service.ManualResend(t.Context(), ManualResendInput{TaskNo: "crossed-sending", ExpectedVersion: 6,
		OperationNo: "retired-resend-operation-0001", Reason: "Must not revive retired delivery", Confirmed: true}); err == nil {
		t.Fatal("manual closure revived a retired provider delivery")
	}
}

func TestEmployeeChannelRetirementIsIdempotentPostgreSQL(t *testing.T) {
	database, cleanup := openNotificationServicePostgreSQL(t)
	defer cleanup()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve channel retirement migration path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	policySQL, err := os.ReadFile(filepath.Join(root, "manifest", "sql", "002-notification-event-policy-materialization.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(t.Context(), string(policySQL)); err != nil {
		t.Fatalf("apply event policies: %v", err)
	}
	legacySQL, err := os.ReadFile(filepath.Join(filepath.Dir(file), "testdata", "legacy_employee_wecom.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(t.Context(), string(legacySQL)); err != nil {
		t.Fatalf("apply original employee binding schema: %v", err)
	}
	// A retired code can still exist on an upgraded installation. Its row remains
	// as evidence for historical foreign keys, while all active operations reject it.
	if _, err := database.Exec(t.Context(), `INSERT INTO plugin_idc_notify_channel
        (tenant_id,channel_code,channel_kind,display_name,status_code,balance_state)
        VALUES (0,'wechat_work','email','Historical employee channel','enabled','sufficient');
        INSERT INTO plugin_idc_notify_wecom_employee_binding
        (tenant_id,binding_no,host_user_id,staff_version,corp_id,channel_generation,identity_digest,
         identity_secret_ref,identity_secret_updated_at,secret_operation_key,status_code,request_no,bound_at)
        VALUES (0,'retired-binding',101,1,'retired-corp',1,repeat('a',64),
                'retired-identity-secret',CURRENT_TIMESTAMP,'retired-secret-operation','active','retired-request',CURRENT_TIMESTAMP);
        INSERT INTO plugin_idc_notify_wecom_binding_audit
        (tenant_id,audit_no,binding_no,actor_user_id,action_code,reason_code,previous_status,current_status,binding_version)
        VALUES (0,'retired-bind-audit','retired-binding',101,'bind','employee_authorized','configuring','active',1);
        CREATE TABLE retirement_test_actor(id BIGINT PRIMARY KEY);
        INSERT INTO retirement_test_actor VALUES (101);
        ALTER TABLE plugin_idc_notify_wecom_binding_audit ADD CONSTRAINT retirement_test_actor_reference
            FOREIGN KEY(actor_user_id) REFERENCES retirement_test_actor(id);
        INSERT INTO plugin_idc_notify_event_policy(event_code,recipient_kind,channel_code,enabled)
        VALUES ('support.ticket_updated','staff_user','wechat_work',TRUE);
        INSERT INTO plugin_idc_notify_template_version
        (template_no,version,event_code,channel_code,locale_code,body_template,status_code,content_digest,created_by)
        VALUES ('retired-template',1,'support.ticket_updated','wechat_work','zh-CN','Historical body','active',repeat('a',64),101);
        INSERT INTO plugin_idc_notify_channel_intent
        (operation_no,channel_code,operation_code,actor_user_id,base_status_code,base_version,
         request_shape_digest,secret_operation_key,lease_token,lease_expires_at)
        VALUES ('retired-intent','wechat_work','configure_create',101,'unconfigured',1,
                repeat('a',64),'retired-channel-secret','retired-lease',CURRENT_TIMESTAMP + INTERVAL '1 minute');
        INSERT INTO plugin_idc_notify_delivery_chain(chain_no,root_task_no,tail_task_no)
        SELECT task_no,task_no,task_no FROM (VALUES ('retired-unsent'),('retired-crossed'),('remaining-unsent')) fixtures(task_no);
        INSERT INTO plugin_idc_notify_delivery_task
        (task_no,chain_no,event_code,topic_code,channel_code,locale_code,business_object_type,
         business_object_no,body_content,content_digest,status_code,external_boundary_crossed)
        SELECT task_no,task_no,'support.ticket_updated','support',channel_code,'zh-CN','support_ticket',
               'retirement-ticket','Historical body',repeat('a',64),'queued',crossed
        FROM (VALUES ('retired-unsent','wechat_work',FALSE),('retired-crossed','wechat_work',TRUE),
                     ('remaining-unsent','email',FALSE)) fixtures(task_no,channel_code,crossed);`); err != nil {
		t.Fatal(err)
	}
	before, err := database.GetValue(t.Context(), `SELECT row_to_json(evidence)::TEXT FROM plugin_idc_notify_wecom_binding_audit evidence WHERE audit_no='retired-bind-audit'`)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, "manifest", "sql", "009-remove-employee-wecom.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = database.Exec(t.Context(), string(content)); err != nil {
			t.Fatalf("repeat channel retirement: %v", err)
		}
	}
	status, err := database.GetValue(t.Context(), `SELECT status_code FROM plugin_idc_notify_channel WHERE channel_code='wechat_work'`)
	if err != nil || status.String() != string(ChannelDisabled) {
		t.Fatalf("retired channel evidence status=%v err=%v", status, err)
	}
	remaining, err := database.GetValue(t.Context(), `SELECT count(*) FROM information_schema.tables
        WHERE table_schema=current_schema() AND table_name IN ('plugin_idc_notify_wecom_employee_binding','plugin_idc_notify_wecom_oauth_intent')`)
	if err != nil || remaining.Int() != 0 {
		t.Fatalf("binding tables remain: count=%v err=%v", remaining, err)
	}
	after, err := database.GetValue(t.Context(), `SELECT row_to_json(evidence)::TEXT FROM plugin_idc_notify_wecom_binding_audit evidence WHERE audit_no='retired-bind-audit'`)
	if err != nil || after.String() != before.String() {
		t.Fatalf("historical binding audit changed: before=%v after=%v err=%v", before, after, err)
	}
	actorReference, err := database.GetValue(t.Context(), `SELECT count(*) FROM pg_constraint
        WHERE conrelid='plugin_idc_notify_wecom_binding_audit'::regclass AND conname='retirement_test_actor_reference' AND contype='f'`)
	if err != nil || actorReference.Int() != 1 {
		t.Fatalf("unrelated audit foreign key changed: count=%v err=%v", actorReference, err)
	}
	policyCount, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_event_policy WHERE channel_code='wechat_work'`)
	if err != nil || policyCount.Int() != 0 {
		t.Fatalf("retired event policies remain: count=%v err=%v", policyCount, err)
	}
	templateStatus, err := database.GetValue(t.Context(), `SELECT status_code FROM plugin_idc_notify_template_version WHERE template_no='retired-template'`)
	if err != nil || templateStatus.String() != "disabled" {
		t.Fatalf("retired template status=%v err=%v", templateStatus, err)
	}
	failedIntent, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_channel_intent
        WHERE operation_no='retired-intent' AND status_code='failed' AND last_error_code='channel_removed'
          AND lease_token IS NULL AND lease_expires_at IS NULL`)
	if err != nil || failedIntent.Int() != 1 {
		t.Fatalf("retired configuration operation can still recover: count=%v err=%v", failedIntent, err)
	}
	for taskNo, expected := range map[string]DeliveryStatus{
		"retired-unsent": DeliveryCancelled, "retired-crossed": DeliveryManualReview, "remaining-unsent": DeliveryQueued,
	} {
		taskStatus, queryErr := database.GetValue(t.Context(), `SELECT status_code FROM plugin_idc_notify_delivery_task WHERE task_no=?`, taskNo)
		if queryErr != nil || taskStatus.String() != string(expected) {
			t.Fatalf("task %s status=%v want=%s err=%v", taskNo, taskStatus, expected, queryErr)
		}
	}
	if _, err = database.Exec(t.Context(), `UPDATE plugin_idc_notify_delivery_task SET next_attempt_at=CURRENT_TIMESTAMP + INTERVAL '1 day'
        WHERE task_no='remaining-unsent'`); err != nil {
		t.Fatal(err)
	}
	worker := &serviceImpl{database: database, now: time.Now}
	if err = worker.ProcessDue(t.Context(), 10); err != nil {
		t.Fatal(err)
	}
	crossedStatus, err := database.GetValue(t.Context(), `SELECT status_code FROM plugin_idc_notify_delivery_task WHERE task_no='retired-crossed'`)
	if err != nil || crossedStatus.String() != string(DeliveryManualReview) {
		t.Fatalf("retired task that crossed the provider boundary must retain an unknown result for review, got=%v err=%v", crossedStatus, err)
	}
}
