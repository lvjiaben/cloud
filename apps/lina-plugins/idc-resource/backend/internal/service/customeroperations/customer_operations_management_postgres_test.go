// These tests exercise native management against real migrations and resource
// task storage, including transactional confirmation and one-time output loss.
package customeroperations

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/customerauthcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
	"lina-plugin-idc-resource/backend/internal/service/serveroperationsource"
)

type confirmationCustomer struct{ *nativeCustomer }

func (c *confirmationCustomer) PrepareCurrentConfirmation(ctx context.Context, digest string) (*customerauthcap.SessionConfirmation, error) {
	_, err := c.database.Exec(ctx, `INSERT INTO test_native_confirmation(digest,used)VALUES(?,false) ON CONFLICT(digest)DO NOTHING`, digest)
	return &customerauthcap.SessionConfirmation{Required: true, Token: digest, ExpiresAt: time.Now().Add(time.Minute)}, err
}
func (c *confirmationCustomer) ConsumeCurrentConfirmation(ctx context.Context, digest, token string) error {
	if token != digest {
		return ErrInvalid
	}
	result, err := c.database.Exec(ctx, `UPDATE test_native_confirmation SET used=true WHERE digest=? AND used=false`, digest)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil || count != 1 {
		return ErrConflict
	}
	return nil
}

type operationSecrets struct {
	manager *operationSecretManager
	secretcap.Source
	calls int
	value []byte
}

func (s *operationSecrets) Accessor(secretcap.Scope) (secretcap.Accessor, error) { return s, nil }
func (s *operationSecrets) Access(_ context.Context, _ secretcap.AccessRequest, consume secretcap.Consumer) error {
	s.calls++
	return consume(s.value)
}

type managementFixture struct {
	ctx       context.Context
	db        gdb.DB
	service   *Service
	customers *confirmationCustomer
	provider  *nativeProvider
	source    *serveroperationsource.Service
	secrets   *operationSecrets
	audit     *nativeAudit
	serverNo  string
}

func newManagementFixture(t *testing.T) *managementFixture {
	t.Helper()
	ctx, db := newNativeDatabase(t)
	seedDirectoryFoundation(t, ctx, db)
	seedDirectoryServer(t, ctx, db, "customer-a", "1", "active", "succeeded")
	if _, err := db.Exec(ctx, `DELETE FROM plugin_idc_resource_capability_snapshot_item;
 CREATE TABLE test_native_confirmation(digest text PRIMARY KEY,used boolean NOT NULL);
 INSERT INTO plugin_idc_resource_provider_capacity_observation(observation_no,channel_no,capacity_kind,status_code,item_count,observed_at,completed_at)VALUES('images-main','channel-main','image','complete',2,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
 INSERT INTO plugin_idc_resource_provider_capacity_observation_item(observation_no,capacity_kind,external_ref,projection,observed_at)VALUES
 ('images-main','image','101','{"name":"Debian 12","group":"Debian","status":1,"os_type":"linux","arch":"x86_64","boot_mode":"uefi"}',CURRENT_TIMESTAMP),
 ('images-main','image','102','{"name":"Unavailable system","status":0}',CURRENT_TIMESTAMP);`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE plugin_idc_resource_capability_snapshot SET item_count=?`, resourcecap.MaximumCapabilityCount); err != nil {
		t.Fatal(err)
	}
	for _, capability := range resourcecap.SupportedCapabilityCodes() {
		support, constraints := "supported", "[]"
		switch capability {
		case resourcecap.CapabilityStop, resourcecap.CapabilityReboot:
			support = "conditional"
			constraints = `[{"name":"mode","required":false,"allowedValues":["graceful","hard"]}]`
		case resourcecap.CapabilityEnterRescue:
			support = "conditional"
			constraints = `[{"name":"system","required":true,"allowedValues":["linux","windows"]}]`
		case resourcecap.CapabilityReinstall:
			support = "conditional"
			constraints = `[{"name":"os","required":true,"minimum":1},{"name":"port","required":false,"minimum":1,"maximum":65535},{"name":"format_data_disk","required":false,"allowedValues":["0","1"]},{"name":"system_disk_size","required":false,"minimum":1}]`
		}
		if _, err := db.Exec(ctx, `INSERT INTO plugin_idc_resource_capability_snapshot_item(snapshot_no,capability_code,support_code,constraints,unsupported_reason)VALUES('capability-main',?,?,?::jsonb,'')`, capability, support, constraints); err != nil {
			t.Fatal(err)
		}
	}
	customers := &confirmationCustomer{&nativeCustomer{database: db, customer: "customer-a", eligible: true}}
	provider := &nativeProvider{state: resourcecap.ResourceStateActive}
	secrets := &operationSecrets{manager: &operationSecretManager{}, value: []byte(`{"username":"root","password":"Generated-Only-In-Isolated-Test","port":22}`)}
	source, err := serveroperationsource.NewWithSecrets(db, secrets)
	if err != nil {
		t.Fatal(err)
	}
	audit := &nativeAudit{}
	service, err := New(db, customers, nativeRuntime{provider}, source, audit, secrets, nativeFacts(t, db, provider), &nativeObjectOwner{})
	if err != nil {
		t.Fatal(err)
	}
	no, err := db.GetValue(ctx, `SELECT server_no FROM plugin_idc_resource_server_entitlement_projection`)
	if err != nil {
		t.Fatal(err)
	}
	return &managementFixture{ctx: ctx, db: db, service: service, customers: customers, provider: provider, source: source, secrets: secrets, audit: audit, serverNo: no.String()}
}

func TestNativeReinstallConfirmationAndDispatchPostgreSQL(t *testing.T) {
	f := newManagementFixture(t)
	command := Command{ServerNo: f.serverNo, Action: "reinstall", IdempotencyKey: "reinstall-1", Confirmed: true, Parameters: json.RawMessage(`{"os":101,"port":1234}`)}
	options, err := f.service.Options(f.ctx, f.serverNo)
	if err != nil || options.Images == nil || len(options.Images.Images) != 2 {
		t.Fatalf("owned image directory: %+v %v", options, err)
	}
	for _, image := range options.Images.Images {
		if image.Selectable != (image.Reference == "101") {
			t.Fatalf("wrong image selectability: %+v", image)
		}
	}
	invalid := command
	invalid.Parameters = json.RawMessage(`{"os":102}`)
	if _, err = f.service.Prepare(f.ctx, invalid); err == nil {
		t.Fatal("unavailable image prepared")
	}
	proof, err := f.service.Prepare(f.ctx, command)
	if err != nil || !proof.Required || proof.Token == "" {
		t.Fatalf("prepare: %v", err)
	}
	if _, err = f.service.Create(f.ctx, command); err == nil {
		t.Fatal("missing proof accepted")
	}
	command.ProofToken = proof.Token
	changed := command
	changed.Parameters = json.RawMessage(`{"os":101,"port":2222}`)
	if _, err = f.service.Create(f.ctx, changed); err == nil {
		t.Fatal("proof accepted changed parameter")
	}
	f.audit.fail = true
	if _, err = f.service.Create(f.ctx, command); err == nil {
		t.Fatal("audit rollback ignored")
	}
	used, err := f.db.GetValue(f.ctx, `SELECT used FROM test_native_confirmation WHERE digest=?`, proof.Token)
	if err != nil || used.Bool() {
		t.Fatal("proof consumed despite task rollback", err)
	}
	f.audit.fail = false
	created, err := f.service.Create(f.ctx, command)
	if err != nil || created == nil {
		t.Fatalf("reinstall admission: %v", err)
	}
	f.customers.eligible = false
	before := f.provider.calls
	replay, err := f.service.Create(f.ctx, command)
	if err != nil || replay.OperationNo != created.OperationNo || f.provider.calls != before {
		t.Fatalf("replay changed after entitlement: %v", err)
	}
	if _, err = f.service.Create(f.ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("JSONB normalization ignored changed original parameters")
	}
	f.customers.eligible = true
	store, err := resourcetask.NewStore(f.db)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := store.Claim(f.ctx, resourcetask.ClaimInput{WorkerID: "native-reinstall", Limit: 1, LeaseDuration: time.Minute})
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim: %v", err)
	}
	task := claimed[0]
	request := resourcecap.OperationRequest{RequestID: task.TaskNo, Capability: resourcecap.CapabilityReinstall, ResourceRef: "external-1", Parameters: []byte(task.SanitizedParameters)}
	gate, err := NewWriteAdmission(f.db, f.customers, f.source, f.service.objects)
	if err != nil {
		t.Fatal(err)
	}
	writes := 0
	invoke := func(context.Context) error { writes++; return nil }
	if err = gate(f.ctx, task, request, f.provider, invoke); err != nil || writes != 1 {
		t.Fatalf("reinstall queued gate: %v", err)
	}
	request.Parameters = []byte(`{"os":"101","port":"2222"}`)
	if err = gate(f.ctx, task, request, f.provider, invoke); err == nil || writes != 1 {
		t.Fatal("tampered translated payload accepted")
	}
	request.Parameters = []byte(task.SanitizedParameters)
	if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_provider_capacity_observation_item SET projection=jsonb_set(projection,'{status}','0') WHERE external_ref='101'`); err != nil {
		t.Fatal(err)
	}
	if err = gate(f.ctx, task, request, f.provider, invoke); err == nil || writes != 1 {
		t.Fatal("image disabled after enqueue reached provider")
	}
}

func TestNativeOperationOutputClaimsPostgreSQL(t *testing.T) {
	f := newManagementFixture(t)
	operationNo := "nso_" + strings.Repeat("a", 32)
	if _, err := f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_task SET capability_code='server.reinstall',owner_metadata=?::jsonb WHERE task_no='task-1'`, `{"operationNo":"`+operationNo+`","action":"reinstall"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(f.ctx, `INSERT INTO plugin_idc_resource_task_secret_binding(binding_no,task_no,binding_name,binding_direction,secret_ref,output_policy,status_code)VALUES('binding-native-output','task-1','generatedCredential','output','secret-native-output','one_time_credential','active')`); err != nil {
		t.Fatal(err)
	}
	before := f.provider.calls
	result, err := f.service.GetOperation(f.ctx, f.serverNo, operationNo)
	if err != nil || len(result.AvailableOutputs) != 1 || f.provider.calls != before || f.secrets.calls != 0 {
		t.Fatalf("local output availability: %+v %v", result, err)
	}
	f.customers.customer = "customer-b"
	if _, err = f.service.GetOperation(f.ctx, f.serverNo, operationNo); err == nil {
		t.Fatal("foreign task visible")
	}
	deliveries := 0
	consume := func([]byte) error { deliveries++; return errors.New("simulated lost HTTP response") }
	if err = f.service.RevealOutput(f.ctx, f.serverNo, operationNo, "generatedCredential", true, consume); err == nil || deliveries != 0 {
		t.Fatal("foreign output delivered")
	}
	f.customers.customer = "customer-a"
	f.audit.fail = true
	if err = f.service.RevealOutput(f.ctx, f.serverNo, operationNo, "generatedCredential", true, consume); err == nil || f.secrets.calls != 0 {
		t.Fatal("audit failure exposed output")
	}
	f.audit.fail = false
	if err = f.service.RevealOutput(f.ctx, f.serverNo, operationNo, "generatedCredential", true, consume); err == nil || deliveries != 1 {
		t.Fatal("lost response simulation did not run")
	}
	if err = f.service.RevealOutput(f.ctx, f.serverNo, operationNo, "generatedCredential", true, consume); err == nil || deliveries != 1 || f.secrets.calls != 1 {
		t.Fatal("lost output delivered twice")
	}
	result, err = f.service.GetOperation(f.ctx, f.serverNo, operationNo)
	if err != nil || len(result.AvailableOutputs) != 0 {
		t.Fatal("consumed output still advertised")
	}
}

func TestNativeOutputConcurrentConsumersReceiveAtMostOncePostgreSQL(t *testing.T) {
	f := newManagementFixture(t)
	operationNo := "nso_" + strings.Repeat("b", 32)
	if _, err := f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_task SET capability_code='server.reinstall',owner_metadata=?::jsonb WHERE task_no='task-1'`, `{"operationNo":"`+operationNo+`","action":"reinstall"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(f.ctx, `INSERT INTO plugin_idc_resource_task_secret_binding(binding_no,task_no,binding_name,binding_direction,secret_ref,output_policy,status_code)VALUES('concurrent-output','task-1','generatedCredential','output','secret-native-output','one_time_credential','active')`); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	var success atomic.Int32
	var group sync.WaitGroup
	start := make(chan struct{})
	for range 4 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			err := f.service.RevealOutput(f.ctx, f.serverNo, operationNo, "generatedCredential", true, func([]byte) error { calls.Add(1); return nil })
			if err == nil {
				success.Add(1)
			}
		}()
	}
	close(start)
	group.Wait()
	if calls.Load() != 1 || success.Load() != 1 || f.secrets.calls != 1 {
		t.Fatalf("concurrent output delivered %d times (%d successes)", calls.Load(), success.Load())
	}
}

// A test manager receives plaintext only inside the same producer/accessor
// contract as the real encrypted owner; no ordinary record stores these bytes.
type operationSecretManager struct {
	secretcap.Manager
	creates, revokes int
}

func (m *operationSecretManager) Create(_ context.Context, _ string, produce secretcap.PlaintextProducer) (secretcap.CreateResult, error) {
	if err := produce(func(value []byte) error {
		if len(value) < 8 {
			return ErrInvalid
		}
		return nil
	}); err != nil {
		return secretcap.CreateResult{}, err
	}
	m.creates++
	handle, err := secretcap.NewHandle("secret-native-input")
	return secretcap.CreateResult{Handle: handle}, err
}
func (m *operationSecretManager) Metadata(context.Context, secretcap.Handle) (secretcap.Metadata, error) {
	return secretcap.Metadata{Configured: true, UpdatedAt: time.Now()}, nil
}
func (m *operationSecretManager) Revoke(context.Context, secretcap.Handle, time.Time) (secretcap.Metadata, error) {
	m.revokes++
	return secretcap.Metadata{}, nil
}
func (s *operationSecrets) Manager(secretcap.Scope) (secretcap.Manager, error) { return s.manager, nil }

func TestNativePasswordResetSecretBoundaryPostgreSQL(t *testing.T) {
	f := newManagementFixture(t)
	command := Command{ServerNo: f.serverNo, Action: "reset_password", IdempotencyKey: "reset-password-1", Confirmed: true}
	proof, err := f.service.Prepare(f.ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	command.ProofToken = proof.Token
	command.Password = []byte("New-Password-Isolated-Only")
	f.audit.fail = true
	if _, err = f.service.Create(f.ctx, command); err == nil {
		t.Fatal("audit failure accepted password operation")
	}
	for _, value := range command.Password {
		if value != 0 {
			t.Fatal("input password not zeroed")
		}
	}
	if f.secrets.manager.revokes != 1 {
		t.Fatal("orphan password not revoked")
	}
	f.audit.fail = false
	command.Password = []byte("New-Password-Isolated-Only")
	created, err := f.service.Create(f.ctx, command)
	if err != nil || created == nil {
		t.Fatalf("reset admission: %v", err)
	}
	row, err := f.db.GetValue(f.ctx, `SELECT row_to_json(task)::text FROM plugin_idc_resource_task task WHERE capability_code='server.password.reset'`)
	if err != nil || strings.Contains(row.String(), "New-Password-Isolated-Only") || strings.Contains(row.String(), "secret-native-input") {
		t.Fatal("ordinary task leaked input", err)
	}
	store, err := resourcetask.NewStore(f.db)
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := store.Claim(f.ctx, resourcetask.ClaimInput{WorkerID: "native-password", Limit: 1, LeaseDuration: time.Minute})
	if err != nil || len(tasks) != 1 {
		t.Fatalf("claim: %v", err)
	}
	task := tasks[0]
	if len(task.SecretBindings) != 1 || task.SecretBindings[0].Name != "password" {
		t.Fatal("queued request lost scoped input")
	}
	request := resourcecap.OperationRequest{RequestID: task.TaskNo, Capability: resourcecap.CapabilityResetPassword, ResourceRef: "external-1", Parameters: []byte(task.SanitizedParameters), SecretBindings: task.SecretBindings}
	gate, err := NewWriteAdmission(f.db, f.customers, f.source, f.service.objects)
	if err != nil {
		t.Fatal(err)
	}
	writes := 0
	invoke := func(context.Context) error { writes++; return nil }
	if err = gate(f.ctx, task, request, f.provider, invoke); err != nil || writes != 1 {
		t.Fatalf("reset queued gate: %v", err)
	}
	request.SecretBindings = nil
	if err = gate(f.ctx, task, request, f.provider, invoke); err == nil || writes != 1 {
		t.Fatal("missing input binding dispatched")
	}
}

func TestNativeRescueUsesSeparateSecretAndObservedModePostgreSQL(t *testing.T) {
	f := newManagementFixture(t)
	command := Command{ServerNo: f.serverNo, Action: "enter_rescue", Parameters: []byte(`{"system":"linux"}`), IdempotencyKey: "rescue-1", Confirmed: true}
	if _, err := f.service.Prepare(f.ctx, command); err == nil {
		t.Fatal("unknown rescue mode accepted")
	}
	f.provider.data = []byte(`{"rescue":false}`)
	proof, err := f.service.Prepare(f.ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	command.ProofToken = proof.Token
	command.Password = []byte("Temporary-Password-Isolated")
	created, err := f.service.Create(f.ctx, command)
	if err != nil || created == nil {
		t.Fatalf("enter rescue: %v", err)
	}
	store, err := resourcetask.NewStore(f.db)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := store.Claim(f.ctx, resourcetask.ClaimInput{WorkerID: "native-rescue", Limit: 1, LeaseDuration: time.Minute})
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim rescue: %v", err)
	}
	task := claimed[0]
	if len(task.SecretBindings) != 1 || task.SecretBindings[0].Name != "temporary_password" {
		t.Fatal("wrong rescue binding")
	}
	request := resourcecap.OperationRequest{RequestID: task.TaskNo, Capability: resourcecap.CapabilityEnterRescue, ResourceRef: "external-1", Parameters: []byte(task.SanitizedParameters), SecretBindings: task.SecretBindings}
	gate, err := NewWriteAdmission(f.db, f.customers, f.source, f.service.objects)
	if err != nil {
		t.Fatal(err)
	}
	writes := 0
	invoke := func(context.Context) error { writes++; return nil }
	if err = gate(f.ctx, task, request, f.provider, invoke); err != nil || writes != 1 {
		t.Fatalf("dispatch rescue: %v", err)
	}
	f.provider.data = []byte(`{"rescue":true}`)
	if err = gate(f.ctx, task, request, f.provider, invoke); err == nil || writes != 1 {
		t.Fatal("already-rescue mode dispatched another enter")
	}
	if _, err = f.db.Exec(f.ctx, `UPDATE plugin_idc_resource_task SET status_code='succeeded',finished_at=CURRENT_TIMESTAMP WHERE task_no=?`, task.TaskNo); err != nil {
		t.Fatal(err)
	}
	exit := Command{ServerNo: f.serverNo, Action: "exit_rescue", IdempotencyKey: "rescue-exit-1", Confirmed: true}
	proof, err = f.service.Prepare(f.ctx, exit)
	if err != nil {
		t.Fatal(err)
	}
	exit.ProofToken = proof.Token
	if _, err = f.service.Create(f.ctx, exit); err != nil {
		t.Fatalf("exit rescue: %v", err)
	}
}
