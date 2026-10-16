// This file verifies employee WeCom binding and self-test safety against a
// real PostgreSQL database, including idempotency and crash recovery.

package notification

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
)

type wecomCurrentStaffStub struct {
	projection notificationcap.CurrentStaffProjection
	err        error
}

func (s wecomCurrentStaffStub) Current(context.Context) (notificationcap.CurrentStaffProjection, error) {
	return s.projection, s.err
}

type wecomOAuthAdapterStub struct {
	mu             sync.Mutex
	userID         []byte
	calls          int
	deliverCalls   int
	destination    []byte
	deliveryResult channelcap.Result
	deliveryErr    error
}

func (*wecomOAuthAdapterStub) ValidateConfiguration(context.Context, map[string]string, map[string][]byte) error {
	return nil
}

func (*wecomOAuthAdapterStub) ValidateDestination([]byte) error { return nil }

func (s *wecomOAuthAdapterStub) Deliver(_ context.Context, input channelcap.Delivery) (channelcap.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliverCalls++
	s.destination = append(s.destination[:0], input.Destination...)
	if s.deliveryErr != nil || s.deliveryResult.Class != "" {
		return s.deliveryResult, s.deliveryErr
	}
	return channelcap.Result{}, errors.New("delivery is outside this test")
}

func (*wecomOAuthAdapterStub) Query(context.Context, channelcap.Query) (channelcap.Result, error) {
	return channelcap.Result{}, errors.New("query is outside this test")
}

func (s *wecomOAuthAdapterStub) ResolveOAuthUser(
	context.Context,
	secretcap.Handle,
	map[string]string,
	string,
) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return append([]byte(nil), s.userID...), nil
}

type wecomSelfTestAccessorStub struct {
	value []byte
}

func (s wecomSelfTestAccessorStub) Access(
	_ context.Context,
	_ secretcap.AccessRequest,
	consume secretcap.Consumer,
) error {
	value := append([]byte(nil), s.value...)
	defer clear(value)
	return consume(value)
}

func TestWeComBindingRejectsDuplicateIdentityAndReusesCompletedCallbackPostgreSQL(t *testing.T) {
	database, cleanup := openWeComBindingPostgreSQL(t)
	defer cleanup()
	manager := newChannelIntentSecretManager()
	now := time.Now().UTC().Truncate(time.Second)
	first := wecomBindingServiceFixture(t, database, manager, 101, 11, "wecom-session-101", []byte("wecom-user-one"), now)

	authorization, state := startWeComAuthorization(t, first, "wecom-bind-first-0001")
	bound, err := first.CompleteOwnWeComBinding(t.Context(), WeComBindingCompletionInput{State: state, Code: "oauth-code-first"})
	if err != nil || !bound.Bound || bound.Status != wecomBindingActive {
		t.Fatalf("complete first binding: status=%+v err=%v", bound, err)
	}
	replayed, err := first.CompleteOwnWeComBinding(t.Context(), WeComBindingCompletionInput{State: state, Code: "oauth-code-replayed"})
	if err != nil || !replayed.Bound || !replayed.Reused || replayed.Version != bound.Version {
		t.Fatalf("completed callback replay changed binding: status=%+v err=%v", replayed, err)
	}
	if authorization.ExpiresAt.Before(now) {
		t.Fatalf("authorization expiry is not in the future: %s", authorization.ExpiresAt)
	}

	second := wecomBindingServiceFixture(t, database, manager, 202, 22, "wecom-session-202", []byte("wecom-user-one"), now)
	_, secondState := startWeComAuthorization(t, second, "wecom-bind-second-0001")
	if _, err = second.CompleteOwnWeComBinding(t.Context(), WeComBindingCompletionInput{State: secondState, Code: "oauth-code-second"}); err == nil {
		t.Fatal("same WeCom identity was silently moved to another staff account")
	}
	count, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_wecom_employee_binding WHERE tenant_id=0 AND status_code='active'`)
	if err != nil || count.Int() != 1 {
		t.Fatalf("duplicate identity changed active binding count: count=%d err=%v", count.Int(), err)
	}
}

func TestWeComBindingRecoversCreateCrashOnlyOnceAcrossInstancesPostgreSQL(t *testing.T) {
	database, cleanup := openWeComBindingPostgreSQL(t)
	defer cleanup()
	manager := newChannelIntentSecretManager()
	now := time.Now().UTC().Truncate(time.Second)
	service := wecomBindingServiceFixture(t, database, manager, 303, 33, "wecom-session-303", []byte("wecom-user-recovery"), now)
	_, state := startWeComAuthorization(t, service, "wecom-bind-recovery-0001")
	staff, _, sessionDigest, err := service.currentWeComStaffSession(t.Context(), true)
	if err != nil {
		t.Fatalf("load current staff session: %v", err)
	}
	intent, reused, err := service.claimWeComOAuthIntent(t.Context(), state, staff, sessionDigest)
	if err != nil || reused {
		t.Fatalf("claim OAuth intent: reused=%v err=%v", reused, err)
	}
	binding, err := service.prepareWeComBinding(t.Context(), intent, []byte("wecom-user-recovery"))
	if err != nil {
		t.Fatalf("prepare recoverable binding: %v", err)
	}
	if _, err = manager.Create(t.Context(), binding.SecretOperationKey,
		func(consume secretcap.Consumer) error { return consume([]byte("wecom-user-recovery")) }); err != nil {
		t.Fatalf("create secret before simulated crash: %v", err)
	}
	forceWeComBindingDue(t, database, binding.BindingNo)

	start := make(chan struct{})
	errorsFound := make(chan error, 2)
	for index := 0; index < 2; index++ {
		instance := wecomBindingServiceFixture(t, database, manager, 303, 33, "wecom-session-303", []byte("unused"), now)
		go func() {
			<-start
			errorsFound <- instance.ProcessWeComBindings(t.Context(), 10)
		}()
	}
	close(start)
	for index := 0; index < 2; index++ {
		if processErr := <-errorsFound; processErr != nil {
			t.Fatalf("recover binding across instances: %v", processErr)
		}
	}
	var row wecomBindingRow
	if err = database.Model(wecomBindingTable).Ctx(t.Context()).Where(tenantIDColumn, notificationTenantID).
		Where("binding_no", binding.BindingNo).Scan(&row); err != nil {
		t.Fatalf("load recovered binding: %v", err)
	}
	if row.StatusCode != wecomBindingActive || row.IdentitySecretRef == nil || manager.recoverCount != 1 {
		t.Fatalf("binding recovery was not single-winner: row=%+v recoverCount=%d", row, manager.recoverCount)
	}
	auditCount, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_wecom_binding_audit WHERE tenant_id=0 AND binding_no=?`, binding.BindingNo)
	if err != nil || auditCount.Int() != 1 {
		t.Fatalf("binding recovery duplicated audit evidence: count=%d err=%v", auditCount.Int(), err)
	}
}

func TestWeComBindingUnbindCancelsOnlyTasksBeforeProviderBoundaryPostgreSQL(t *testing.T) {
	database, cleanup := openWeComBindingPostgreSQL(t)
	defer cleanup()
	manager := newChannelIntentSecretManager()
	now := time.Now().UTC().Truncate(time.Second)
	service := wecomBindingServiceFixture(t, database, manager, 404, 44, "wecom-session-404", []byte("wecom-user-unbind"), now)
	_, state := startWeComAuthorization(t, service, "wecom-bind-unbind-0001")
	bound, err := service.CompleteOwnWeComBinding(t.Context(), WeComBindingCompletionInput{State: state, Code: "oauth-code-unbind"})
	if err != nil {
		t.Fatalf("bind before unbind: %v", err)
	}
	insertWeComDeliveryFixture(t, database, 404, "wecom-task-queued", "queued", false)
	insertWeComDeliveryFixture(t, database, 404, "wecom-task-crossed", "retry_wait", true)

	status, err := service.RemoveOwnWeComBinding(t.Context(), WeComBindingRemovalInput{
		ExpectedVersion: bound.Version, IdempotencyKey: "wecom-unbind-0001", Confirmed: true,
	})
	if err != nil || status.Bound || status.Status != wecomBindingUnbound {
		t.Fatalf("unbind current employee: status=%+v err=%v", status, err)
	}
	queuedStatus, err := database.GetValue(t.Context(), `SELECT status_code FROM plugin_idc_notify_delivery_task WHERE tenant_id=0 AND task_no='wecom-task-queued'`)
	if err != nil || queuedStatus.String() != string(DeliveryCancelled) {
		t.Fatalf("pre-boundary task was not cancelled: status=%s err=%v", queuedStatus.String(), err)
	}
	crossedStatus, err := database.GetValue(t.Context(), `SELECT status_code FROM plugin_idc_notify_delivery_task WHERE tenant_id=0 AND task_no='wecom-task-crossed'`)
	if err != nil || crossedStatus.String() != string(DeliveryRetryWait) {
		t.Fatalf("crossed task was changed during unbind: status=%s err=%v", crossedStatus.String(), err)
	}
	if manager.revokeCount != 1 {
		t.Fatalf("binding secret was not revoked exactly once: %d", manager.revokeCount)
	}
}

func TestWeComBindingCorpChangeBetweenExchangeAndActivationFailsClosedPostgreSQL(t *testing.T) {
	database, cleanup := openWeComBindingPostgreSQL(t)
	defer cleanup()
	manager := newChannelIntentSecretManager()
	now := time.Now().UTC().Truncate(time.Second)
	service := wecomBindingServiceFixture(t, database, manager, 505, 55, "wecom-session-505", []byte("wecom-user-race"), now)
	_, state := startWeComAuthorization(t, service, "wecom-bind-corp-race-0001")
	staff, _, sessionDigest, err := service.currentWeComStaffSession(t.Context(), true)
	if err != nil {
		t.Fatalf("load current staff session: %v", err)
	}
	intent, _, err := service.claimWeComOAuthIntent(t.Context(), state, staff, sessionDigest)
	if err != nil {
		t.Fatalf("claim OAuth intent: %v", err)
	}
	binding, err := service.prepareWeComBinding(t.Context(), intent, []byte("wecom-user-race"))
	if err != nil {
		t.Fatalf("prepare binding: %v", err)
	}
	created, err := manager.Create(t.Context(), binding.SecretOperationKey,
		func(consume secretcap.Consumer) error { return consume([]byte("wecom-user-race")) })
	if err != nil {
		t.Fatalf("create binding secret: %v", err)
	}
	if _, err = database.Exec(t.Context(), `UPDATE plugin_idc_notify_channel
        SET non_secret_configuration='{"corp_id":"ww_other_corp","agent_id":"1000002"}'::jsonb,
            configuration_generation=configuration_generation+1,configuration_version=configuration_version+1
        WHERE tenant_id=0 AND channel_code='wechat_work'`); err != nil {
		t.Fatalf("change WeCom corporation: %v", err)
	}
	active, err := service.activateWeComBinding(t.Context(), binding, created.Handle, created.Metadata)
	if err != nil || active {
		t.Fatalf("old corporation binding became active: active=%v err=%v", active, err)
	}
	forceWeComBindingDue(t, database, binding.BindingNo)
	if err = service.ProcessWeComBindings(t.Context(), 10); err != nil {
		t.Fatalf("revoke invalidated binding secret: %v", err)
	}
	var row wecomBindingRow
	if err = database.Model(wecomBindingTable).Ctx(t.Context()).Where(tenantIDColumn, notificationTenantID).
		Where("binding_no", binding.BindingNo).Scan(&row); err != nil {
		t.Fatalf("load invalidated binding: %v", err)
	}
	if row.StatusCode != wecomBindingInvalidated || row.IdentitySecretRef != nil || manager.revokeCount != 1 {
		t.Fatalf("corporation change did not fail closed: row=%+v revokes=%d", row, manager.revokeCount)
	}
}

func TestWeComSelfTestUsesOnlyCurrentBindingAndIsIdempotentPostgreSQL(t *testing.T) {
	database, cleanup := openWeComBindingPostgreSQL(t)
	defer cleanup()
	manager := newChannelIntentSecretManager()
	now := time.Now().UTC().Truncate(time.Second)
	service := wecomBindingServiceFixture(t, database, manager, 707, 77, "wecom-session-707", []byte("wecom-user-self-test"), now)
	_, state := startWeComAuthorization(t, service, "wecom-bind-self-test-0001")
	bound, err := service.CompleteOwnWeComBinding(t.Context(), WeComBindingCompletionInput{
		State: state, Code: "oauth-code-self-test",
	})
	if err != nil || !bound.Bound {
		t.Fatalf("bind current staff before test send: status=%+v err=%v", bound, err)
	}
	binding, err := service.loadActiveWeComBinding(t.Context(), 707)
	if err != nil || binding == nil {
		t.Fatalf("load current staff binding before test send: binding=%+v err=%v", binding, err)
	}

	templateNo := "wecom-self-test-template"
	if _, err = database.Exec(t.Context(), `INSERT INTO plugin_idc_notify_template_version
        (tenant_id,template_no,version,event_code,channel_code,locale_code,subject_template,body_template,
         status_code,content_digest,created_by)
        VALUES (0,?,1,'support.ticket_updated','wechat_work','zh-CN','工单 {{ticket_no}}','{{title}}',
                'active',?,707)`, templateNo, digestText("wecom-self-test-template-v1")); err != nil {
		t.Fatalf("insert active WeCom test template: %v", err)
	}
	adapter := service.adapters[providerWeCom].adapter.(*wecomOAuthAdapterStub)
	adapter.deliveryResult = channelcap.Result{
		Class: channelcap.ResultDelivered, CrossedExternalBorder: true, ProviderMessageRef: "wecom-self-test-provider-ref",
	}
	audit := &channelIntentAuditStub{}
	service.management = managementcap.Runtime{Authorizer: resendAuthorizerStub{actor: 707}, Audit: audit}
	service.secrets["wechat_work"] = SecretBinding{
		Manager: manager, Accessor: wecomSelfTestAccessorStub{value: []byte("wecom-user-self-test")},
	}
	input := ChannelTestInput{
		ChannelCode: "wechat_work", TemplateNo: templateNo, TemplateVersion: 1,
		Variables:       map[string]string{"ticket_no": "T-707", "title": "本人测试"},
		ExpectedVersion: 2, IdempotencyKey: "wecom-self-test-0001", Confirmed: true,
	}
	created, err := service.TestOwnWeCom(t.Context(), input)
	if err != nil || created.Reused || created.ResultClass != string(channelcap.ResultDelivered) ||
		created.TargetMask != wecomSelfTestTargetMask {
		t.Fatalf("send current staff WeCom test: result=%+v err=%v", created, err)
	}
	replayed, err := service.TestOwnWeCom(t.Context(), input)
	if err != nil || !replayed.Reused || replayed.OperationNo != created.OperationNo {
		t.Fatalf("replay current staff WeCom test: result=%+v err=%v", replayed, err)
	}
	adapter.mu.Lock()
	deliverCalls := adapter.deliverCalls
	destination := append([]byte(nil), adapter.destination...)
	adapter.mu.Unlock()
	if deliverCalls != 1 || string(destination) != "wecom-user-self-test" {
		t.Fatalf("self test did not use exactly one current binding delivery: calls=%d destination=%q", deliverCalls, destination)
	}
	auditCount, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_wecom_binding_audit
		WHERE tenant_id=0 AND binding_no=? AND action_code='test' AND reason_code='employee_self_test'`, binding.BindingNo)
	if err != nil || auditCount.Int() != 1 {
		t.Fatalf("self test audit evidence mismatch: count=%d err=%v", auditCount.Int(), err)
	}
	auditSnapshot, snapshotOK := audit.transitions[0].BeforeSnapshot.(map[string]any)
	if audit.count() != 1 || !snapshotOK || auditSnapshot["target_mask"] != wecomSelfTestTargetMask {
		t.Fatalf("central self test audit did not contain only the safe binding label: %+v", audit.transitions)
	}
	operationCount, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_channel_test_operation
        WHERE tenant_id=0 AND operation_no=? AND target_mask='current_staff_binding' AND status_code='completed'`, created.OperationNo)
	if err != nil || operationCount.Int() != 1 {
		t.Fatalf("self test operation evidence mismatch: count=%d err=%v", operationCount.Int(), err)
	}

	staleLogin := now.Add(time.Minute)
	service.sessions = resendSessionStub{current: &sessioncap.SessionInfo{
		ID: "wecom-session-707", TenantID: "0", UserID: "707", LoginAt: &staleLogin,
	}}
	input.IdempotencyKey = "wecom-self-test-stale-login-0001"
	if _, err = service.TestOwnWeCom(t.Context(), input); err == nil {
		t.Fatal("WeCom self test accepted a future login")
	}
	adapter.mu.Lock()
	deliverCalls = adapter.deliverCalls
	adapter.mu.Unlock()
	if deliverCalls != 1 {
		t.Fatalf("stale login reached the provider: calls=%d", deliverCalls)
	}
}

func TestWeComBindingMigrationIsIdempotentAndRejectsOtherTenantsPostgreSQL(t *testing.T) {
	database, cleanup := openWeComBindingPostgreSQL(t)
	defer cleanup()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve WeCom binding test path")
	}
	pluginRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	content, err := os.ReadFile(filepath.Join(pluginRoot, "manifest", "sql", "003-notification-wecom-employee-binding.sql"))
	if err != nil {
		t.Fatalf("read WeCom binding migration: %v", err)
	}
	if _, err = database.Exec(t.Context(), string(content)); err != nil {
		t.Fatalf("reapply WeCom binding migration: %v", err)
	}
	if _, err = database.Exec(t.Context(), `INSERT INTO plugin_idc_notify_wecom_oauth_intent
        (tenant_id,intent_no,state_digest,host_user_id,staff_version,session_digest,
         channel_generation,channel_version,corp_id,agent_id,status_code,expires_at)
        VALUES (1,'cross-tenant-wecom',?,101,1,?,1,1,'ww_test_corp',1000002,'pending',CURRENT_TIMESTAMP+INTERVAL '5 minutes')`,
		digestText("cross-tenant-wecom-state"), digestText("cross-tenant-wecom-session")); err == nil {
		t.Fatal("WeCom OAuth intent accepted a non-default tenant")
	}
}

func openWeComBindingPostgreSQL(t *testing.T) (gdb.DB, func()) {
	t.Helper()
	database, cleanup := openNotificationServicePostgreSQL(t)
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		cleanup()
		t.Fatal("resolve WeCom binding test path")
	}
	pluginRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	content, err := os.ReadFile(filepath.Join(pluginRoot, "manifest", "sql", "003-notification-wecom-employee-binding.sql"))
	if err != nil {
		cleanup()
		t.Fatalf("read WeCom binding migration: %v", err)
	}
	if _, err = database.Exec(t.Context(), string(content)); err != nil {
		cleanup()
		t.Fatalf("apply WeCom binding migration: %v", err)
	}
	if _, err = database.Exec(t.Context(), `UPDATE plugin_idc_notify_channel
        SET provider_code=?,status_code='enabled',configuration_generation=1,configuration_version=2,
            non_secret_configuration='{"corp_id":"ww_test_corp","agent_id":"1000002"}'::jsonb,
            credential_secret_ref='sec_notify_wecom_credential_001',credential_updated_at=CURRENT_TIMESTAMP
        WHERE tenant_id=0 AND channel_code='wechat_work'`, providerWeCom); err != nil {
		cleanup()
		t.Fatalf("configure WeCom channel fixture: %v", err)
	}
	t.Setenv(wecomPublicOriginEnvironment, "https://notify.example.test")
	return database, cleanup
}

func wecomBindingServiceFixture(
	t *testing.T,
	database gdb.DB,
	manager *channelIntentSecretManager,
	hostUserID int,
	staffVersion int64,
	sessionID string,
	userID []byte,
	now time.Time,
) *serviceImpl {
	t.Helper()
	loginAt := now.Add(-time.Minute)
	adapter := &wecomOAuthAdapterStub{userID: append([]byte(nil), userID...)}
	return &serviceImpl{
		database: database,
		sessions: resendSessionStub{current: &sessioncap.SessionInfo{
			ID: sessioncap.SessionID(sessionID), TenantID: "0", UserID: strconv.Itoa(hostUserID), LoginAt: &loginAt,
		}},
		sources: notificationcap.Runtime{Staff: wecomCurrentStaffStub{projection: notificationcap.CurrentStaffProjection{
			HostUserID: hostUserID, StaffVersion: staffVersion,
		}}},
		secrets: map[string]SecretBinding{"wechat_work": {Manager: manager, Accessor: channelIntentAccessor{}}},
		adapters: map[string]adapterBinding{providerWeCom: {
			descriptor: channelcap.Descriptor{ProviderCode: providerWeCom, Kind: channelcap.KindWeChatWork}, adapter: adapter,
		}},
		now: func() time.Time { return now },
	}
}

func startWeComAuthorization(t *testing.T, service *serviceImpl, idempotencyKey string) (*WeComBindingAuthorization, string) {
	t.Helper()
	authorization, err := service.StartOwnWeComBinding(t.Context(), WeComBindingAuthorizationInput{IdempotencyKey: idempotencyKey})
	if err != nil {
		t.Fatalf("start WeCom authorization: %v", err)
	}
	parsed, err := url.Parse(authorization.AuthorizationURL)
	if err != nil {
		t.Fatalf("parse WeCom authorization URL: %v", err)
	}
	state := parsed.Query().Get("state")
	if state == "" || parsed.Query().Get("redirect_uri") != "https://notify.example.test"+wecomOAuthCallbackPath {
		t.Fatalf("authorization URL does not contain the fixed callback and state: %s", authorization.AuthorizationURL)
	}
	return authorization, state
}

func forceWeComBindingDue(t *testing.T, database gdb.DB, bindingNo string) {
	t.Helper()
	if _, err := database.Exec(t.Context(), `UPDATE plugin_idc_notify_wecom_employee_binding
        SET available_at=CURRENT_TIMESTAMP-INTERVAL '1 second',lease_token=NULL,lease_expires_at=NULL
        WHERE tenant_id=0 AND binding_no=?`, bindingNo); err != nil {
		t.Fatalf("make WeCom binding due: %v", err)
	}
}

func insertWeComDeliveryFixture(t *testing.T, database gdb.DB, hostUserID int, taskNo, status string, crossed bool) {
	t.Helper()
	chainNo := "chain-" + taskNo
	err := database.Transaction(t.Context(), func(ctx context.Context, _ gdb.TX) error {
		if _, err := database.Exec(ctx, `SET CONSTRAINTS ALL DEFERRED`); err != nil {
			return err
		}
		if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_notify_delivery_chain
            (tenant_id,chain_no,root_task_no,tail_task_no) VALUES (0,?,?,?)`, chainNo, taskNo, taskNo); err != nil {
			return err
		}
		_, err := database.Exec(ctx, `INSERT INTO plugin_idc_notify_delivery_task
            (tenant_id,task_no,chain_no,event_code,topic_code,channel_code,locale_code,owner_user_id,
             business_object_type,business_object_no,body_content,content_digest,status_code,external_boundary_crossed)
            VALUES (0,?,?,'support.ticket_updated','support','wechat_work','zh-CN',?,
                    'support_ticket','ticket-wecom','body',?, ?, ?)`,
			taskNo, chainNo, hostUserID, digestText("wecom-task", taskNo), status, crossed)
		return err
	})
	if err != nil {
		t.Fatalf("insert WeCom delivery fixture %s: %v", taskNo, err)
	}
}
