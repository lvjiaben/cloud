package notification

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
)

type resendAuthorizerStub struct{ actor int }

func (stub resendAuthorizerStub) Authorize(context.Context, managementcap.AuthorizationRequest) (int, error) {
	return stub.actor, nil
}

type resendSessionStub struct {
	sessioncap.Service
	current *sessioncap.SessionInfo
}

func (stub resendSessionStub) Current(context.Context) (*sessioncap.SessionInfo, error) {
	return stub.current, nil
}

type resendBusinessObjectStub struct {
	allowed  bool
	requests []notificationcap.BusinessObjectRequest
}

func (stub *resendBusinessObjectStub) AuthorizeCurrent(_ context.Context, request notificationcap.BusinessObjectRequest) (notificationcap.BusinessObjectProjection, error) {
	stub.requests = append(stub.requests, request)
	if !stub.allowed {
		return notificationcap.BusinessObjectProjection{}, errors.New("object is not visible")
	}
	return notificationcap.BusinessObjectProjection{Type: request.Type, ObjectNo: request.ObjectNo, CurrentVersion: 3}, nil
}

func TestManualResendRechecksBusinessVisibilityAndKeepsOneLinearSuccessorPostgreSQL(t *testing.T) {
	database, cleanup := openNotificationServicePostgreSQL(t)
	defer cleanup()
	ctx := t.Context()
	digest := strings.Repeat("a", 64)
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_notify_channel
        SET status_code='enabled',balance_state='sufficient',provider_code=NULL,
            credential_secret_ref=NULL,credential_updated_at=NULL
		WHERE tenant_id=0 AND channel_code='in_app'`); err != nil {
		t.Fatalf("prepare in-app channel: %v", err)
	}
	if err := database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Ctx(ctx).Exec(`INSERT INTO plugin_idc_notify_delivery_chain
            (chain_no,root_task_no,tail_task_no) VALUES ('nchain_resend','ntask_source','ntask_source')`); err != nil {
			return err
		}
		_, err := tx.Ctx(ctx).Exec(`INSERT INTO plugin_idc_notify_delivery_task
            (task_no,chain_no,event_code,topic_code,channel_code,locale_code,owner_user_id,
             business_object_type,business_object_no,business_object_version,subject_content,
             body_content,content_digest,status_code,result_class,version,completed_at)
            VALUES ('ntask_source','nchain_resend','support.ticket_updated','support','in_app','zh-CN',101,
                    'support_ticket','ticket_1',3,'subject','body',?,'failed',
                    'definitive_not_received_permanent',4,CURRENT_TIMESTAMP)`, digest)
		return err
	}); err != nil {
		t.Fatalf("prepare failed notification chain: %v", err)
	}

	loginAt := time.Now().UTC()
	objects := &resendBusinessObjectStub{}
	service := &serviceImpl{
		database:   database,
		management: managementcap.Runtime{Authorizer: resendAuthorizerStub{actor: 101}},
		sessions:   resendSessionStub{current: &sessioncap.SessionInfo{UserID: "101", LoginAt: &loginAt}},
		sources:    notificationcap.Runtime{BusinessObjects: objects}, now: func() time.Time { return loginAt },
	}
	input := ManualResendInput{TaskNo: "ntask_source", ExpectedVersion: 4,
		OperationNo: "resend-operation-0001", Reason: "provider confirmed no receipt", Confirmed: true}
	if _, err := service.ManualResend(ctx, input); err == nil {
		t.Fatal("manual resend succeeded after authoritative business visibility denial")
	}
	count, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_notify_delivery_task WHERE chain_no='nchain_resend'`)
	if err != nil || count.Int() != 1 {
		t.Fatalf("visibility denial changed notification chain, count=%d err=%v", count.Int(), err)
	}

	objects.allowed = true
	created, err := service.ManualResend(ctx, input)
	if err != nil {
		t.Fatalf("create manual resend successor: %v", err)
	}
	if created.PreviousTaskNo != "ntask_source" || !created.IsChainTail || created.Status != DeliveryQueued {
		t.Fatalf("unexpected manual resend successor: %+v", created)
	}
	replayed, err := service.ManualResend(ctx, input)
	if err != nil || replayed.TaskNo != created.TaskNo {
		t.Fatalf("idempotent replay mismatch: result=%+v err=%v", replayed, err)
	}
	count, err = database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_notify_delivery_task WHERE chain_no='nchain_resend'`)
	if err != nil || count.Int() != 2 {
		t.Fatalf("manual resend created duplicate successors, count=%d err=%v", count.Int(), err)
	}
	if len(objects.requests) < 2 || objects.requests[0].Type != notificationcap.BusinessObjectSupportTicket ||
		objects.requests[0].ObjectNo != "ticket_1" {
		t.Fatalf("manual resend did not recheck the authoritative support object: %+v", objects.requests)
	}
}

func TestManualResendEnforcesChainLimitAndDatabaseCooldownPostgreSQL(t *testing.T) {
	database, cleanup := openNotificationServicePostgreSQL(t)
	defer cleanup()
	ctx := t.Context()
	digest := strings.Repeat("c", 64)
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_notify_channel
        SET status_code='enabled',balance_state='sufficient',provider_code=NULL,
            credential_secret_ref=NULL,credential_updated_at=NULL
        WHERE tenant_id=0 AND channel_code='in_app'`); err != nil {
		t.Fatalf("prepare in-app channel: %v", err)
	}
	if err := database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Ctx(ctx).Exec(`INSERT INTO plugin_idc_notify_delivery_chain
        (tenant_id,chain_no,root_task_no,tail_task_no,manual_resend_count,last_manual_resend_at,version)
        VALUES (0,'nchain_limit','ntask_limit_root','ntask_limit_tail',2,CURRENT_TIMESTAMP-INTERVAL '11 minutes',3),
               (0,'nchain_cooldown','ntask_cooldown','ntask_cooldown',1,CURRENT_TIMESTAMP,2)`); err != nil {
			return err
		}
		if _, err := tx.Ctx(ctx).Exec(`INSERT INTO plugin_idc_notify_delivery_task
        (tenant_id,task_no,chain_no,previous_task_no,event_code,topic_code,channel_code,locale_code,owner_user_id,
         business_object_type,business_object_no,business_object_version,subject_content,body_content,content_digest,
         status_code,result_class,version,completed_at)
        VALUES
        (0,'ntask_limit_root','nchain_limit',NULL,'support.ticket_updated','support','in_app','zh-CN',101,
         'support_ticket','ticket_limit',3,'subject','body',?,'failed','definitive_not_received_permanent',4,CURRENT_TIMESTAMP),
        (0,'ntask_limit_middle','nchain_limit','ntask_limit_root','support.ticket_updated','support','in_app','zh-CN',101,
         'support_ticket','ticket_limit',3,'subject','body',?,'failed','definitive_not_received_permanent',4,CURRENT_TIMESTAMP),
        (0,'ntask_limit_tail','nchain_limit','ntask_limit_middle','support.ticket_updated','support','in_app','zh-CN',101,
         'support_ticket','ticket_limit',3,'subject','body',?,'failed','definitive_not_received_permanent',4,CURRENT_TIMESTAMP),
        (0,'ntask_cooldown','nchain_cooldown',NULL,'support.ticket_updated','support','in_app','zh-CN',101,
         'support_ticket','ticket_cooldown',3,'subject','body',?,'failed','definitive_not_received_permanent',4,CURRENT_TIMESTAMP)`,
			digest, digest, digest, digest); err != nil {
			return err
		}
		if _, err := tx.Ctx(ctx).Exec(`INSERT INTO plugin_idc_notify_manual_resend
        (tenant_id,operation_no,chain_no,source_task_no,created_task_no,actor_user_id,reason_text,source_task_version)
        VALUES
        (0,'resend-limit-history-1','nchain_limit','ntask_limit_root','ntask_limit_middle',101,'first verified resend',4),
        (0,'resend-limit-history-2','nchain_limit','ntask_limit_middle','ntask_limit_tail',101,'second verified resend',4)`); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("prepare resend history: %v", err)
	}

	loginAt := time.Now().UTC()
	objects := &resendBusinessObjectStub{allowed: true}
	service := &serviceImpl{
		database:   database,
		management: managementcap.Runtime{Authorizer: resendAuthorizerStub{actor: 101}},
		sessions:   resendSessionStub{current: &sessioncap.SessionInfo{UserID: "101", LoginAt: &loginAt}},
		sources:    notificationcap.Runtime{BusinessObjects: objects}, now: func() time.Time { return loginAt },
	}
	third, err := service.ManualResend(ctx, ManualResendInput{
		TaskNo: "ntask_limit_tail", ExpectedVersion: 4, OperationNo: "resend-limit-third-0001",
		Reason: "third verified resend", Confirmed: true,
	})
	if err != nil || third == nil || third.PreviousTaskNo != "ntask_limit_tail" {
		t.Fatalf("create third manual resend: task=%+v err=%v", third, err)
	}
	eligibility, err := service.GetResendEligibility(ctx, third.TaskNo)
	if err != nil || eligibility.RemainingCount != 0 ||
		!slices.Contains(eligibility.Reasons, ResendReasonLimitReached) {
		t.Fatalf("third resend did not exhaust the chain limit: eligibility=%+v err=%v", eligibility, err)
	}
	if _, err = service.ManualResend(ctx, ManualResendInput{
		TaskNo: "ntask_cooldown", ExpectedVersion: 4, OperationNo: "resend-cooldown-0001",
		Reason: "too soon", Confirmed: true,
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("database cooldown allowed an early resend: %v", err)
	}
	cooldown, err := service.GetResendEligibility(ctx, "ntask_cooldown")
	if err != nil || cooldown.NextAllowedAt == nil ||
		!slices.Contains(cooldown.Reasons, ResendReasonCooldown) {
		t.Fatalf("cooldown projection is incomplete: eligibility=%+v err=%v", cooldown, err)
	}
}

func TestManualUnknownClosureKeepsProviderEvidenceAndAuditPostgreSQL(t *testing.T) {
	database, cleanup := openNotificationServicePostgreSQL(t)
	defer cleanup()
	ctx := t.Context()
	digest := strings.Repeat("d", 64)
	if err := database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Ctx(ctx).Exec(`INSERT INTO plugin_idc_notify_delivery_chain
        (tenant_id,chain_no,root_task_no,tail_task_no)
        VALUES (0,'nchain_manual_close','ntask_manual_close','ntask_manual_close')`); err != nil {
			return err
		}
		if _, err := tx.Ctx(ctx).Exec(`INSERT INTO plugin_idc_notify_delivery_task
        (tenant_id,task_no,chain_no,event_code,topic_code,channel_code,locale_code,owner_user_id,
         business_object_type,business_object_no,business_object_version,subject_content,body_content,content_digest,
         status_code,result_class,external_boundary_crossed,provider_message_ref,provider_query_deadline,
         provider_query_deadline_source,version)
        VALUES (0,'ntask_manual_close','nchain_manual_close','support.ticket_updated','support','in_app','zh-CN',101,
                'support_ticket','ticket_manual_close',3,'subject','body',?,'manual_review','result_unknown',TRUE,
                'provider-reference','2026-08-15T12:00:00Z','provider_receipt',2)`, digest); err != nil {
			return err
		}
		if _, err := tx.Ctx(ctx).Exec(`INSERT INTO plugin_idc_notify_delivery_attempt
        (tenant_id,attempt_no,task_no,retry_sequence,attempt_fence,crossed_external_boundary,
         result_class,evidence_digest,provider_reference,started_at,finished_at)
        VALUES (0,'nattempt_manual_close','ntask_manual_close',0,'ntask_manual_close:0',TRUE,
                'result_unknown',?,'provider-reference',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, digest); err != nil {
			return err
		}
		if _, err := tx.Ctx(ctx).Exec(`INSERT INTO plugin_idc_notify_delivery_query
        (tenant_id,query_no,task_no,source_code,result_class,evidence_digest)
        VALUES (0,'nquery_manual_close','ntask_manual_close','provider_receipt','result_unknown',?)`, digest); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("prepare manual-close evidence: %v", err)
	}
	loginAt := time.Now().UTC()
	service := &serviceImpl{
		database:   database,
		management: managementcap.Runtime{Authorizer: resendAuthorizerStub{actor: 101}},
		sessions:   resendSessionStub{current: &sessioncap.SessionInfo{UserID: "101", LoginAt: &loginAt}},
		sources:    notificationcap.Runtime{BusinessObjects: &resendBusinessObjectStub{allowed: true}},
		now:        func() time.Time { return loginAt },
	}
	if _, err := service.ManualCloseUnknown(ctx, ManualClosureInput{
		TaskNo: "ntask_manual_close", ExpectedVersion: 2,
		OperationNo: "manual-close-no-reason-0001", Confirmed: true,
	}); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("manual closure accepted an empty reason: %v", err)
	}
	if _, err := service.ManualCloseUnknown(ctx, ManualClosureInput{
		TaskNo: "ntask_manual_close", ExpectedVersion: 1, OperationNo: "manual-close-stale-0001",
		Reason: "provider result cannot be reconciled", Confirmed: true,
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("manual closure accepted a stale task version: %v", err)
	}
	closed, err := service.ManualCloseUnknown(ctx, ManualClosureInput{
		TaskNo: "ntask_manual_close", ExpectedVersion: 2, OperationNo: "manual-close-success-0001",
		Reason: "provider result cannot be reconciled", Confirmed: true,
	})
	if err != nil || closed == nil || closed.Status != DeliveryManualUnknownClosed ||
		closed.ResultClass != "manually_closed_unknown" {
		t.Fatalf("close unknown result: task=%+v err=%v", closed, err)
	}
	var closure struct {
		Actor   int    `orm:"actor_user_id"`
		Reason  string `orm:"reason_text"`
		Version int64  `orm:"expected_task_version"`
	}
	if err = database.Model("plugin_idc_notify_manual_closure").Ctx(ctx).
		Fields("actor_user_id,reason_text,expected_task_version").
		Where("task_no", "ntask_manual_close").Scan(&closure); err != nil || closure.Actor != 101 ||
		closure.Reason != "provider result cannot be reconciled" || closure.Version != 2 {
		t.Fatalf("manual closure evidence is incomplete: closure=%+v err=%v", closure, err)
	}
	for table, want := range map[string]int{
		"plugin_idc_notify_delivery_attempt": 1,
		"plugin_idc_notify_delivery_query":   1,
		"plugin_idc_notify_delivery_audit":   1,
	} {
		count, countErr := database.Model(table).Ctx(ctx).Where("task_no", "ntask_manual_close").Count()
		if countErr != nil || count != want {
			t.Fatalf("manual closure changed %s evidence: count=%d err=%v", table, count, countErr)
		}
	}
}

func openNotificationServicePostgreSQL(t *testing.T) (gdb.DB, func()) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_NOTIFY_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_NOTIFY_TEST_DSN to run notification service PostgreSQL tests")
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatalf("generate schema suffix: %v", err)
	}
	schema := "linapro_notify_service_test_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(t.Context(), "CREATE SCHEMA "+quoteNotificationServiceIdentifier(schema)); err != nil {
		t.Fatalf("create notification service schema: %v", err)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: appendNotificationServiceDSN(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatalf("open scoped PostgreSQL: %v", err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve notification service test path")
	}
	pluginRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	content, err := os.ReadFile(filepath.Join(pluginRoot, "manifest", "sql", "001-notification-foundation.sql"))
	if err != nil {
		t.Fatalf("read notification migration: %v", err)
	}
	if _, err = database.Exec(t.Context(), string(content)); err != nil {
		t.Fatalf("apply notification migration: %v", err)
	}
	cleanup := func() {
		_ = database.Close(context.Background())
		if strings.HasPrefix(schema, "linapro_notify_service_test_") {
			_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+quoteNotificationServiceIdentifier(schema)+" CASCADE")
		}
		_ = admin.Close(context.Background())
	}
	return database, cleanup
}

func quoteNotificationServiceIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func appendNotificationServiceDSN(dsn, option string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + option
}
