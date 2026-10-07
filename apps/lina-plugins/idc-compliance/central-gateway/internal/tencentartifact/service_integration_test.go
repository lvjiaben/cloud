package tencentartifact

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"lina-platform/idc-compliance-gateway/internal/callbacksecurity"
	"lina-platform/idc-compliance-gateway/internal/migration"
	"lina-platform/idc-compliance-gateway/internal/suppliercallback"
	"lina-platform/idc-compliance-gateway/internal/supplierguard"
	"lina-platform/idc-compliance-gateway/internal/tencentess"
)

const artifactTestInstallationID = "linst_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type reportClientStub struct {
	guard       *supplierguard.Service
	createCalls int
	fetchCalls  int
	reportRoute string
	observedAt  time.Time
}

type terminalQueryStub struct {
	observedAt time.Time
}

func (s *terminalQueryStub) Query(
	_ context.Context,
	request suppliercallback.QueryRequest,
) (suppliercallback.QueryResponse, error) {
	if string(request.ExternalReference) != "provider-flow-report" {
		return suppliercallback.QueryResponse{}, suppliercallback.ErrRejected
	}
	return suppliercallback.QueryResponse{
		Status: suppliercallback.StatusSucceeded, ProviderStatus: "ALL",
		EvidenceFrame: []byte("terminal-query-evidence"), SupplierObservedAt: s.observedAt,
	}, nil
}

func (*reportClientStub) HasCapability(capability tencentess.Capability) bool {
	return capability == tencentess.CapabilityEvidenceReport
}

func (*reportClientStub) FetchFinalPDF(context.Context, string) (*tencentess.ArtifactDocument, error) {
	return nil, &tencentess.Error{Code: tencentess.ErrorRejected}
}

func (s *reportClientStub) CreateEvidenceReport(
	ctx context.Context,
	requestNo string,
	flowID string,
) (tencentess.EvidenceReportCreateResult, error) {
	s.createCalls++
	if flowID != "provider-flow-report" {
		return tencentess.EvidenceReportCreateResult{}, &tencentess.Error{Code: tencentess.ErrorInvalidInput}
	}
	lease, err := s.guard.Acquire(ctx, supplierguard.Admission{
		RequestNo: requestNo, Provider: supplierguard.ProviderTencentESSBasic,
		OperationScope: tencentess.OperationEvidenceReportCreate,
	})
	if err != nil {
		return tencentess.EvidenceReportCreateResult{}, err
	}
	s.reportRoute = lease.RouteNo
	if _, err = s.guard.Observe(ctx, supplierguard.Observation{
		AttemptNo: lease.AttemptNo, Result: supplierguard.ResultSucceeded,
		StableCode: "tencent_ok", Elapsed: time.Millisecond,
	}); err != nil {
		return tencentess.EvidenceReportCreateResult{}, err
	}
	return tencentess.EvidenceReportCreateResult{
		RouteNo: lease.RouteNo, ReportID: "encrypted-report-id",
		Status: "EvidenceStatusExecuting", ObservedAt: s.observedAt,
	}, nil
}

func (s *reportClientStub) FetchEvidenceReport(
	_ context.Context,
	reportID string,
) (tencentess.EvidenceReportResult, error) {
	s.fetchCalls++
	if reportID != "encrypted-report-id" {
		return tencentess.EvidenceReportResult{}, &tencentess.Error{Code: tencentess.ErrorInvalidInput}
	}
	body := []byte("%PDF-1.7\nreport")
	return tencentess.EvidenceReportResult{
		Status: "EvidenceStatusSuccess", ObservedAt: s.observedAt.Add(time.Hour),
		Document: &tencentess.ArtifactDocument{
			Content: body, SHA256: "placeholder", ObservedAt: s.observedAt.Add(time.Hour),
		},
	}, nil
}

func TestEvidenceReportRequestIsEncryptedAndNeverRepeatedPostgreSQL(t *testing.T) {
	database, cleanup := openArtifactPostgreSQL(t)
	defer cleanup()
	file, err := callbacksecurity.Generate(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ring, err := callbacksecurity.New(file)
	if err != nil {
		t.Fatal(err)
	}
	defer ring.Clear()
	guard, err := supplierguard.NewService(database, artifactTestInstallationID, supplierguard.DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	observedAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	query, err := suppliercallback.NewQueryService(
		database, artifactTestInstallationID, ring, map[string]suppliercallback.ReadOnlyQuerier{
			suppliercallback.ProviderTencentESSBasic: &terminalQueryStub{observedAt: observedAt},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	sourceRequestNo := "lbr_11111111111111111111111111111111"
	insertArtifactBusinessRequest(t, database, sourceRequestNo, tencentess.OperationFlowCreate)
	sourceLease, err := guard.Acquire(t.Context(), supplierguard.Admission{
		RequestNo:      sourceRequestNo,
		Provider:       supplierguard.ProviderTencentESSBasic,
		OperationScope: tencentess.OperationFlowCreate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = query.BindExternalReference(t.Context(), suppliercallback.ReferenceBinding{
		RouteNo: sourceLease.RouteNo, Provider: sourceLease.Provider,
		OperationScope:    sourceLease.OperationScope,
		ExternalReference: []byte("provider-flow-report"),
	}); err != nil {
		t.Fatal(err)
	}
	if result, queryErr := query.RunRoute(
		t.Context(), sourceLease.RouteNo, sourceLease.Provider, sourceLease.OperationScope,
	); queryErr != nil || result.ProjectedStatus != suppliercallback.StatusSucceeded ||
		result.ProviderStatus != "ALL" {
		t.Fatalf("converge source route result=%+v err=%v", result, queryErr)
	}
	client := &reportClientStub{guard: guard, observedAt: observedAt}
	serviceValue, err := New(database, artifactTestInstallationID, ring, query, client)
	if err != nil {
		t.Fatal(err)
	}
	reportRequestNo := "lbr_22222222222222222222222222222222"
	insertArtifactBusinessRequest(t, database, reportRequestNo, tencentess.OperationEvidenceReportCreate)
	result, err := serviceValue.EnsureSignReport(
		t.Context(), sourceLease.RouteNo, reportRequestNo,
	)
	if err != nil || result.State != StatePending || result.NextCheckAt.IsZero() || client.createCalls != 1 {
		t.Fatalf("initial report result=%+v createCalls=%d err=%v", result, client.createCalls, err)
	}
	var ciphertext []byte
	if err = database.QueryRowContext(t.Context(), `
SELECT report_ciphertext
FROM idc_gateway_tencent_contract_report_job
WHERE installation_id = $1 AND source_route_no = $2`,
		artifactTestInstallationID, sourceLease.RouteNo,
	).Scan(&ciphertext); err != nil {
		t.Fatal(err)
	}
	if len(ciphertext) == 0 || strings.Contains(string(ciphertext), "encrypted-report-id") {
		t.Fatal("report identifier was not stored as ciphertext")
	}
	if _, err = database.ExecContext(t.Context(), `
UPDATE idc_gateway_tencent_contract_report_job
SET next_query_at = CURRENT_TIMESTAMP - INTERVAL '1 second'
WHERE installation_id = $1 AND source_route_no = $2`,
		artifactTestInstallationID, sourceLease.RouteNo,
	); err != nil {
		t.Fatal(err)
	}
	ready, err := serviceValue.EnsureSignReport(
		t.Context(), sourceLease.RouteNo, reportRequestNo,
	)
	if err != nil || ready.State != StateReady || ready.Document == nil ||
		client.createCalls != 1 || client.fetchCalls != 1 {
		t.Fatalf("ready result=%+v createCalls=%d fetchCalls=%d err=%v",
			ready, client.createCalls, client.fetchCalls, err)
	}
	replayed, err := serviceValue.EnsureSignReport(
		t.Context(), sourceLease.RouteNo, reportRequestNo,
	)
	if err != nil || replayed.State != StateReady || client.createCalls != 1 || client.fetchCalls != 2 {
		t.Fatalf("replayed result=%+v createCalls=%d fetchCalls=%d err=%v",
			replayed, client.createCalls, client.fetchCalls, err)
	}
}

func insertArtifactBusinessRequest(t *testing.T, database *sql.DB, requestNo, scope string) {
	t.Helper()
	target := sha256.Sum256([]byte("artifact-target\x00" + requestNo))
	idempotency := sha256.Sum256([]byte("artifact-idempotency\x00" + requestNo))
	content := sha256.Sum256([]byte("artifact-content\x00" + requestNo))
	body := sha256.Sum256([]byte("artifact-body\x00" + requestNo))
	if _, err := database.ExecContext(t.Context(), `
INSERT INTO idc_gateway_local_business_request
    (request_no, installation_id, operation_scope, method, request_target_sha256,
     idempotency_hmac, business_content_sha256, body_sha256)
VALUES ($1, $2, $3, 'POST', $4, $5, $6, $7)`,
		requestNo, artifactTestInstallationID, scope, target[:], idempotency[:], content[:], body[:],
	); err != nil {
		t.Fatalf("insert artifact business request: %v", err)
	}
}

func openArtifactPostgreSQL(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN to run Tencent artifact PostgreSQL tests")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "idc_tencent_artifact_test_" + hex.EncodeToString(suffix[:])
	if _, err = admin.ExecContext(t.Context(), `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := sql.Open("pgx", dsn+separator+"search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	if err = migration.Apply(t.Context(), database); err != nil {
		t.Fatalf("apply local gateway migrations: %v", err)
	}
	if _, err = database.ExecContext(t.Context(), `
INSERT INTO idc_gateway_local_supplier_instance (installation_id) VALUES ($1)`,
		artifactTestInstallationID,
	); err != nil {
		t.Fatalf("insert local artifact installation: %v", err)
	}
	return database, func() {
		_ = database.Close()
		_, _ = admin.ExecContext(context.Background(), `DROP SCHEMA "`+schema+`" CASCADE`)
		_ = admin.Close()
	}
}
