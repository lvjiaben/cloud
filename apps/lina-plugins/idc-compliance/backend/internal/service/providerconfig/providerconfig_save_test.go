// This file verifies credential create, replay, replacement, rollback recovery, and authorization in PostgreSQL.
package providerconfig

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const providerConfigIntegrationDSNEnv = "LINAPRO_IDC_COMPLIANCE_TEST_DSN"

type providerConfigAuthorizer struct {
	requests []managementcap.AuthorizationRequest
	denied   bool
}

type providerConfigSessions struct {
	sessioncap.Service
	current *sessioncap.SessionInfo
	err     error
}

func (s *providerConfigSessions) Current(context.Context) (*sessioncap.SessionInfo, error) {
	return s.current, s.err
}

func newProviderConfigSessions(actorUserID string) *providerConfigSessions {
	loginAt := time.Now().UTC().Add(-30 * 24 * time.Hour)
	return &providerConfigSessions{current: &sessioncap.SessionInfo{
		ID: "provider-config-session", TenantID: "0", UserID: actorUserID, LoginAt: &loginAt,
	}}
}

func (a *providerConfigAuthorizer) Authorize(
	_ context.Context,
	request managementcap.AuthorizationRequest,
) (int, error) {
	a.requests = append(a.requests, request)
	if a.denied {
		return 0, errors.New("denied")
	}
	return 7001, nil
}

type providerConfigAudit struct {
	transitions []idccap.BusinessAuditTransition
	failScope   string
}

func (a *providerConfigAudit) AppendBusinessTransition(
	_ context.Context,
	transition idccap.BusinessAuditTransition,
) error {
	if transition.BeforeSnapshot == nil || transition.AfterSnapshot == nil {
		return errors.New("missing audit snapshots")
	}
	switch transition.ResultCode {
	case "succeeded", "denied", "failed", "result_unknown":
	default:
		return errors.New("invalid audit result")
	}

	a.transitions = append(a.transitions, transition)
	if transition.IdempotencyScope == a.failScope {
		return errors.New("synthetic audit failure")
	}
	return nil
}

func (*providerConfigAudit) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

type providerConfigSecret struct {
	metadata secretcap.Metadata
}

type providerConfigSecretManager struct {
	clock        time.Time
	sequence     int
	createCalls  int
	replaceCalls int
	byHandle     map[string]providerConfigSecret
	createKeys   map[string]string
	replaceKeys  map[string]secretcap.Metadata
}

type providerConfigConnectivityChecker struct {
	outcome ValidationStatus
	calls   int
	handle  secretcap.Handle
	sceneID string
}

func (c *providerConfigConnectivityChecker) Check(
	_ context.Context,
	handle secretcap.Handle,
	sceneID string,
) ValidationStatus {
	c.calls++
	c.handle = handle
	c.sceneID = sceneID
	if c.outcome == "" {
		return ValidationStatusReachable
	}
	return c.outcome
}

func newProviderConfigSecretManager() *providerConfigSecretManager {
	return &providerConfigSecretManager{
		clock:    time.Date(2026, 8, 12, 6, 0, 0, 0, time.UTC),
		byHandle: make(map[string]providerConfigSecret), createKeys: make(map[string]string),
		replaceKeys: make(map[string]secretcap.Metadata),
	}
}

func (m *providerConfigSecretManager) Create(
	_ context.Context,
	key string,
	produce secretcap.PlaintextProducer,
) (secretcap.CreateResult, error) {
	m.createCalls++
	if existing := m.createKeys[key]; existing != "" {
		handle, _ := secretcap.NewHandle(existing)
		return secretcap.CreateResult{
			Handle: handle, Metadata: m.byHandle[existing].metadata, Reused: true,
		}, nil
	}
	if err := consumeTestCredential(produce); err != nil {
		return secretcap.CreateResult{}, err
	}
	m.sequence++
	reference := "test-compliance-credential-" + string(rune('a'+m.sequence-1))
	handle, err := secretcap.NewHandle(reference)
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	m.clock = m.clock.Add(time.Second)
	metadata := secretcap.Metadata{Configured: true, MaskedTail: "0001", UpdatedAt: m.clock}
	m.byHandle[reference] = providerConfigSecret{metadata: metadata}
	m.createKeys[key] = reference
	return secretcap.CreateResult{Handle: handle, Metadata: metadata}, nil
}

func (m *providerConfigSecretManager) RecoverCreate(
	_ context.Context,
	key string,
) (secretcap.CreateResult, error) {
	reference := m.createKeys[key]
	if reference == "" {
		return secretcap.CreateResult{}, errors.New("secret create not found")
	}
	handle, err := secretcap.NewHandle(reference)
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	return secretcap.CreateResult{
		Handle: handle, Metadata: m.byHandle[reference].metadata, Reused: true,
	}, nil
}

func (m *providerConfigSecretManager) Replace(
	_ context.Context,
	handle secretcap.Handle,
	expected time.Time,
	key string,
	produce secretcap.PlaintextProducer,
) (secretcap.MutationResult, error) {
	m.replaceCalls++
	if replay, ok := m.replaceKeys[key]; ok {
		return secretcap.MutationResult{Metadata: replay, Reused: true}, nil
	}
	current, ok := m.byHandle[handle.OpaqueRef()]
	if !ok || !current.metadata.UpdatedAt.Equal(expected) {
		return secretcap.MutationResult{}, errors.New("secret optimistic fence mismatch")
	}
	if err := consumeTestCredential(produce); err != nil {
		return secretcap.MutationResult{}, err
	}
	m.clock = m.clock.Add(time.Second)
	current.metadata = secretcap.Metadata{
		Configured: true, MaskedTail: "0002", UpdatedAt: m.clock,
	}
	m.byHandle[handle.OpaqueRef()] = current
	m.replaceKeys[key] = current.metadata
	return secretcap.MutationResult{Metadata: current.metadata}, nil
}

func (m *providerConfigSecretManager) Metadata(
	_ context.Context,
	handle secretcap.Handle,
) (secretcap.Metadata, error) {
	value, ok := m.byHandle[handle.OpaqueRef()]
	if !ok {
		return secretcap.Metadata{}, errors.New("secret not found")
	}
	return value.metadata, nil
}

func (*providerConfigSecretManager) Revoke(
	context.Context,
	secretcap.Handle,
	time.Time,
) (secretcap.Metadata, error) {
	return secretcap.Metadata{}, nil
}

func (*providerConfigSecretManager) Rotate(
	context.Context,
	int,
) (secretcap.RotationResult, error) {
	return secretcap.RotationResult{}, nil
}

func consumeTestCredential(produce secretcap.PlaintextProducer) error {
	if produce == nil {
		return errors.New("missing producer")
	}
	calls := 0
	err := produce(func(plaintext []byte) error {
		calls++
		defer func() {
			for index := range plaintext {
				plaintext[index] = 0
			}
		}()
		if calls != 1 || len(plaintext) == 0 || !strings.Contains(string(plaintext), "accessKeyId") {
			return errors.New("invalid credential envelope")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if calls != 1 {
		return errors.New("credential producer call count is invalid")
	}
	return nil
}

func TestProviderConfigurationPostgreSQLCreateReplayAndReplace(t *testing.T) {
	ctx := context.Background()
	providerConfigTestDatabase(t, ctx)
	manager := newProviderConfigSecretManager()
	authorizer := &providerConfigAuthorizer{}
	audit := &providerConfigAudit{}
	service, err := New(
		manager,
		managementcap.Runtime{Authorizer: authorizer, Audit: audit},
		newProviderConfigSessions("7001"),
		&providerConfigConnectivityChecker{},
	)
	if err != nil {
		t.Fatal(err)
	}

	accessID := []byte("merchant-access-key-id")
	accessSecret := []byte("merchant-access-key-secret-0001")
	created, err := service.Save(ctx, SaveInput{
		SceneID: "100000001", AccessKeyID: accessID, AccessKeySecret: accessSecret,
		ExpectedVersion: 0, IdempotencyKey: "credential-create-0001",
	})
	if err != nil {
		t.Fatalf("create provider configuration failed: %v", err)
	}
	if created.Status != statusConfigured || !created.CredentialConfigured ||
		created.CredentialGeneration != 1 || created.Version != 2 ||
		created.CredentialMaskedTail != "0001" {
		t.Fatalf("created projection=%+v", created)
	}
	assertCleared(t, accessID)
	assertCleared(t, accessSecret)
	if manager.createCalls != 1 || len(audit.transitions) != 2 ||
		audit.transitions[0].RequiredPermission != PermissionWrite ||
		audit.transitions[1].IdempotencyScope != auditCompleteOperation {
		t.Fatalf("create calls=%d audit=%+v", manager.createCalls, audit.transitions)
	}

	replayed, err := service.Save(ctx, SaveInput{
		SceneID: "100000001", AccessKeyID: []byte("merchant-access-key-id"),
		AccessKeySecret: []byte("merchant-access-key-secret-0001"),
		ExpectedVersion: 0, IdempotencyKey: "credential-create-0001",
	})
	if err != nil || replayed.Version != created.Version || manager.createCalls != 1 || len(audit.transitions) != 2 {
		t.Fatalf("create replay=%+v calls=%d audit=%d err=%v", replayed, manager.createCalls, len(audit.transitions), err)
	}
	validated, err := service.ValidateConnectivity(ctx)
	if err != nil || validated.ValidationStatus != ValidationStatusReachable || validated.ValidatedAt == nil {
		t.Fatalf("validate before replacement=%+v err=%v", validated, err)
	}

	replaced, err := service.Save(ctx, SaveInput{
		SceneID: "100000002", AccessKeyID: []byte("merchant-access-key-id-2"),
		AccessKeySecret: []byte("merchant-access-key-secret-0002"),
		ExpectedVersion: created.Version, IdempotencyKey: "credential-replace-0001",
	})
	if err != nil {
		t.Fatalf("replace provider credential failed: %v", err)
	}
	if replaced.SceneID != "100000002" || replaced.CredentialGeneration != 2 ||
		replaced.Version != 3 || replaced.CredentialMaskedTail != "0002" || manager.replaceCalls != 1 ||
		replaced.ValidationStatus != ValidationStatusUntested || replaced.ValidatedAt != nil {
		t.Fatalf("replaced projection=%+v replaceCalls=%d", replaced, manager.replaceCalls)
	}
	if _, err = service.Save(ctx, SaveInput{
		SceneID: "100000003", AccessKeyID: []byte("merchant-access-key-id-3"),
		AccessKeySecret: []byte("merchant-access-key-secret-0003"),
		ExpectedVersion: created.Version, IdempotencyKey: "credential-stale-0001",
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("stale write error=%v want conflict", err)
	}
	queried, err := service.Get(ctx)
	if err != nil || queried.Version != replaced.Version ||
		authorizer.requests[len(authorizer.requests)-1].Permission != PermissionQuery {
		t.Fatalf("query=%+v auth=%+v err=%v", queried, authorizer.requests, err)
	}
}

func TestProviderConfigurationPostgreSQLValidatesConnectivityOutcomes(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		outcome    ValidationStatus
		resultCode string
	}{
		{name: "reachable", outcome: ValidationStatusReachable, resultCode: "succeeded"},
		{name: "rejected", outcome: ValidationStatusRejected, resultCode: "failed"},
		{name: "unavailable", outcome: ValidationStatusUnavailable, resultCode: "failed"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := context.Background()
			providerConfigTestDatabase(t, ctx)
			manager := newProviderConfigSecretManager()
			authorizer := &providerConfigAuthorizer{}
			audit := &providerConfigAudit{}
			checker := &providerConfigConnectivityChecker{outcome: testCase.outcome}
			service, err := New(
				manager,
				managementcap.Runtime{Authorizer: authorizer, Audit: audit},
				newProviderConfigSessions("7001"),
				checker,
			)
			if err != nil {
				t.Fatal(err)
			}
			created, err := service.Save(ctx, SaveInput{
				SceneID: "100000001", AccessKeyID: []byte("merchant-access-key-id"),
				AccessKeySecret: []byte("merchant-access-key-secret-0001"),
				IdempotencyKey:  "credential-validation-" + testCase.name,
			})
			if err != nil {
				t.Fatal(err)
			}
			validated, err := service.ValidateConnectivity(ctx)
			if err != nil || validated == nil || validated.ValidationStatus != testCase.outcome ||
				validated.ValidatedAt == nil || validated.Version != created.Version ||
				checker.calls != 1 || checker.sceneID != "100000001" || checker.handle.IsZero() {
				t.Fatalf("validated=%+v checker=%+v err=%v", validated, checker, err)
			}
			lastAuthorization := authorizer.requests[len(authorizer.requests)-1]
			lastAudit := audit.transitions[len(audit.transitions)-1]
			if lastAuthorization.Permission != PermissionValidate ||
				lastAudit.RequiredPermission != PermissionValidate ||
				lastAudit.IdempotencyScope != auditValidateOperation ||
				lastAudit.ResultCode != testCase.resultCode {
				t.Fatalf("authorization=%+v audit=%+v", lastAuthorization, lastAudit)
			}
		})
	}
}

func TestProviderConfigurationPostgreSQLRecoversFailedCompletionAudit(t *testing.T) {
	ctx := context.Background()
	providerConfigTestDatabase(t, ctx)
	manager := newProviderConfigSecretManager()
	audit := &providerConfigAudit{failScope: auditCompleteOperation}
	service, err := New(
		manager,
		managementcap.Runtime{Authorizer: &providerConfigAuthorizer{}, Audit: audit},
		newProviderConfigSessions("7001"),
		&providerConfigConnectivityChecker{},
	)
	if err != nil {
		t.Fatal(err)
	}
	input := SaveInput{
		SceneID: "100000001", AccessKeyID: []byte("merchant-access-key-id"),
		AccessKeySecret: []byte("merchant-access-key-secret-0001"),
		ExpectedVersion: 0, IdempotencyKey: "credential-recover-0001",
	}
	if _, err = service.Save(ctx, input); err == nil {
		t.Fatal("configuration committed despite failed completion audit")
	}
	row, err := service.Get(ctx)
	if err != nil || row.Status != statusUpdating || row.CredentialConfigured || row.Version != 1 {
		t.Fatalf("failed completion projection=%+v err=%v", row, err)
	}
	audit.failScope = ""
	recoveryID := []byte("merchant-access-key-id")
	recoverySecret := []byte("merchant-access-key-secret-0001")
	recovered, err := service.Recover(ctx, RecoveryInput{
		AccessKeyID: recoveryID, AccessKeySecret: recoverySecret,
	})
	if err != nil || recovered.Configuration.Status != statusConfigured ||
		recovered.Configuration.Version != 2 || !recovered.ReusedExistingMutation ||
		manager.createCalls != 2 || len(manager.createKeys) != 1 {
		t.Fatalf("recovered=%+v createCalls=%d handles=%d err=%v",
			recovered, manager.createCalls, len(manager.createKeys), err)
	}
	assertCleared(t, recoveryID)
	assertCleared(t, recoverySecret)
}

func TestProviderConfigurationPostgreSQLRecoversBeforeSecretMutation(t *testing.T) {
	ctx := context.Background()
	providerConfigTestDatabase(t, ctx)
	manager := newProviderConfigSecretManager()
	service, err := New(
		manager,
		managementcap.Runtime{
			Authorizer: &providerConfigAuthorizer{}, Audit: &providerConfigAudit{},
		},
		newProviderConfigSessions("7001"),
		&providerConfigConnectivityChecker{},
	)
	if err != nil {
		t.Fatal(err)
	}
	implementation := service.(*serviceImpl)
	input, digest, err := normalizeSave(SaveInput{
		SceneID: "100000001", ExpectedVersion: 0,
		IdempotencyKey: "credential-recover-before-secret-0001",
	})
	if err != nil {
		t.Fatal(err)
	}
	operationKey, err := newMutationOperationKey()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = implementation.prepareMutation(ctx, input, digest, operationKey); err != nil {
		t.Fatal(err)
	}
	accessID := []byte("merchant-access-key-id")
	accessSecret := []byte("merchant-access-key-secret-0001")
	recovered, err := service.Recover(ctx, RecoveryInput{
		AccessKeyID: accessID, AccessKeySecret: accessSecret,
	})
	if err != nil || recovered == nil || recovered.Configuration == nil ||
		recovered.Configuration.Status != statusConfigured ||
		recovered.Configuration.Version != 2 || recovered.ReusedExistingMutation ||
		manager.createCalls != 1 || len(manager.createKeys) != 1 {
		t.Fatalf("recovered=%+v createCalls=%d handles=%d err=%v",
			recovered, manager.createCalls, len(manager.createKeys), err)
	}
	assertCleared(t, accessID)
	assertCleared(t, accessSecret)
}

func TestProviderConfigurationPostgreSQLDeniesBeforeMutation(t *testing.T) {
	ctx := context.Background()
	database := providerConfigTestDatabase(t, ctx)
	manager := newProviderConfigSecretManager()
	authorizer := &providerConfigAuthorizer{denied: true}
	service, err := New(
		manager,
		managementcap.Runtime{Authorizer: authorizer, Audit: &providerConfigAudit{}},
		newProviderConfigSessions("7001"),
		&providerConfigConnectivityChecker{},
	)
	if err != nil {
		t.Fatal(err)
	}
	accessID := []byte("merchant-access-key-id")
	accessSecret := []byte("merchant-access-key-secret-0001")
	if _, err = service.Save(ctx, SaveInput{
		SceneID: "100000001", AccessKeyID: accessID,
		AccessKeySecret: accessSecret,
		IdempotencyKey:  "credential-denied-0001",
	}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("denied write error=%v", err)
	}
	assertCleared(t, accessID)
	assertCleared(t, accessSecret)
	count, err := database.GetValue(ctx, "SELECT count(*) FROM plugin_idc_compliance_provider_configuration")
	if err != nil || count.Int() != 0 || manager.createCalls != 0 {
		t.Fatalf("denied write rows=%d manager calls=%d err=%v", count.Int(), manager.createCalls, err)
	}
}

func TestProviderConfigurationRejectsMissingSessionBeforeMutation(t *testing.T) {
	manager := newProviderConfigSecretManager()
	sessions := newProviderConfigSessions("7001")
	sessions.current.LoginAt = nil
	service, err := New(
		manager,
		managementcap.Runtime{
			Authorizer: &providerConfigAuthorizer{}, Audit: &providerConfigAudit{},
		},
		sessions,
		&providerConfigConnectivityChecker{},
	)
	if err != nil {
		t.Fatal(err)
	}
	accessID := []byte("merchant-access-key-id")
	accessSecret := []byte("merchant-access-key-secret-0001")
	_, err = service.Save(context.Background(), SaveInput{
		SceneID: "100000001", AccessKeyID: accessID, AccessKeySecret: accessSecret,
		IdempotencyKey: "credential-stale-login-0001",
	})
	if !bizerr.Is(err, CodeRecentAuthenticationRequired) || manager.createCalls != 0 {
		t.Fatalf("stale login error=%v create calls=%d", err, manager.createCalls)
	}
	assertCleared(t, accessID)
	assertCleared(t, accessSecret)
}

func providerConfigTestDatabase(t *testing.T, ctx context.Context) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(providerConfigIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run provider configuration PostgreSQL tests", providerConfigIntegrationDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := "idc_provider_config_test_" + randomTestHex(t, 8)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	originalConfig := gdb.GetAllConfig()
	var database gdb.DB
	t.Cleanup(func() {
		if database != nil {
			if closeErr := database.Close(ctx); closeErr != nil {
				t.Errorf("close provider configuration database: %v", closeErr)
			}
		}
		if restoreErr := gdb.SetConfig(originalConfig); restoreErr != nil {
			t.Errorf("restore provider configuration database config: %v", restoreErr)
		}
		if _, dropErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Errorf("drop provider configuration schema: %v", dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close provider configuration admin database: %v", closeErr)
		}
	})
	if err = gdb.SetConfig(gdb.Config{
		gdb.DefaultGroupName: gdb.ConfigGroup{{
			Link: appendTestDSNParameter(dsn, "search_path="+schema),
		}},
	}); err != nil {
		t.Fatal(err)
	}
	database = g.DB()
	createProviderConfigDictionaryFixtures(t, ctx, database)
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "manifest", "sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"001-compliance-foundation.sql", "002-aliyun-personal-verification.sql"} {
		content, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, readErr = database.Exec(ctx, string(content)); readErr != nil {
			t.Fatalf("execute %s: %v", name, readErr)
		}
	}
	return database
}

func createProviderConfigDictionaryFixtures(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
CREATE TABLE sys_dict_type (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    name VARCHAR(128) NOT NULL DEFAULT '',
    type VARCHAR(128) NOT NULL,
    status SMALLINT NOT NULL DEFAULT 1,
    is_builtin SMALLINT NOT NULL DEFAULT 0,
    allow_tenant_override BOOLEAN NOT NULL DEFAULT FALSE,
    remark VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_provider_config_test_dict_type UNIQUE (tenant_id, type)
);
CREATE TABLE sys_dict_data (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    dict_type VARCHAR(128) NOT NULL,
    label VARCHAR(128) NOT NULL DEFAULT '',
    value VARCHAR(128) NOT NULL,
    sort INT NOT NULL DEFAULT 0,
    tag_style VARCHAR(32) NOT NULL DEFAULT 'default',
    status SMALLINT NOT NULL DEFAULT 1,
    is_builtin SMALLINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_provider_config_test_dict_data UNIQUE (tenant_id, dict_type, value)
)`); err != nil {
		t.Fatalf("create provider configuration dictionary fixtures: %v", err)
	}
}

func randomTestHex(t *testing.T, bytes int) string {
	t.Helper()
	value := make([]byte, bytes)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(value)
}

func appendTestDSNParameter(dsn string, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

func assertCleared(t *testing.T, value []byte) {
	t.Helper()
	for _, character := range value {
		if character != 0 {
			t.Fatal("credential input was not cleared")
		}
	}
}
