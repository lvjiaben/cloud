package channelsecret

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const channelSecretTestDSNEnv = "LINAPRO_IDC_RESOURCE_TEST_DSN"

type testAuthorizer struct{ deny bool }

func (a *testAuthorizer) Authorize(
	_ context.Context,
	request managementcap.AuthorizationRequest,
) (int, error) {
	if a.deny || request.Permission != PermissionManage || request.TargetNo == "" {
		return 0, errors.New("denied")
	}
	return 42, nil
}

type testSensitiveAccess struct {
	mu       sync.Mutex
	next     int
	bindings map[string]managementcap.SensitiveAccessBinding
}

func (s *testSensitiveAccess) IssueSensitiveAccess(
	_ context.Context,
	binding managementcap.SensitiveAccessBinding,
) (*managementcap.SensitiveAccessContext, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bindings == nil {
		s.bindings = make(map[string]managementcap.SensitiveAccessBinding)
	}
	s.next++
	token := fmt.Sprintf("proof-%d", s.next)
	s.bindings[token] = binding
	return &managementcap.SensitiveAccessContext{Token: token, ExpiresAt: time.Now().Add(time.Minute)}, nil
}

func (s *testSensitiveAccess) ConsumeSensitiveAccess(
	_ context.Context,
	token string,
	binding managementcap.SensitiveAccessBinding,
) (*managementcap.SensitiveAccessProof, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	expected, exists := s.bindings[token]
	delete(s.bindings, token)
	if !exists || expected != binding {
		return nil, errors.New("invalid proof")
	}
	return &managementcap.SensitiveAccessProof{ActorUserID: 42, ExpiresAt: time.Now().Add(time.Minute)}, nil
}

type testAudit struct {
	mu           sync.Mutex
	events       []idccap.BusinessAuditTransition
	failComplete bool
	failRotation bool
}

func (a *testAudit) AppendBusinessTransition(
	_ context.Context,
	event idccap.BusinessAuditTransition,
) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.failComplete && strings.HasSuffix(event.OperationCode, ".create.complete") {
		a.failComplete = false
		return errors.New("audit unavailable")
	}
	if a.failRotation && strings.HasSuffix(event.OperationCode, ".rotate.complete") {
		a.failRotation = false
		return errors.New("rotation audit unavailable")
	}
	a.events = append(a.events, event)
	return nil
}

func (*testAudit) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

type testSecret struct {
	handle    secretcap.Handle
	value     []byte
	metadata  secretcap.Metadata
	revoked   bool
	createKey string
	replace   map[string]secretcap.MutationResult
}

type testManager struct {
	mu         sync.Mutex
	secret     testSecret
	creates    int
	rotates    int
	rotationDB gdb.DB
}

func (m *testManager) Create(
	_ context.Context,
	key string,
	produce secretcap.PlaintextProducer,
) (secretcap.CreateResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.secret.createKey == key {
		return secretcap.CreateResult{Handle: m.secret.handle, Metadata: m.secret.metadata, Reused: true}, nil
	}
	var value []byte
	if err := produce(func(input []byte) error {
		value = append([]byte(nil), input...)
		return nil
	}); err != nil {
		return secretcap.CreateResult{}, err
	}
	handle, err := secretcap.NewHandle(fmt.Sprintf("sec_channel_management_test_%d", m.creates+1))
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	m.creates++
	m.secret = testSecret{
		handle: handle, value: value, createKey: key, replace: make(map[string]secretcap.MutationResult),
		metadata: secretcap.Metadata{Configured: true, MaskedTail: "tail", UpdatedAt: time.Now().UTC()},
	}
	return secretcap.CreateResult{Handle: handle, Metadata: m.secret.metadata}, nil
}

func (m *testManager) RecoverCreate(_ context.Context, key string) (secretcap.CreateResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.secret.createKey != key {
		return secretcap.CreateResult{}, errors.New("missing")
	}
	return secretcap.CreateResult{Handle: m.secret.handle, Metadata: m.secret.metadata, Reused: true}, nil
}

func (m *testManager) Replace(
	_ context.Context,
	handle secretcap.Handle,
	expected time.Time,
	key string,
	produce secretcap.PlaintextProducer,
) (secretcap.MutationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if result, exists := m.secret.replace[key]; exists {
		result.Reused = true
		return result, nil
	}
	if !m.secret.handle.Equal(handle) || !m.secret.metadata.UpdatedAt.Equal(expected) || m.secret.revoked {
		return secretcap.MutationResult{}, errors.New("conflict")
	}
	var value []byte
	if err := produce(func(input []byte) error {
		value = append([]byte(nil), input...)
		return nil
	}); err != nil {
		return secretcap.MutationResult{}, err
	}
	m.secret.value = value
	m.secret.metadata.UpdatedAt = m.secret.metadata.UpdatedAt.Add(time.Second)
	result := secretcap.MutationResult{Metadata: m.secret.metadata}
	m.secret.replace[key] = result
	return result, nil
}

func (m *testManager) Metadata(
	_ context.Context,
	handle secretcap.Handle,
) (secretcap.Metadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.secret.handle.Equal(handle) {
		return secretcap.Metadata{}, errors.New("missing")
	}
	metadata := m.secret.metadata
	metadata.Configured = !m.secret.revoked
	return metadata, nil
}

func (m *testManager) Revoke(
	_ context.Context,
	handle secretcap.Handle,
	expected time.Time,
) (secretcap.Metadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.secret.handle.Equal(handle) || !m.secret.metadata.UpdatedAt.Equal(expected) {
		return secretcap.Metadata{}, errors.New("conflict")
	}
	m.secret.revoked = true
	m.secret.metadata.Configured = false
	m.secret.metadata.UpdatedAt = m.secret.metadata.UpdatedAt.Add(time.Second)
	return m.secret.metadata, nil
}

func (m *testManager) Rotate(ctx context.Context, limit int) (secretcap.RotationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rotates++
	if m.rotationDB != nil {
		if _, err := m.rotationDB.Exec(ctx, `INSERT INTO rotation_test_evidence VALUES (1)`); err != nil {
			return secretcap.RotationResult{}, err
		}
	}
	return secretcap.RotationResult{Rewrapped: min(limit, 1)}, nil
}

func TestChannelSecretLifecycleAndRecoveryPostgreSQL(t *testing.T) {
	ctx, database := channelSecretDatabase(t)
	channelNo := fmt.Sprintf("channel_secret_%d", time.Now().UnixNano())
	_, err := database.Exec(ctx, `INSERT INTO plugin_idc_resource_channel
    (tenant_id,channel_no,provider_id,provider_plugin_id,display_name,endpoint,
     non_secret_settings,status_code,version,created_by,updated_by)
VALUES (0,?,'provider-test','plugin-test','Secret test','https://example.com','{}','enabled',1,42,42)`, channelNo)
	if err != nil {
		t.Fatalf("insert channel: %v", err)
	}
	interfaceNo := "pfi_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	initialRevision := "rci_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_resource_interface_channel_binding
    (interface_instance_no,channel_no,binding_version,current_configuration_revision)
VALUES (?,?,1,?)`, interfaceNo, channelNo, initialRevision); err != nil {
		t.Fatalf("insert channel interface binding: %v", err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_resource_interface_configuration_revision
    (revision_no,interface_instance_no,channel_no,endpoint,non_secret_settings,secret_bindings,created_by)
VALUES (?,?,?,'https://example.com','{}'::jsonb,'{}'::jsonb,42)`, initialRevision, interfaceNo, channelNo); err != nil {
		t.Fatalf("insert initial channel revision: %v", err)
	}
	t.Cleanup(func() {
		_, _ = database.Exec(ctx, `DELETE FROM plugin_idc_resource_channel_secret_binding WHERE channel_no = ?`, channelNo)
		_, _ = database.Exec(ctx, `DELETE FROM plugin_idc_resource_channel WHERE channel_no = ?`, channelNo)
	})

	manager := &testManager{}
	audit := &testAudit{failComplete: true}
	proofs := &testSensitiveAccess{}
	service, err := New(database, managementcap.Runtime{
		Authorizer: &testAuthorizer{}, Audit: audit, SensitiveAccess: proofs,
	}, func(string) (secretcap.Manager, error) { return manager, nil })
	if err != nil {
		t.Fatal(err)
	}

	secret := []byte("channel-password-do-not-leak")
	create := MutationInput{ChannelNo: channelNo, BindingName: "password", Secret: append([]byte(nil), secret...), IdempotencyKey: "create-secret-0001"}
	create.AccessContext = issueTestAccess(t, service, proofs, AccessRequest{
		Operation: OperationCreate, ChannelNo: channelNo, BindingName: "password",
		SecretDigest: secretDigest(secret), IdempotencyKey: create.IdempotencyKey,
	})
	if _, err = service.Create(ctx, create); err == nil {
		t.Fatal("audit failure did not fail closed")
	}

	create.Secret = append([]byte(nil), secret...)
	create.AccessContext = issueTestAccess(t, service, proofs, AccessRequest{
		Operation: OperationCreate, ChannelNo: channelNo, BindingName: "password",
		SecretDigest: secretDigest(secret), IdempotencyKey: create.IdempotencyKey,
	})
	created, err := service.Create(ctx, create)
	if err != nil || created == nil || !created.Configured || manager.creates != 1 {
		t.Fatalf("recover create: status=%+v creates=%d err=%v", created, manager.creates, err)
	}
	createdRevision, createdBindings := readCurrentRevision(t, ctx, database, channelNo)
	if createdRevision == initialRevision || !strings.Contains(createdBindings, "sec_channel_management_test_1") {
		t.Fatalf("create did not append an immutable revision: revision=%q bindings=%s", createdRevision, createdBindings)
	}

	replacement := []byte("replacement-password-do-not-leak")
	replace := MutationInput{
		ChannelNo: channelNo, BindingName: "password", Secret: append([]byte(nil), replacement...),
		ExpectedUpdatedAt: created.UpdatedAt, IdempotencyKey: "replace-secret-0001",
	}
	replace.AccessContext = issueTestAccess(t, service, proofs, AccessRequest{
		Operation: OperationReplace, ChannelNo: channelNo, BindingName: "password",
		SecretDigest: secretDigest(replacement), ExpectedUpdatedAt: created.UpdatedAt,
		IdempotencyKey: replace.IdempotencyKey,
	})
	replaced, err := service.Replace(ctx, replace)
	if err != nil || replaced == nil || !replaced.UpdatedAt.After(created.UpdatedAt) {
		t.Fatalf("replace: status=%+v err=%v", replaced, err)
	}
	replacedRevision, replacedBindings := readCurrentRevision(t, ctx, database, channelNo)
	if replacedRevision == createdRevision || !strings.Contains(replacedBindings, "sec_channel_management_test_2") ||
		strings.Contains(replacedBindings, "sec_channel_management_test_1") {
		t.Fatalf("replace did not switch to a new immutable secret revision: revision=%q bindings=%s", replacedRevision, replacedBindings)
	}
	createdSnapshot, snapshotErr := database.GetValue(ctx, `SELECT secret_bindings::TEXT
FROM plugin_idc_resource_interface_configuration_revision WHERE revision_no=?`, createdRevision)
	if snapshotErr != nil || !strings.Contains(createdSnapshot.String(), "sec_channel_management_test_1") {
		t.Fatalf("replace mutated the historical revision: bindings=%s err=%v", createdSnapshot.String(), snapshotErr)
	}

	revoke := MutationInput{
		ChannelNo: channelNo, BindingName: "password", ExpectedUpdatedAt: replaced.UpdatedAt,
		IdempotencyKey: "revoke-secret-0001",
	}
	revoke.AccessContext = issueTestAccess(t, service, proofs, AccessRequest{
		Operation: OperationRevoke, ChannelNo: channelNo, BindingName: "password",
		ExpectedUpdatedAt: replaced.UpdatedAt, IdempotencyKey: revoke.IdempotencyKey,
	})
	revoked, err := service.Revoke(ctx, revoke)
	if err != nil || revoked == nil || revoked.Configured {
		t.Fatalf("revoke: status=%+v err=%v", revoked, err)
	}
	revokedRevision, revokedBindings := readCurrentRevision(t, ctx, database, channelNo)
	if revokedRevision == replacedRevision || revokedBindings != "{}" {
		t.Fatalf("revoke did not append a secret-free revision: revision=%q bindings=%s", revokedRevision, revokedBindings)
	}
	replacedSnapshot, snapshotErr := database.GetValue(ctx, `SELECT secret_bindings::TEXT
FROM plugin_idc_resource_interface_configuration_revision WHERE revision_no=?`, replacedRevision)
	if snapshotErr != nil || !strings.Contains(replacedSnapshot.String(), "sec_channel_management_test_2") {
		t.Fatalf("revoke mutated the replaced historical revision: bindings=%s err=%v", replacedSnapshot.String(), snapshotErr)
	}

	rotate := MutationInput{ChannelNo: channelNo, RotateLimit: 10, IdempotencyKey: "rotate-secret-0001"}
	rotate.AccessContext = issueTestAccess(t, service, proofs, AccessRequest{
		Operation: OperationRotate, ChannelNo: channelNo, RotateLimit: 10,
		IdempotencyKey: rotate.IdempotencyKey,
	})
	if _, err = database.Exec(ctx, `CREATE TABLE rotation_test_evidence(id INT)`); err != nil {
		t.Fatal(err)
	}
	manager.rotationDB = database
	audit.failRotation = true
	if _, err = service.Rotate(ctx, rotate); err == nil {
		t.Fatal("rotation audit failure did not fail closed")
	}
	evidence, err := database.GetValue(ctx, `SELECT COUNT(*) FROM rotation_test_evidence`)
	if err != nil || evidence.Int() != 0 {
		t.Fatalf("rotation owner writes escaped audit rollback: %v %v", evidence, err)
	}
	rotate.AccessContext = issueTestAccess(t, service, proofs, AccessRequest{
		Operation: OperationRotate, ChannelNo: channelNo, RotateLimit: 10, IdempotencyKey: rotate.IdempotencyKey,
	})
	rotated, err := service.Rotate(ctx, rotate)
	if err != nil || rotated.Rewrapped != 1 {
		t.Fatalf("rotate: result=%+v err=%v", rotated, err)
	}

	var counts struct {
		Active     int `orm:"active"`
		Superseded int `orm:"superseded"`
		Revoked    int `orm:"revoked"`
		Pending    int `orm:"pending"`
	}
	if err = database.GetScan(ctx, &counts, `SELECT
  COUNT(*) FILTER (WHERE status_code='active')::INT AS active,
  COUNT(*) FILTER (WHERE status_code='superseded')::INT AS superseded,
  COUNT(*) FILTER (WHERE status_code='revoked')::INT AS revoked,
  COUNT(*) FILTER (WHERE status_code IN ('preparing','retryable_failed'))::INT AS pending
FROM plugin_idc_resource_channel_secret_binding WHERE channel_no=?`, channelNo); err != nil ||
		counts.Active != 0 || counts.Superseded != 1 || counts.Revoked != 1 || counts.Pending != 0 {
		t.Fatalf("binding lifecycle=%+v err=%v", counts, err)
	}
	persistedRows, err := database.GetValue(ctx, `SELECT COALESCE(string_agg(row_to_json(binding)::text, ''), '')
FROM "plugin_idc_resource_channel_secret_binding" AS binding
WHERE "channel_no"=?`, channelNo)
	if err != nil {
		t.Fatalf("scan persisted channel-secret rows: %v", err)
	}
	persistedText := persistedRows.String()
	if strings.Contains(persistedText, string(secret)) || strings.Contains(persistedText, string(replacement)) {
		t.Fatal("secret plaintext leaked into the channel-secret database rows")
	}

	audit.mu.Lock()
	for _, operation := range []Operation{OperationCreate, OperationReplace, OperationRevoke, OperationRotate} {
		found := false
		for _, event := range audit.events {
			if event.OperationCode == "resource.channel-secret."+string(operation)+".complete" {
				found = true
			}
		}
		if !found {
			t.Errorf("missing secret-free %s audit", operation)
		}
	}
	auditText := fmt.Sprintf("%+v", audit.events)
	audit.mu.Unlock()
	if strings.Contains(auditText, string(secret)) || strings.Contains(auditText, string(replacement)) ||
		strings.Contains(auditText, "sec_channel_management_test_") {
		t.Fatal("secret material or opaque reference leaked into audit")
	}
	manager.mu.Lock()
	if !bytes.Equal(manager.secret.value, replacement) {
		t.Fatal("replacement plaintext was not delivered exactly once")
	}
	manager.mu.Unlock()
}

func readCurrentRevision(t *testing.T, ctx context.Context, database gdb.DB, channelNo string) (string, string) {
	t.Helper()
	var current struct {
		RevisionNo     string `orm:"revision_no"`
		SecretBindings string `orm:"secret_bindings"`
	}
	if err := database.GetScan(ctx, &current, `SELECT binding.current_configuration_revision AS revision_no,
       revision.secret_bindings::TEXT AS secret_bindings
FROM plugin_idc_resource_interface_channel_binding binding
JOIN plugin_idc_resource_interface_configuration_revision revision
  ON revision.revision_no=binding.current_configuration_revision
WHERE binding.channel_no=?`, channelNo); err != nil || current.RevisionNo == "" {
		t.Fatalf("read current immutable channel revision: %+v err=%v", current, err)
	}
	return current.RevisionNo, current.SecretBindings
}

func issueTestAccess(
	t *testing.T,
	service Service,
	_ *testSensitiveAccess,
	request AccessRequest,
) string {
	t.Helper()
	issued, err := service.IssueAccess(context.Background(), request)
	if err != nil {
		t.Fatalf("issue access: %v", err)
	}
	return issued.Token
}

func channelSecretDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(channelSecretTestDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run channel-secret PostgreSQL tests", channelSecretTestDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Type: "pgsql", Link: dsn})
	if err != nil {
		t.Fatalf("open PostgreSQL admin connection: %v", err)
	}
	schema := fmt.Sprintf("linapro_channel_secret_test_%d", time.Now().UnixNano())
	quotedSchema := `"` + strings.ReplaceAll(schema, `"`, `""`) + `"`
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		_ = admin.Close(ctx)
		t.Fatalf("create isolated channel-secret schema: %v", err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{
		Type: "pgsql",
		Link: dsn + separator + "search_path=" + schema,
	})
	if err != nil {
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+quotedSchema+" CASCADE")
		_ = admin.Close(ctx)
		t.Fatalf("open isolated PostgreSQL connection: %v", err)
	}
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve channel-secret test file")
	}
	pluginRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "..", ".."))
	paths, globErr := filepath.Glob(filepath.Join(pluginRoot, "manifest", "sql", "[0-9][0-9][0-9]-*.sql"))
	if globErr != nil || len(paths) == 0 {
		t.Fatalf("resolve channel-secret migrations: %v", globErr)
	}
	for _, path := range paths {
		name := filepath.Base(path)
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read channel-secret migration %s: %v", name, readErr)
		}
		if _, execErr := database.Exec(ctx, string(body)); execErr != nil {
			t.Fatalf("apply channel-secret migration %s: %v", name, execErr)
		}
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close isolated channel-secret database: %v", closeErr)
		}
		if !strings.HasPrefix(schema, "linapro_channel_secret_test_") {
			t.Errorf("refusing to drop unexpected schema %q", schema)
		} else if _, dropErr := admin.Exec(ctx, "DROP SCHEMA "+quotedSchema+" CASCADE"); dropErr != nil {
			t.Errorf("drop isolated channel-secret schema: %v", dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close channel-secret admin database: %v", closeErr)
		}
	})
	return ctx, database
}
