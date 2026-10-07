// This file verifies the resource foundation migration against isolated PostgreSQL schemas.

package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/poolsourcecap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/accessorydirectory"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
	"lina-plugin-idc-resource/backend/internal/service/poolcandidate"
	"lina-plugin-idc-resource/backend/internal/service/providerauthsession"
	"lina-plugin-idc-resource/backend/internal/service/providercapacity"
	"lina-plugin-idc-resource/backend/internal/service/providerhealth"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

const resourceIntegrationDSNEnv = "LINAPRO_IDC_RESOURCE_TEST_DSN"

const resourceCodegenDSNEnv = "LINAPRO_IDC_RESOURCE_CODEGEN_DSN"

// resourceTriggerContract captures one installed current trigger and its
// deterministic legacy-name counterpart for upgrade fixtures.
type resourceTriggerContract struct {
	currentName string
	legacyName  string
	tableName   string
	definition  string
}

type accessoryDirectoryProvider struct {
	providerCapacityPageProvider
	calls int
}

func (p *accessoryDirectoryProvider) QueryAccessories(
	_ context.Context,
	query resourcecap.AccessoryQuery,
) (*resourcecap.AccessorySnapshot, error) {
	p.calls++
	now := time.Now().UTC()
	return &resourcecap.AccessorySnapshot{ObservedAt: now, ExpiresAt: now.Add(5 * time.Minute),
		Items: []resourcecap.AccessoryItem{{Kind: resourcecap.AccessoryKindIP,
			ExternalRef: "upstream-ip-2", DisplayName: "203.0.113.10",
			State: resourcecap.AccessoryStateAttached, Data: json.RawMessage(`{"address":"203.0.113.10","version":4}`)}},
	}, query.Validate()
}

type authenticationSecretFixture struct {
	mu      sync.Mutex
	next    int
	values  map[string][]byte
	byKey   map[string]secretcap.CreateResult
	updated map[string]time.Time
}

func newAuthenticationSecretFixture() *authenticationSecretFixture {
	return &authenticationSecretFixture{
		values: make(map[string][]byte), byKey: make(map[string]secretcap.CreateResult),
		updated: make(map[string]time.Time),
	}
}

func (f *authenticationSecretFixture) Access(
	_ context.Context,
	request secretcap.AccessRequest,
	consume secretcap.Consumer,
) error {
	if err := secretcap.ValidateAccess(request, consume); err != nil {
		return err
	}
	f.mu.Lock()
	value, exists := f.values[request.Handle.OpaqueRef()]
	copyValue := append([]byte(nil), value...)
	f.mu.Unlock()
	if !exists {
		return errors.New("fixture secret not found")
	}
	defer clear(copyValue)
	return consume(copyValue)
}

func (f *authenticationSecretFixture) Create(
	_ context.Context,
	idempotencyKey string,
	produce secretcap.PlaintextProducer,
) (secretcap.CreateResult, error) {
	f.mu.Lock()
	if existing, exists := f.byKey[idempotencyKey]; exists {
		f.mu.Unlock()
		existing.Reused = true
		return existing, nil
	}
	f.mu.Unlock()
	var plaintext []byte
	if err := produce(func(value []byte) error {
		plaintext = append([]byte(nil), value...)
		return nil
	}); err != nil {
		return secretcap.CreateResult{}, err
	}
	defer clear(plaintext)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	handle, err := secretcap.NewHandle("sec_auth_fixture_" + strconv.Itoa(f.next))
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	now := time.Now().UTC()
	result := secretcap.CreateResult{Handle: handle, Metadata: secretcap.Metadata{
		Configured: true, UpdatedAt: now,
	}}
	f.values[handle.OpaqueRef()] = append([]byte(nil), plaintext...)
	f.updated[handle.OpaqueRef()] = now
	f.byKey[idempotencyKey] = result
	return result, nil
}

func (f *authenticationSecretFixture) RecoverCreate(
	_ context.Context,
	idempotencyKey string,
) (secretcap.CreateResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result, exists := f.byKey[idempotencyKey]
	if !exists {
		return secretcap.CreateResult{}, errors.New("fixture create not found")
	}
	result.Reused = true
	return result, nil
}

func (f *authenticationSecretFixture) Replace(
	_ context.Context,
	handle secretcap.Handle,
	expectedUpdatedAt time.Time,
	_ string,
	produce secretcap.PlaintextProducer,
) (secretcap.MutationResult, error) {
	var plaintext []byte
	if err := produce(func(value []byte) error {
		plaintext = append([]byte(nil), value...)
		return nil
	}); err != nil {
		return secretcap.MutationResult{}, err
	}
	defer clear(plaintext)
	f.mu.Lock()
	defer f.mu.Unlock()
	updated, exists := f.updated[handle.OpaqueRef()]
	if !exists || !updated.Equal(expectedUpdatedAt) {
		return secretcap.MutationResult{}, errors.New("fixture replace fence mismatch")
	}
	now := updated.Add(time.Microsecond)
	f.values[handle.OpaqueRef()] = append([]byte(nil), plaintext...)
	f.updated[handle.OpaqueRef()] = now
	return secretcap.MutationResult{Metadata: secretcap.Metadata{
		Configured: true, UpdatedAt: now,
	}}, nil
}

func (f *authenticationSecretFixture) Metadata(
	_ context.Context,
	handle secretcap.Handle,
) (secretcap.Metadata, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	updated, exists := f.updated[handle.OpaqueRef()]
	return secretcap.Metadata{Configured: exists, UpdatedAt: updated}, nil
}

func (f *authenticationSecretFixture) Revoke(
	_ context.Context,
	handle secretcap.Handle,
	_ time.Time,
) (secretcap.Metadata, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.values, handle.OpaqueRef())
	delete(f.updated, handle.OpaqueRef())
	return secretcap.Metadata{}, nil
}

func (f *authenticationSecretFixture) Rotate(
	context.Context,
	int,
) (secretcap.RotationResult, error) {
	return secretcap.RotationResult{}, nil
}

func TestResourceFoundationMigrationPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(resourceIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL resource migration tests", resourceIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create resource migration admin database failed: %v", err)
	}
	schema := newResourceIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteResourcePostgreSQLIdentifier(schema)); err != nil {
		_ = adminDB.Close(ctx)
		t.Fatalf("create isolated resource schema failed: %v", err)
	}
	defer func() {
		if !strings.HasPrefix(schema, "linapro_resource_test_") {
			t.Errorf("refusing to drop unexpected resource schema %q", schema)
		} else if _, dropErr := adminDB.Exec(
			ctx,
			"DROP SCHEMA "+quoteResourcePostgreSQLIdentifier(schema)+" CASCADE",
		); dropErr != nil {
			t.Errorf("drop isolated resource schema failed: %v", dropErr)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close resource migration admin database failed: %v", closeErr)
		}
	}()

	scopedDB, err := gdb.New(gdb.ConfigNode{Link: appendResourceDSNParameter(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatalf("create scoped resource database failed: %v", err)
	}
	defer func() {
		if closeErr := scopedDB.Close(ctx); closeErr != nil {
			t.Errorf("close scoped resource database failed: %v", closeErr)
		}
	}()
	executeResourceMigrations(t, ctx, scopedDB)
	assertCatalogPublicationRetirementMigration(t, ctx, scopedDB)
	originalConfig := gdb.GetAllConfig()
	if err = gdb.SetConfig(gdb.Config{
		gdb.DefaultGroupName: gdb.ConfigGroup{{
			Link: appendResourceDSNParameter(dsn, "search_path="+schema),
		}},
	}); err != nil {
		t.Fatalf("configure scoped resource DAO database failed: %v", err)
	}
	t.Cleanup(func() {
		if restoreErr := gdb.SetConfig(originalConfig); restoreErr != nil {
			t.Errorf("restore resource DAO database config failed: %v", restoreErr)
		}
	})

	assertResourceSchemaShape(t, ctx, scopedDB, schema)
	assertResourceSchemaInvariants(t, ctx, scopedDB)
	assertProviderCapacityRenamePreservesDevelopmentData(t, ctx, scopedDB)
	assertProviderAuthenticationSessions(t, ctx, scopedDB)
	assertAccessoryDirectory(t, ctx, scopedDB)
	assertAccessoryDirectoryRefresh(t, ctx, scopedDB)
	assertResourcePoolCandidates(t, ctx, scopedDB)
	assertProviderCapacityObservationPublication(t, ctx, scopedDB)
	assertProviderCapacityOperations(t, ctx, scopedDB)
	assertEnabledChannelReader(t, ctx, scopedDB)
	assertResourceHealthProtection(t, ctx, scopedDB)
	assertResourceHealthManagement(t, ctx, scopedDB)
	assertResourceTaskManagement(t, ctx, scopedDB)
}

// TestResourceOfficialTriggerDebrandingPostgreSQL proves that the patch
// migration accepts only exact legacy resource guards and preserves all rows.
func TestResourceOfficialTriggerDebrandingPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(resourceIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL resource migration tests", resourceIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create resource trigger migration admin database failed: %v", err)
	}
	defer adminDB.Close(ctx)
	schema := newResourceIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteResourcePostgreSQLIdentifier(schema)); err != nil {
		t.Fatalf("create resource trigger migration schema failed: %v", err)
	}
	t.Cleanup(func() {
		if !strings.HasPrefix(schema, "linapro_resource_test_") {
			t.Errorf("refusing to drop unexpected resource trigger schema %q", schema)
			return
		}
		if _, dropErr := adminDB.Exec(ctx, "DROP SCHEMA "+quoteResourcePostgreSQLIdentifier(schema)+" CASCADE"); dropErr != nil {
			t.Errorf("drop resource trigger migration schema failed: %v", dropErr)
		}
	})
	database, err := gdb.New(gdb.ConfigNode{Link: appendResourceDSNParameter(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatalf("create resource trigger migration database failed: %v", err)
	}
	defer database.Close(ctx)
	executeResourceMigrations(t, ctx, database)
	contracts := resourceCurrentTriggerContracts(t, ctx, database)
	if len(contracts) != 4 {
		t.Fatalf("current resource branded-trigger contract count=%d want=4", len(contracts))
	}
	rowCountBefore := resourceBusinessRowCount(t, ctx, database)

	for _, contract := range contracts {
		if _, err = database.Exec(ctx, fmt.Sprintf(
			"ALTER TRIGGER %s ON %s RENAME TO %s",
			quoteResourcePostgreSQLIdentifier(contract.currentName),
			quoteResourcePostgreSQLIdentifier(contract.tableName),
			quoteResourcePostgreSQLIdentifier(contract.legacyName),
		)); err != nil {
			t.Fatalf("prepare old-only resource trigger %q failed: %v", contract.legacyName, err)
		}
	}
	executeResourceTriggerDebranding(t, ctx, database)
	assertResourceTriggerBrandState(t, ctx, database, 4, 0)

	for _, contract := range resourceCurrentTriggerContracts(t, ctx, database) {
		legacyDefinition := strings.Replace(
			contract.definition,
			"TRIGGER "+contract.currentName,
			"TRIGGER "+contract.legacyName,
			1,
		)
		if _, err = database.Exec(ctx, legacyDefinition); err != nil {
			t.Fatalf("prepare new-and-old resource trigger %q failed: %v", contract.legacyName, err)
		}
	}
	executeResourceTriggerDebranding(t, ctx, database)
	assertResourceTriggerBrandState(t, ctx, database, 4, 0)

	first := resourceCurrentTriggerContracts(t, ctx, database)[0]
	if _, err = database.Exec(ctx, fmt.Sprintf(
		"CREATE TRIGGER %s AFTER UPDATE ON %s FOR EACH ROW EXECUTE FUNCTION plugin_idc_resource_reject_accessory_item_mutation()",
		quoteResourcePostgreSQLIdentifier(first.legacyName),
		quoteResourcePostgreSQLIdentifier(first.tableName),
	)); err != nil {
		t.Fatalf("prepare ambiguous resource trigger failed: %v", err)
	}
	cleanupSQL := resourceTriggerDebrandingSQL(t)
	if _, err = database.Exec(ctx, cleanupSQL); err == nil || !strings.Contains(err.Error(), "ambiguous legacy idc-resource trigger") {
		t.Fatalf("ambiguous resource trigger cleanup error=%v", err)
	}
	if _, err = database.Exec(ctx, fmt.Sprintf(
		"DROP TRIGGER %s ON %s",
		quoteResourcePostgreSQLIdentifier(first.legacyName),
		quoteResourcePostgreSQLIdentifier(first.tableName),
	)); err != nil {
		t.Fatalf("remove ambiguous resource trigger fixture failed: %v", err)
	}
	executeResourceTriggerDebranding(t, ctx, database)
	executeResourceTriggerDebranding(t, ctx, database)
	assertResourceTriggerBrandState(t, ctx, database, 4, 0)
	if rowCountAfter := resourceBusinessRowCount(t, ctx, database); rowCountAfter != rowCountBefore {
		t.Fatalf("resource business row count changed during trigger cleanup: before=%d after=%d", rowCountBefore, rowCountAfter)
	}
}

// resourceCurrentTriggerContracts reads the authoritative current guards.
func resourceCurrentTriggerContracts(t *testing.T, ctx context.Context, database gdb.DB) []resourceTriggerContract {
	t.Helper()
	rows, err := database.GetAll(ctx, `
SELECT trigger_row.tgname,
       trigger_table.relname AS table_name,
       pg_get_triggerdef(trigger_row.oid, true) AS definition
FROM pg_trigger AS trigger_row
JOIN pg_class AS trigger_table ON trigger_table.oid = trigger_row.tgrelid
JOIN pg_namespace AS trigger_schema ON trigger_schema.oid = trigger_table.relnamespace
WHERE trigger_schema.nspname = current_schema()
  AND NOT trigger_row.tgisinternal
  AND trigger_row.tgname LIKE 'trg_plugin_idc_resource_%'
ORDER BY trigger_row.tgname`)
	if err != nil {
		t.Fatalf("query current resource trigger contracts failed: %v", err)
	}
	contracts := make([]resourceTriggerContract, 0, len(rows))
	for _, row := range rows {
		currentName := row["tgname"].String()
		contracts = append(contracts, resourceTriggerContract{
			currentName: currentName,
			legacyName:  strings.Replace(currentName, "trg_plugin_", "trg_plugin_linapro_", 1),
			tableName:   row["table_name"].String(),
			definition:  row["definition"].String(),
		})
	}
	return contracts
}

// resourceTriggerDebrandingSQL reads the patch migration under test.
func resourceTriggerDebrandingSQL(t *testing.T) string {
	t.Helper()
	for _, path := range resourceMigrationPaths(t) {
		if filepath.Base(path) != "019-official-trigger-debranding.sql" {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read resource trigger debranding migration failed: %v", err)
		}
		return string(content)
	}
	t.Fatal("resource trigger debranding migration was not found")
	return ""
}

// executeResourceTriggerDebranding applies the patch migration once.
func executeResourceTriggerDebranding(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, resourceTriggerDebrandingSQL(t)); err != nil {
		t.Fatalf("execute resource trigger debranding migration failed: %v", err)
	}
}

// assertResourceTriggerBrandState verifies current and branded trigger totals.
func assertResourceTriggerBrandState(t *testing.T, ctx context.Context, database gdb.DB, currentWant, legacyWant int) {
	t.Helper()
	row, err := database.GetOne(ctx, `
SELECT count(*) FILTER (WHERE trigger_row.tgname LIKE 'trg_plugin_idc_resource_%') AS current_count,
       count(*) FILTER (WHERE trigger_row.tgname LIKE '%linapro%') AS legacy_count
FROM pg_trigger AS trigger_row
JOIN pg_class AS trigger_table ON trigger_table.oid = trigger_row.tgrelid
JOIN pg_namespace AS trigger_schema ON trigger_schema.oid = trigger_table.relnamespace
WHERE trigger_schema.nspname = current_schema()
  AND NOT trigger_row.tgisinternal`)
	if err != nil || row["current_count"].Int() != currentWant || row["legacy_count"].Int() != legacyWant {
		t.Fatalf("resource trigger state current=%d legacy=%d want current=%d legacy=%d err=%v", row["current_count"].Int(), row["legacy_count"].Int(), currentWant, legacyWant, err)
	}
}

// resourceBusinessRowCount totals plugin-owned rows before and after DDL.
func resourceBusinessRowCount(t *testing.T, ctx context.Context, database gdb.DB) int64 {
	t.Helper()
	tables, err := database.GetAll(ctx, `
SELECT table_name
FROM information_schema.tables
WHERE table_schema = current_schema()
  AND table_type = 'BASE TABLE'
  AND table_name LIKE 'plugin_idc_resource_%'
ORDER BY table_name`)
	if err != nil {
		t.Fatalf("list resource business tables failed: %v", err)
	}
	var total int64
	for _, table := range tables {
		row, countErr := database.GetOne(ctx, "SELECT count(*) AS total FROM "+quoteResourcePostgreSQLIdentifier(table["table_name"].String()))
		if countErr != nil {
			t.Fatalf("count resource business table %q failed: %v", table["table_name"].String(), countErr)
		}
		total += row["total"].Int64()
	}
	return total
}

func assertProviderAuthenticationSessions(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	secrets := newAuthenticationSecretFixture()
	first, err := providerauthsession.New(database, "channel-test", secrets, secrets)
	if err != nil {
		t.Fatalf("construct first shared authentication session: %v", err)
	}
	second, err := providerauthsession.New(database, "channel-test", secrets, secrets)
	if err != nil {
		t.Fatalf("construct second shared authentication session: %v", err)
	}
	request := resourcecap.AuthenticationSessionRequest{ChannelRef: "channel-test", TTL: time.Minute}
	var loginCount atomic.Int32
	producer := func(consume resourcecap.SecretConsumer) error {
		attempt := loginCount.Add(1)
		token := []byte("shared-token-" + strconv.Itoa(int(attempt)))
		defer clear(token)
		return consume(token)
	}
	type observedToken struct {
		value      string
		generation int64
	}
	results := make(chan observedToken, 2)
	errorsFound := make(chan error, 2)
	var workers sync.WaitGroup
	for _, service := range []providerauthsession.Service{first, second} {
		workers.Add(1)
		go func(current providerauthsession.Service) {
			defer workers.Done()
			errorsFound <- current.Access(ctx, request, producer, func(token []byte, generation int64) error {
				results <- observedToken{value: string(token), generation: generation}
				return nil
			})
		}(service)
	}
	workers.Wait()
	close(errorsFound)
	for accessErr := range errorsFound {
		if accessErr != nil {
			t.Fatalf("concurrent shared authentication access failed: %v", accessErr)
		}
	}
	close(results)
	var generation int64
	for result := range results {
		if result.value != "shared-token-1" || result.generation <= 0 {
			t.Fatalf("unexpected shared authentication result: %#v", result)
		}
		generation = result.generation
	}
	if loginCount.Load() != 1 {
		t.Fatalf("concurrent Provider sessions performed %d logins, want 1", loginCount.Load())
	}
	if err = first.Invalidate(ctx, resourcecap.AuthenticationSessionInvalidation{
		ChannelRef: "channel-test", Generation: generation,
	}); err != nil {
		t.Fatalf("invalidate shared authentication generation: %v", err)
	}
	var refreshed observedToken
	if err = second.Access(ctx, request, producer, func(token []byte, nextGeneration int64) error {
		refreshed = observedToken{value: string(token), generation: nextGeneration}
		return nil
	}); err != nil {
		t.Fatalf("refresh invalidated authentication session: %v", err)
	}
	if loginCount.Load() != 2 || refreshed.value != "shared-token-2" || refreshed.generation <= generation {
		t.Fatalf("shared authentication refresh did not advance: count=%d result=%#v",
			loginCount.Load(), refreshed)
	}
	if err = first.Invalidate(ctx, resourcecap.AuthenticationSessionInvalidation{
		ChannelRef: "channel-test", Generation: generation,
	}); err != nil {
		t.Fatalf("stale authentication invalidation failed: %v", err)
	}
	if err = first.Access(ctx, request, producer, func(token []byte, currentGeneration int64) error {
		if string(token) != "shared-token-2" || currentGeneration != refreshed.generation {
			return errors.New("stale invalidation discarded the newer token")
		}
		return nil
	}); err != nil {
		t.Fatalf("new authentication generation did not survive stale invalidation: %v", err)
	}
	if loginCount.Load() != 2 {
		t.Fatalf("stale invalidation caused another login: count=%d", loginCount.Load())
	}
	row, err := database.GetOne(ctx, `SELECT status_code,generation,secret_ref,
       lease_token_digest,lease_expires_at
FROM plugin_idc_resource_provider_auth_session WHERE channel_no='channel-test'`)
	if err != nil || row["status_code"].String() != "ready" ||
		row["generation"].Int64() != refreshed.generation ||
		strings.Contains(row["secret_ref"].String(), "shared-token") ||
		!row["lease_token_digest"].IsNil() || !row["lease_expires_at"].IsNil() {
		t.Fatalf("shared authentication row is unsafe or inconsistent: row=%v err=%v", row, err)
	}
}

func assertAccessoryDirectory(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	service, err := accessorydirectory.New(database)
	if err != nil {
		t.Fatal(err)
	}
	existing, err := service.Read(ctx, "resource-1")
	if err != nil || existing == nil || len(existing.Items) != 1 || existing.Items[0].ObjectNo != "accessory-object-1" {
		t.Fatalf("read seeded accessory directory=%+v err=%v", existing, err)
	}
	resolved, err := service.Resolve(ctx, "resource-1", "accessory-object-1", resourcecap.AccessoryKindDisk)
	if err != nil || resolved == nil || resolved.ExternalRef != "upstream-disk-1" {
		t.Fatalf("resolve owned accessory object=%+v err=%v", resolved, err)
	}
	if _, err = service.Resolve(ctx, "resource-1", "accessory-object-1", resourcecap.AccessoryKindSnapshot); !errors.Is(err, accessorydirectory.ErrObjectUnavailable) {
		t.Fatalf("cross-kind accessory object returned %v", err)
	}
	now := time.Now().UTC()
	snapshot := resourcecap.AccessorySnapshot{ObservedAt: now, ExpiresAt: now.Add(15 * time.Minute), Items: []resourcecap.AccessoryItem{
		{Kind: resourcecap.AccessoryKindIP, ExternalRef: "upstream-ip-1", DisplayName: "203.0.113.10", State: resourcecap.AccessoryStateActive, Data: json.RawMessage(`{"address":"203.0.113.10","primary":true}`)},
		{Kind: resourcecap.AccessoryKindDisk, ExternalRef: "upstream-disk-2", DisplayName: "Data disk", State: resourcecap.AccessoryStateAttached, Data: json.RawMessage(`{"sizeGiB":200,"system":false}`)},
	}}
	if err = snapshot.ValidateAt(time.Now().UTC()); err != nil {
		t.Fatalf("validate accessory publication fixture: %v", err)
	}
	published, err := service.Publish(ctx, accessorydirectory.PublishInput{
		MappingNo: "mapping-1", LocalResourceNo: "resource-1", ChannelNo: "channel-test",
		ExternalResourceRef: "1001",
		Snapshot:            snapshot,
	})
	if err != nil || published == nil || len(published.Items) != 2 || published.Items[0].Kind != resourcecap.AccessoryKindDisk || published.Items[1].Kind != resourcecap.AccessoryKindIP {
		t.Fatalf("publish accessory directory=%+v err=%v", published, err)
	}
	if _, err = service.Resolve(ctx, "resource-1", "accessory-object-1", resourcecap.AccessoryKindDisk); !errors.Is(err, accessorydirectory.ErrObjectUnavailable) {
		t.Fatalf("superseded accessory object returned %v", err)
	}
	firstDiskObjectNo := published.Items[0].ObjectNo
	republished, err := service.Publish(ctx, accessorydirectory.PublishInput{
		MappingNo: "mapping-1", LocalResourceNo: "resource-1", ChannelNo: "channel-test",
		ExternalResourceRef: "1001",
		Snapshot:            snapshot,
	})
	if err != nil || republished == nil || len(republished.Items) != 2 ||
		republished.Items[0].ObjectNo != firstDiskObjectNo {
		t.Fatalf("stable accessory object identity=%+v previous=%q err=%v", republished, firstDiskObjectNo, err)
	}
	statusCounts, err := database.GetAll(ctx, `SELECT status_code,count(*) AS count
FROM plugin_idc_resource_accessory_snapshot GROUP BY status_code ORDER BY status_code`)
	if err != nil || len(statusCounts) != 2 || statusCounts[0]["status_code"].String() != "complete" ||
		statusCounts[0]["count"].Int() != 1 || statusCounts[1]["status_code"].String() != "superseded" ||
		statusCounts[1]["count"].Int() != 2 {
		t.Fatalf("accessory publication statuses=%v err=%v", statusCounts, err)
	}
}

func assertAccessoryDirectoryRefresh(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	provider := &accessoryDirectoryProvider{}
	worker, err := accessorydirectory.NewWorker(database, &providerCapacityProviderConstructor{provider: provider})
	if err != nil {
		t.Fatal(err)
	}
	worked, err := worker.RunOne(ctx)
	if err != nil || !worked || provider.calls != 1 {
		t.Fatalf("refresh accessory directory worked=%v calls=%d err=%v", worked, provider.calls, err)
	}
	directory, err := accessorydirectory.New(database)
	if err != nil {
		t.Fatal(err)
	}
	current, err := directory.Read(ctx, "resource-1")
	if err != nil || current == nil || len(current.Items) != 1 ||
		current.Items[0].Name != "203.0.113.10" {
		t.Fatalf("refreshed accessory directory=%+v err=%v", current, err)
	}
	state, err := database.GetOne(ctx, `SELECT attempt_count,failure_code,lease_token_digest,lease_expires_at
FROM plugin_idc_resource_accessory_refresh_state WHERE mapping_no='mapping-1'`)
	if err != nil || state.IsEmpty() || state["attempt_count"].Int() != 0 ||
		state["failure_code"].String() != "" || !state["lease_token_digest"].IsNil() ||
		!state["lease_expires_at"].IsNil() {
		t.Fatalf("accessory refresh state=%v err=%v", state, err)
	}
}

func assertResourcePoolCandidates(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	service, err := poolcandidate.New(database)
	if err != nil {
		t.Fatalf("construct resource pool candidate service: %v", err)
	}
	candidates, err := service.ListEligible(ctx, poolsourcecap.ListRequest{
		ResourceKind: poolsourcecap.ResourceKindVirtualServer,
	})
	if err != nil || len(candidates) != 2 {
		t.Fatalf("list resource pool candidates=%+v err=%v", candidates, err)
	}
	var selected poolsourcecap.Candidate
	for _, candidate := range candidates {
		if candidate.PoolNo == "pool-test" {
			selected = candidate
			break
		}
	}
	if selected.PoolNo == "" || selected.ResourceKind != poolsourcecap.ResourceKindVirtualServer {
		t.Fatalf("pool-test candidate is missing: %+v", candidates)
	}
	verified, err := service.VerifyEligible(ctx, poolsourcecap.VerifyRequest{
		PoolNo: selected.PoolNo, ResourceKind: selected.ResourceKind, Revision: selected.Revision,
	})
	if err != nil || verified == nil || verified.Revision != selected.Revision {
		t.Fatalf("verify resource pool candidate=%+v err=%v", verified, err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_resource_pool
SET version = version + 1 WHERE pool_no = 'pool-test'`); err != nil {
		t.Fatalf("advance resource pool candidate version: %v", err)
	}
	if _, err = service.VerifyEligible(ctx, poolsourcecap.VerifyRequest{
		PoolNo: selected.PoolNo, ResourceKind: selected.ResourceKind, Revision: selected.Revision,
	}); !errors.Is(err, poolsourcecap.ErrCandidateUnavailable) {
		t.Fatalf("stale resource pool candidate revision returned %v", err)
	}
	bareMetal, err := service.ListEligible(ctx, poolsourcecap.ListRequest{
		ResourceKind: poolsourcecap.ResourceKindBareMetal,
	})
	if err != nil || len(bareMetal) != 0 {
		t.Fatalf("list incompatible bare-metal candidates=%+v err=%v", bareMetal, err)
	}
}

func assertResourceHealthProtection(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	health := providerhealth.New()
	var err error
	for index, outcome := range []providerhealth.Outcome{
		providerhealth.OutcomeSucceeded,
		providerhealth.OutcomeSucceeded,
		providerhealth.OutcomeFailed,
		providerhealth.OutcomeResultUnknown,
		providerhealth.OutcomeResultUnknown,
	} {
		if err = health.Record(ctx, providerhealth.Observation{
			ChannelNo: "channel-test", Operation: providerhealth.OperationWrite,
			Outcome: outcome, SummaryCode: "integration_" + strconv.Itoa(index),
			Latency: time.Duration(index+1) * time.Millisecond,
		}); err != nil {
			t.Fatalf("record resource Provider health observation %d failed: %v", index, err)
		}
	}
	shared := providerhealth.New()
	gate, err := shared.CheckWrite(ctx, "channel-test")
	if err != nil || gate.AllowsWrite() || gate.ProtectedUntil.IsZero() {
		t.Fatalf("shared write protection gate=%+v err=%v", gate, err)
	}
	store, err := resourcetask.NewStore(database)
	if err != nil {
		t.Fatalf("construct protected resource task store failed: %v", err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_channel
SET status_code = 'enabled', version = version + 1
WHERE channel_no = 'channel-test'`); err != nil {
		t.Fatalf("re-enable isolated health channel fixture failed: %v", err)
	}
	protectedTask, err := store.Create(ctx, resourcetask.CreateInput{
		RequestID: "health-protected-request", Capability: resourcecap.CapabilityStart,
		LocalResourceNo: "health-protected-resource", ChannelNo: "channel-test",
		Parameters: []byte(`{}`), Priority: 100, MaxAttempts: 4,
	})
	if err != nil {
		t.Fatalf("create protected resource task failed: %v", err)
	}
	claimed, err := store.Claim(ctx, resourcetask.ClaimInput{
		WorkerID: "health-worker", Limit: 1, LeaseDuration: time.Minute,
	})
	if err != nil || len(claimed) != 0 {
		t.Fatalf("protected resource task was claimable: tasks=%v err=%v", claimed, err)
	}
	attemptCount, err := database.GetValue(ctx, `
SELECT attempt_count FROM plugin_idc_resource_task WHERE task_no = ?`,
		protectedTask.TaskNo,
	)
	if err != nil || attemptCount.Int() != 0 {
		t.Fatalf("protected resource task consumed an attempt: count=%d err=%v", attemptCount.Int(), err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_channel_health_state
SET circuit_open = FALSE, write_protected_until = NULL
WHERE channel_no = 'channel-test'`); err != nil {
		t.Fatalf("temporarily release isolated health protection failed: %v", err)
	}
	claimed, err = store.Claim(ctx, resourcetask.ClaimInput{
		WorkerID: "health-worker", Limit: 1, LeaseDuration: time.Minute,
	})
	if err != nil || len(claimed) != 1 || claimed[0].TaskNo != protectedTask.TaskNo {
		t.Fatalf("isolated race fixture was not claimed: tasks=%v err=%v", claimed, err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_channel_health_state
SET circuit_open = TRUE, write_protected_until = CURRENT_TIMESTAMP + INTERVAL '5 minutes'
WHERE channel_no = 'channel-test'`); err != nil {
		t.Fatalf("restore isolated health protection after claim failed: %v", err)
	}
	if err = store.DeferWriteForProtection(ctx, resourcetask.LeaseRef{
		TaskNo: claimed[0].TaskNo, Token: claimed[0].LeaseToken,
		AttemptNo: claimed[0].AttemptNo,
	}, gate.ProtectedUntil); err != nil {
		t.Fatalf("defer newly protected resource task failed: %v", err)
	}
	deferred, err := database.GetOne(ctx, `
SELECT status_code, attempt_count, locked_by, lease_expires_at
FROM plugin_idc_resource_task WHERE task_no = ?`, protectedTask.TaskNo)
	if err != nil || deferred["status_code"].String() != string(resourcetask.StatusRetryWait) ||
		deferred["attempt_count"].Int() != 0 || deferred["locked_by"].String() != "" ||
		deferred["lease_expires_at"].String() != "" {
		t.Fatalf("protected race consumed an attempt or retained a lease: row=%v err=%v", deferred, err)
	}
	if _, err = database.Exec(ctx, `
DELETE FROM plugin_idc_resource_task WHERE task_no = ?`, protectedTask.TaskNo); err != nil {
		t.Fatalf("remove isolated protected resource task fixture failed: %v", err)
	}
	snapshots, err := database.GetValue(ctx, `
SELECT count(*)
FROM plugin_idc_resource_health_snapshot
WHERE channel_no = 'channel-test' AND operation_kind = 'write'`)
	if err != nil || snapshots.Int() != 5 {
		t.Fatalf("resource Provider health snapshot count=%d want=5 err=%v", snapshots.Int(), err)
	}
	if err = shared.Record(ctx, providerhealth.Observation{
		ChannelNo: "channel-test", Operation: providerhealth.OperationQuery,
		Outcome:               providerhealth.OutcomeFailed,
		SummaryCode:           string(resourcecap.ProviderCallErrorCodeAuthentication),
		AuthenticationFailure: true, Latency: time.Millisecond,
	}); err != nil {
		t.Fatalf("record authentication health observation failed: %v", err)
	}
	authenticationRows, err := database.GetValue(ctx, `
SELECT count(*)
FROM plugin_idc_resource_health_snapshot
WHERE channel_no = 'channel-test'
  AND operation_kind IN ('query', 'authentication')
  AND summary_code = 'authentication'`)
	if err != nil || authenticationRows.Int() != 2 {
		t.Fatalf("authentication health projection count=%d want=2 err=%v",
			authenticationRows.Int(), err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_channel_health_state
SET write_protected_until = CURRENT_TIMESTAMP - INTERVAL '1 second'
WHERE channel_no = 'channel-test'`); err != nil {
		t.Fatalf("expire isolated resource Provider protection failed: %v", err)
	}
	gate, err = health.CheckWrite(ctx, "channel-test")
	if err != nil || !gate.AllowsWrite() {
		t.Fatalf("expired resource Provider protection gate=%+v err=%v", gate, err)
	}
	state, err := database.GetOne(ctx, `
SELECT sample_count, failure_count, unknown_count, consecutive_failures,
       circuit_open, write_protected_until
FROM plugin_idc_resource_channel_health_state
WHERE channel_no = 'channel-test'`)
	if err != nil || state["sample_count"].Int() != 0 || state["failure_count"].Int() != 0 ||
		state["unknown_count"].Int() != 0 || state["consecutive_failures"].Int() != 0 ||
		state["circuit_open"].Bool() || state["write_protected_until"].String() != "" {
		t.Fatalf("expired resource Provider health state was not reset: row=%v err=%v", state, err)
	}
}

func assertProviderCapacityObservationPublication(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	service, err := providercapacity.New(database)
	if err != nil {
		t.Fatalf("construct providerCapacity snapshot service failed: %v", err)
	}
	observedAt := time.Now().UTC().Truncate(time.Microsecond)
	first, err := service.Begin(ctx, providercapacity.BeginInput{
		ChannelNo: "channel-test", Kind: resourcecap.ProviderCapacityKindImage, PageSize: 2,
	})
	if err != nil {
		t.Fatalf("begin first providerCapacity snapshot failed: %v", err)
	}
	first, err = service.Stage(ctx, providercapacity.StageInput{
		ObservationNo: first.ObservationNo,
		Request:       resourcecap.ProviderCapacityRequest{Kind: resourcecap.ProviderCapacityKindImage, Limit: 2},
		Page: resourcecap.ProviderCapacityPage{Complete: true, Items: []resourcecap.ProviderCapacityItem{{
			Kind: resourcecap.ProviderCapacityKindImage, ExternalRef: "image-old",
			Data: []byte(`{"name":"old"}`), ObservedAt: observedAt,
		}}},
	})
	if err != nil || first.Status != providercapacity.StatusComplete || first.ItemCount != 1 ||
		first.PageCount != 1 || len(first.Fingerprint) != 64 {
		t.Fatalf("publish first providerCapacity snapshot failed: snapshot=%+v err=%v", first, err)
	}
	second, err := service.Begin(ctx, providercapacity.BeginInput{
		ChannelNo: "channel-test", Kind: resourcecap.ProviderCapacityKindImage, PageSize: 2,
	})
	if err != nil {
		t.Fatalf("begin replacement providerCapacity snapshot failed: %v", err)
	}
	if _, err = service.Begin(ctx, providercapacity.BeginInput{
		ChannelNo: "channel-test", Kind: resourcecap.ProviderCapacityKindImage, PageSize: 2,
	}); err == nil {
		t.Fatal("providerCapacity owner accepted two collecting snapshots for one channel and kind")
	}
	if _, err = service.Stage(ctx, providercapacity.StageInput{
		ObservationNo: second.ObservationNo,
		Request: resourcecap.ProviderCapacityRequest{
			Kind: resourcecap.ProviderCapacityKindImage, Limit: 2, Filters: []byte(`{"region":"hidden"}`),
		},
		Page: resourcecap.ProviderCapacityPage{Complete: true},
	}); err == nil {
		t.Fatal("full providerCapacity publication accepted adapter-owned subset filters")
	}
	second, err = service.Stage(ctx, providercapacity.StageInput{
		ObservationNo: second.ObservationNo,
		Request:       resourcecap.ProviderCapacityRequest{Kind: resourcecap.ProviderCapacityKindImage, Limit: 2},
		Page: resourcecap.ProviderCapacityPage{
			NextCursor: "image-cursor-2",
			Items: []resourcecap.ProviderCapacityItem{{
				Kind: resourcecap.ProviderCapacityKindImage, ExternalRef: "image-new-1",
				Data: []byte(`{"name":"new-1"}`), ObservedAt: observedAt,
			}},
		},
	})
	if err != nil || second.Status != providercapacity.StatusCollecting ||
		second.NextCursor != "image-cursor-2" || second.PageCount != 1 {
		t.Fatalf("stage replacement providerCapacity page failed: snapshot=%+v err=%v", second, err)
	}
	status, err := database.GetValue(ctx, `
SELECT status_code
FROM plugin_idc_resource_provider_capacity_observation
WHERE observation_no = ?`, first.ObservationNo)
	if err != nil || status.String() != string(providercapacity.StatusComplete) {
		t.Fatalf("partial replacement hid the previous complete snapshot: status=%v err=%v", status, err)
	}
	if _, err = service.Stage(ctx, providercapacity.StageInput{
		ObservationNo: second.ObservationNo,
		Request: resourcecap.ProviderCapacityRequest{
			Kind: resourcecap.ProviderCapacityKindImage, Cursor: "wrong-cursor", Limit: 2,
		},
		Page: resourcecap.ProviderCapacityPage{Complete: true},
	}); err == nil {
		t.Fatal("providerCapacity snapshot accepted a page that skipped its persisted cursor")
	}
	second, err = service.Stage(ctx, providercapacity.StageInput{
		ObservationNo: second.ObservationNo,
		Request: resourcecap.ProviderCapacityRequest{
			Kind: resourcecap.ProviderCapacityKindImage, Cursor: "image-cursor-2", Limit: 2,
		},
		Page: resourcecap.ProviderCapacityPage{Complete: true, Items: []resourcecap.ProviderCapacityItem{{
			Kind: resourcecap.ProviderCapacityKindImage, ExternalRef: "image-new-2",
			Data: []byte(`{"name":"new-2"}`), ObservedAt: observedAt,
		}}},
	})
	if err != nil || second.Status != providercapacity.StatusComplete || second.ItemCount != 2 ||
		second.PageCount != 2 || second.NextCursor != "" {
		t.Fatalf("publish replacement providerCapacity snapshot failed: snapshot=%+v err=%v", second, err)
	}
	publicationRows, err := database.GetAll(ctx, `
SELECT observation_no, status_code, item_count, page_count, next_cursor, completed_at
FROM plugin_idc_resource_provider_capacity_observation
WHERE observation_no IN (?, ?)
ORDER BY observation_no`, first.ObservationNo, second.ObservationNo)
	if err != nil || len(publicationRows) != 2 {
		t.Fatalf("load providerCapacity publication rows failed: rows=%v err=%v", publicationRows, err)
	}
	publicationStatus := map[string]string{}
	for _, row := range publicationRows {
		publicationStatus[row["observation_no"].String()] = row["status_code"].String()
	}
	if publicationStatus[first.ObservationNo] != string(providercapacity.StatusSuperseded) ||
		publicationStatus[second.ObservationNo] != string(providercapacity.StatusComplete) {
		t.Fatalf("providerCapacity terminal publication was not atomic: statuses=%v", publicationStatus)
	}

	duplicate, err := service.Begin(ctx, providercapacity.BeginInput{
		ChannelNo: "channel-test", Kind: resourcecap.ProviderCapacityKindImage, PageSize: 2,
	})
	if err != nil {
		t.Fatalf("begin duplicate providerCapacity snapshot failed: %v", err)
	}
	duplicate, err = service.Stage(ctx, providercapacity.StageInput{
		ObservationNo: duplicate.ObservationNo,
		Request:       resourcecap.ProviderCapacityRequest{Kind: resourcecap.ProviderCapacityKindImage, Limit: 2},
		Page: resourcecap.ProviderCapacityPage{
			NextCursor: "duplicate-cursor",
			Items: []resourcecap.ProviderCapacityItem{{
				Kind: resourcecap.ProviderCapacityKindImage, ExternalRef: "image-duplicate",
				Data: []byte(`{"name":"duplicate"}`), ObservedAt: observedAt,
			}},
		},
	})
	if err != nil {
		t.Fatalf("stage duplicate providerCapacity first page failed: %v", err)
	}
	if _, err = service.Stage(ctx, providercapacity.StageInput{
		ObservationNo: duplicate.ObservationNo,
		Request: resourcecap.ProviderCapacityRequest{
			Kind: resourcecap.ProviderCapacityKindImage, Cursor: duplicate.NextCursor, Limit: 2,
		},
		Page: resourcecap.ProviderCapacityPage{Complete: true, Items: []resourcecap.ProviderCapacityItem{{
			Kind: resourcecap.ProviderCapacityKindImage, ExternalRef: "image-duplicate",
			Data: []byte(`{"name":"changed"}`), ObservedAt: observedAt,
		}}},
	}); err == nil {
		t.Fatal("providerCapacity snapshot accepted a duplicate reference across pages")
	}
	duplicateRow, err := database.GetOne(ctx, `
SELECT status_code, item_count, page_count, next_cursor
FROM plugin_idc_resource_provider_capacity_observation
WHERE observation_no = ?`, duplicate.ObservationNo)
	if err != nil || duplicateRow["status_code"].String() != string(providercapacity.StatusCollecting) ||
		duplicateRow["item_count"].Int() != 1 || duplicateRow["page_count"].Int() != 1 ||
		duplicateRow["next_cursor"].String() != "duplicate-cursor" {
		t.Fatalf("failed terminal page advanced durable providerCapacity state: row=%v err=%v", duplicateRow, err)
	}
	if err = service.Fail(ctx, duplicate.ObservationNo); err != nil {
		t.Fatalf("fail partial providerCapacity snapshot failed: %v", err)
	}
	latestComplete, err := database.GetValue(ctx, `
SELECT count(*)
FROM plugin_idc_resource_provider_capacity_observation
WHERE observation_no = ? AND status_code = 'complete'`, second.ObservationNo)
	if err != nil || latestComplete.Int() != 1 {
		t.Fatalf("failing a partial providerCapacity snapshot hid the latest complete snapshot: %v", err)
	}
}

func assertEnabledChannelReader(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	reader, err := channelconfig.NewReader(database)
	if err != nil {
		t.Fatalf("construct enabled channel reader failed: %v", err)
	}
	configuration, err := reader.ReadEnabled(ctx, "channel-test")
	if err != nil || configuration.PluginID != "plugin-test" ||
		configuration.ProviderID != "provider-test" || len(configuration.Config.SecretBindings) != 1 ||
		configuration.Config.SecretBindings[0].Name != "password" ||
		configuration.Config.SecretBindings[0].Handle.OpaqueRef() != "secret-ref-2" {
		t.Fatalf("enabled channel reader returned an invalid detached configuration: err=%v", err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_channel
SET status_code = 'disabled', version = version + 1
WHERE channel_no = 'channel-test'`); err != nil {
		t.Fatalf("disable resource channel failed: %v", err)
	}
	if _, err = reader.ReadEnabled(ctx, "channel-test"); err == nil {
		t.Fatal("disabled resource channel constructed a Provider configuration")
	}
	store, err := resourcetask.NewStore(database)
	if err != nil {
		t.Fatalf("construct task store after channel disable failed: %v", err)
	}
	if _, err = store.Create(ctx, resourcetask.CreateInput{
		RequestID: "disabled-channel-request", Capability: resourcecap.CapabilityStart,
		LocalResourceNo: "resource-disabled", ChannelNo: "channel-test",
		Parameters: []byte(`{}`), Priority: 100, MaxAttempts: 8,
	}); err == nil {
		t.Fatal("disabled resource channel accepted a new Provider task")
	}
}

// TestResourceFoundationCodegenPostgreSQL prepares only the explicitly named
// disposable code-generation database. It refuses every other database so a
// developer cannot use this helper against application or production data.
func TestResourceFoundationCodegenPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(resourceCodegenDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to prepare the resource DAO code-generation database", resourceCodegenDSNEnv)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create resource code-generation database failed: %v", err)
	}
	defer func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close resource code-generation database failed: %v", closeErr)
		}
	}()
	name, err := database.GetValue(ctx, "SELECT current_database()")
	if err != nil || name.String() != "linapro_idc_codegen" {
		t.Fatalf("refusing non-codegen database %q: %v", name.String(), err)
	}
	if _, err = database.Exec(ctx, resourceCodegenCleanupSQL); err != nil {
		t.Fatalf("reset disposable resource code-generation tables failed: %v", err)
	}
	executeResourceMigrations(t, ctx, database)
}

func assertResourceSchemaShape(t *testing.T, ctx context.Context, database gdb.DB, schema string) {
	t.Helper()
	count, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.tables
WHERE table_schema = ? AND table_name LIKE 'plugin_idc_resource_%'`, schema)
	if err != nil || count.Int() != 31 {
		t.Fatalf("resource table count=%d want=31 err=%v", count.Int(), err)
	}
	newTables, err := database.GetValue(ctx, `
SELECT count(*) FROM information_schema.tables
WHERE table_schema=? AND table_name IN (
  'plugin_idc_resource_ip_reservation',
  'plugin_idc_resource_provider_auth_session',
  'plugin_idc_resource_server_entitlement_projection',
  'plugin_idc_resource_interface_channel_binding',
  'plugin_idc_resource_interface_projection_outbox',
  'plugin_idc_resource_interface_configuration_revision'
)`, schema)
	if err != nil || newTables.Int() != 6 {
		t.Fatalf("resource reservation/session/ownership/interface table count=%d want=6 err=%v", newTables.Int(), err)
	}
	softDeleteRows, err := database.GetAll(ctx, `
SELECT table_name
FROM information_schema.columns
WHERE table_schema = ?
  AND table_name LIKE 'plugin_idc_resource_%'
  AND column_name = 'deleted_at'
ORDER BY table_name`, schema)
	if err != nil || len(softDeleteRows) != 2 ||
		softDeleteRows[0]["table_name"].String() != "plugin_idc_resource_channel" ||
		softDeleteRows[1]["table_name"].String() != "plugin_idc_resource_pool" {
		t.Fatalf("unexpected resource soft-delete tables: rows=%v err=%v", softDeleteRows, err)
	}
	observationIndex, err := database.GetValue(ctx, `
SELECT count(*)
FROM pg_indexes
WHERE schemaname = ?
  AND tablename = 'plugin_idc_resource_task'
  AND indexname = 'idx_plugin_idc_resource_task_observe'`, schema)
	if err != nil || observationIndex.Int() != 1 {
		t.Fatalf("resource observation index count=%d want=1 err=%v", observationIndex.Int(), err)
	}
	ownerMetadata, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.columns
WHERE table_schema = ?
  AND table_name = 'plugin_idc_resource_task'
  AND column_name = 'owner_metadata'
  AND data_type = 'jsonb'`, schema)
	if err != nil || ownerMetadata.Int() != 1 {
		t.Fatalf("resource task owner metadata columns=%d want=1 err=%v",
			ownerMetadata.Int(), err)
	}
	candidateIndex, err := database.GetValue(ctx, `
SELECT count(*)
FROM pg_indexes
WHERE schemaname = ?
  AND tablename = 'plugin_idc_resource_pool'
  AND indexname = 'idx_plugin_idc_resource_pool_candidate'`, schema)
	if err != nil || candidateIndex.Int() != 1 {
		t.Fatalf("resource pool candidate index count=%d want=1 err=%v", candidateIndex.Int(), err)
	}
	publicationColumns, err := database.GetAll(ctx, `
SELECT column_name
FROM information_schema.columns
WHERE table_schema = ?
  AND table_name = 'plugin_idc_resource_provider_capacity_observation'
  AND column_name IN ('page_size', 'next_cursor', 'page_count')
ORDER BY column_name`, schema)
	if err != nil || len(publicationColumns) != 3 {
		t.Fatalf("resource providerCapacity publication columns=%v want=3 err=%v", publicationColumns, err)
	}
	collectingIndex, err := database.GetValue(ctx, `
SELECT count(*)
FROM pg_indexes
WHERE schemaname = ?
  AND tablename = 'plugin_idc_resource_provider_capacity_observation'
  AND indexname = 'uk_plugin_idc_resource_provider_capacity_collecting'`, schema)
	if err != nil || collectingIndex.Int() != 1 {
		t.Fatalf("resource collecting providerCapacity index count=%d want=1 err=%v", collectingIndex.Int(), err)
	}
	operationIndexes, err := database.GetValue(ctx, `
SELECT count(*)
FROM pg_indexes
WHERE schemaname = ?
  AND tablename = 'plugin_idc_resource_provider_capacity_observation'
  AND indexname IN (
    'idx_plugin_idc_resource_provider_capacity_due',
    'idx_plugin_idc_resource_provider_capacity_operations',
    'idx_plugin_idc_resource_provider_capacity_operations_channel',
    'idx_plugin_idc_resource_provider_capacity_operations_kind',
    'idx_plugin_idc_resource_provider_capacity_operations_status'
  )`, schema)
	if err != nil || operationIndexes.Int() != 5 {
		t.Fatalf("resource providerCapacity operation index count=%d want=5 err=%v", operationIndexes.Int(), err)
	}
	retiredCatalogObjects, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.tables
WHERE table_schema = ?
  AND table_name IN (
    'plugin_idc_resource_catalog_draft_publication',
    'plugin_idc_resource_catalog_draft_publication_event'
  )`, schema)
	if err != nil || retiredCatalogObjects.Int() != 0 {
		t.Fatalf("retired resource catalog publication tables remain: count=%d err=%v",
			retiredCatalogObjects.Int(), err)
	}
	accessoryIndexes, err := database.GetValue(ctx, `
SELECT count(*)
FROM pg_indexes
WHERE schemaname = ?
  AND indexname IN (
    'uk_plugin_idc_resource_accessory_snapshot_current',
    'idx_plugin_idc_resource_accessory_snapshot_latest',
    'idx_plugin_idc_resource_accessory_item_directory',
    'idx_plugin_idc_resource_accessory_refresh_due'
  )`, schema)
	if err != nil || accessoryIndexes.Int() != 4 {
		t.Fatalf("resource accessory directory index count=%d want=4 err=%v", accessoryIndexes.Int(), err)
	}
}

func assertResourceSchemaInvariants(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_channel
    (channel_no, provider_id, provider_plugin_id, display_name, endpoint, status_code)
VALUES ('channel-test', 'provider-test', 'plugin-test', 'Test', 'https://provider.invalid', 'enabled')`); err != nil {
		t.Fatalf("insert resource channel failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_channel
    (channel_no, provider_id, provider_plugin_id, provider_runtime_type,
     provider_contract_version, provider_release_version,
     display_name, endpoint, status_code)
VALUES ('channel-dynamic-incomplete', 'provider-test', 'plugin-test', 'dynamic',
        'v1', 'v0.2.0', 'Incomplete', 'https://provider.invalid', 'disabled')`); err == nil {
		t.Fatal("dynamic resource channel without frozen digests was accepted")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_channel
    (channel_no, provider_id, provider_plugin_id, provider_runtime_type,
     provider_contract_version, provider_release_version,
     provider_release_digest, provider_declaration_digest,
     display_name, endpoint, status_code)
VALUES ('channel-dynamic', 'provider-test', 'plugin-test', 'dynamic',
        'v1', 'v0.2.0', repeat('a',64), repeat('b',64),
        'Dynamic', 'https://provider.invalid', 'disabled')`); err != nil {
		t.Fatalf("insert dynamic channel with frozen evidence failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_channel_secret_binding
    (binding_no, channel_no, binding_name, secret_ref)
VALUES ('binding-1', 'channel-test', 'password', 'secret-ref-1')`); err != nil {
		t.Fatalf("insert active secret binding failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_channel_secret_binding
    (binding_no, channel_no, binding_name, secret_ref)
VALUES ('binding-duplicate', 'channel-test', 'password', 'secret-ref-2')`); err == nil {
		t.Fatal("duplicate active channel secret binding was accepted")
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_resource_channel_secret_binding
SET status_code = 'superseded', superseded_at = CURRENT_TIMESTAMP
WHERE binding_no = 'binding-1';
INSERT INTO plugin_idc_resource_channel_secret_binding
    (binding_no, channel_no, binding_name, secret_ref)
VALUES ('binding-2', 'channel-test', 'password', 'secret-ref-2')`); err != nil {
		t.Fatalf("replace active secret binding failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_pool
    (pool_no, channel_no, display_name, status_code)
VALUES ('pool-test', 'channel-test', 'Test pool', 'enabled');
INSERT INTO plugin_idc_resource_mapping
    (mapping_no, local_resource_no, channel_no, pool_no, external_resource_ref, status_code)
VALUES ('mapping-1', 'resource-1', 'channel-test', 'pool-test', '1001', 'active')`); err != nil {
		t.Fatalf("insert pool and resource mapping failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_accessory_snapshot
    (snapshot_no,mapping_no,local_resource_no,channel_no,snapshot_fingerprint,item_count,observed_at,expires_at)
VALUES ('accessory-snapshot-1','mapping-1','resource-1','channel-test',repeat('a',64),1,
        CURRENT_TIMESTAMP,CURRENT_TIMESTAMP + INTERVAL '15 minutes');
INSERT INTO plugin_idc_resource_accessory_snapshot_item
    (snapshot_no,object_no,accessory_kind,external_ref,display_name,state_code,projection,observed_at)
VALUES ('accessory-snapshot-1','accessory-object-1','disk','upstream-disk-1','System disk','attached',
        '{"sizeGiB":100,"system":true}'::jsonb,CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("insert resource accessory directory failed: %v", err)
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_resource_accessory_snapshot_item
SET display_name='Changed' WHERE object_no='accessory-object-1'`); err == nil {
		t.Fatal("resource accessory snapshot item mutation was accepted")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_accessory_snapshot
    (snapshot_no,mapping_no,local_resource_no,channel_no,snapshot_fingerprint,item_count,observed_at,expires_at)
VALUES ('accessory-snapshot-duplicate','mapping-1','resource-1','channel-test',repeat('b',64),0,
        CURRENT_TIMESTAMP,CURRENT_TIMESTAMP + INTERVAL '15 minutes')`); err == nil {
		t.Fatal("second current resource accessory snapshot was accepted")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_mapping
    (mapping_no, local_resource_no, channel_no, pool_no, external_resource_ref, status_code)
VALUES ('mapping-duplicate', 'resource-2', 'channel-test', 'pool-test', '1001', 'active')`); err == nil {
		t.Fatal("duplicate active upstream resource mapping was accepted")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_task
    (task_no, request_id_digest, capability_code, channel_no, sanitized_parameters)
VALUES ('task-invalid', repeat('a', 64), 'server.start', 'channel-test', '[]'::jsonb)`); err == nil {
		t.Fatal("non-object sanitized resource task parameters were accepted")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_channel
    (channel_no, provider_id, provider_plugin_id, display_name, endpoint, status_code)
VALUES ('channel-other', 'provider-test', 'plugin-test', 'Other', 'https://other.invalid', 'enabled');
INSERT INTO plugin_idc_resource_pool
    (pool_no, channel_no, display_name, status_code)
VALUES ('pool-other', 'channel-other', 'Other pool', 'enabled')`); err != nil {
		t.Fatalf("insert second channel and pool failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_mapping
    (mapping_no, local_resource_no, channel_no, pool_no, external_resource_ref, status_code)
VALUES ('mapping-cross-channel', 'resource-3', 'channel-test', 'pool-other', '1003', 'active')`); err == nil {
		t.Fatal("cross-channel resource pool mapping was accepted")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_task
    (task_no, request_id_digest, capability_code, channel_no, sanitized_parameters)
VALUES ('task-1', repeat('b', 64), 'server.start', 'channel-test', '{}'::jsonb)`); err != nil {
		t.Fatalf("insert valid resource task failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_task_secret_binding
    (binding_no, task_no, binding_name, binding_direction, secret_ref)
VALUES ('task-secret-1', 'task-1', 'password', 'input', 'secret-task-ref-1')`); err != nil {
		t.Fatalf("insert task input secret binding failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_task_secret_binding
    (binding_no, task_no, binding_name, binding_direction, secret_ref)
VALUES ('task-secret-duplicate', 'task-1', 'password', 'input', 'secret-task-ref-2')`); err == nil {
		t.Fatal("duplicate active task secret binding was accepted")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_upstream_task_binding
    (binding_no, task_no, channel_no, external_task_ref, last_observed_at)
VALUES ('upstream-cross-channel', 'task-1', 'channel-other', '9001', CURRENT_TIMESTAMP)`); err == nil {
		t.Fatal("cross-channel upstream task binding was accepted")
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_resource_task
SET status_code = 'result_unknown', version = version + 1
WHERE task_no = 'task-1'`); err == nil {
		t.Fatal("result-unknown task without its replay fence was accepted")
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_resource_task
SET status_code = 'failed', finished_at = NULL, version = version + 1
WHERE task_no = 'task-1'`); err == nil {
		t.Fatal("terminal resource task without completion time was accepted")
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_resource_task
SET status_code = 'result_unknown', result_unknown = TRUE, version = version + 1
WHERE task_no = 'task-1'`); err != nil {
		t.Fatalf("valid result-unknown replay fence was rejected: %v", err)
	}
	assertResourceTaskClaimSafety(t, ctx, database)
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_provider_capacity_observation
    (observation_no, channel_no, capacity_kind, observed_at)
VALUES ('providerCapacity-1', 'channel-test', 'stock', CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("insert providerCapacity snapshot failed: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_provider_capacity_observation_item
    (observation_no, capacity_kind, external_ref, projection, observed_at)
VALUES ('providerCapacity-1', 'region', 'region-1', '{}'::jsonb, CURRENT_TIMESTAMP)`); err == nil {
		t.Fatal("providerCapacity item kind outside its snapshot was accepted")
	}
}

func assertResourceTaskClaimSafety(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_task
    (task_no, request_id_digest, capability_code, local_resource_no, channel_no, sanitized_parameters)
VALUES ('task-invalid-secret', repeat('e', 64), 'server.start', 'resource-invalid', 'channel-test',
        '{"password":"must-not-pass"}'::jsonb);
INSERT INTO plugin_idc_resource_task
    (task_no, request_id_digest, capability_code, local_resource_no, channel_no, sanitized_parameters)
VALUES ('task-pending', repeat('c', 64), 'server.reinstall', 'resource-pending', 'channel-test', '{"reason":"test"}'::jsonb);
INSERT INTO plugin_idc_resource_task_secret_binding
    (binding_no, task_no, binding_name, binding_direction, secret_ref)
VALUES ('task-pending-secret', 'task-pending', 'password', 'input', 'secret-pending-ref');
INSERT INTO plugin_idc_resource_task
    (task_no, request_id_digest, capability_code, local_resource_no, channel_no, status_code,
     sanitized_parameters, attempt_count, locked_by, lease_expires_at)
VALUES ('task-expired-write', repeat('d', 64), 'server.restart', 'resource-expired', 'channel-test', 'submitting',
        '{}'::jsonb, 1, 'expired-lease', CURRENT_TIMESTAMP - INTERVAL '1 minute')`); err != nil {
		t.Fatalf("prepare resource task claim rows failed: %v", err)
	}
	store, err := resourcetask.NewStore(database)
	if err != nil {
		t.Fatalf("construct resource task store failed: %v", err)
	}
	claimed, err := store.Claim(ctx, resourcetask.ClaimInput{
		WorkerID: "integration-worker", Limit: 2, LeaseDuration: time.Minute,
	})
	if err != nil || len(claimed) != 1 || claimed[0].TaskNo != "task-pending" ||
		claimed[0].AttemptNo != 1 || claimed[0].LeaseToken == "" ||
		claimed[0].SanitizedParameters != `{"reason": "test"}` ||
		len(claimed[0].SecretBindings) != 1 ||
		claimed[0].SecretBindings[0].Handle.OpaqueRef() != "secret-pending-ref" {
		t.Fatalf("unexpected resource task claim: claimed=%+v err=%v", claimed, err)
	}
	invalid, err := database.GetOne(ctx, `
SELECT status_code, finished_at, failure_code
FROM plugin_idc_resource_task
WHERE task_no = 'task-invalid-secret'`)
	if err != nil || invalid["status_code"].String() != "failed" ||
		invalid["finished_at"].IsNil() || invalid["failure_code"].String() != "INVALID_PERSISTED_TASK" {
		t.Fatalf("invalid persisted task was not rejected safely: row=%v err=%v", invalid, err)
	}
	observeAt := claimed[0].LeaseExpiresAt
	outputHandle, err := secretcap.NewHandle("secret-output-ref")
	if err != nil {
		t.Fatalf("construct resource task output handle failed: %v", err)
	}
	if err = store.Transition(ctx, resourcetask.TransitionInput{
		Lease: resourcetask.LeaseRef{
			TaskNo: claimed[0].TaskNo, Token: claimed[0].LeaseToken, AttemptNo: claimed[0].AttemptNo,
		},
		To:       resourcetask.StatusAccepted,
		Evidence: resourcetask.TransitionEvidence{AcceptanceProven: true},
		NextAt:   &observeAt,
		Upstream: &resourcetask.UpstreamTask{
			TaskRef: "upstream-task-pending", ResourceRef: "upstream-resource-pending",
			Status: resourcecap.TaskStatusAccepted,
		},
		SecretOutputs: []resourcecap.SecretOutput{{
			Name: "generatedCredential", Policy: resourcecap.SecretOutputPolicyOneTimeCredential,
			Handle: outputHandle,
		}},
	}); err != nil {
		t.Fatalf("transition claimed resource task failed: %v", err)
	}
	accepted, err := database.GetOne(ctx, `
SELECT status_code, locked_by, lease_expires_at, accepted_at, next_observe_at
FROM plugin_idc_resource_task
WHERE task_no = 'task-pending'`)
	if err != nil || accepted["status_code"].String() != "accepted" ||
		accepted["locked_by"].String() != "" || !accepted["lease_expires_at"].IsNil() ||
		accepted["accepted_at"].IsNil() || accepted["next_observe_at"].IsNil() {
		t.Fatalf("accepted resource task was not persisted safely: row=%v err=%v", accepted, err)
	}
	upstream, err := database.GetOne(ctx, `
SELECT channel_no, external_task_ref, external_resource_ref, status_code
FROM plugin_idc_resource_upstream_task_binding
WHERE task_no = 'task-pending' AND step_no = 1`)
	if err != nil || upstream["channel_no"].String() != "channel-test" ||
		upstream["external_task_ref"].String() != "upstream-task-pending" ||
		upstream["external_resource_ref"].String() != "upstream-resource-pending" ||
		upstream["status_code"].String() != "accepted" {
		t.Fatalf("accepted upstream task identity was not persisted atomically: row=%v err=%v", upstream, err)
	}
	revokedInput, err := database.GetOne(ctx, `
SELECT status_code, revoked_at
FROM plugin_idc_resource_task_secret_binding
WHERE task_no = 'task-pending' AND binding_name = 'password'`)
	if err != nil || revokedInput["status_code"].String() != "revoked" ||
		revokedInput["revoked_at"].IsNil() {
		t.Fatalf("accepted task input binding remained active: row=%v err=%v", revokedInput, err)
	}
	output, err := database.GetOne(ctx, `
SELECT binding_direction, secret_ref, output_policy, status_code, expires_at
FROM plugin_idc_resource_task_secret_binding
WHERE task_no = 'task-pending' AND binding_name = 'generatedCredential'`)
	if err != nil || output["binding_direction"].String() != "output" ||
		output["secret_ref"].String() != outputHandle.OpaqueRef() ||
		output["output_policy"].String() != "one_time_credential" ||
		output["status_code"].String() != "active" || !output["expires_at"].IsNil() {
		t.Fatalf("accepted task output binding was not persisted atomically: row=%v err=%v", output, err)
	}
	if err = store.Transition(ctx, resourcetask.TransitionInput{
		Lease: resourcetask.LeaseRef{
			TaskNo: claimed[0].TaskNo, Token: claimed[0].LeaseToken, AttemptNo: claimed[0].AttemptNo,
		},
		To:       resourcetask.StatusAccepted,
		Evidence: resourcetask.TransitionEvidence{AcceptanceProven: true},
		NextAt:   &observeAt,
		Upstream: &resourcetask.UpstreamTask{
			TaskRef: "upstream-task-pending", ResourceRef: "upstream-resource-pending",
			Status: resourcecap.TaskStatusAccepted,
		},
	}); err == nil {
		t.Fatal("stale resource task lease overwrote an accepted task")
	}
	assertResourceMappingResolution(t, ctx, database, store)
	assertProvisionMappingConvergence(t, ctx, database, store)
	assertResourceTaskObservationSafety(t, ctx, database, store)
	expired, err := database.GetOne(ctx, `
SELECT status_code, result_unknown, locked_by, lease_expires_at, failure_code
FROM plugin_idc_resource_task
WHERE task_no = 'task-expired-write'`)
	if err != nil || expired["status_code"].String() != "result_unknown" ||
		!expired["result_unknown"].Bool() || expired["locked_by"].String() != "" ||
		!expired["lease_expires_at"].IsNil() || expired["failure_code"].String() != "WRITE_LEASE_EXPIRED" {
		t.Fatalf("expired Provider write was not frozen safely: row=%v err=%v", expired, err)
	}
	claimedAgain, err := store.Claim(ctx, resourcetask.ClaimInput{
		WorkerID: "integration-worker", Limit: 2, LeaseDuration: time.Minute,
	})
	if err != nil || len(claimedAgain) != 0 {
		t.Fatalf("live or unknown Provider writes were reclaimed: claimed=%+v err=%v", claimedAgain, err)
	}
	assertConcurrentResourceTaskClaims(t, ctx, database, store)
	assertConcurrentResourceTaskObservationClaims(t, ctx, database, store)
	assertIdempotentResourceTaskCreation(t, ctx, database, store)
}

// assertProvisionMappingConvergence verifies synchronous and task-observed
// provisioning cannot succeed without an atomic authoritative mapping.
func assertProvisionMappingConvergence(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	store *resourcetask.Store,
) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_task
    (task_no, request_id_digest, capability_code, local_resource_no, channel_no, pool_no,
     status_code, sanitized_parameters, attempt_count, locked_by, lease_expires_at)
VALUES
    ('task-provision-sync', repeat('1', 64), 'server.provision', 'resource-provision-sync',
     'channel-test', 'pool-test', 'submitting', '{}'::jsonb, 1,
     'provision-worker:11111111111111111111111111111111', CURRENT_TIMESTAMP + INTERVAL '1 minute'),
    ('task-provision-conflict', repeat('2', 64), 'server.provision', 'resource-provision-conflict',
     'channel-test', 'pool-test', 'submitting', '{}'::jsonb, 1,
     'provision-worker:22222222222222222222222222222222', CURRENT_TIMESTAMP + INTERVAL '1 minute'),
    ('task-provision-async', repeat('3', 64), 'server.provision', 'resource-provision-async',
     'channel-test', 'pool-test', 'running', '{}'::jsonb, 1,
     '', NULL);
UPDATE plugin_idc_resource_task
SET next_observe_at = CURRENT_TIMESTAMP - INTERVAL '1 second'
WHERE task_no = 'task-provision-async';
INSERT INTO plugin_idc_resource_upstream_task_binding
    (binding_no, task_no, channel_no, external_task_ref, external_resource_ref,
     status_code, last_observed_at)
VALUES ('binding-provision-async', 'task-provision-async', 'channel-test',
        'upstream-task-provision-async', 'upstream-resource-provision-async',
        'running', CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("prepare provision mapping convergence failed: %v", err)
	}
	observedAt := time.Now().UTC()
	syncEvidence := resourcecap.Evidence{
		Kind: "provisionResult", Reference: "upstream-resource-provision-sync",
		Summary: "resource created", ObservedAt: observedAt,
	}
	if err := store.Transition(ctx, resourcetask.TransitionInput{
		Lease: resourcetask.LeaseRef{
			TaskNo: "task-provision-sync",
			Token:  "provision-worker:11111111111111111111111111111111", AttemptNo: 1,
		},
		To:            resourcetask.StatusSucceeded,
		Evidence:      resourcetask.TransitionEvidence{AcceptanceProven: true},
		ResourceRef:   "upstream-resource-provision-sync",
		ProviderFacts: []resourcecap.Evidence{syncEvidence},
	}); err != nil {
		t.Fatalf("synchronous provision did not converge: %v", err)
	}
	assertProvisionMappingRow(
		t, ctx, database, "task-provision-sync", "resource-provision-sync",
		"upstream-resource-provision-sync", "provider_resource",
	)
	if err := store.Transition(ctx, resourcetask.TransitionInput{
		Lease: resourcetask.LeaseRef{
			TaskNo: "task-provision-conflict",
			Token:  "provision-worker:22222222222222222222222222222222", AttemptNo: 1,
		},
		To:          resourcetask.StatusSucceeded,
		Evidence:    resourcetask.TransitionEvidence{AcceptanceProven: true},
		ResourceRef: "1001",
	}); err == nil {
		t.Fatal("provision succeeded with an upstream resource already owned by another mapping")
	}
	conflict, err := database.GetOne(ctx, `
SELECT status_code FROM plugin_idc_resource_task
WHERE task_no = 'task-provision-conflict'`)
	if err != nil || conflict["status_code"].String() != "submitting" {
		t.Fatalf("mapping conflict did not roll back the task transition: row=%v err=%v", conflict, err)
	}
	claimed, err := store.ClaimObservations(ctx, resourcetask.ObservationClaimInput{
		WorkerID: "provision-observer", Limit: 1, LeaseDuration: time.Minute,
	})
	if err != nil || len(claimed) != 1 || claimed[0].TaskNo != "task-provision-async" {
		t.Fatalf("claim async provision observation failed: claimed=%+v err=%v", claimed, err)
	}
	asyncEvidence := resourcecap.Evidence{
		Kind: "taskQuery", Reference: "upstream-task-provision-async",
		Summary: "provision task completed", ObservedAt: observedAt,
	}
	if err = store.ObserveTask(ctx, resourcetask.ObservationInput{
		Lease: resourcetask.ObservationLeaseRef{
			TaskNo: claimed[0].TaskNo, Token: claimed[0].LeaseToken, Version: claimed[0].LeaseVersion,
		},
		Result: resourcecap.OperationResult{
			Status:              resourcecap.TaskStatusSucceeded,
			ExternalTaskRef:     "upstream-task-provision-async",
			ExternalResourceRef: "upstream-resource-provision-async",
			Evidence:            []resourcecap.Evidence{asyncEvidence},
		},
	}); err != nil {
		t.Fatalf("async provision observation did not converge: %v", err)
	}
	assertProvisionMappingRow(
		t, ctx, database, "task-provision-async", "resource-provision-async",
		"upstream-resource-provision-async", "provider_task",
	)
}

// assertProvisionMappingRow verifies the mapping, task, and evidence share one committed outcome.
func assertProvisionMappingRow(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	taskNo string,
	localResourceNo string,
	externalResourceRef string,
	evidenceSource string,
) {
	t.Helper()
	row, err := database.GetOne(ctx, `
SELECT task.status_code, mapping.channel_no, mapping.pool_no,
       mapping.external_resource_ref, mapping.status_code AS mapping_status,
       evidence.source_kind, evidence.reference
FROM plugin_idc_resource_task AS task
JOIN plugin_idc_resource_mapping AS mapping
  ON mapping.local_resource_no = task.local_resource_no
JOIN plugin_idc_resource_task_evidence AS evidence
  ON evidence.task_no = task.task_no
WHERE task.task_no = ? AND task.local_resource_no = ?`, taskNo, localResourceNo)
	if err != nil || row["status_code"].String() != "succeeded" ||
		row["channel_no"].String() != "channel-test" || row["pool_no"].String() != "pool-test" ||
		row["external_resource_ref"].String() != externalResourceRef ||
		row["mapping_status"].String() != "active" ||
		row["source_kind"].String() != evidenceSource ||
		row["reference"].String() == "" {
		t.Fatalf("provision mapping did not converge atomically: row=%v err=%v", row, err)
	}
}

// assertResourceMappingResolution verifies bounded mapping resolution and
// no-task-reference resource reconciliation against real PostgreSQL.
func assertResourceMappingResolution(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	store *resourcetask.Store,
) {
	t.Helper()
	resolved, err := store.ResolveResources(ctx, []resourcetask.ResourceResolutionRequest{
		{
			TaskNo: "task-resolve-1", Capability: resourcecap.CapabilityStart,
			LocalResourceNo: "resource-1", ChannelNo: "channel-test", PoolNo: "pool-test",
		},
		{
			TaskNo: "task-resolve-mismatch", Capability: resourcecap.CapabilityStop,
			LocalResourceNo: "resource-1", ChannelNo: "channel-test", PoolNo: "pool-other",
		},
		{
			TaskNo: "task-resolve-provision", Capability: resourcecap.CapabilityProvision,
			LocalResourceNo: "resource-new", ChannelNo: "channel-test", PoolNo: "pool-test",
		},
	})
	if err != nil || len(resolved) != 1 || resolved["task-resolve-1"] != "1001" {
		t.Fatalf("active resource mapping batch resolved incorrectly: resolved=%v err=%v", resolved, err)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_resource_task
    (task_no, request_id_digest, capability_code, local_resource_no, channel_no, pool_no,
     status_code, sanitized_parameters, result_unknown, next_observe_at)
VALUES ('task-resource-unknown', repeat('f', 64), 'server.start', 'resource-1',
        'channel-test', 'pool-test', 'result_unknown', '{}'::jsonb, TRUE,
        CURRENT_TIMESTAMP - INTERVAL '1 second')`); err != nil {
		t.Fatalf("prepare unknown resource observation failed: %v", err)
	}
	claimed, err := store.ClaimResourceObservations(ctx, resourcetask.ObservationClaimInput{
		WorkerID: "resource-observer", Limit: 1, LeaseDuration: time.Minute,
	})
	if err != nil || len(claimed) != 1 || claimed[0].TaskNo != "task-resource-unknown" ||
		claimed[0].ExternalResourceRef != "1001" {
		t.Fatalf("claim unknown resource observation failed: claimed=%+v err=%v", claimed, err)
	}
	lease := resourcetask.ObservationLeaseRef{
		TaskNo: claimed[0].TaskNo, Token: claimed[0].LeaseToken, Version: claimed[0].LeaseVersion,
	}
	persistedClaimedAt, err := database.GetValue(ctx, `
SELECT observation_claimed_at
FROM plugin_idc_resource_task
WHERE task_no = ?`, claimed[0].TaskNo)
	if err != nil || !persistedClaimedAt.Time().UTC().Equal(claimed[0].ClaimedAt.UTC()) {
		t.Fatalf(
			"resource observation claim time diverged: persisted=%v returned=%v err=%v",
			persistedClaimedAt, claimed[0].ClaimedAt, err,
		)
	}
	if err = store.ObserveResource(ctx, resourcetask.ResourceObservationInput{
		Lease: lease,
		Snapshot: resourcecap.ResourceSnapshot{
			ExternalResourceRef: "1001", State: resourcecap.ResourceStateRunning,
			ObservedAt: claimed[0].ClaimedAt.Add(-time.Second),
		},
	}); err == nil {
		t.Fatal("snapshot predating the observation lease closed an unknown task")
	}
	observedAt := claimed[0].ClaimedAt
	if err = store.ObserveResource(ctx, resourcetask.ResourceObservationInput{
		Lease: lease,
		Snapshot: resourcecap.ResourceSnapshot{
			ExternalResourceRef: "1002", State: resourcecap.ResourceStateRunning,
			ObservedAt: observedAt,
		},
	}); err == nil {
		t.Fatal("mismatched resource snapshot closed an unknown task")
	}
	if err = store.ObserveResource(ctx, resourcetask.ResourceObservationInput{
		Lease: lease,
		Snapshot: resourcecap.ResourceSnapshot{
			ExternalResourceRef: "1001", State: resourcecap.ResourceStateRunning,
			ObservedAt: observedAt,
			Evidence: []resourcecap.Evidence{{
				Kind: "resourceQuery", Reference: "1001", Summary: "running state observed",
				ObservedAt: observedAt,
			}},
		},
	}); err != nil {
		t.Fatalf("matching resource snapshot did not close unknown task: %v", err)
	}
	row, err := database.GetOne(ctx, `
SELECT status_code, result_unknown, locked_by, lease_expires_at, next_observe_at, finished_at
FROM plugin_idc_resource_task
WHERE task_no = 'task-resource-unknown'`)
	if err != nil || row["status_code"].String() != "succeeded" || row["result_unknown"].Bool() ||
		row["locked_by"].String() != "" || !row["lease_expires_at"].IsNil() ||
		!row["next_observe_at"].IsNil() || row["finished_at"].IsNil() {
		t.Fatalf("resource snapshot did not converge unknown task atomically: row=%v err=%v", row, err)
	}
	evidence, err := database.GetAll(ctx, `
SELECT source_kind, evidence_kind, reference, summary, observed_at
FROM plugin_idc_resource_task_evidence
WHERE task_no = 'task-resource-unknown'
ORDER BY id`)
	if err != nil || len(evidence) != 2 ||
		evidence[0]["source_kind"].String() != "owner_resource_state" ||
		evidence[0]["evidence_kind"].String() != "resourceState" ||
		evidence[0]["reference"].String() != "1001" ||
		evidence[0]["summary"].String() != "running" ||
		evidence[1]["source_kind"].String() != "provider_resource" ||
		evidence[1]["evidence_kind"].String() != "resourceQuery" ||
		evidence[1]["reference"].String() != "1001" ||
		evidence[1]["summary"].String() != "running state observed" {
		t.Fatalf("resource reconciliation evidence was not persisted atomically: evidence=%v err=%v", evidence, err)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_resource_task
    (task_no, request_id_digest, capability_code, local_resource_no, channel_no, pool_no,
     status_code, sanitized_parameters, result_unknown, next_observe_at)
VALUES ('task-resource-ip-unknown', repeat('a', 64), 'server.ip.change', 'resource-1',
        'channel-test', 'pool-test', 'result_unknown', '{"ip_num":2}'::jsonb, TRUE,
        CURRENT_TIMESTAMP - INTERVAL '1 second')`); err != nil {
		t.Fatalf("prepare unknown IP observation failed: %v", err)
	}
	claimed, err = store.ClaimResourceObservations(ctx, resourcetask.ObservationClaimInput{
		WorkerID: "resource-ip-observer", Limit: 1, LeaseDuration: time.Minute,
	})
	if err != nil || len(claimed) != 1 || claimed[0].TaskNo != "task-resource-ip-unknown" {
		t.Fatalf("claim unknown IP observation failed: claimed=%+v err=%v", claimed, err)
	}
	lease = resourcetask.ObservationLeaseRef{
		TaskNo: claimed[0].TaskNo, Token: claimed[0].LeaseToken, Version: claimed[0].LeaseVersion,
	}
	observedAt = claimed[0].ClaimedAt
	if err = store.ObserveResource(ctx, resourcetask.ResourceObservationInput{
		Lease: lease,
		Snapshot: resourcecap.ResourceSnapshot{
			ExternalResourceRef: "1001", State: resourcecap.ResourceStateActive,
			Data: []byte(`{"ip_count":1}`), ObservedAt: observedAt,
		},
	}); err == nil {
		t.Fatal("wrong observed IP count closed an unknown IP task")
	}
	if err = store.ObserveResource(ctx, resourcetask.ResourceObservationInput{
		Lease: lease,
		Snapshot: resourcecap.ResourceSnapshot{
			ExternalResourceRef: "1001", State: resourcecap.ResourceStateActive,
			Data: []byte(`{"ip_count":2}`), ObservedAt: observedAt,
		},
	}); err != nil {
		t.Fatalf("matching observed IP count did not close unknown IP task: %v", err)
	}
	ipOutcome, err := database.GetAll(ctx, `
SELECT task.status_code, evidence.evidence_kind, evidence.reference, evidence.summary
FROM plugin_idc_resource_task AS task
JOIN plugin_idc_resource_task_evidence AS evidence ON evidence.task_no = task.task_no
WHERE task.task_no = 'task-resource-ip-unknown'
ORDER BY evidence.id`)
	if err != nil || len(ipOutcome) != 2 || ipOutcome[0]["status_code"].String() != "succeeded" ||
		ipOutcome[0]["evidence_kind"].String() != "resourceState" ||
		ipOutcome[1]["evidence_kind"].String() != "ipCount" ||
		ipOutcome[1]["reference"].String() != "1001" || ipOutcome[1]["summary"].String() != "2" {
		t.Fatalf("IP count reconciliation did not persist exact owner evidence: rows=%v err=%v", ipOutcome, err)
	}
}

func assertResourceTaskObservationSafety(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	store *resourcetask.Store,
) {
	t.Helper()
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_resource_task
SET next_observe_at = CURRENT_TIMESTAMP - INTERVAL '1 second'
WHERE task_no = 'task-pending'`); err != nil {
		t.Fatalf("make accepted resource task observation due failed: %v", err)
	}
	claimed, err := store.ClaimObservations(ctx, resourcetask.ObservationClaimInput{
		WorkerID: "integration-observer", Limit: 1, LeaseDuration: time.Minute,
	})
	if err != nil || len(claimed) != 1 || claimed[0].TaskNo != "task-pending" ||
		claimed[0].CurrentStatus != resourcetask.StatusAccepted ||
		claimed[0].ExternalTaskRef != "upstream-task-pending" ||
		claimed[0].ExternalResourceRef != "upstream-resource-pending" ||
		claimed[0].LeaseToken == "" || claimed[0].LeaseVersion <= 1 ||
		claimed[0].ObservationAttempt != 1 {
		t.Fatalf("unexpected resource task observation claim: claimed=%+v err=%v", claimed, err)
	}
	claimedAgain, err := store.ClaimObservations(ctx, resourcetask.ObservationClaimInput{
		WorkerID: "integration-observer-other", Limit: 1, LeaseDuration: time.Minute,
	})
	if err != nil || len(claimedAgain) != 0 {
		t.Fatalf("live resource task observation lease was reclaimed: claimed=%+v err=%v", claimedAgain, err)
	}
	lease := resourcetask.ObservationLeaseRef{
		TaskNo: claimed[0].TaskNo, Token: claimed[0].LeaseToken, Version: claimed[0].LeaseVersion,
	}
	rescheduledAt := time.Now().UTC().Add(time.Minute)
	if err = store.RescheduleObservation(ctx, resourcetask.ObservationRescheduleInput{
		Lease: lease, NextAt: rescheduledAt,
	}); err != nil {
		t.Fatalf("reschedule failed resource task observation failed: %v", err)
	}
	rescheduled, err := database.GetOne(ctx, `
SELECT status_code, locked_by, lease_expires_at, next_observe_at, failure_code,
       attempt_count, observation_attempt_count
FROM plugin_idc_resource_task
WHERE task_no = 'task-pending'`)
	if err != nil || rescheduled["status_code"].String() != "accepted" ||
		rescheduled["locked_by"].String() != "" || !rescheduled["lease_expires_at"].IsNil() ||
		rescheduled["next_observe_at"].IsNil() ||
		rescheduled["failure_code"].String() != "OBSERVATION_UNAVAILABLE" ||
		rescheduled["attempt_count"].Int() != 1 ||
		rescheduled["observation_attempt_count"].Int() != 1 {
		t.Fatalf("query outage changed accepted task incorrectly: row=%v err=%v", rescheduled, err)
	}
	if err = store.RescheduleObservation(ctx, resourcetask.ObservationRescheduleInput{
		Lease: lease, NextAt: rescheduledAt,
	}); err == nil {
		t.Fatal("stale observation lease rescheduled a task twice")
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_task
SET next_observe_at = CURRENT_TIMESTAMP - INTERVAL '1 second'
WHERE task_no = 'task-pending'`); err != nil {
		t.Fatalf("make rescheduled observation due failed: %v", err)
	}
	claimed, err = store.ClaimObservations(ctx, resourcetask.ObservationClaimInput{
		WorkerID: "integration-observer", Limit: 1, LeaseDuration: time.Minute,
	})
	if err != nil || len(claimed) != 1 || claimed[0].ObservationAttempt != 2 {
		t.Fatalf("reclaim rescheduled observation failed: claimed=%+v err=%v", claimed, err)
	}
	lease = resourcetask.ObservationLeaseRef{
		TaskNo: claimed[0].TaskNo, Token: claimed[0].LeaseToken, Version: claimed[0].LeaseVersion,
	}
	next := time.Now().UTC().Add(time.Minute)
	if err = store.ObserveTask(ctx, resourcetask.ObservationInput{
		Lease: lease,
		Result: resourcecap.OperationResult{
			Status: resourcecap.TaskStatusRunning, ExternalTaskRef: "upstream-task-other",
			ExternalResourceRef: "upstream-resource-pending",
		},
		NextAt: &next,
	}); err == nil {
		t.Fatal("mismatched observed upstream task reference changed local state")
	}
	progress := 50
	if err = store.ObserveTask(ctx, resourcetask.ObservationInput{
		Lease: lease,
		Result: resourcecap.OperationResult{
			Status: resourcecap.TaskStatusRunning, ExternalTaskRef: "upstream-task-pending",
			ExternalResourceRef: "upstream-resource-pending", Progress: &progress,
		},
		NextAt: &next,
	}); err != nil {
		t.Fatalf("running resource task observation failed: %v", err)
	}
	running, err := database.GetOne(ctx, `
SELECT task.status_code, task.failure_code, task.locked_by, task.lease_expires_at,
       task.attempt_count, task.observation_attempt_count,
       binding.status_code AS upstream_status, binding.last_progress
FROM plugin_idc_resource_task AS task
JOIN plugin_idc_resource_upstream_task_binding AS binding
  ON binding.task_no = task.task_no AND binding.step_no = 1
WHERE task.task_no = 'task-pending'`)
	if err != nil || running["status_code"].String() != "running" ||
		running["failure_code"].String() != "" || running["locked_by"].String() != "" ||
		!running["lease_expires_at"].IsNil() || running["upstream_status"].String() != "running" ||
		running["last_progress"].Int() != progress || running["attempt_count"].Int() != 1 ||
		running["observation_attempt_count"].Int() != 0 {
		t.Fatalf("running observation did not converge atomically: row=%v err=%v", running, err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_task
SET next_observe_at = CURRENT_TIMESTAMP - INTERVAL '1 second'
WHERE task_no = 'task-pending'`); err != nil {
		t.Fatalf("make running task observation due failed: %v", err)
	}
	claimed, err = store.ClaimObservations(ctx, resourcetask.ObservationClaimInput{
		WorkerID: "integration-observer", Limit: 1, LeaseDuration: time.Minute,
	})
	if err != nil || len(claimed) != 1 || claimed[0].ObservationAttempt != 1 {
		t.Fatalf("claim out-of-order observation failed: claimed=%+v err=%v", claimed, err)
	}
	outOfOrderNext := time.Now().UTC().Add(time.Minute)
	if err = store.ObserveTask(ctx, resourcetask.ObservationInput{
		Lease: resourcetask.ObservationLeaseRef{
			TaskNo:  claimed[0].TaskNo,
			Token:   claimed[0].LeaseToken,
			Version: claimed[0].LeaseVersion,
		},
		Result: resourcecap.OperationResult{
			Status: resourcecap.TaskStatusAccepted, ExternalTaskRef: "upstream-task-pending",
			ExternalResourceRef: "upstream-resource-pending",
		},
		NextAt: &outOfOrderNext,
	}); err != nil {
		t.Fatalf("out-of-order accepted observation failed: %v", err)
	}
	outOfOrder, err := database.GetOne(ctx, `
SELECT task.status_code, task.locked_by, task.lease_expires_at,
       task.observation_attempt_count, binding.status_code AS upstream_status
FROM plugin_idc_resource_task AS task
JOIN plugin_idc_resource_upstream_task_binding AS binding
  ON binding.task_no = task.task_no AND binding.step_no = 1
WHERE task.task_no = 'task-pending'`)
	if err != nil || outOfOrder["status_code"].String() != "running" ||
		outOfOrder["locked_by"].String() != "" || !outOfOrder["lease_expires_at"].IsNil() ||
		outOfOrder["observation_attempt_count"].Int() != 1 ||
		outOfOrder["upstream_status"].String() != "accepted" {
		t.Fatalf("out-of-order observation regressed owner state: row=%v err=%v", outOfOrder, err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_resource_task
SET next_observe_at = CURRENT_TIMESTAMP - INTERVAL '1 second'
WHERE task_no = 'task-pending'`); err != nil {
		t.Fatalf("make out-of-order task observation due failed: %v", err)
	}
	claimed, err = store.ClaimObservations(ctx, resourcetask.ObservationClaimInput{
		WorkerID: "integration-observer", Limit: 1, LeaseDuration: time.Minute,
	})
	if err != nil || len(claimed) != 1 || claimed[0].ObservationAttempt != 2 {
		t.Fatalf("claim terminal observation failed: claimed=%+v err=%v", claimed, err)
	}
	terminalInput := resourcetask.ObservationInput{
		Lease: resourcetask.ObservationLeaseRef{
			TaskNo:  claimed[0].TaskNo,
			Token:   claimed[0].LeaseToken,
			Version: claimed[0].LeaseVersion,
		},
		Result: resourcecap.OperationResult{
			Status: resourcecap.TaskStatusSucceeded, ExternalTaskRef: "upstream-task-pending",
			ExternalResourceRef: "upstream-resource-pending",
			Evidence: []resourcecap.Evidence{{
				Kind:       "upstream_task",
				Reference:  "upstream-task-pending",
				Summary:    "controlled terminal task observation",
				ObservedAt: time.Now().UTC(),
			}},
		},
	}
	if err = store.ObserveTask(ctx, terminalInput); err != nil {
		t.Fatalf("successful resource task observation failed: %v", err)
	}
	if err = store.ObserveTask(ctx, terminalInput); err == nil {
		t.Fatal("duplicate terminal observation reused a consumed lease")
	}
	succeeded, err := database.GetOne(ctx, `
SELECT status_code, result_unknown, locked_by, lease_expires_at, next_observe_at, finished_at,
       attempt_count, observation_attempt_count
FROM plugin_idc_resource_task
WHERE task_no = 'task-pending'`)
	if err != nil || succeeded["status_code"].String() != "succeeded" ||
		succeeded["result_unknown"].Bool() || succeeded["locked_by"].String() != "" ||
		!succeeded["lease_expires_at"].IsNil() || !succeeded["next_observe_at"].IsNil() ||
		succeeded["finished_at"].IsNil() || succeeded["attempt_count"].Int() != 1 ||
		succeeded["observation_attempt_count"].Int() != 0 {
		t.Fatalf("terminal observation did not close resource task: row=%v err=%v", succeeded, err)
	}
	evidence, err := database.GetOne(ctx, `
SELECT count(*) AS evidence_count
FROM plugin_idc_resource_task_evidence
WHERE task_no = 'task-pending'
  AND source_kind = 'provider_task'
  AND evidence_kind = 'upstream_task'
  AND reference = 'upstream-task-pending'`)
	if err != nil || evidence["evidence_count"].Int() != 1 {
		t.Fatalf("duplicate terminal observation appended evidence twice: row=%v err=%v", evidence, err)
	}
}

func assertIdempotentResourceTaskCreation(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	store *resourcetask.Store,
) {
	t.Helper()
	availableAt := time.Now().UTC().Add(time.Hour)
	input := resourcetask.CreateInput{
		RequestID: "idempotent-create-request", Capability: resourcecap.CapabilityProvision,
		LocalResourceNo: "resource-created", ChannelNo: "channel-test", PoolNo: "pool-test",
		Parameters: []byte(`{"b":2,"a":1}`), Priority: 50, MaxAttempts: 4,
		AvailableAt: &availableAt,
	}
	handle, err := secretcap.NewHandle("secret-created-ref")
	if err != nil {
		t.Fatalf("construct created task secret handle failed: %v", err)
	}
	input.SecretBindings = []resourcecap.SecretBinding{{Name: "password", Handle: handle}}
	correlationHandle, handleErr := secretcap.NewHandle("secret-downstream-token-ref")
	if handleErr != nil {
		t.Fatalf("construct callback token handle failed: %v", handleErr)
	}
	input.ProviderCorrelation = &resourcetask.ProviderCorrelationInput{
		PluginID: "idc-zjmf-finance", ConfigurationRevision: "rci_11111111111111111111111111111111",
		DownstreamURL: "https://platform.example.test/api/plugins/idc-resource/provider-callback",
		CreateToken: func(context.Context, int64, string) (secretcap.CreateResult, error) {
			return secretcap.CreateResult{Handle: correlationHandle, Metadata: secretcap.Metadata{Configured: true, UpdatedAt: time.Now().UTC()}}, nil
		},
	}
	created, err := store.Create(ctx, input)
	if err != nil || !created.Created || created.TaskNo == "" || created.Status != resourcetask.StatusPending ||
		created.ProviderCorrelation == nil || created.ProviderCorrelation.DownstreamID != created.ID {
		t.Fatalf("create idempotent resource task failed: task=%+v err=%v", created, err)
	}
	replayInput := input
	replayInput.Parameters = []byte(`{ "a": 1, "b": 2 }`)
	replayed, err := store.Create(ctx, replayInput)
	if err != nil || replayed.Created || replayed.TaskNo != created.TaskNo {
		t.Fatalf("idempotent resource task replay diverged: task=%+v err=%v", replayed, err)
	}
	conflictInput := input
	conflictInput.Capability = resourcecap.CapabilityStop
	if _, err = store.Create(ctx, conflictInput); err == nil {
		t.Fatal("reused resource task idempotency key accepted a changed operation")
	}
	changedHandle, err := secretcap.NewHandle("secret-changed-ref")
	if err != nil {
		t.Fatalf("construct changed task secret handle failed: %v", err)
	}
	secretConflict := input
	secretConflict.SecretBindings = []resourcecap.SecretBinding{{Name: "password", Handle: changedHandle}}
	if _, err = store.Create(ctx, secretConflict); err == nil {
		t.Fatal("reused resource task idempotency key accepted a changed secret binding")
	}
	row, err := database.GetOne(ctx, `
SELECT task.request_id_digest, length(task.request_id_digest) AS digest_length,
       count(binding.id) AS binding_count
FROM plugin_idc_resource_task AS task
LEFT JOIN plugin_idc_resource_task_secret_binding AS binding
  ON binding.task_no = task.task_no AND binding.status_code = 'active'
WHERE task.task_no = ?
GROUP BY task.request_id_digest`, created.TaskNo)
	if err != nil || row["digest_length"].Int() != 64 ||
		row["request_id_digest"].String() == input.RequestID || row["binding_count"].Int() != 2 {
		t.Fatalf("resource task did not persist only an idempotency digest: row=%v err=%v", row, err)
	}
	correlationRow, err := database.GetOne(ctx, `SELECT task_id,task_no,plugin_id,channel_no,configuration_revision,
       downstream_url,token_secret_ref,row_to_json(correlation)::TEXT AS persisted
FROM plugin_idc_resource_provider_correlation correlation WHERE task_no=?`, created.TaskNo)
	if err != nil || correlationRow["task_id"].Int64() != created.ID ||
		correlationRow["token_secret_ref"].String() != correlationHandle.OpaqueRef() ||
		strings.Contains(correlationRow["persisted"].String(), "callback-plaintext") {
		t.Fatalf("provider correlation was not bound safely: row=%v err=%v", correlationRow, err)
	}
}

func assertConcurrentResourceTaskClaims(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	store *resourcetask.Store,
) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_task
    (task_no, request_id_digest, capability_code, local_resource_no, channel_no, sanitized_parameters)
SELECT
    'task-concurrent-' || value::text,
    md5(value::text) || md5(value::text),
    'server.start',
    'resource-concurrent-' || value::text,
    'channel-test',
    '{}'::jsonb
FROM generate_series(1, 16) AS value`); err != nil {
		t.Fatalf("prepare concurrent resource tasks failed: %v", err)
	}
	type claimResult struct {
		tasks []resourcetask.ClaimedTask
		err   error
	}
	results := make(chan claimResult, 4)
	var workers sync.WaitGroup
	for worker := 1; worker <= 4; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			tasks, err := store.Claim(ctx, resourcetask.ClaimInput{
				WorkerID: "concurrent-worker-" + strconv.Itoa(worker),
				Limit:    4, LeaseDuration: time.Minute,
			})
			results <- claimResult{tasks: tasks, err: err}
		}(worker)
	}
	workers.Wait()
	close(results)
	claimed := make(map[string]struct{}, 16)
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent resource task claim failed: %v", result.err)
		}
		if len(result.tasks) != 4 {
			t.Fatalf("concurrent claim size=%d want=4", len(result.tasks))
		}
		for _, task := range result.tasks {
			if _, duplicate := claimed[task.TaskNo]; duplicate {
				t.Fatalf("resource task %q was claimed more than once", task.TaskNo)
			}
			claimed[task.TaskNo] = struct{}{}
		}
	}
	if len(claimed) != 16 {
		t.Fatalf("concurrent unique claims=%d want=16", len(claimed))
	}
}

func assertConcurrentResourceTaskObservationClaims(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	store *resourcetask.Store,
) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_resource_task
    (task_no, request_id_digest, capability_code, local_resource_no, channel_no,
     status_code, sanitized_parameters, next_observe_at, accepted_at)
SELECT
    'task-observe-concurrent-' || value::text,
    md5('observe-' || value::text) || md5('observe-' || value::text),
    'server.start',
    'resource-observe-concurrent-' || value::text,
    'channel-test',
    'accepted',
    '{}'::jsonb,
    CURRENT_TIMESTAMP - INTERVAL '1 second',
    CURRENT_TIMESTAMP
FROM generate_series(1, 16) AS value;
INSERT INTO plugin_idc_resource_upstream_task_binding
    (binding_no, task_no, channel_no, external_task_ref, last_observed_at)
SELECT
    'binding-observe-concurrent-' || value::text,
    'task-observe-concurrent-' || value::text,
    'channel-test',
    'upstream-observe-concurrent-' || value::text,
    CURRENT_TIMESTAMP
FROM generate_series(1, 16) AS value`); err != nil {
		t.Fatalf("prepare concurrent resource task observations failed: %v", err)
	}
	type observationClaimResult struct {
		observations []resourcetask.ClaimedObservation
		err          error
	}
	results := make(chan observationClaimResult, 4)
	var workers sync.WaitGroup
	for worker := 1; worker <= 4; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			observations, err := store.ClaimObservations(ctx, resourcetask.ObservationClaimInput{
				WorkerID: "concurrent-observer-" + strconv.Itoa(worker),
				Limit:    4, LeaseDuration: time.Minute,
			})
			results <- observationClaimResult{observations: observations, err: err}
		}(worker)
	}
	workers.Wait()
	close(results)
	claimed := make(map[string]struct{}, 16)
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent resource observation claim failed: %v", result.err)
		}
		if len(result.observations) != 4 {
			t.Fatalf("concurrent observation claim size=%d want=4", len(result.observations))
		}
		for _, observation := range result.observations {
			if _, duplicate := claimed[observation.TaskNo]; duplicate {
				t.Fatalf("resource task observation %q was claimed more than once", observation.TaskNo)
			}
			claimed[observation.TaskNo] = struct{}{}
		}
	}
	if len(claimed) != 16 {
		t.Fatalf("concurrent unique observation claims=%d want=16", len(claimed))
	}
}

// resourceMigrationPaths returns every ordered plugin migration used by the isolated test schema.
func resourceMigrationPaths(t *testing.T) []string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve resource migration test path failed")
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(file), "..", "manifest", "sql", "[0-9][0-9][0-9]-*.sql"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("resolve resource migration paths failed: %v", err)
	}
	return paths
}

// executeResourceMigrations applies every resource migration twice to prove idempotency.
func executeResourceMigrations(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	paths := resourceMigrationPaths(t)
	for attempt := 1; attempt <= 2; attempt++ {
		for _, path := range paths {
			migration, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read resource migration %q failed: %v", filepath.Base(path), err)
			}
			if _, err = database.Exec(ctx, string(migration)); err != nil {
				t.Fatalf(
					"execute resource migration %q attempt %d failed: %v",
					filepath.Base(path),
					attempt,
					err,
				)
			}
		}
	}
}

func assertCatalogPublicationRetirementMigration(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
) {
	t.Helper()
	if _, err := database.Exec(ctx, `
CREATE TABLE plugin_idc_resource_catalog_draft_publication (id BIGINT PRIMARY KEY);
CREATE TABLE plugin_idc_resource_catalog_draft_publication_event (id BIGINT PRIMARY KEY);
CREATE FUNCTION plugin_idc_resource_reject_catalog_publication_mutation()
RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN RETURN OLD; END $$;
CREATE TRIGGER trg_plugin_idc_resource_catalog_publication_immutable
BEFORE DELETE ON plugin_idc_resource_catalog_draft_publication
FOR EACH ROW EXECUTE FUNCTION plugin_idc_resource_reject_catalog_publication_mutation();
ALTER TABLE plugin_idc_resource_provider_capacity_observation
    ADD CONSTRAINT uk_plugin_idc_resource_inventory_pub_source
    UNIQUE (observation_no, capacity_kind, status_code);
CREATE TABLE sys_dict_type (tenant_id INT NOT NULL, type VARCHAR(128) NOT NULL);
CREATE TABLE sys_dict_data (tenant_id INT NOT NULL, dict_type VARCHAR(128) NOT NULL);
INSERT INTO sys_dict_type VALUES (0, 'idc_resource_catalog_publication_status');
INSERT INTO sys_dict_data VALUES (0, 'idc_resource_catalog_publication_status');`); err != nil {
		t.Fatalf("stage retired resource catalog publication objects failed: %v", err)
	}
	var retirementPath string
	for _, path := range resourceMigrationPaths(t) {
		if filepath.Base(path) == "022-remove-catalog-publication.sql" {
			retirementPath = path
			break
		}
	}
	if retirementPath == "" {
		t.Fatal("resource catalog publication retirement migration is missing")
	}
	migration, err := os.ReadFile(retirementPath)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("execute resource catalog retirement attempt %d failed: %v", attempt, err)
		}
	}
	remaining, err := database.GetOne(ctx, `
SELECT
  (to_regclass('plugin_idc_resource_catalog_draft_publication') IS NOT NULL)::int AS publication_table,
  (to_regclass('plugin_idc_resource_catalog_draft_publication_event') IS NOT NULL)::int AS event_table,
  (to_regprocedure('plugin_idc_resource_reject_catalog_publication_mutation()') IS NOT NULL)::int AS function_object,
  (SELECT count(*) FROM pg_constraint constraint_value
   JOIN pg_class relation ON relation.oid=constraint_value.conrelid
   JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
   WHERE namespace.nspname=current_schema()
     AND constraint_value.conname='uk_plugin_idc_resource_inventory_pub_source') AS source_constraint,
  (SELECT count(*) FROM sys_dict_type WHERE type='idc_resource_catalog_publication_status') AS dict_type,
  (SELECT count(*) FROM sys_dict_data WHERE dict_type='idc_resource_catalog_publication_status') AS dict_data`)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range remaining {
		if value.Int() != 0 {
			t.Fatalf("retired resource catalog object %s remains: %v", key, remaining)
		}
	}
}

func assertProviderCapacityRenamePreservesDevelopmentData(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
) {
	t.Helper()
	if _, err := database.Exec(ctx, `
ALTER TABLE plugin_idc_resource_channel_health_state
    ADD COLUMN last_inventory_latency_ms INT;
INSERT INTO plugin_idc_resource_channel_health_state (channel_no)
VALUES ('channel-test')
ON CONFLICT (tenant_id,channel_no) DO NOTHING;
UPDATE plugin_idc_resource_channel_health_state
   SET last_inventory_latency_ms=1234,
       last_provider_capacity_latency_ms=NULL;
INSERT INTO plugin_idc_resource_provider_capacity_observation
    (observation_no,channel_no,capacity_kind,status_code,observation_fingerprint,item_count,
     observed_at,completed_at,page_size,page_count)
VALUES ('capacity-migration-preserve','channel-test','line','complete',repeat('a',64),1,
        CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,100,1);
INSERT INTO plugin_idc_resource_provider_capacity_observation_item
    (observation_no,capacity_kind,external_ref,projection,observed_at)
VALUES ('capacity-migration-preserve','line','line-preserved','{"name":"preserved"}'::jsonb,CURRENT_TIMESTAMP);

DO $$
DECLARE
    constraint_row record;
    index_row record;
BEGIN
    FOR constraint_row IN
        SELECT relation.relname AS table_name, constraint_value.conname
        FROM pg_constraint constraint_value
        JOIN pg_class relation ON relation.oid=constraint_value.conrelid
        JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
        WHERE namespace.nspname=current_schema()
          AND relation.relname IN (
              'plugin_idc_resource_provider_capacity_observation',
              'plugin_idc_resource_provider_capacity_observation_item'
          )
          AND constraint_value.conname LIKE '%provider_capacity%'
    LOOP
        EXECUTE format(
            'ALTER TABLE %I RENAME CONSTRAINT %I TO %I',
            constraint_row.table_name,
            constraint_row.conname,
            replace(constraint_row.conname,'provider_capacity','inventory')
        );
    END LOOP;
    FOR index_row IN
        SELECT relation.relname
        FROM pg_class relation
        JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
        JOIN pg_index index_value ON index_value.indexrelid=relation.oid
        JOIN pg_class owner_table ON owner_table.oid=index_value.indrelid
        WHERE namespace.nspname=current_schema() AND relation.relkind='i'
          AND owner_table.relname IN (
              'plugin_idc_resource_provider_capacity_observation',
              'plugin_idc_resource_provider_capacity_observation_item'
          )
          AND relation.relname LIKE '%provider_capacity%'
    LOOP
        EXECUTE format(
            'ALTER INDEX %I RENAME TO %I',
            index_row.relname,
            replace(index_row.relname,'provider_capacity','inventory')
        );
    END LOOP;
END $$;

ALTER TABLE plugin_idc_resource_ip_reservation
    RENAME COLUMN capacity_observation_no TO snapshot_no;
ALTER TABLE plugin_idc_resource_ip_reservation
    RENAME COLUMN capacity_kind TO inventory_kind;
ALTER TABLE plugin_idc_resource_provider_capacity_observation_item
    RENAME COLUMN observation_no TO snapshot_no;
ALTER TABLE plugin_idc_resource_provider_capacity_observation_item
    RENAME COLUMN capacity_kind TO inventory_kind;
ALTER TABLE plugin_idc_resource_provider_capacity_observation
    RENAME COLUMN observation_no TO snapshot_no;
ALTER TABLE plugin_idc_resource_provider_capacity_observation
    RENAME COLUMN capacity_kind TO inventory_kind;
ALTER TABLE plugin_idc_resource_provider_capacity_observation
    RENAME COLUMN observation_fingerprint TO snapshot_fingerprint;
ALTER TABLE plugin_idc_resource_provider_capacity_observation_item
    RENAME TO plugin_idc_resource_inventory_snapshot_item;
ALTER TABLE plugin_idc_resource_provider_capacity_observation
    RENAME TO plugin_idc_resource_inventory_snapshot;`); err != nil {
		t.Fatalf("stage legacy provider capacity tables failed: %v", err)
	}

	for _, wanted := range []string{
		"001-resource-foundation.sql",
		"003-resource-provider-capacity-observation.sql",
		"007-resource-provider-capacity-operations.sql",
		"017-resource-ip-reservation.sql",
		"024-provider-capacity-observation-rename.sql",
	} {
		var path string
		for _, candidate := range resourceMigrationPaths(t) {
			if filepath.Base(candidate) == wanted {
				path = candidate
				break
			}
		}
		if path == "" {
			t.Fatalf("provider capacity migration %s is missing", wanted)
		}
		migration, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("execute provider capacity migration %s failed: %v", wanted, err)
		}
	}
	for attempt := 1; attempt <= 2; attempt++ {
		migrationPath := ""
		for _, candidate := range resourceMigrationPaths(t) {
			if filepath.Base(candidate) == "024-provider-capacity-observation-rename.sql" {
				migrationPath = candidate
				break
			}
		}
		migration, err := os.ReadFile(migrationPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("repeat provider capacity rename attempt %d failed: %v", attempt, err)
		}
	}

	row, err := database.GetOne(ctx, `
SELECT observation.observation_no,item.external_ref,item.projection->>'name' AS name
FROM plugin_idc_resource_provider_capacity_observation observation
JOIN plugin_idc_resource_provider_capacity_observation_item item
  ON item.observation_no=observation.observation_no AND item.capacity_kind=observation.capacity_kind
WHERE observation.observation_no='capacity-migration-preserve'`)
	if err != nil || row["observation_no"].String() != "capacity-migration-preserve" ||
		row["external_ref"].String() != "line-preserved" || row["name"].String() != "preserved" {
		t.Fatalf("provider capacity migration did not preserve data: row=%v err=%v", row, err)
	}
	legacyTables, err := database.GetValue(ctx, `SELECT count(*) FROM pg_class relation
JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
WHERE namespace.nspname=current_schema()
  AND relation.relname IN ('plugin_idc_resource_inventory_snapshot','plugin_idc_resource_inventory_snapshot_item')`)
	if err != nil || legacyTables.Int() != 0 {
		t.Fatalf("retired provider capacity tables remain: count=%d err=%v", legacyTables.Int(), err)
	}
	capacityForeignKeys, err := database.GetValue(ctx, `
SELECT count(*)
FROM pg_constraint constraint_value
JOIN pg_class source_table ON source_table.oid=constraint_value.conrelid
JOIN pg_class target_table ON target_table.oid=constraint_value.confrelid
JOIN pg_namespace namespace ON namespace.oid=source_table.relnamespace
WHERE namespace.nspname=current_schema()
  AND constraint_value.contype='f'
  AND source_table.relname='plugin_idc_resource_ip_reservation'
  AND target_table.relname='plugin_idc_resource_provider_capacity_observation'`)
	if err != nil || capacityForeignKeys.Int() != 1 {
		t.Fatalf("provider capacity reservation foreign keys=%d want=1 err=%v", capacityForeignKeys.Int(), err)
	}
	legacyHealthColumns, err := database.GetValue(ctx, `
SELECT count(*) FROM information_schema.columns
WHERE table_schema=current_schema()
  AND table_name='plugin_idc_resource_channel_health_state'
  AND column_name LIKE 'last_inventory_%'`)
	if err != nil || legacyHealthColumns.Int() != 0 {
		t.Fatalf("legacy inventory health columns=%d want=0 err=%v", legacyHealthColumns.Int(), err)
	}
	providerLatency, err := database.GetValue(ctx, `
SELECT last_provider_capacity_latency_ms
FROM plugin_idc_resource_channel_health_state
WHERE channel_no='channel-test'`)
	if err != nil || providerLatency.Int() != 1234 {
		t.Fatalf("provider capacity health latency=%d want=1234 err=%v", providerLatency.Int(), err)
	}

	// Exercise the old-only path separately. PostgreSQL keeps the legacy foreign
	// key attached while the referenced table is renamed; the migration must
	// replace it instead of adding a duplicate target key.
	if _, err = database.Exec(ctx, `
ALTER TABLE plugin_idc_resource_ip_reservation
    RENAME CONSTRAINT fk_idc_resource_ip_reservation_observation
    TO fk_idc_resource_ip_reservation_snapshot;
ALTER TABLE plugin_idc_resource_provider_capacity_observation_item
    RENAME COLUMN observation_no TO snapshot_no;
ALTER TABLE plugin_idc_resource_provider_capacity_observation_item
    RENAME COLUMN capacity_kind TO inventory_kind;
ALTER TABLE plugin_idc_resource_provider_capacity_observation
    RENAME COLUMN observation_no TO snapshot_no;
ALTER TABLE plugin_idc_resource_provider_capacity_observation
    RENAME COLUMN capacity_kind TO inventory_kind;
ALTER TABLE plugin_idc_resource_provider_capacity_observation
    RENAME COLUMN observation_fingerprint TO snapshot_fingerprint;
ALTER TABLE plugin_idc_resource_provider_capacity_observation_item
    RENAME TO plugin_idc_resource_inventory_snapshot_item;
ALTER TABLE plugin_idc_resource_provider_capacity_observation
    RENAME TO plugin_idc_resource_inventory_snapshot;`); err != nil {
		t.Fatalf("stage old-only provider capacity migration failed: %v", err)
	}
	var renamePath string
	for _, candidate := range resourceMigrationPaths(t) {
		if filepath.Base(candidate) == "024-provider-capacity-observation-rename.sql" {
			renamePath = candidate
			break
		}
	}
	renameMigration, err := os.ReadFile(renamePath)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = database.Exec(ctx, string(renameMigration)); err != nil {
			t.Fatalf("execute old-only provider capacity rename attempt %d failed: %v", attempt, err)
		}
	}
	capacityForeignKeys, err = database.GetValue(ctx, `
SELECT count(*)
FROM pg_constraint constraint_value
JOIN pg_class source_table ON source_table.oid=constraint_value.conrelid
JOIN pg_class target_table ON target_table.oid=constraint_value.confrelid
JOIN pg_namespace namespace ON namespace.oid=source_table.relnamespace
WHERE namespace.nspname=current_schema()
  AND constraint_value.contype='f'
  AND source_table.relname='plugin_idc_resource_ip_reservation'
  AND target_table.relname='plugin_idc_resource_provider_capacity_observation'`)
	if err != nil || capacityForeignKeys.Int() != 1 {
		t.Fatalf("old-only capacity reservation foreign keys=%d want=1 err=%v", capacityForeignKeys.Int(), err)
	}
	if _, err = database.Exec(ctx, `
DELETE FROM plugin_idc_resource_provider_capacity_observation_item
WHERE observation_no='capacity-migration-preserve';
DELETE FROM plugin_idc_resource_provider_capacity_observation
WHERE observation_no='capacity-migration-preserve';`); err != nil {
		t.Fatalf("clean provider capacity migration fixture failed: %v", err)
	}
}

func newResourceIntegrationSchema(t *testing.T) string {
	t.Helper()
	var content [8]byte
	if _, err := rand.Read(content[:]); err != nil {
		t.Fatalf("generate resource integration schema failed: %v", err)
	}
	return "linapro_resource_test_" + hex.EncodeToString(content[:])
}

func appendResourceDSNParameter(dsn string, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

func quoteResourcePostgreSQLIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

const resourceCodegenCleanupSQL = `
DROP TABLE IF EXISTS plugin_idc_resource_channel_health_state CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_task_manual_takeover CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_provision_outcome_receipt CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_provision_credential CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_principal_task CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_principal_mapping CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_provider_capacity_observation_item CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_capability_snapshot_item CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_upstream_task_binding CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_task_evidence CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_task_secret_binding CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_health_snapshot CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_financial_snapshot CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_provider_capacity_observation CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_task CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_capability_snapshot CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_mapping CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_pool CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_channel_secret_binding CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_channel CASCADE;`
