// This file verifies supplier-subject proof, state, and plaintext boundaries in PostgreSQL.

package supplierconfig

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

const supplierConfigurationIntegrationDSNEnvironment = "LINAPRO_IDC_COMPLIANCE_TEST_DSN"

type supplierProofStore struct {
	now      time.Time
	issued   map[string]managementcap.SensitiveAccessBinding
	consumed map[string]bool
	next     int
}

func (s *supplierProofStore) IssueSensitiveAccess(
	_ context.Context,
	binding managementcap.SensitiveAccessBinding,
) (*managementcap.SensitiveAccessContext, error) {
	if s.issued == nil {
		s.issued = make(map[string]managementcap.SensitiveAccessBinding)
	}
	if s.consumed == nil {
		s.consumed = make(map[string]bool)
	}
	s.next++
	token := fmt.Sprintf("supplier-proof-%d", s.next)
	s.issued[token] = binding
	return &managementcap.SensitiveAccessContext{
		Token: token, ExpiresAt: s.now.Add(5 * time.Minute),
	}, nil
}

func (s *supplierProofStore) ConsumeSensitiveAccess(
	_ context.Context,
	token string,
	binding managementcap.SensitiveAccessBinding,
) (*managementcap.SensitiveAccessProof, error) {
	issued, exists := s.issued[token]
	if !exists || s.consumed[token] || issued != binding {
		return nil, errors.New("supplier proof was rejected")
	}
	s.consumed[token] = true
	return &managementcap.SensitiveAccessProof{
		ActorUserID: 42, ExpiresAt: s.now.Add(5 * time.Minute),
	}, nil
}

type supplierSubjectGatewayRecorder struct {
	bindCalls int
	failBind  bool
	subjects  [][]byte
}

func (*supplierSubjectGatewayRecorder) RecognizerAvailable(string) bool { return true }

func (g *supplierSubjectGatewayRecorder) Bind(
	_ context.Context,
	_ string,
	subject []byte,
	_ string,
) (GatewayResult, error) {
	g.bindCalls++
	g.subjects = append(g.subjects, append([]byte(nil), subject...))
	if g.failBind {
		return GatewayResult{}, errors.New("synthetic local gateway failure")
	}
	return GatewayResult{
		OperationNo: "lsi_0123456789abcdef0123456789abcdef",
		BindingNo:   "lsb_0123456789abcdef0123456789abcdef",
	}, nil
}

func (*supplierSubjectGatewayRecorder) Revoke(
	context.Context,
	string,
	string,
	string,
	string,
	string,
	string,
) (GatewayResult, error) {
	return GatewayResult{}, errors.New("revocation is not used by this binding test")
}

func (*supplierSubjectGatewayRecorder) Reopen(
	context.Context,
	string,
	string,
	string,
	[]byte,
	string,
	string,
	string,
) (GatewayResult, error) {
	return GatewayResult{}, errors.New("reopening is not used by this binding test")
}

type supplierAuditRecorder struct {
	idccap.BusinessAuditWriter
	transitions []idccap.BusinessAuditTransition
}

func (a *supplierAuditRecorder) AppendBusinessTransition(
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
	return nil
}

func TestSupplierSubjectBindingProofAndFailureStatePostgreSQL(t *testing.T) {
	ctx := context.Background()
	database := supplierConfigurationTestDatabase(t, ctx)
	now := time.Date(2026, time.August, 14, 9, 0, 0, 0, time.UTC)
	proofs := &supplierProofStore{now: now}
	gateway := &supplierSubjectGatewayRecorder{}
	audit := &supplierAuditRecorder{}
	service, err := New(
		&supplierManager{},
		managementcap.Runtime{
			Authorizer: &supplierAuthorizer{actor: 42}, Audit: audit, SensitiveAccess: proofs,
		},
		&supplierSessions{current: &sessioncap.SessionInfo{
			ID: "supplier-session", UserID: "42", TenantID: capmodel.DomainID("0"),
			LoginAt: timePointer(now),
		}},
		gateway,
	)
	if err != nil {
		t.Fatalf("create supplier service: %v", err)
	}
	service.(*serviceImpl).now = func() time.Time { return now }

	request := SubjectAuthorizationRequest{
		ProviderCode: ProviderNuonuoInvoice, Operation: OperationSubjectBind,
		IdempotencyKey: "supplier-subject-bind-0001",
	}
	issued, err := service.IssueSubjectAuthorization(ctx, request)
	if err != nil || issued == nil || issued.Token == "" ||
		!issued.ExpiresAt.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("issued proof=%+v err=%v", issued, err)
	}
	authorizationResult := []byte("provider-owned-subject-result")
	configuration, err := service.CompleteSubjectAuthorization(ctx, SubjectAuthorizationResult{
		Request: request, ProofToken: issued.Token, AuthorizationResult: authorizationResult,
	})
	if err != nil || configuration == nil || configuration.SubjectStatus != SubjectActive ||
		configuration.LastOperationResult != OperationResultSucceeded ||
		configuration.SubjectBindingNo != "lsb_0123456789abcdef0123456789abcdef" ||
		configuration.CapabilityEvidenceCode != EvidenceExternalConfirmationRequired ||
		configuration.AdapterEnabled || gateway.bindCalls != 1 {
		t.Fatalf("configuration=%+v gatewayCalls=%d err=%v", configuration, gateway.bindCalls, err)
	}
	assertClearedSupplierBytes(t, authorizationResult)
	persistedProofs, proofErr := database.GetValue(ctx, `
SELECT count(*)
FROM plugin_idc_compliance_supplier_configuration
WHERE provider_code = ?
  AND subject_proof_digest = ?
  AND subject_proof_expires_at = ?`,
		ProviderNuonuoInvoice, proofTokenDigest(issued.Token), issued.ExpiresAt,
	)
	if proofErr != nil || persistedProofs.Int() != 1 {
		t.Fatalf("persisted proof count=%d want=1 err=%v", persistedProofs.Int(), proofErr)
	}
	assertSupplierPlaintextAbsent(t, ctx, database, "provider-owned-subject-result", audit)

	replayedResult := []byte("provider-owned-subject-result")
	if _, err = service.CompleteSubjectAuthorization(ctx, SubjectAuthorizationResult{
		Request: request, ProofToken: issued.Token, AuthorizationResult: replayedResult,
	}); err == nil || gateway.bindCalls != 1 {
		t.Fatalf("replayed proof error=%v gatewayCalls=%d", err, gateway.bindCalls)
	}
	assertClearedSupplierBytes(t, replayedResult)

}

// TestSupplierSubjectFailureStatePostgreSQL uses a fresh supplier fixture after electronic-sign retirement.
func TestSupplierSubjectFailureStatePostgreSQL(t *testing.T) {
	ctx := context.Background()
	database := supplierConfigurationTestDatabase(t, ctx)
	now := time.Date(2026, time.August, 14, 9, 0, 0, 0, time.UTC)
	proofs := &supplierProofStore{now: now}
	gateway := &supplierSubjectGatewayRecorder{}
	audit := &supplierAuditRecorder{}
	service, err := New(
		&supplierManager{},
		managementcap.Runtime{
			Authorizer: &supplierAuthorizer{actor: 42}, Audit: audit, SensitiveAccess: proofs,
		},
		&supplierSessions{current: &sessioncap.SessionInfo{
			ID: "supplier-session", UserID: "42", TenantID: capmodel.DomainID("0"),
			LoginAt: timePointer(now),
		}},
		gateway,
	)
	if err != nil {
		t.Fatalf("create supplier service: %v", err)
	}
	service.(*serviceImpl).now = func() time.Time { return now }

	gateway.failBind = true
	failureRequest := SubjectAuthorizationRequest{
		ProviderCode: ProviderNuonuoInvoice, Operation: OperationSubjectBind,
		IdempotencyKey: "supplier-subject-bind-0002",
	}
	failureProof, err := service.IssueSubjectAuthorization(ctx, failureRequest)
	if err != nil {
		t.Fatalf("issue failure-path proof: %v", err)
	}
	failureResult := []byte("provider-owned-failure-result")
	if _, err = service.CompleteSubjectAuthorization(ctx, SubjectAuthorizationResult{
		Request: failureRequest, ProofToken: failureProof.Token, AuthorizationResult: failureResult,
	}); err == nil {
		t.Fatal("synthetic gateway failure was accepted")
	}
	assertClearedSupplierBytes(t, failureResult)
	var row struct {
		SubjectStatusCode       string `json:"subject_status_code"`
		LastOperationResultCode string `json:"last_operation_result_code"`
		AdapterEnabled          bool   `json:"adapter_enabled"`
	}
	if err = database.Model("plugin_idc_compliance_supplier_configuration").
		Fields("subject_status_code", "last_operation_result_code", "adapter_enabled").
		Where("provider_code", ProviderNuonuoInvoice).Scan(&row); err != nil {
		t.Fatalf("read retryable supplier state: %v", err)
	}
	if row.SubjectStatusCode != string(SubjectRetryableFailed) ||
		row.LastOperationResultCode != string(OperationResultRetryableFailed) || row.AdapterEnabled {
		t.Fatalf("retryable supplier state=%+v", row)
	}
	assertSupplierPlaintextAbsent(t, ctx, database, "provider-owned-failure-result", audit)
}

func assertClearedSupplierBytes(t *testing.T, value []byte) {
	t.Helper()
	for _, item := range value {
		if item != 0 {
			t.Fatal("supplier authorization result was not cleared")
		}
	}
}

func assertSupplierPlaintextAbsent(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	plaintext string,
	audit *supplierAuditRecorder,
) {
	t.Helper()
	stored, err := database.GetValue(ctx, `
SELECT COALESCE(string_agg(row_to_json(configuration)::text, ''), '')
FROM plugin_idc_compliance_supplier_configuration AS configuration`)
	if err != nil {
		t.Fatalf("read supplier configuration rows: %v", err)
	}
	if strings.Contains(stored.String(), plaintext) || strings.Contains(fmt.Sprint(audit.transitions), plaintext) {
		t.Fatal("supplier plaintext reached ordinary storage or audit")
	}
}

func supplierConfigurationTestDatabase(t *testing.T, ctx context.Context) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(supplierConfigurationIntegrationDSNEnvironment))
	if dsn == "" {
		t.Skipf("set %s to run supplier configuration PostgreSQL tests", supplierConfigurationIntegrationDSNEnvironment)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := "idc_supplier_configuration_test_" + randomSupplierTestHex(t, 8)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	originalConfig := gdb.GetAllConfig()
	var database gdb.DB
	t.Cleanup(func() {
		if database != nil {
			if closeErr := database.Close(ctx); closeErr != nil {
				t.Errorf("close supplier configuration database: %v", closeErr)
			}
		}
		if restoreErr := gdb.SetConfig(originalConfig); restoreErr != nil {
			t.Errorf("restore database config: %v", restoreErr)
		}
		if _, dropErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Errorf("drop supplier configuration schema: %v", dropErr)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close supplier configuration admin database: %v", closeErr)
		}
	})
	if err = gdb.SetConfig(gdb.Config{
		gdb.DefaultGroupName: gdb.ConfigGroup{{
			Link: appendSupplierTestDSNParameter(dsn, "search_path="+schema),
		}},
	}); err != nil {
		t.Fatal(err)
	}
	database = g.DB()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "manifest", "sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"006-local-supplier-configuration.sql", "007-local-supplier-subject-boundary.sql",
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

func randomSupplierTestHex(t *testing.T, size int) string {
	t.Helper()
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(value)
}

func appendSupplierTestDSNParameter(dsn string, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}
