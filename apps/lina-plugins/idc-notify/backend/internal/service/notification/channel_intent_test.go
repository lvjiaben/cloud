// This file verifies crash recovery, idempotency, and multi-instance fencing
// for notification-channel credential intents against real PostgreSQL.

package notification

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
)

type channelIntentAuditStub struct {
	mu          sync.Mutex
	transitions []idccap.BusinessAuditTransition
}

func (s *channelIntentAuditStub) AppendBusinessTransition(
	_ context.Context,
	input idccap.BusinessAuditTransition,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transitions = append(s.transitions, input)
	return nil
}

func (*channelIntentAuditStub) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, errors.New("unexpected batch audit transition")
}

func (s *channelIntentAuditStub) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.transitions)
}

type channelIntentSecretValue struct {
	handle      secretcap.Handle
	metadata    secretcap.Metadata
	fingerprint string
}

type channelIntentSecretManager struct {
	mu           sync.Mutex
	clock        time.Time
	next         int
	createCount  int
	replaceCount int
	revokeCount  int
	recoverCount int
	creates      map[string]channelIntentSecretValue
	replaces     map[string]channelIntentSecretValue
	values       map[string]channelIntentSecretValue
}

func newChannelIntentSecretManager() *channelIntentSecretManager {
	return &channelIntentSecretManager{
		clock:    time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
		creates:  make(map[string]channelIntentSecretValue),
		replaces: make(map[string]channelIntentSecretValue),
		values:   make(map[string]channelIntentSecretValue),
	}
}

func (m *channelIntentSecretManager) Create(
	_ context.Context,
	key string,
	produce secretcap.PlaintextProducer,
) (secretcap.CreateResult, error) {
	payload, err := captureChannelIntentSecret(produce)
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	defer clear(payload)
	fingerprint := digestText(string(payload))
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.creates[key]; ok {
		if existing.fingerprint != fingerprint {
			return secretcap.CreateResult{}, errors.New("secret idempotency conflict")
		}
		return secretcap.CreateResult{Handle: existing.handle, Metadata: existing.metadata, Reused: true}, nil
	}
	m.next++
	handle, err := secretcap.NewHandle("sec_notify_channel_intent_" + stringInt64(int64(m.next)))
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	m.clock = m.clock.Add(time.Second)
	value := channelIntentSecretValue{
		handle: handle,
		metadata: secretcap.Metadata{
			Configured: true, MaskedTail: "100" + stringInt64(int64(m.next)), UpdatedAt: m.clock,
		},
		fingerprint: fingerprint,
	}
	m.createCount++
	m.creates[key] = value
	m.values[handle.OpaqueRef()] = value
	return secretcap.CreateResult{Handle: handle, Metadata: value.metadata}, nil
}

func (m *channelIntentSecretManager) RecoverCreate(
	_ context.Context,
	key string,
) (secretcap.CreateResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recoverCount++
	value, ok := m.creates[key]
	if !ok {
		return secretcap.CreateResult{}, errors.New("secret create is not configured")
	}
	return secretcap.CreateResult{Handle: value.handle, Metadata: value.metadata, Reused: true}, nil
}

func (m *channelIntentSecretManager) Replace(
	_ context.Context,
	handle secretcap.Handle,
	expectedUpdatedAt time.Time,
	key string,
	produce secretcap.PlaintextProducer,
) (secretcap.MutationResult, error) {
	payload, err := captureChannelIntentSecret(produce)
	if err != nil {
		return secretcap.MutationResult{}, err
	}
	defer clear(payload)
	fingerprint := digestText(string(payload))
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.replaces[key]; ok {
		if existing.fingerprint != fingerprint || !existing.handle.Equal(handle) {
			return secretcap.MutationResult{}, errors.New("secret replacement idempotency conflict")
		}
		return secretcap.MutationResult{Metadata: existing.metadata, Reused: true}, nil
	}
	current, ok := m.values[handle.OpaqueRef()]
	if !ok || !current.metadata.Configured || !current.metadata.UpdatedAt.Equal(expectedUpdatedAt) {
		return secretcap.MutationResult{}, errors.New("secret replacement fence conflict")
	}
	m.clock = m.clock.Add(time.Second)
	current.metadata = secretcap.Metadata{Configured: true, MaskedTail: "2001", UpdatedAt: m.clock}
	current.fingerprint = fingerprint
	m.replaceCount++
	m.replaces[key] = current
	m.values[handle.OpaqueRef()] = current
	return secretcap.MutationResult{Metadata: current.metadata}, nil
}

func (m *channelIntentSecretManager) Metadata(
	_ context.Context,
	handle secretcap.Handle,
) (secretcap.Metadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.values[handle.OpaqueRef()]
	if !ok {
		return secretcap.Metadata{}, errors.New("secret is not configured")
	}
	return value.metadata, nil
}

func (m *channelIntentSecretManager) Revoke(
	_ context.Context,
	handle secretcap.Handle,
	expectedUpdatedAt time.Time,
) (secretcap.Metadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.values[handle.OpaqueRef()]
	if !ok {
		return secretcap.Metadata{}, errors.New("secret is not configured")
	}
	if !value.metadata.Configured {
		return value.metadata, nil
	}
	if !value.metadata.UpdatedAt.Equal(expectedUpdatedAt) {
		return secretcap.Metadata{}, errors.New("secret revoke fence conflict")
	}
	m.clock = m.clock.Add(time.Second)
	value.metadata = secretcap.Metadata{Configured: false, UpdatedAt: m.clock}
	m.revokeCount++
	m.values[handle.OpaqueRef()] = value
	return value.metadata, nil
}

func (*channelIntentSecretManager) Rotate(context.Context, int) (secretcap.RotationResult, error) {
	return secretcap.RotationResult{}, nil
}

func captureChannelIntentSecret(produce secretcap.PlaintextProducer) ([]byte, error) {
	if produce == nil {
		return nil, errors.New("secret producer is required")
	}
	var payload []byte
	err := produce(func(value []byte) error {
		if len(payload) != 0 || len(value) == 0 {
			return errors.New("secret producer returned invalid data")
		}
		payload = append([]byte(nil), value...)
		return nil
	})
	return payload, err
}

type channelIntentAdapter struct{}

type channelIntentAccessor struct{}

func (channelIntentAccessor) Access(
	context.Context,
	secretcap.AccessRequest,
	secretcap.Consumer,
) error {
	return errors.New("secret access is outside this test")
}

func (channelIntentAdapter) ValidateConfiguration(context.Context, map[string]string, map[string][]byte) error {
	return nil
}
func (channelIntentAdapter) ValidateDestination([]byte) error { return nil }

func (channelIntentAdapter) Deliver(context.Context, channelcap.Delivery) (channelcap.Result, error) {
	return channelcap.Result{}, errors.New("delivery is outside this test")
}

func (channelIntentAdapter) Query(context.Context, channelcap.Query) (channelcap.Result, error) {
	return channelcap.Result{}, errors.New("query is outside this test")
}

func channelIntentServiceFixture(
	t *testing.T,
	database gdb.DB,
	manager *channelIntentSecretManager,
	audit *channelIntentAuditStub,
) *serviceImpl {
	t.Helper()
	loginAt := time.Now().UTC()
	descriptor := channelcap.Descriptor{
		ProviderCode: "test_email", Kind: channelcap.KindEmail,
		Fields: []channelcap.Field{{Key: "token", LabelKey: "notify.channel.token", Secret: true, Required: true, MaxLength: 128}},
	}
	return &serviceImpl{
		database: database,
		management: managementcap.Runtime{
			Authorizer: resendAuthorizerStub{actor: 101}, Audit: audit,
		},
		sessions: resendSessionStub{current: &sessioncap.SessionInfo{UserID: "101", LoginAt: &loginAt}},
		secrets:  map[string]SecretBinding{"email": {Manager: manager, Accessor: channelIntentAccessor{}}},
		adapters: map[string]adapterBinding{
			"test_email": {descriptor: descriptor, adapter: channelIntentAdapter{}},
		},
		now: func() time.Time { return loginAt },
	}
}

func TestChannelIntentRepeatedConfigureDoesNotCreateAnotherSecretPostgreSQL(t *testing.T) {
	database, cleanup := openNotificationServicePostgreSQL(t)
	defer cleanup()
	manager := newChannelIntentSecretManager()
	audit := &channelIntentAuditStub{}
	service := channelIntentServiceFixture(t, database, manager, audit)

	first, err := service.ConfigureChannel(t.Context(), channelConfigurationTestInput("configure-repeat-0001", "first-secret"))
	if err != nil {
		t.Fatalf("configure channel: %v", err)
	}
	second, err := service.ConfigureChannel(t.Context(), channelConfigurationTestInput("configure-repeat-0001", "first-secret"))
	if err != nil {
		t.Fatalf("replay configure channel: %v", err)
	}
	if first.Version != 2 || second.Version != first.Version || manager.createCount != 1 {
		t.Fatalf("configure replay changed state: first=%+v second=%+v creates=%d", first, second, manager.createCount)
	}
	intentCount, err := database.GetValue(t.Context(), `SELECT count(*) FROM plugin_idc_notify_channel_intent`)
	if err != nil || intentCount.Int() != 1 || audit.count() != 1 {
		t.Fatalf("configure replay duplicated evidence: intents=%d audits=%d err=%v", intentCount.Int(), audit.count(), err)
	}
}

func TestChannelIntentRecoversCreateCrashBeforeLocalSecretResultPostgreSQL(t *testing.T) {
	database, cleanup := openNotificationServicePostgreSQL(t)
	defer cleanup()
	manager := newChannelIntentSecretManager()
	audit := &channelIntentAuditStub{}
	service := channelIntentServiceFixture(t, database, manager, audit)
	input := channelConfigurationTestInput("configure-crash-0001", "create-secret")
	intent, err := service.prepareConfigureIntent(t.Context(), 101, input, "{}")
	if err != nil {
		t.Fatalf("prepare configure intent: %v", err)
	}
	payload := channelSecretPayload(t, input.Secrets)
	if _, err = manager.Create(t.Context(), intent.SecretOperationKey,
		func(consume secretcap.Consumer) error { return consume(payload) }); err != nil {
		t.Fatalf("commit secret before simulated crash: %v", err)
	}
	forceChannelIntentDue(t, database, intent.OperationNo)

	restarted := channelIntentServiceFixture(t, database, manager, audit)
	if err = restarted.ProcessChannelIntents(t.Context(), 10); err != nil {
		t.Fatalf("recover configure intent: %v", err)
	}
	assertChannelIntentCompleted(t, database, intent.OperationNo, ChannelEnabled, 2)
	if manager.createCount != 1 || manager.recoverCount != 1 || audit.count() != 1 {
		t.Fatalf("create recovery duplicated work: creates=%d recovers=%d audits=%d",
			manager.createCount, manager.recoverCount, audit.count())
	}
}

func TestChannelIntentMissingCreateStaysFailClosedUntilSameOperationRetryPostgreSQL(t *testing.T) {
	database, cleanup := openNotificationServicePostgreSQL(t)
	defer cleanup()
	manager := newChannelIntentSecretManager()
	audit := &channelIntentAuditStub{}
	service := channelIntentServiceFixture(t, database, manager, audit)
	input := channelConfigurationTestInput("configure-not-started-0001", "retry-secret")
	intent, err := service.prepareConfigureIntent(t.Context(), 101, input, "{}")
	if err != nil {
		t.Fatalf("prepare configure intent: %v", err)
	}
	forceChannelIntentDue(t, database, intent.OperationNo)
	if err = service.ProcessChannelIntents(t.Context(), 10); err != nil {
		t.Fatalf("inspect missing create intent: %v", err)
	}
	var pending channelIntentRow
	if err = database.Model(channelIntentTable).Ctx(t.Context()).
		Where(tenantIDColumn, notificationTenantID).Where("operation_no", intent.OperationNo).Scan(&pending); err != nil {
		t.Fatalf("load pending create intent: %v", err)
	}
	var channel channelRow
	if err = database.Model(channelTable).Ctx(t.Context()).
		Where(tenantIDColumn, notificationTenantID).Where("channel_code", "email").Scan(&channel); err != nil {
		t.Fatalf("load fail-closed channel: %v", err)
	}
	if pending.StatusCode != channelIntentPrepared || ChannelStatus(channel.StatusCode) != ChannelConfiguring ||
		audit.count() != 0 || manager.createCount != 0 {
		t.Fatalf("missing create was falsely completed: intent=%+v channel=%+v audits=%d creates=%d",
			pending, channel, audit.count(), manager.createCount)
	}
	completed, err := service.ConfigureChannel(t.Context(),
		channelConfigurationTestInput("configure-not-started-0001", "retry-secret"))
	if err != nil {
		t.Fatalf("retry original configure operation: %v", err)
	}
	if completed.Status != ChannelEnabled || completed.Version != 2 || manager.createCount != 1 {
		t.Fatalf("original retry did not complete safely: channel=%+v creates=%d", completed, manager.createCount)
	}
}

func TestChannelIntentRecoversReplaceAndClearCrashPostgreSQL(t *testing.T) {
	database, cleanup := openNotificationServicePostgreSQL(t)
	defer cleanup()
	manager := newChannelIntentSecretManager()
	audit := &channelIntentAuditStub{}
	service := channelIntentServiceFixture(t, database, manager, audit)
	configured, err := service.ConfigureChannel(t.Context(), channelConfigurationTestInput("configure-base-0001", "base-secret"))
	if err != nil {
		t.Fatalf("prepare configured channel: %v", err)
	}

	replaceInput := channelConfigurationTestInput("replace-crash-0001", "replacement-secret")
	replaceInput.ExpectedVersion = configured.Version
	replaceIntent, err := service.prepareConfigureIntent(t.Context(), 101, replaceInput, "{}")
	if err != nil {
		t.Fatalf("prepare replace intent: %v", err)
	}
	replaceHandle, err := secretcap.NewHandle(*replaceIntent.CredentialSecretRef)
	if err != nil {
		t.Fatalf("load replacement handle: %v", err)
	}
	replacePayload := channelSecretPayload(t, replaceInput.Secrets)
	if _, err = manager.Replace(t.Context(), replaceHandle, replaceIntent.SecretExpectedUpdatedAt.UTC(),
		replaceIntent.SecretOperationKey, func(consume secretcap.Consumer) error { return consume(replacePayload) }); err != nil {
		t.Fatalf("commit replacement before simulated crash: %v", err)
	}
	forceChannelIntentDue(t, database, replaceIntent.OperationNo)
	if err = channelIntentServiceFixture(t, database, manager, audit).ProcessChannelIntents(t.Context(), 10); err != nil {
		t.Fatalf("recover replacement: %v", err)
	}
	assertChannelIntentCompleted(t, database, replaceIntent.OperationNo, ChannelEnabled, 3)

	disabled, err := service.DisableChannel(t.Context(), ChannelLifecycleInput{
		ChannelCode: "email", ExpectedVersion: 3, IdempotencyKey: "disable-before-clear-0001", Confirmed: true,
	})
	if err != nil {
		t.Fatalf("disable channel before clear: %v", err)
	}
	clearInput := ChannelLifecycleInput{
		ChannelCode: "email", ExpectedVersion: disabled.Version, IdempotencyKey: "clear-crash-0001", Confirmed: true,
	}
	clearIntent, err := service.prepareClearIntent(t.Context(), 101, clearInput)
	if err != nil {
		t.Fatalf("prepare clear intent: %v", err)
	}
	clearHandle, err := secretcap.NewHandle(*clearIntent.CredentialSecretRef)
	if err != nil {
		t.Fatalf("load clear handle: %v", err)
	}
	if _, err = manager.Revoke(t.Context(), clearHandle, clearIntent.SecretExpectedUpdatedAt.UTC()); err != nil {
		t.Fatalf("revoke secret before simulated crash: %v", err)
	}
	forceChannelIntentDue(t, database, clearIntent.OperationNo)
	if err = channelIntentServiceFixture(t, database, manager, audit).ProcessChannelIntents(t.Context(), 10); err != nil {
		t.Fatalf("recover clear: %v", err)
	}
	assertChannelIntentCompleted(t, database, clearIntent.OperationNo, ChannelUnconfigured, 5)
	if manager.replaceCount != 1 || manager.revokeCount != 1 {
		t.Fatalf("recovery repeated external secret mutation: replaces=%d revokes=%d",
			manager.replaceCount, manager.revokeCount)
	}
}

func TestChannelIntentOnlyOneInstanceClaimsRecoveryPostgreSQL(t *testing.T) {
	database, cleanup := openNotificationServicePostgreSQL(t)
	defer cleanup()
	manager := newChannelIntentSecretManager()
	audit := &channelIntentAuditStub{}
	service := channelIntentServiceFixture(t, database, manager, audit)
	input := channelConfigurationTestInput("configure-multi-instance-0001", "shared-secret")
	intent, err := service.prepareConfigureIntent(t.Context(), 101, input, "{}")
	if err != nil {
		t.Fatalf("prepare configure intent: %v", err)
	}
	payload := channelSecretPayload(t, input.Secrets)
	if _, err = manager.Create(t.Context(), intent.SecretOperationKey,
		func(consume secretcap.Consumer) error { return consume(payload) }); err != nil {
		t.Fatalf("commit secret before simulated crash: %v", err)
	}
	forceChannelIntentDue(t, database, intent.OperationNo)

	start := make(chan struct{})
	errorsFound := make(chan error, 2)
	for index := 0; index < 2; index++ {
		instance := channelIntentServiceFixture(t, database, manager, audit)
		go func() {
			<-start
			errorsFound <- instance.ProcessChannelIntents(t.Context(), 10)
		}()
	}
	close(start)
	for index := 0; index < 2; index++ {
		if processErr := <-errorsFound; processErr != nil {
			t.Fatalf("multi-instance recovery: %v", processErr)
		}
	}
	assertChannelIntentCompleted(t, database, intent.OperationNo, ChannelEnabled, 2)
	if manager.recoverCount != 1 || audit.count() != 1 {
		t.Fatalf("multiple instances processed one intent: recovers=%d audits=%d", manager.recoverCount, audit.count())
	}
}

func TestChannelDisableCancelsUnsentWorkAndPendingReviewBlocksClearPostgreSQL(t *testing.T) {
	database, cleanup := openNotificationServicePostgreSQL(t)
	defer cleanup()
	manager := newChannelIntentSecretManager()
	audit := &channelIntentAuditStub{}
	service := channelIntentServiceFixture(t, database, manager, audit)
	configured, err := service.ConfigureChannel(t.Context(),
		channelConfigurationTestInput("configure-lifecycle-0001", "lifecycle-secret"))
	if err != nil {
		t.Fatalf("configure lifecycle channel: %v", err)
	}
	digest := strings.Repeat("b", 64)
	err = database.Transaction(t.Context(), func(ctx context.Context, tx gdb.TX) error {
		for _, fixture := range []struct {
			taskNo    string
			status    string
			crossed   bool
			retry     int
			queryTime string
		}{
			{taskNo: "ntask_disable_queued", status: string(DeliveryQueued)},
			{taskNo: "ntask_disable_retry", status: string(DeliveryRetryWait), retry: 2},
			{taskNo: "ntask_disable_pending", status: string(DeliverySubmitted), crossed: true,
				queryTime: "CURRENT_TIMESTAMP + INTERVAL '10 minutes'"},
		} {
			chainNo := strings.Replace(fixture.taskNo, "ntask_", "nchain_", 1)
			if _, insertErr := tx.Ctx(ctx).Exec(`INSERT INTO plugin_idc_notify_delivery_chain
            (tenant_id,chain_no,root_task_no,tail_task_no) VALUES (0,?,?,?)`,
				chainNo, fixture.taskNo, fixture.taskNo); insertErr != nil {
				return insertErr
			}
			queryDeadline := "NULL"
			if fixture.queryTime != "" {
				queryDeadline = fixture.queryTime
			}
			if _, insertErr := tx.Ctx(ctx).Exec(`INSERT INTO plugin_idc_notify_delivery_task
            (tenant_id,task_no,chain_no,event_code,topic_code,channel_code,locale_code,
             business_object_type,business_object_no,business_object_version,body_content,
             content_digest,status_code,result_class,external_boundary_crossed,retry_sequence,
             next_attempt_at,provider_query_deadline,provider_query_deadline_source)
            VALUES (0,?,?, 'support.ticket_updated','support','email','zh-CN',
                    'support_ticket',?,1,'body',?,?,?, ?,?,
                    CASE WHEN ?='retry_wait' THEN CURRENT_TIMESTAMP + INTERVAL '5 minutes' ELSE NULL END,`+queryDeadline+`,
                    CASE WHEN ?='submitted' THEN 'provider_receipt' ELSE '' END)`,
				fixture.taskNo, chainNo, fixture.taskNo, digest, fixture.status,
				map[bool]string{true: "accepted", false: "none"}[fixture.crossed], fixture.crossed,
				fixture.retry, fixture.status, fixture.status); insertErr != nil {
				return insertErr
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("prepare lifecycle tasks: %v", err)
	}
	disabled, err := service.DisableChannel(t.Context(), ChannelLifecycleInput{
		ChannelCode: "email", ExpectedVersion: configured.Version,
		IdempotencyKey: "disable-lifecycle-0001", Confirmed: true,
	})
	if err != nil || disabled.Status != ChannelDisabled {
		t.Fatalf("disable lifecycle channel: channel=%+v err=%v", disabled, err)
	}
	var statuses []struct {
		TaskNo string `orm:"task_no"`
		Status string `orm:"status_code"`
	}
	if err = database.Model(deliveryTable).Ctx(t.Context()).Fields("task_no,status_code").
		WhereIn("task_no", []string{"ntask_disable_queued", "ntask_disable_retry", "ntask_disable_pending"}).
		OrderAsc("task_no").Scan(&statuses); err != nil {
		t.Fatalf("read lifecycle task statuses: %v", err)
	}
	wantStatus := map[string]string{
		"ntask_disable_pending": string(DeliverySubmitted),
		"ntask_disable_queued":  string(DeliveryCancelled),
		"ntask_disable_retry":   string(DeliveryCancelled),
	}
	for _, item := range statuses {
		if item.Status != wantStatus[item.TaskNo] {
			t.Fatalf("disable changed task %s to %s, want %s", item.TaskNo, item.Status, wantStatus[item.TaskNo])
		}
	}
	if _, err = service.ClearChannel(t.Context(), ChannelLifecycleInput{
		ChannelCode: "email", ExpectedVersion: disabled.Version,
		IdempotencyKey: "clear-pending-lifecycle-0001", Confirmed: true,
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("pending provider review must block credential clearance: %v", err)
	}
	enabled, err := service.EnableChannel(t.Context(), ChannelLifecycleInput{
		ChannelCode: "email", ExpectedVersion: disabled.Version,
		IdempotencyKey: "enable-lifecycle-0001", Confirmed: true,
	})
	if err != nil || enabled.Status != ChannelEnabled {
		t.Fatalf("re-enable lifecycle channel: channel=%+v err=%v", enabled, err)
	}
	for taskNo, want := range wantStatus {
		status, statusErr := database.Model(deliveryTable).Ctx(t.Context()).Fields("status_code").
			Where("task_no", taskNo).Value()
		if statusErr != nil || status.String() != want {
			t.Fatalf("re-enable changed historical task %s: status=%s err=%v", taskNo, status.String(), statusErr)
		}
	}
}

func TestChannelIntentMigrationIsIdempotentAndRejectsOtherTenantsPostgreSQL(t *testing.T) {
	database, cleanup := openNotificationServicePostgreSQL(t)
	defer cleanup()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve channel intent test path")
	}
	pluginRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	content, err := os.ReadFile(filepath.Join(pluginRoot, "manifest", "sql", "001-notification-foundation.sql"))
	if err != nil {
		t.Fatalf("read notification migration: %v", err)
	}
	if _, err = database.Exec(t.Context(), string(content)); err != nil {
		t.Fatalf("reapply notification migration: %v", err)
	}
	if _, err = database.Exec(t.Context(), `INSERT INTO plugin_idc_notify_channel_intent
        (tenant_id,operation_no,channel_code,operation_code,status_code,actor_user_id,
         base_status_code,base_version,request_shape_digest,desired_non_secret_configuration,secret_operation_key)
        VALUES (1,'cross_tenant_intent','email','configure_create','prepared',101,
                'unconfigured',1,?,'{}'::jsonb,'cross_tenant_secret_operation')`, strings.Repeat("a", 64)); err == nil {
		t.Fatal("channel intent accepted a non-default tenant")
	}
}

func channelConfigurationTestInput(key, secret string) ChannelConfigurationInput {
	return ChannelConfigurationInput{
		ChannelCode: "email", ProviderCode: "test_email", NonSecret: map[string]string{},
		Secrets: map[string][]byte{"token": []byte(secret)}, ExpectedVersion: 1, IdempotencyKey: key,
	}
}

func channelSecretPayload(t *testing.T, secrets map[string][]byte) []byte {
	t.Helper()
	payload, err := json.Marshal(secrets)
	if err != nil {
		t.Fatalf("marshal channel secret payload: %v", err)
	}
	return payload
}

func forceChannelIntentDue(t *testing.T, database gdb.DB, operationNo string) {
	t.Helper()
	if _, err := database.Exec(t.Context(), `UPDATE plugin_idc_notify_channel_intent
        SET available_at=CURRENT_TIMESTAMP - INTERVAL '1 second',lease_token=NULL,lease_expires_at=NULL
        WHERE tenant_id=0 AND operation_no=?`, operationNo); err != nil {
		t.Fatalf("make channel intent due: %v", err)
	}
}

func assertChannelIntentCompleted(
	t *testing.T,
	database gdb.DB,
	operationNo string,
	status ChannelStatus,
	version int64,
) {
	t.Helper()
	var intent channelIntentRow
	if err := database.Model(channelIntentTable).Ctx(t.Context()).
		Where(tenantIDColumn, notificationTenantID).Where("operation_no", operationNo).Scan(&intent); err != nil {
		t.Fatalf("load recovered channel intent: %v", err)
	}
	var channel channelRow
	if err := database.Model(channelTable).Ctx(t.Context()).
		Where(tenantIDColumn, notificationTenantID).Where("channel_code", "email").Scan(&channel); err != nil {
		t.Fatalf("load recovered channel: %v", err)
	}
	if intent.StatusCode != channelIntentCompleted || intent.CompletedAt == nil ||
		ChannelStatus(channel.StatusCode) != status || channel.ConfigurationVersion != version {
		t.Fatalf("unexpected recovery state: intent=%+v channel=%+v", intent, channel)
	}
	if status == ChannelUnconfigured {
		if channel.CredentialSecretRef != nil || channel.CredentialUpdatedAt != nil {
			t.Fatalf("cleared channel retained credential routing metadata: %+v", channel)
		}
	} else if channel.CredentialSecretRef == nil || channel.CredentialUpdatedAt == nil ||
		strings.TrimSpace(*channel.CredentialSecretRef) == "" {
		t.Fatalf("configured channel did not retain its opaque credential reference: %+v", channel)
	}
}
