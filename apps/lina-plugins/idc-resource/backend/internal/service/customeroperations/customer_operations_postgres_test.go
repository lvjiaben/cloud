// This file verifies real owner tasks, customer isolation, replay, rollback and one-time delivery.
package customeroperations

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/customerauthcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
	"lina-plugin-idc-resource/backend/internal/service/serverfacts"
	"lina-plugin-idc-resource/backend/internal/service/serveroperationsource"
)

type nativeCustomer struct {
	database gdb.DB
	customer string
	eligible bool
}

func (c *nativeCustomer) PrepareCurrentConfirmation(context.Context, string) (*customerauthcap.SessionConfirmation, error) {
	return &customerauthcap.SessionConfirmation{}, nil
}
func (c *nativeCustomer) ConsumeCurrentConfirmation(context.Context, string, string) error {
	return nil
}

func (c *nativeCustomer) CurrentCustomer(context.Context) (*customerauthcap.Subject, error) {
	return &customerauthcap.Subject{CustomerNo: c.customer}, nil
}
func (c *nativeCustomer) ReadCurrentEntitlement(_ context.Context, no string) (*customerauthcap.Entitlement, error) {
	return &customerauthcap.Entitlement{EntitlementNo: no, CustomerNo: c.customer, CanOperate: c.eligible, Version: 1, Status: "active"}, nil
}
func (c *nativeCustomer) WithCurrentEntitlement(ctx context.Context, no string, action func(context.Context, *customerauthcap.Entitlement) error) error {
	if !c.eligible {
		return ErrUnavailable
	}
	return c.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		item, _ := c.ReadCurrentEntitlement(ctx, no)
		return action(ctx, item)
	})
}

func (c *nativeCustomer) WithQueuedEntitlement(ctx context.Context, no string, version int64, action func(context.Context, *customerauthcap.Entitlement) error) error {
	if !c.eligible || version != 1 {
		return ErrUnavailable
	}
	return c.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		item, err := c.ReadCurrentEntitlement(ctx, no)
		if err != nil {
			return err
		}
		return action(ctx, item)
	})
}

type nativeProvider struct {
	resourcecap.Provider
	state resourcecap.ResourceState
	data  []byte
	calls int
	delay time.Duration
}

func (p *nativeProvider) QueryResource(ctx context.Context, q resourcecap.ResourceQuery) (*resourcecap.ResourceSnapshot, error) {
	p.calls++
	if p.delay > 0 {
		timer := time.NewTimer(p.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	data := p.data
	if data == nil {
		data = []byte(`{"observed":true}`)
	}
	return &resourcecap.ResourceSnapshot{ExternalResourceRef: q.ExternalResourceRef, State: p.state, Data: data, ObservedAt: time.Now().UTC()}, nil
}

func TestNativeServerDetailDoesNotWaitForUnresponsiveSupplierPostgreSQL(t *testing.T) {
	ctx, database := newNativeDatabase(t)
	seedDirectoryFoundation(t, ctx, database)
	seedDirectoryServer(t, ctx, database, "customer-a", "1", "active", "succeeded")
	customers := &nativeCustomer{database: database, customer: "customer-a", eligible: true}
	provider := &nativeProvider{state: resourcecap.ResourceStateActive, delay: 10 * time.Second}
	secrets := &nativeSecrets{}
	source, err := serveroperationsource.NewWithSecrets(database, secrets)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(database, customers, nativeRuntime{provider}, source, &nativeAudit{}, secrets, nativeFacts(t, database, provider), &nativeObjectOwner{})
	if err != nil {
		t.Fatal(err)
	}
	public, err := database.GetValue(ctx, `SELECT server_no FROM plugin_idc_resource_server_entitlement_projection`)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	detail, err := service.Get(ctx, public.String())
	if err != nil || detail == nil || detail.ServerNo != public.String() || time.Since(started) > 2500*time.Millisecond {
		t.Fatalf("supplier blocked local server detail: duration=%s err=%v", time.Since(started), err)
	}
	if detail.RuntimeState != resourcecap.ResourceStateUnknown || len(detail.AvailableActions) != 0 || detail.CredentialAvailable || detail.AddressesConfirmed || provider.calls != 1 {
		t.Fatal("unconfirmed observation enabled actions or repeated the supplier request")
	}
}

type nativeRuntime struct{ provider *nativeProvider }

func (r nativeRuntime) ConstructSession(context.Context, string) (resourcecap.Provider, func(), error) {
	return r.provider, func() {}, nil
}

type nativeAudit struct {
	idccap.BusinessAuditWriter
	fail  bool
	calls int
}

func (a *nativeAudit) AppendBusinessTransition(_ context.Context, input idccap.BusinessAuditTransition) error {
	a.calls++
	// Match the real audit owner's transition contract; nil is not an empty snapshot.
	if input.BeforeSnapshot == nil || input.AfterSnapshot == nil {
		return errors.New("audit transition snapshots are required")
	}
	if a.fail {
		return errors.New("forced audit failure")
	}
	return nil
}

type nativeSecrets struct {
	secretcap.Source
	calls int
}

func (s *nativeSecrets) Accessor(secretcap.Scope) (secretcap.Accessor, error) { return s, nil }
func (s *nativeSecrets) Access(_ context.Context, _ secretcap.AccessRequest, consume secretcap.Consumer) error {
	s.calls++
	value := []byte("Generated-Password-For-Isolated-Test")
	defer clear(value)
	return consume(value)
}

func TestNativeCustomerOperationsPostgreSQL(t *testing.T) {
	ctx, database := newNativeDatabase(t)
	seedDirectoryFoundation(t, ctx, database)
	seedDirectoryServer(t, ctx, database, "customer-a", "1", "active", "succeeded")
	if _, err := database.Exec(ctx, `DELETE FROM plugin_idc_resource_capability_snapshot_item`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_resource_capability_snapshot SET item_count=?`, resourcecap.MaximumCapabilityCount); err != nil {
		t.Fatal(err)
	}
	for _, capability := range resourcecap.SupportedCapabilityCodes() {
		support := "supported"
		constraints := `[]`
		if capability == resourcecap.CapabilityStop || capability == resourcecap.CapabilityReboot {
			support = "conditional"
			constraints = `[{"name":"mode","required":false,"allowedValues":["graceful","hard"]}]`
		}
		if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_capability_snapshot_item(snapshot_no,capability_code,support_code,constraints,unsupported_reason)VALUES('capability-main',?,?,?::jsonb,'')`, capability, support, constraints); err != nil {
			t.Fatal(err)
		}
	}
	customers := &nativeCustomer{database: database, customer: "customer-a", eligible: true}
	provider := &nativeProvider{state: resourcecap.ResourceStateActive}
	audit := &nativeAudit{}
	secrets := &nativeSecrets{}
	source, err := serveroperationsource.NewWithSecrets(database, secrets)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(database, customers, nativeRuntime{provider}, source, audit, secrets, nativeFacts(t, database, provider), &nativeObjectOwner{})
	if err != nil {
		t.Fatal(err)
	}
	public, err := database.GetValue(ctx, `SELECT server_no FROM plugin_idc_resource_server_entitlement_projection`)
	if err != nil {
		t.Fatal(err)
	}
	serverNo := public.String()
	if resolved, resolveErr := service.Resolve(ctx, "ent_"+strings.Repeat("1", 32)); resolveErr != nil || resolved != serverNo {
		t.Fatalf("separate entitlement/resource identity did not resolve: %v", resolveErr)
	}
	detail, err := service.Get(ctx, serverNo)
	if err != nil || !slices.Contains(detail.AvailableActions, "reboot") || detail.CredentialAvailable {
		t.Fatalf("native detail=%+v err=%v", detail, err)
	}
	customers.customer = "customer-b"
	beforeCalls := provider.calls
	if _, err = service.Get(ctx, serverNo); !errors.Is(err, ErrUnavailable) || provider.calls != beforeCalls {
		t.Fatal("foreign read reached supplier")
	}
	if _, err = service.Create(ctx, Command{ServerNo: serverNo, Action: "start", IdempotencyKey: "foreign"}); err == nil {
		t.Fatal("foreign write accepted")
	}
	customers.customer = "customer-a"
	command := Command{ServerNo: serverNo, Action: "reboot", IdempotencyKey: "request-1", Confirmed: true}
	rejected := command
	rejected.Confirmed = false
	if _, err = service.Create(ctx, rejected); !errors.Is(err, ErrInvalid) {
		t.Fatal("ordinary reboot skipped confirmation")
	}
	audit.fail = true
	if _, err = service.Create(ctx, command); err == nil {
		t.Fatal("audit failure committed operation")
	}
	count, err := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_resource_task WHERE capability_code='server.reboot'`)
	if err != nil || count.Int() != 0 {
		t.Fatal("task escaped audit rollback")
	}
	audit.fail = false
	created, err := service.Create(ctx, command)
	if err != nil || created == nil || created.Status != "pending" {
		t.Fatalf("create native operation=%+v err=%v", created, err)
	}
	beforeCalls = provider.calls
	provider.state = resourcecap.ResourceStateStopped
	replay, err := service.Create(ctx, command)
	if err != nil || replay.OperationNo != created.OperationNo || provider.calls != beforeCalls {
		t.Fatalf("replay touched supplier or lost identity: %+v %v", replay, err)
	}
	changed := command
	changed.Action = "stop"
	if _, err = service.Create(ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("same request key changed action")
	}
	detail, err = service.Get(ctx, serverNo)
	if err != nil || len(detail.AvailableActions) != 0 || len(detail.Operations) != 1 || detail.Operations[0].OperationNo != created.OperationNo {
		t.Fatalf("pending task did not fence detail: %+v %v", detail, err)
	}

	// Dispatch uses the real stored metadata and claim projection, then repeats
	// current owner and runtime admission after the HTTP request has ended.
	store, err := resourcetask.NewStore(database)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := store.Claim(ctx, resourcetask.ClaimInput{WorkerID: "native-dispatch", Limit: 1, LeaseDuration: time.Minute})
	if err != nil || len(claimed) != 1 {
		t.Fatalf("native task claim: %+v %v", claimed, err)
	}
	task := claimed[0]
	refs, err := store.ResolveResources(ctx, []resourcetask.ResourceResolutionRequest{{TaskNo: task.TaskNo, Capability: resourcecap.CapabilityCode(task.CapabilityCode), LocalResourceNo: task.LocalResourceNo, ChannelNo: task.ChannelNo, PoolNo: task.PoolNo}})
	if err != nil || refs[task.TaskNo] == "" {
		t.Fatal("native mapping unavailable")
	}
	admission, err := NewWriteAdmission(database, customers, source, &nativeObjectOwner{})
	if err != nil {
		t.Fatal(err)
	}
	request := resourcecap.OperationRequest{RequestID: task.TaskNo, Capability: resourcecap.CapabilityCode(task.CapabilityCode), ResourceRef: refs[task.TaskNo], Parameters: []byte(task.SanitizedParameters)}
	writes := 0
	invoke := func(context.Context) error { writes++; return nil }
	customers.eligible = false
	beforeCalls = provider.calls
	if gateErr := admission(ctx, task, request, provider, invoke); gateErr == nil || writes != 0 || provider.calls != beforeCalls {
		t.Fatal("queued revoked authority reached provider")
	}
	customers.eligible = true
	customers.customer = "customer-b"
	if gateErr := admission(ctx, task, request, provider, invoke); gateErr == nil || writes != 0 || provider.calls != beforeCalls {
		t.Fatal("queued transferred owner reached provider")
	}
	customers.customer = "customer-a"
	if gateErr := admission(ctx, task, request, provider, invoke); gateErr == nil || writes != 0 {
		t.Fatal("queued stale runtime permitted reboot")
	}
	provider.state = resourcecap.ResourceStateActive
	if gateErr := admission(ctx, task, request, provider, invoke); gateErr != nil || writes != 1 {
		t.Fatalf("valid queued native admission: %v writes=%d", gateErr, writes)
	}
	provider.state = resourcecap.ResourceStateStopped
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_task SET status_code='result_unknown',result_unknown=TRUE WHERE capability_code='server.reboot'`); err != nil {
		t.Fatal(err)
	}
	another := Command{ServerNo: serverNo, Action: "start", IdempotencyKey: "request-2"}
	if _, err = service.Create(ctx, another); !errors.Is(err, ErrConflict) {
		t.Fatal("unknown outcome allowed conflicting operation")
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_task SET status_code='failed',result_unknown=FALSE,finished_at=CURRENT_TIMESTAMP WHERE capability_code='server.reboot'`); err != nil {
		t.Fatal(err)
	}
	customers.eligible = false
	if _, err = service.Create(ctx, another); err == nil {
		t.Fatal("core entitlement gate was bypassed")
	}
	customers.eligible = true
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_resource_provision_credential(credential_no,service_no,task_no,channel_no,secret_ref,status_code)VALUES('credential-1',?,'task-1','channel-main','secret-native-initial','revealable')`, "SRV_"+strings.Repeat("1", 32)); err != nil {
		t.Fatal(err)
	}
	consumed := 0
	consumer := func([]byte) error { consumed++; return nil }
	if err = service.Reveal(ctx, serverNo, true, consumer); err == nil || consumed != 0 || secrets.calls != 0 {
		t.Fatal("unconfirmed core outcome exposed credential")
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_resource_provision_outcome_receipt(receipt_no,task_no,service_no,status_code,request_digest,task_evidence_digest,core_evidence_digest,operation_no,customer_no,configuration_snapshot_no)VALUES('receipt-1','task-1',?,'succeeded',repeat('a',64),repeat('b',64),repeat('c',64),'provision-1','customer-a','config-1')`, "SRV_"+strings.Repeat("1", 32)); err != nil {
		t.Fatal(err)
	}
	audit.fail = true
	if err = service.Reveal(ctx, serverNo, true, consumer); err == nil || consumed != 0 {
		t.Fatal("credential escaped audit failure")
	}
	audit.fail = false
	if err = service.Reveal(ctx, serverNo, true, consumer); err != nil || consumed != 1 || secrets.calls != 1 {
		t.Fatalf("confirmed credential delivery: consumed=%d accesses=%d err=%v", consumed, secrets.calls, err)
	}
	if err = service.Reveal(ctx, serverNo, true, consumer); err == nil || consumed != 1 || secrets.calls != 1 {
		t.Fatal("credential delivery replayed")
	}
}

// TestNativeServerNumberedPagesPostgreSQL proves accurate totals, page boundaries
// and customer isolation without issuing a supplier query per list item.
func TestNativeServerNumberedPagesPostgreSQL(t *testing.T) {
	ctx, database := newNativeDatabase(t)
	seedDirectoryFoundation(t, ctx, database)
	seedDirectoryServer(t, ctx, database, "customer-a", "1", "active", "succeeded")
	seedDirectoryServer(t, ctx, database, "customer-a", "2", "active", "succeeded")
	seedDirectoryServer(t, ctx, database, "customer-b", "3", "active", "succeeded")
	provider := &nativeProvider{state: resourcecap.ResourceStateActive}
	customers := &nativeCustomer{database: database, customer: "customer-a", eligible: true}
	secrets := &nativeSecrets{}
	source, err := serveroperationsource.NewWithSecrets(database, secrets)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(database, customers, nativeRuntime{provider}, source, &nativeAudit{}, secrets, nativeFacts(t, database, provider), &nativeObjectOwner{})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.List(ctx, 1, 1)
	if err != nil || first.Total != 2 || len(first.Items) != 1 {
		t.Fatalf("first page: %+v %v", first, err)
	}
	second, err := service.List(ctx, 2, 1)
	if err != nil || second.Total != 2 || len(second.Items) != 1 || second.Items[0].ServerNo == first.Items[0].ServerNo {
		t.Fatalf("second page: %+v %v", second, err)
	}
	empty, err := service.List(ctx, 3, 1)
	if err != nil || empty.Total != 2 || len(empty.Items) != 0 {
		t.Fatalf("empty last page: %+v %v", empty, err)
	}
	customers.customer = "customer-b"
	foreign, err := service.List(ctx, 1, 20)
	if err != nil || foreign.Total != 1 || len(foreign.Items) != 1 || foreign.Items[0].ServerNo == first.Items[0].ServerNo || foreign.Items[0].ServerNo == second.Items[0].ServerNo {
		t.Fatalf("foreign page: %+v %v", foreign, err)
	}
	if provider.calls != 0 {
		t.Fatal("listing queried a supplier")
	}
}

// Pending power operations expose only the active local protection deadline;
// expiry and cleared protection cannot leave a stale waiting explanation.
func TestNativePowerWaitingProjectionPostgreSQL(t *testing.T) {
	ctx, database := newNativeDatabase(t)
	seedDirectoryFoundation(t, ctx, database)
	seedDirectoryServer(t, ctx, database, "customer-a", "1", "active", "succeeded")
	customers := &nativeCustomer{database: database, customer: "customer-a", eligible: true}
	provider := &nativeProvider{state: resourcecap.ResourceStateStopped}
	secrets := &nativeSecrets{}
	source, err := serveroperationsource.NewWithSecrets(database, secrets)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(database, customers, nativeRuntime{provider}, source, &nativeAudit{}, secrets, nativeFacts(t, database, provider), &nativeObjectOwner{})
	if err != nil {
		t.Fatal(err)
	}
	public, err := database.GetValue(ctx, `SELECT server_no FROM plugin_idc_resource_server_entitlement_projection`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_resource_task(task_no,request_id_digest,capability_code,local_resource_no,channel_no,pool_no,status_code,owner_metadata)
VALUES('task-waiting',?,'server.start',?,'channel-main','pool-main','pending','{"operationNo":"nso_waiting","action":"start"}'::jsonb)`, strings.Repeat("c", 64), "SRV_"+strings.Repeat("1", 32)); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_resource_channel_health_state(channel_no,circuit_open,write_protected_until,window_started_at) VALUES('channel-main',TRUE,CURRENT_TIMESTAMP+INTERVAL '1 minute',CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	detail, err := service.Get(ctx, public.String())
	if err != nil || len(detail.Operations) != 1 || detail.Operations[0].WaitingUntil == nil || detail.Operations[0].Status != "pending" || len(detail.AvailableActions) != 0 {
		t.Fatalf("missing queue explanation: %+v %v", detail, err)
	}
	for _, mutation := range []string{
		`UPDATE plugin_idc_resource_channel_health_state SET circuit_open=FALSE`,
		`UPDATE plugin_idc_resource_channel_health_state SET circuit_open=TRUE,write_protected_until=CURRENT_TIMESTAMP-INTERVAL '1 second'`,
	} {
		if _, err = database.Exec(ctx, mutation); err != nil {
			t.Fatal(err)
		}
		detail, err = service.Get(ctx, public.String())
		if err != nil || len(detail.Operations) != 1 || detail.Operations[0].WaitingUntil != nil {
			t.Fatalf("stale waiting reason: %+v %v", detail, err)
		}
	}
}

func nativeFacts(t *testing.T, database gdb.DB, provider *nativeProvider) serverfacts.Service {
	t.Helper()
	facts, err := serverfacts.New(database, nativeRuntime{provider})
	if err != nil {
		t.Fatal(err)
	}
	return facts
}
