// This file verifies the personal verification state machine against isolated real PostgreSQL schemas.

package personalverification

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
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
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/provider/aliyunface"
	"lina-plugin-idc-compliance/backend/internal/service/providerconfig"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const personalVerificationIntegrationDSNEnv = "LINAPRO_IDC_COMPLIANCE_TEST_DSN"

type fakeAuditWriter struct {
	transitions []idccap.BusinessAuditTransition
	err         error
	transaction bool
}

func (f *fakeAuditWriter) AppendBusinessTransition(
	ctx context.Context,
	input idccap.BusinessAuditTransition,
) error {
	f.transitions = append(f.transitions, input)
	f.transaction = gdb.TXFromCtx(ctx, dao.VerificationCase.Group()) != nil
	return f.err
}

func (*fakeAuditWriter) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

type identityProjectionCall struct {
	status identitycap.Status
	name   string
	number string
}

type fakeIdentityService struct {
	subject   identitycap.PersonalSubject
	calls     []identityProjectionCall
	duplicate bool
}

func (f *fakeIdentityService) CurrentPersonal(context.Context) (*identitycap.PersonalSubject, error) {
	copyValue := f.subject
	return &copyValue, nil
}

func (*fakeIdentityService) RecordPersonalConsent(
	context.Context,
	identitycap.PersonalConsentInput,
) (*identitycap.PersonalConsent, error) {
	return nil, errors.New("personal consent recording is not used by this state-machine test")
}

func (f *fakeIdentityService) PreparePersonal(
	_ context.Context,
	consentNo string,
) (*identitycap.PersonalSubject, error) {
	if consentNo != "consent_personal_0001" {
		return nil, errors.New("consent denied")
	}
	copyValue := f.subject
	return &copyValue, nil
}

func (f *fakeIdentityService) ProjectPersonal(
	_ context.Context,
	input identitycap.PersonalProjectionInput,
) (*identitycap.PersonalProjection, error) {
	defer clearBytes(input.LegalName)
	defer clearBytes(input.IdentityNumber)
	f.calls = append(f.calls, identityProjectionCall{
		status: input.Status, name: string(input.LegalName), number: string(input.IdentityNumber),
	})
	if input.Status == identitycap.StatusVerified && f.duplicate {
		return &identitycap.PersonalProjection{
			Status: identitycap.StatusPending, Version: 2, DuplicateRisk: true,
		}, nil
	}
	digest := ""
	if input.Status == identitycap.StatusVerified {
		value := sha256.Sum256(append([]byte("test-keyed:"), input.IdentityNumber...))
		digest = hex.EncodeToString(value[:])
	}
	return &identitycap.PersonalProjection{
		Status: input.Status, IdentityLookupDigest: digest, Version: int64(len(f.calls) + 1),
	}, nil
}

func (*fakeIdentityService) CurrentEnterprise(context.Context) (*identitycap.EnterpriseSubject, error) {
	return nil, errors.New("enterprise identity is not used by personal verification tests")
}

func (*fakeIdentityService) RecordEnterpriseConsent(
	context.Context,
	identitycap.EnterpriseConsentInput,
) (*identitycap.EnterpriseConsent, error) {
	return nil, errors.New("enterprise consent is not used by personal verification tests")
}

func (*fakeIdentityService) PrepareEnterprise(
	context.Context,
	string,
	string,
) (*identitycap.EnterprisePreparation, error) {
	return nil, errors.New("enterprise preparation is not used by personal verification tests")
}

func (*fakeIdentityService) ProjectEnterprise(
	context.Context,
	identitycap.EnterpriseProjectionInput,
) (*identitycap.EnterpriseProjection, error) {
	return nil, errors.New("enterprise projection is not used by personal verification tests")
}

type fakeRuntimeSource struct {
	configuration providerconfig.RuntimeConfiguration
}

func (f *fakeRuntimeSource) Resolve(context.Context) (providerconfig.RuntimeConfiguration, error) {
	return f.configuration, nil
}

type fakeClientFactory struct{ client *fakeAliyunClient }

func (f *fakeClientFactory) Create(providerconfig.RuntimeConfiguration) (aliyunface.Client, error) {
	return f.client, nil
}

type fakeAliyunClient struct {
	initiation aliyunface.Initiation
	initErr    error
	queries    []aliyunface.QueryResult
	initCalls  int
	queryCalls int
}

func (f *fakeAliyunClient) Initiate(
	_ context.Context,
	request aliyunface.InitiateRequest,
) (aliyunface.Initiation, error) {
	f.initCalls++
	if request.CertName == "" || request.CertNo == "" {
		return aliyunface.Initiation{}, errors.New("identity not supplied")
	}
	return f.initiation, f.initErr
}

func (f *fakeAliyunClient) Query(
	context.Context,
	aliyunface.QueryRequest,
) (aliyunface.QueryResult, error) {
	f.queryCalls++
	if len(f.queries) == 0 {
		return aliyunface.QueryResult{State: aliyunface.QueryStateNotPassed}, nil
	}
	result := f.queries[0]
	f.queries = f.queries[1:]
	return result, nil
}

func (*fakeAliyunClient) Probe(context.Context, string) aliyunface.ProbeOutcome {
	return aliyunface.ProbeOutcomeUnavailable
}

type fakeSnapshotSecrets struct {
	values     map[string][]byte
	metadata   map[string]secretcap.Metadata
	createKeys map[string]string
	sequence   int
}

func newFakeSnapshotSecrets() *fakeSnapshotSecrets {
	return &fakeSnapshotSecrets{
		values: map[string][]byte{}, metadata: map[string]secretcap.Metadata{},
		createKeys: map[string]string{},
	}
}

func (f *fakeSnapshotSecrets) Create(
	_ context.Context,
	key string,
	producer secretcap.PlaintextProducer,
) (secretcap.CreateResult, error) {
	if reference := f.createKeys[key]; reference != "" {
		handle, err := secretcap.NewHandle(reference)
		if err != nil {
			return secretcap.CreateResult{}, err
		}
		return secretcap.CreateResult{Handle: handle, Metadata: f.metadata[reference], Reused: true}, nil
	}
	f.sequence++
	reference := "test-personal-snapshot-" + string(rune('a'+f.sequence-1))
	handle, handleErr := secretcap.NewHandle(reference)
	if handleErr != nil {
		return secretcap.CreateResult{}, handleErr
	}
	err := producer(func(payload []byte) error {
		f.values[reference] = append([]byte(nil), payload...)
		return nil
	})
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	metadata := secretcap.Metadata{
		Configured: true, UpdatedAt: time.Date(2026, 8, 12, 8, 0, f.sequence, 0, time.UTC),
	}
	f.metadata[reference] = metadata
	f.createKeys[key] = reference
	return secretcap.CreateResult{Handle: handle, Metadata: metadata}, nil
}

func (f *fakeSnapshotSecrets) RecoverCreate(
	_ context.Context,
	key string,
) (secretcap.CreateResult, error) {
	reference := f.createKeys[key]
	if reference == "" {
		return secretcap.CreateResult{}, errors.New("secret create not found")
	}
	handle, err := secretcap.NewHandle(reference)
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	return secretcap.CreateResult{Handle: handle, Metadata: f.metadata[reference], Reused: true}, nil
}

func (*fakeSnapshotSecrets) Replace(
	context.Context,
	secretcap.Handle,
	time.Time,
	string,
	secretcap.PlaintextProducer,
) (secretcap.MutationResult, error) {
	return secretcap.MutationResult{}, errors.New("replace is unsupported")
}

func (f *fakeSnapshotSecrets) Metadata(
	_ context.Context,
	handle secretcap.Handle,
) (secretcap.Metadata, error) {
	metadata, found := f.metadata[handle.OpaqueRef()]
	if !found {
		return secretcap.Metadata{}, errors.New("secret missing")
	}
	return metadata, nil
}

func (f *fakeSnapshotSecrets) Revoke(
	_ context.Context,
	handle secretcap.Handle,
	_ time.Time,
) (secretcap.Metadata, error) {
	delete(f.values, handle.OpaqueRef())
	delete(f.metadata, handle.OpaqueRef())
	return secretcap.Metadata{}, nil
}

func (*fakeSnapshotSecrets) Rotate(context.Context, int) (secretcap.RotationResult, error) {
	return secretcap.RotationResult{}, nil
}

func (f *fakeSnapshotSecrets) Access(
	_ context.Context,
	request secretcap.AccessRequest,
	consume secretcap.Consumer,
) error {
	payload, found := f.values[request.Handle.OpaqueRef()]
	if !found {
		return errors.New("secret missing")
	}
	detached := append([]byte(nil), payload...)
	defer clearBytes(detached)
	return consume(detached)
}

type verificationHarness struct {
	service  Service
	impl     *serviceImpl
	identity *fakeIdentityService
	runtime  *fakeRuntimeSource
	client   *fakeAliyunClient
	secrets  *fakeSnapshotSecrets
	audit    *fakeAuditWriter
	now      time.Time
	database gdb.DB
}

func newVerificationHarness(t *testing.T, ctx context.Context) *verificationHarness {
	t.Helper()
	database := personalVerificationTestDatabase(t, ctx)
	credential, err := secretcap.NewHandle("test-provider-credential")
	if err != nil {
		t.Fatal(err)
	}
	runtime := &fakeRuntimeSource{configuration: providerconfig.RuntimeConfiguration{
		ConfigurationNo: "compliance-provider-aliyun-face", SceneID: "100000001",
		Version: 2, CredentialGeneration: 1, Credential: credential,
	}}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_provider_configuration (
    tenant_id, configuration_no, provider_code, status_code, scene_id,
    credential_secret_ref, credential_generation, version
) VALUES (0, ?, 'aliyun_face', 'configured', ?, ?, 1, 2)`,
		runtime.configuration.ConfigurationNo, runtime.configuration.SceneID,
		credential.OpaqueRef()); err != nil {
		t.Fatal(err)
	}
	identity := &fakeIdentityService{subject: identitycap.PersonalSubject{
		CustomerNo: "cust_personal_0001", PrincipalReference: "usr:7001",
	}}
	client := &fakeAliyunClient{initiation: aliyunface.Initiation{
		CertifyID: "certify-test-0001", CertifyURL: "https://provider.example.test/certify/one",
	}}
	secrets := newFakeSnapshotSecrets()
	audit := &fakeAuditWriter{}
	service, err := New(
		database, identity, runtime, &fakeClientFactory{client: client}, secrets, secrets, audit,
	)
	if err != nil {
		t.Fatal(err)
	}
	impl := service.(*serviceImpl)
	harness := &verificationHarness{
		service: service, impl: impl, identity: identity, runtime: runtime,
		client: client, secrets: secrets, now: time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC),
		database: database, audit: audit,
	}
	impl.now = func() time.Time { return harness.now }
	impl.random = bytes.NewReader(bytes.Repeat([]byte{0x42}, 256))
	return harness
}

func TestPersonalVerificationPostgreSQLStartAndAuthoritativePass(t *testing.T) {
	ctx := context.Background()
	harness := newVerificationHarness(t, ctx)
	harness.client.queries = []aliyunface.QueryResult{{State: aliyunface.QueryStatePassed}}
	current, err := harness.service.Current(ctx)
	if err != nil || current.Status != StatusUnverified || harness.client.queryCalls != 0 {
		t.Fatalf("initial current=%+v queryCalls=%d err=%v", current, harness.client.queryCalls, err)
	}
	name := []byte("测试用户")
	identityNumber := syntheticIdentityNumber(t, time.Date(2000, 2, 29, 0, 0, 0, 0, time.UTC))
	rawIdentity := string(identityNumber)
	started, err := harness.service.Start(ctx, StartInput{
		ConsentNo: "consent_personal_0001", CertType: aliyunface.CertTypeIdentityCard,
		CertName: name, CertNo: identityNumber,
		ReturnURL: "https://portal.example.test/identity/return",
	})
	if err != nil || started.Status != StatusVerifying || started.ContinuationURL == "" ||
		harness.client.initCalls != 1 {
		t.Fatalf("started=%+v initCalls=%d err=%v", started, harness.client.initCalls, err)
	}
	assertZeroed(t, name)
	assertZeroed(t, identityNumber)
	current, err = harness.service.Current(ctx)
	if err != nil || current.Status != StatusVerifying || current.AttemptNo != started.AttemptNo ||
		current.ContinuationURL != "" || harness.client.queryCalls != 0 {
		t.Fatalf("verifying current=%+v queryCalls=%d err=%v", current, harness.client.queryCalls, err)
	}
	completed, err := harness.service.QueryCurrent(ctx, started.AttemptNo)
	if err != nil || completed.Status != StatusVerified || harness.client.queryCalls != 1 {
		t.Fatalf("completed=%+v queryCalls=%d err=%v", completed, harness.client.queryCalls, err)
	}
	if len(harness.audit.transitions) != 1 || !harness.audit.transaction ||
		harness.audit.transitions[0].TargetNo != completed.CaseNo ||
		harness.audit.transitions[0].CustomerNo != "cust_personal_0001" ||
		harness.audit.transitions[0].ResultCode != "succeeded" {
		t.Fatalf("unexpected verification audit: %+v", harness.audit.transitions)
	}
	if len(harness.identity.calls) != 2 ||
		harness.identity.calls[0].status != identitycap.StatusPending ||
		harness.identity.calls[1].status != identitycap.StatusVerified ||
		harness.identity.calls[1].name != "测试用户" ||
		harness.identity.calls[1].number != rawIdentity {
		t.Fatalf("identity projections=%+v", harness.identity.calls)
	}
	storedValue, err := harness.database.GetValue(ctx, `
SELECT row_to_json(stored_rows)::text FROM (
    SELECT case_no, customer_no, principal_no, status_code, provider_case_ref,
           legal_profile_snapshot_ref, legal_profile_sha256, masked_name, masked_identity_no,
           identity_lookup_digest, decision_reason_code
    FROM plugin_idc_compliance_verification_case
) AS stored_rows`)
	if err != nil {
		t.Fatal(err)
	}
	stored := storedValue.String()
	identityPublicDigest := sha256.Sum256([]byte(rawIdentity))
	if strings.Contains(stored, "测试用户") || strings.Contains(stored, rawIdentity) ||
		strings.Contains(stored, started.ContinuationURL) ||
		strings.Contains(stored, hex.EncodeToString(identityPublicDigest[:])) {
		t.Fatalf("compliance rows retained forbidden plaintext: %s", stored)
	}
	replayed, err := harness.service.QueryCurrent(ctx, started.AttemptNo)
	if err != nil || replayed.Status != StatusVerified || harness.client.queryCalls != 1 {
		t.Fatalf("terminal replay=%+v queryCalls=%d err=%v", replayed, harness.client.queryCalls, err)
	}
	current, err = harness.service.Current(ctx)
	if err != nil || current.Status != StatusVerified || current.AttemptNo != completed.AttemptNo ||
		current.ContinuationURL != "" || harness.client.queryCalls != 1 {
		t.Fatalf("verified current=%+v queryCalls=%d err=%v", current, harness.client.queryCalls, err)
	}
}

func TestPersonalVerificationPostgreSQLAuditFailureRollsBackTerminalState(t *testing.T) {
	ctx := context.Background()
	harness := newVerificationHarness(t, ctx)
	harness.client.queries = []aliyunface.QueryResult{
		{State: aliyunface.QueryStatePassed},
		{State: aliyunface.QueryStatePassed},
	}
	started, err := harness.service.Start(ctx, StartInput{
		ConsentNo: "consent_personal_0001", CertType: aliyunface.CertTypeIdentityCard,
		CertName:  []byte("审计用户"),
		CertNo:    syntheticIdentityNumber(t, time.Date(2000, 2, 29, 0, 0, 0, 0, time.UTC)),
		ReturnURL: "https://portal.example.test/identity/return",
	})
	if err != nil {
		t.Fatal(err)
	}
	harness.audit.err = errors.New("audit unavailable")
	if _, err = harness.service.QueryCurrent(ctx, started.AttemptNo); !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("audit failure returned %v", err)
	}
	status, err := harness.database.GetValue(ctx, `
SELECT status_code FROM plugin_idc_compliance_verification_case WHERE case_no = ?`,
		started.CaseNo)
	if err != nil || status.String() != string(caseStatusVerifying) {
		t.Fatalf("case status=%q err=%v", status.String(), err)
	}
	harness.audit.err = nil
	completed, err := harness.service.QueryCurrent(ctx, started.AttemptNo)
	if err != nil || completed.Status != StatusVerified {
		t.Fatalf("retry completed=%+v err=%v", completed, err)
	}
}

func TestPersonalVerificationPostgreSQLNotPassedThenLocalExpiry(t *testing.T) {
	ctx := context.Background()
	harness := newVerificationHarness(t, ctx)
	started := startSyntheticAdult(t, ctx, harness)
	pending, err := harness.service.QueryCurrent(ctx, started.AttemptNo)
	if err != nil || pending.Status != StatusVerifying || harness.client.queryCalls != 1 {
		t.Fatalf("pending=%+v queries=%d err=%v", pending, harness.client.queryCalls, err)
	}
	harness.now = harness.now.Add(31 * time.Minute)
	expired, err := harness.service.QueryCurrent(ctx, started.AttemptNo)
	if err != nil || expired.Status != StatusExpired || harness.client.queryCalls != 1 {
		t.Fatalf("expired=%+v queries=%d err=%v", expired, harness.client.queryCalls, err)
	}
	if harness.identity.calls[len(harness.identity.calls)-1].status != identitycap.StatusExpired {
		t.Fatalf("identity projections=%+v", harness.identity.calls)
	}
}

func TestPersonalVerificationPostgreSQLInitializationUnknownNeverReplays(t *testing.T) {
	ctx := context.Background()
	harness := newVerificationHarness(t, ctx)
	harness.client.initErr = &aliyunface.CallError{
		Code: aliyunface.ErrorResultUnknown, Unknown: true,
	}
	input := syntheticStartInput(t)
	_, err := harness.service.Start(ctx, input)
	if !bizerr.Is(err, CodeReviewRequired) || harness.client.initCalls != 1 {
		t.Fatalf("start error=%v initCalls=%d", err, harness.client.initCalls)
	}
	recovery, err := harness.service.Recover(ctx, 10)
	if err != nil || harness.client.initCalls != 1 || recovery.ReviewRequired != 1 {
		t.Fatalf("recovery=%+v initCalls=%d err=%v", recovery, harness.client.initCalls, err)
	}
	second := syntheticStartInput(t)
	if _, err = harness.service.Start(ctx, second); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("second active start error=%v", err)
	}
}

func TestPersonalVerificationPostgreSQLRecoveryReportsItemFailureWithoutReplayingInitialization(t *testing.T) {
	ctx := context.Background()
	harness := newVerificationHarness(t, ctx)
	started, err := harness.service.Start(ctx, syntheticStartInput(t))
	if err != nil || started == nil || harness.client.initCalls != 1 {
		t.Fatalf("started=%+v initCalls=%d err=%v", started, harness.client.initCalls, err)
	}
	harness.client.queries = []aliyunface.QueryResult{{State: aliyunface.QueryStatePassed}}
	harness.audit.err = errors.New("audit unavailable")
	recovery, err := harness.service.Recover(ctx, 10)
	if recovery == nil || recovery.Selected != 1 || recovery.Queried != 1 || recovery.Pending != 1 ||
		harness.client.initCalls != 1 || !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("recovery=%+v initCalls=%d err=%v", recovery, harness.client.initCalls, err)
	}
}

func TestPersonalVerificationPostgreSQLChangedConfigurationAndDuplicateRequireReview(t *testing.T) {
	ctx := context.Background()
	t.Run("configuration changed", func(t *testing.T) {
		harness := newVerificationHarness(t, ctx)
		started := startSyntheticAdult(t, ctx, harness)
		harness.runtime.configuration.Version++
		result, err := harness.service.QueryCurrent(ctx, started.AttemptNo)
		if err != nil || result.Status != StatusReviewRequired || harness.client.queryCalls != 0 ||
			result.ReasonCode != "provider_configuration_changed" {
			t.Fatalf("result=%+v queries=%d err=%v", result, harness.client.queryCalls, err)
		}
	})
	t.Run("duplicate identity", func(t *testing.T) {
		harness := newVerificationHarness(t, ctx)
		harness.identity.duplicate = true
		harness.client.queries = []aliyunface.QueryResult{{State: aliyunface.QueryStatePassed}}
		started := startSyntheticAdult(t, ctx, harness)
		result, err := harness.service.QueryCurrent(ctx, started.AttemptNo)
		if err != nil || result.Status != StatusReviewRequired ||
			result.ReasonCode != "duplicate_identity_requires_review" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	})
}

func startSyntheticAdult(t *testing.T, ctx context.Context, harness *verificationHarness) *Verification {
	t.Helper()
	result, err := harness.service.Start(ctx, syntheticStartInput(t))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func syntheticStartInput(t *testing.T) StartInput {
	t.Helper()
	return StartInput{
		ConsentNo: "consent_personal_0001", CertType: aliyunface.CertTypeIdentityCard,
		CertName:  []byte("测试用户"),
		CertNo:    syntheticIdentityNumber(t, time.Date(2000, 2, 29, 0, 0, 0, 0, time.UTC)),
		ReturnURL: "https://portal.example.test/identity/return",
	}
}

func personalVerificationTestDatabase(t *testing.T, ctx context.Context) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(personalVerificationIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run personal verification PostgreSQL tests", personalVerificationIntegrationDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := "idc_personal_verification_test_" + randomPersonalTestHex(t, 8)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	originalConfig := gdb.GetAllConfig()
	var database gdb.DB
	t.Cleanup(func() {
		if database != nil {
			if closeErr := database.Close(ctx); closeErr != nil {
				t.Errorf("close personal verification database: %v", closeErr)
			}
		}
		if restoreErr := gdb.SetConfig(originalConfig); restoreErr != nil {
			t.Errorf("restore database config: %v", restoreErr)
		}
		if _, dropErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Errorf("drop personal verification schema: %v", dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close personal verification admin database: %v", closeErr)
		}
	})
	if err = gdb.SetConfig(gdb.Config{
		gdb.DefaultGroupName: gdb.ConfigGroup{{
			Link: appendPersonalTestDSNParameter(dsn, "search_path="+schema),
		}},
	}); err != nil {
		t.Fatal(err)
	}
	database = g.DB()
	createPersonalVerificationDictionaryFixtures(t, ctx, database)
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "manifest", "sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"001-compliance-foundation.sql", "002-aliyun-personal-verification.sql",
		"024-privacy-governance.sql",
	} {
		content, readErr := os.ReadFile(filepath.Join(root, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, readErr = database.Exec(ctx, string(content)); readErr != nil {
			t.Fatalf("execute %s: %v", name, readErr)
		}
		if _, readErr = database.Exec(ctx, string(content)); readErr != nil {
			t.Fatalf("repeat %s idempotently: %v", name, readErr)
		}
	}
	return database
}

func createPersonalVerificationDictionaryFixtures(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
) {
	t.Helper()
	if _, err := database.Exec(ctx, `
CREATE TABLE sys_dict_type (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0, name VARCHAR(128) NOT NULL DEFAULT '',
    type VARCHAR(128) NOT NULL, status SMALLINT NOT NULL DEFAULT 1,
    is_builtin SMALLINT NOT NULL DEFAULT 0, allow_tenant_override BOOLEAN NOT NULL DEFAULT FALSE,
    remark VARCHAR(512) NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_personal_test_dict_type UNIQUE (tenant_id, type)
);
CREATE TABLE sys_dict_data (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0, dict_type VARCHAR(128) NOT NULL,
    label VARCHAR(128) NOT NULL DEFAULT '', value VARCHAR(128) NOT NULL,
    sort INT NOT NULL DEFAULT 0, tag_style VARCHAR(32) NOT NULL DEFAULT 'default',
    status SMALLINT NOT NULL DEFAULT 1, is_builtin SMALLINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_personal_test_dict_data UNIQUE (tenant_id, dict_type, value)
)`); err != nil {
		t.Fatal(err)
	}
}

func randomPersonalTestHex(t *testing.T, size int) string {
	t.Helper()
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(value)
}

func appendPersonalTestDSNParameter(dsn, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

func assertZeroed(t *testing.T, value []byte) {
	t.Helper()
	for _, character := range value {
		if character != 0 {
			t.Fatal("identity input was not cleared")
		}
	}
}
