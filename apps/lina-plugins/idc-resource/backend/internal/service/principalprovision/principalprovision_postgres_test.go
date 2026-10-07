package principalprovision

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const principalIntegrationDSNEnv = "LINAPRO_IDC_RESOURCE_TEST_DSN"

func TestPrincipalProvisionPostgreSQLUnknownWriteReconcilesWithoutReplay(t *testing.T) {
	ctx, database := principalTestDatabase(t)
	seedPrincipalRoute(t, ctx, database)
	manager := newPrincipalTestManager()
	serviceValue, err := New(database, func(string) (secretcap.Manager, error) {
		return manager, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	service := serviceValue.(*serviceImpl)

	const concurrent = 12
	errorsOut := make(chan error, concurrent)
	var group sync.WaitGroup
	for index := 0; index < concurrent; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			ready, ensureErr := service.EnsureForPool(ctx, "customer-principal-1", "pool-principal-1")
			if ready || ensureErr != nil {
				errorsOut <- errors.New("unexpected principal readiness result")
			}
		}()
	}
	group.Wait()
	close(errorsOut)
	for ensureErr := range errorsOut {
		t.Fatal(ensureErr)
	}
	count, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_resource_principal_task`)
	if err != nil || count.Int() != 1 || manager.createCalls.Load() != 2 {
		t.Fatalf("principal task count=%d secret creates=%d err=%v", count.Int(), manager.createCalls.Load(), err)
	}

	provider := &principalTestProvider{}
	worker, err := NewWorker(database, func(string) (secretcap.Manager, error) {
		return manager, nil
	}, func(context.Context, string) (resourcecap.Provider, func(), error) {
		return provider, func() {}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 4; step++ {
		if step == 2 {
			if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_principal_task
SET available_at = CURRENT_TIMESTAMP
WHERE customer_no = 'customer-principal-1' AND status_code = 'verify_wait'`); err != nil {
				t.Fatal(err)
			}
		}
		worked, runErr := worker.RunOne(ctx)
		if runErr != nil || !worked {
			t.Fatalf("principal worker step %d worked=%v err=%v", step, worked, runErr)
		}
	}
	if provider.createCalls.Load() != 1 || provider.lookupCalls.Load() != 2 {
		t.Fatalf("principal calls lookup=%d create=%d", provider.lookupCalls.Load(), provider.createCalls.Load())
	}
	var state struct {
		StatusCode, ExternalPrincipalRef string
		ResultUnknown                    bool
	}
	if err = database.GetScan(ctx, &state, `
SELECT task.status_code, task.result_unknown, mapping.external_principal_ref
FROM plugin_idc_resource_principal_task task
INNER JOIN plugin_idc_resource_principal_mapping mapping
  ON mapping.channel_no = task.channel_no AND mapping.customer_no = task.customer_no
WHERE task.customer_no = 'customer-principal-1'`); err != nil {
		t.Fatal(err)
	}
	if state.StatusCode != "succeeded" || !state.ResultUnknown || state.ExternalPrincipalRef != "42" {
		t.Fatalf("principal terminal state=%+v", state)
	}
	if manager.activeCount() != 0 || manager.revokeCalls.Load() != 2 {
		t.Fatalf("active secrets=%d revoke calls=%d", manager.activeCount(), manager.revokeCalls.Load())
	}
	ready, err := service.EnsureForPool(ctx, "customer-principal-1", "pool-principal-1")
	if err != nil || !ready {
		t.Fatalf("confirmed principal mapping was not reusable: ready=%v err=%v", ready, err)
	}
}

func TestPrincipalProvisionPostgreSQLExpiredCreateLeaseBecomesLookupOnly(t *testing.T) {
	ctx, database := principalTestDatabase(t)
	seedPrincipalRoute(t, ctx, database)
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_principal_task
    (task_no, customer_no, channel_no, username_secret_ref, password_secret_ref,
     status_code, attempt_count, lease_token_digest, lease_expires_at)
VALUES ('prt_expired', 'customer-expired', 'channel-principal-1',
        'sec_expired_username', 'sec_expired_password', 'create_leased', 1,
        repeat('a', 64), CURRENT_TIMESTAMP - INTERVAL '1 second')`); err != nil {
		t.Fatal(err)
	}
	manager := newPrincipalTestManager()
	provider := &principalAlwaysAbsentProvider{}
	worker, err := NewWorker(database, func(string) (secretcap.Manager, error) {
		return manager, nil
	}, func(context.Context, string) (resourcecap.Provider, func(), error) {
		return provider, func() {}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	worked, err := worker.RunOne(ctx)
	if err != nil || !worked {
		t.Fatalf("recover expired principal create worked=%v err=%v", worked, err)
	}
	if provider.createCalls.Load() != 0 || provider.lookupCalls.Load() != 1 {
		t.Fatalf("expired create lookup=%d create=%d", provider.lookupCalls.Load(), provider.createCalls.Load())
	}
	var state struct {
		StatusCode string
		Unknown    bool `orm:"result_unknown"`
		Attempts   int  `orm:"attempt_count"`
	}
	if err = database.GetScan(ctx, &state, `
SELECT status_code, result_unknown, attempt_count
FROM plugin_idc_resource_principal_task WHERE task_no = 'prt_expired'`); err != nil {
		t.Fatal(err)
	}
	if state.StatusCode != "verify_wait" || !state.Unknown || state.Attempts != 1 {
		t.Fatalf("expired create state=%+v", state)
	}
}

func TestPrincipalProvisionPostgreSQLKnownRejectionRequiresFreshLookupBeforeRetry(t *testing.T) {
	ctx, database := principalTestDatabase(t)
	seedPrincipalRoute(t, ctx, database)
	manager := newPrincipalTestManager()
	serviceValue, err := New(database, func(string) (secretcap.Manager, error) {
		return manager, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	service := serviceValue.(*serviceImpl)
	ready, err := service.EnsureForPool(ctx, "customer-known-rejection", "pool-principal-1")
	if err != nil || ready {
		t.Fatalf("ensure principal ready=%v err=%v", ready, err)
	}
	provider := &principalKnownRejectionProvider{}
	worker, err := NewWorker(database, func(string) (secretcap.Manager, error) {
		return manager, nil
	}, func(context.Context, string) (resourcecap.Provider, func(), error) {
		return provider, func() {}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 4; step++ {
		if step == 2 {
			if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_principal_task
SET available_at = CURRENT_TIMESTAMP
WHERE customer_no = 'customer-known-rejection' AND status_code = 'verify_wait'`); err != nil {
				t.Fatal(err)
			}
		}
		worked, runErr := worker.RunOne(ctx)
		if runErr != nil || !worked {
			t.Fatalf("principal known rejection step %d worked=%v err=%v", step, worked, runErr)
		}
	}
	if got := provider.callOrder(); got != "lookup,create,lookup,create" {
		t.Fatalf("principal call order=%q, want exact lookup before each create", got)
	}
	var state struct {
		StatusCode           string `orm:"status_code"`
		ExternalPrincipalRef string `orm:"expected_external_principal_ref"`
		FailureCode          string `orm:"failure_code"`
		ResultUnknown        bool   `orm:"result_unknown"`
		Attempts             int    `orm:"attempt_count"`
	}
	if err = database.GetScan(ctx, &state, `
SELECT status_code, expected_external_principal_ref, failure_code,
       result_unknown, attempt_count
FROM plugin_idc_resource_principal_task
WHERE customer_no = 'customer-known-rejection'`); err != nil {
		t.Fatal(err)
	}
	if state.StatusCode != "verify_wait" || state.ExternalPrincipalRef != "7001" ||
		state.FailureCode != "" || state.ResultUnknown || state.Attempts != 2 {
		t.Fatalf("principal known rejection state=%+v", state)
	}
}

func TestPrincipalProvisionPostgreSQLExpiredRecoveryIsBounded(t *testing.T) {
	ctx, database := principalTestDatabase(t)
	seedPrincipalRoute(t, ctx, database)
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_principal_task
    (task_no, customer_no, channel_no, username_secret_ref, password_secret_ref,
     status_code, lease_token_digest, lease_expires_at)
SELECT 'prt_bounded_' || value, 'customer-bounded-' || value, 'channel-principal-1',
       'sec_bounded_username_' || value, 'sec_bounded_password_' || value,
       'lookup_leased', repeat('b', 64), CURRENT_TIMESTAMP - INTERVAL '1 second'
FROM generate_series(1, 40) AS value`); err != nil {
		t.Fatal(err)
	}
	provider := &principalAlwaysAbsentProvider{}
	worker, err := NewWorker(database, func(string) (secretcap.Manager, error) {
		return newPrincipalTestManager(), nil
	}, func(context.Context, string) (resourcecap.Provider, func(), error) {
		return provider, func() {}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	worked, err := worker.RunOne(ctx)
	if err != nil || !worked {
		t.Fatalf("bounded recovery worked=%v err=%v", worked, err)
	}
	remaining, err := database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_resource_principal_task
WHERE status_code = 'lookup_leased'`)
	if err != nil {
		t.Fatal(err)
	}
	if remaining.Int() != 40-maximumExpiredRecoveryBatch || provider.lookupCalls.Load() != 1 {
		t.Fatalf("remaining expired=%d lookup calls=%d", remaining.Int(), provider.lookupCalls.Load())
	}
}

func TestPrincipalProvisionPostgreSQLManualReviewRevokesPasswordOnly(t *testing.T) {
	ctx, database := principalTestDatabase(t)
	seedPrincipalRoute(t, ctx, database)
	manager := newPrincipalTestManager()
	serviceValue, err := New(database, func(string) (secretcap.Manager, error) {
		return manager, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	service := serviceValue.(*serviceImpl)
	ready, err := service.EnsureForPool(ctx, "customer-manual-review", "pool-principal-1")
	if err != nil || ready {
		t.Fatalf("ensure principal ready=%v err=%v", ready, err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_principal_task
SET status_code = 'verify_wait', result_unknown = TRUE,
    observation_attempt_count = ?, available_at = CURRENT_TIMESTAMP
WHERE customer_no = 'customer-manual-review'`, maximumVerificationAttempts-1); err != nil {
		t.Fatal(err)
	}
	provider := &principalAlwaysAbsentProvider{}
	worker, err := NewWorker(database, func(string) (secretcap.Manager, error) {
		return manager, nil
	}, func(context.Context, string) (resourcecap.Provider, func(), error) {
		return provider, func() {}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for step := 0; step < 2; step++ {
		worked, runErr := worker.RunOne(ctx)
		if runErr != nil || !worked {
			t.Fatalf("manual review step %d worked=%v err=%v", step, worked, runErr)
		}
	}
	var state struct {
		StatusCode, FailureCode string
		FinishedAt              *time.Time `orm:"finished_at"`
	}
	if err = database.GetScan(ctx, &state, `
SELECT status_code, failure_code, finished_at
FROM plugin_idc_resource_principal_task
WHERE customer_no = 'customer-manual-review'`); err != nil {
		t.Fatal(err)
	}
	if state.StatusCode != "manual_review" || state.FailureCode != "PRINCIPAL_RESULT_UNRESOLVED" ||
		state.FinishedAt == nil || manager.revokeCalls.Load() != 1 || manager.activeCount() != 1 {
		t.Fatalf("manual review state=%+v revokes=%d active=%d", state, manager.revokeCalls.Load(), manager.activeCount())
	}
}

type principalTestProvider struct {
	lookupCalls atomic.Int64
	createCalls atomic.Int64
}

func (p *principalTestProvider) LookupPrincipal(
	context.Context,
	resourcecap.PrincipalLookupRequest,
) (*resourcecap.PrincipalLookupResult, error) {
	call := p.lookupCalls.Add(1)
	now := time.Now().UTC()
	result := &resourcecap.PrincipalLookupResult{
		Exists: call > 1, ObservedAt: now,
		Evidence: []resourcecap.Evidence{{
			Kind: "principal_lookup", Reference: "prt_test",
			Summary: "exact principal lookup completed", ObservedAt: now,
		}},
	}
	if result.Exists {
		result.ExternalPrincipalRef = "42"
	}
	return result, nil
}

func (p *principalTestProvider) CreatePrincipal(
	context.Context,
	resourcecap.PrincipalCreateRequest,
) (*resourcecap.PrincipalCreateResult, error) {
	p.createCalls.Add(1)
	callError, _ := resourcecap.NewProviderCallError(resourcecap.ProviderCallErrorCodeTimeout, false, true)
	return nil, callError
}

type principalAlwaysAbsentProvider struct{ principalTestProvider }

func (p *principalAlwaysAbsentProvider) LookupPrincipal(
	context.Context,
	resourcecap.PrincipalLookupRequest,
) (*resourcecap.PrincipalLookupResult, error) {
	p.lookupCalls.Add(1)
	now := time.Now().UTC()
	return &resourcecap.PrincipalLookupResult{
		ObservedAt: now,
		Evidence: []resourcecap.Evidence{{
			Kind: "principal_lookup", Reference: "prt_expired",
			Summary: "exact principal lookup completed", ObservedAt: now,
		}},
	}, nil
}

type principalKnownRejectionProvider struct {
	principalTestProvider
	mu    sync.Mutex
	order []string
}

func (p *principalKnownRejectionProvider) LookupPrincipal(
	context.Context,
	resourcecap.PrincipalLookupRequest,
) (*resourcecap.PrincipalLookupResult, error) {
	p.recordCall("lookup")
	now := time.Now().UTC()
	return &resourcecap.PrincipalLookupResult{
		ObservedAt: now,
		Evidence: []resourcecap.Evidence{{
			Kind: "principal_lookup", Reference: "prt_known_rejection",
			Summary: "exact principal lookup completed", ObservedAt: now,
		}},
	}, nil
}

func (p *principalKnownRejectionProvider) CreatePrincipal(
	context.Context,
	resourcecap.PrincipalCreateRequest,
) (*resourcecap.PrincipalCreateResult, error) {
	p.recordCall("create")
	if p.createCalls.Add(1) == 1 {
		callError, _ := resourcecap.NewProviderCallError(
			resourcecap.ProviderCallErrorCodeInvalidResponse, false, false,
		)
		return nil, callError
	}
	now := time.Now().UTC()
	return &resourcecap.PrincipalCreateResult{
		ExternalPrincipalRef: "7001",
		ObservedAt:           now,
		Evidence: []resourcecap.Evidence{{
			Kind: "principal_acceptance", Reference: "prt_known_rejection",
			Summary: "principal creation returned an identifier", ObservedAt: now,
		}},
	}, nil
}

func (p *principalKnownRejectionProvider) recordCall(call string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.order = append(p.order, call)
}

func (p *principalKnownRejectionProvider) callOrder() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return strings.Join(p.order, ",")
}

func (*principalTestProvider) DiscoverCapabilities(context.Context) (*resourcecap.CapabilitySnapshot, error) {
	return nil, errors.New("unused")
}
func (*principalTestProvider) ListProviderCapacity(context.Context, resourcecap.ProviderCapacityRequest) (*resourcecap.ProviderCapacityPage, error) {
	return nil, errors.New("unused")
}
func (*principalTestProvider) Execute(context.Context, resourcecap.OperationRequest) (*resourcecap.OperationResult, error) {
	return nil, errors.New("unused")
}
func (*principalTestProvider) QueryTask(context.Context, resourcecap.TaskQuery) (*resourcecap.OperationResult, error) {
	return nil, errors.New("unused")
}
func (*principalTestProvider) QueryResource(context.Context, resourcecap.ResourceQuery) (*resourcecap.ResourceSnapshot, error) {
	return nil, errors.New("unused")
}

type principalStoredSecret struct {
	handle     secretcap.Handle
	configured bool
	updatedAt  time.Time
}

type principalTestManager struct {
	mu          sync.Mutex
	byKey       map[string]*principalStoredSecret
	createCalls atomic.Int64
	revokeCalls atomic.Int64
}

func newPrincipalTestManager() *principalTestManager {
	return &principalTestManager{byKey: make(map[string]*principalStoredSecret)}
}

func (m *principalTestManager) Create(
	_ context.Context,
	key string,
	produce secretcap.PlaintextProducer,
) (secretcap.CreateResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if stored := m.byKey[key]; stored != nil {
		return secretcap.CreateResult{
			Handle: stored.handle, Metadata: secretcap.Metadata{Configured: stored.configured, UpdatedAt: stored.updatedAt}, Reused: true,
		}, nil
	}
	var plaintext []byte
	if err := produce(func(value []byte) error {
		plaintext = append([]byte(nil), value...)
		return nil
	}); err != nil {
		return secretcap.CreateResult{}, err
	}
	defer clear(plaintext)
	if strings.Contains(string(plaintext), "customer-principal-1") || len(plaintext) < 32 {
		return secretcap.CreateResult{}, errors.New("principal plaintext was not opaque and bounded")
	}
	handle, err := secretcap.NewHandle("sec_principal_test_" + hex.EncodeToString([]byte{byte(len(m.byKey) + 1)}))
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	updatedAt := time.Now().UTC()
	stored := &principalStoredSecret{handle: handle, configured: true, updatedAt: updatedAt}
	m.byKey[key] = stored
	m.createCalls.Add(1)
	return secretcap.CreateResult{Handle: handle, Metadata: secretcap.Metadata{Configured: true, UpdatedAt: updatedAt}}, nil
}

func (m *principalTestManager) RecoverCreate(
	_ context.Context,
	key string,
) (secretcap.CreateResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored := m.byKey[key]
	if stored == nil {
		return secretcap.CreateResult{}, errors.New("secret create result missing")
	}
	return secretcap.CreateResult{
		Handle: stored.handle,
		Metadata: secretcap.Metadata{
			Configured: stored.configured,
			UpdatedAt:  stored.updatedAt,
		},
		Reused: true,
	}, nil
}

func (m *principalTestManager) CreateGenerated(
	_ context.Context,
	key string,
	policy string,
) (secretcap.CreateResult, error) {
	if policy != secretcap.GeneratedSecretPolicyURLSafe256 {
		return secretcap.CreateResult{}, errors.New("unexpected generated secret policy")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if stored := m.byKey[key]; stored != nil {
		return secretcap.CreateResult{
			Handle: stored.handle,
			Metadata: secretcap.Metadata{
				Configured: stored.configured, UpdatedAt: stored.updatedAt,
			},
			Reused: true,
		}, nil
	}
	handle, err := secretcap.NewHandle("sec_principal_test_" + hex.EncodeToString([]byte{byte(len(m.byKey) + 1)}))
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	updatedAt := time.Now().UTC()
	stored := &principalStoredSecret{handle: handle, configured: true, updatedAt: updatedAt}
	m.byKey[key] = stored
	m.createCalls.Add(1)
	return secretcap.CreateResult{
		Handle:   handle,
		Metadata: secretcap.Metadata{Configured: true, UpdatedAt: updatedAt},
	}, nil
}

func (*principalTestManager) Replace(context.Context, secretcap.Handle, time.Time, string, secretcap.PlaintextProducer) (secretcap.MutationResult, error) {
	return secretcap.MutationResult{}, errors.New("unused")
}
func (m *principalTestManager) Metadata(_ context.Context, handle secretcap.Handle) (secretcap.Metadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, stored := range m.byKey {
		if stored.handle.Equal(handle) {
			return secretcap.Metadata{Configured: stored.configured, UpdatedAt: stored.updatedAt}, nil
		}
	}
	return secretcap.Metadata{}, errors.New("secret missing")
}
func (m *principalTestManager) Revoke(_ context.Context, handle secretcap.Handle, expected time.Time) (secretcap.Metadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, stored := range m.byKey {
		if stored.handle.Equal(handle) && stored.updatedAt.Equal(expected) {
			stored.configured = false
			stored.updatedAt = time.Now().UTC()
			m.revokeCalls.Add(1)
			return secretcap.Metadata{UpdatedAt: stored.updatedAt}, nil
		}
	}
	return secretcap.Metadata{}, errors.New("secret revoke conflict")
}
func (*principalTestManager) Rotate(context.Context, int) (secretcap.RotationResult, error) {
	return secretcap.RotationResult{}, errors.New("unused")
}
func (m *principalTestManager) activeCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, stored := range m.byKey {
		if stored.configured {
			count++
		}
	}
	return count
}

func principalTestDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(principalIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run principal PostgreSQL tests", principalIntegrationDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_resource_principal_test_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "linapro_resource_principal_test_") {
			_, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
		}
		_ = admin.Close(ctx)
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve principal test path")
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "[0-9][0-9][0-9]-*.sql"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("resolve principal migrations: %v", err)
	}
	for _, path := range paths {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("apply %s: %v", filepath.Base(path), err)
		}
	}
	return ctx, database
}

func seedPrincipalRoute(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_channel
    (channel_no, provider_id, provider_plugin_id, display_name, endpoint, status_code)
VALUES ('channel-principal-1', 'zjmfcloud-v1', 'idc-zjmfcloud',
        'Principal Test', 'https://provider.invalid', 'enabled');
INSERT INTO plugin_idc_resource_pool
    (pool_no, channel_no, display_name, resource_kind, status_code)
VALUES ('pool-principal-1', 'channel-principal-1', 'Principal Pool', 'virtual_server', 'enabled')`)
	if err != nil {
		t.Fatal(err)
	}
}
