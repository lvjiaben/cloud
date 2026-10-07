package contractarchive

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-compliance/backend/internal/localgatewayclient"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type archiveGatewayStub struct {
	finalCalls  int
	reportCalls int
	document    []byte
	observedAt  time.Time
}

func (g *archiveGatewayStub) FetchTencentContractFinalPDF(
	context.Context,
	string,
) (*localgatewayclient.TencentContractArtifact, error) {
	g.finalCalls++
	return archiveArtifact("final_pdf", g.document, g.observedAt), nil
}

func (g *archiveGatewayStub) EnsureTencentContractSignReport(
	context.Context,
	string,
	string,
) (*localgatewayclient.TencentContractArtifact, localgatewayclient.TencentContractArtifactState, error) {
	g.reportCalls++
	return nil, localgatewayclient.TencentContractArtifactState{
		State: "skipped", Code: "report_not_enabled",
	}, nil
}

func archiveArtifact(kind string, body []byte, observedAt time.Time) *localgatewayclient.TencentContractArtifact {
	digest := sha256.Sum256(body)
	return &localgatewayclient.TencentContractArtifact{
		Kind: kind, Body: append([]byte(nil), body...), SHA256: hex.EncodeToString(digest[:]),
		SupplierObservedAt: observedAt,
	}
}

type archiveStorageStub struct {
	storagecap.Service
	mu      sync.Mutex
	objects map[string][]byte
}

func (s *archiveStorageStub) Put(
	_ context.Context,
	input storagecap.PutInput,
) (*storagecap.PutOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	content, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	if _, exists := s.objects[input.Path]; exists && !input.Overwrite {
		return nil, os.ErrExist
	}
	s.objects[input.Path] = append([]byte(nil), content...)
	return &storagecap.PutOutput{Object: archiveStorageObject(input.Path, content)}, nil
}

func (s *archiveStorageStub) Get(
	_ context.Context,
	input storagecap.GetInput,
) (*storagecap.GetOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	content, exists := s.objects[input.Path]
	if !exists {
		return &storagecap.GetOutput{Found: false}, nil
	}
	copyOfContent := append([]byte(nil), content...)
	return &storagecap.GetOutput{
		Found: true, Object: archiveStorageObject(input.Path, copyOfContent),
		Body: io.NopCloser(bytes.NewReader(copyOfContent)),
	}, nil
}

func archiveStorageObject(path string, content []byte) *storagecap.Object {
	return &storagecap.Object{
		Path: path, Size: int64(len(content)), ContentType: "application/pdf",
		Visibility: storagecap.VisibilityPrivate,
	}
}

type archiveAuditStub struct {
	transitions []idccap.BusinessAuditTransition
}

func (a *archiveAuditStub) AppendBusinessTransition(
	_ context.Context,
	input idccap.BusinessAuditTransition,
) error {
	a.transitions = append(a.transitions, input)
	return nil
}

func (*archiveAuditStub) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

func TestContractArchiveCompletesPrivatePDFAndShortLivedMetadataPostgreSQL(t *testing.T) {
	database, cleanup := openArchivePostgreSQL(t)
	defer cleanup()
	ctx := context.Background()
	completedAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract
    (contract_no, customer_no, status_code)
VALUES ('contract-1', 'customer-1', 'completed')`); err != nil {
		t.Fatalf("insert completed contract fixture: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract_flow
    (flow_no, contract_no, purpose_code, provider_code, status_code,
     gateway_route_no, completed_at)
VALUES ('flow-1', 'contract-1', 'issue', 'tencent_essbasic', 'completed',
		'lwr_11111111111111111111111111111111', ?)`, completedAt); err != nil {
		t.Fatalf("insert completed contract fixture: %v", err)
	}
	gateway := &archiveGatewayStub{
		document: []byte("%PDF-1.7\ncontract archive"), observedAt: completedAt,
	}
	storage := &archiveStorageStub{objects: map[string][]byte{}}
	audit := &archiveAuditStub{}
	serviceValue, err := New(database, gateway, storage, audit)
	if err != nil {
		t.Fatalf("construct contract archive service: %v", err)
	}
	result, err := serviceValue.Run(ctx, MaximumBatchSize)
	if err != nil || result.Enqueued != 3 || result.Processed != 3 {
		t.Fatalf("run archive result=%+v err=%v", result, err)
	}
	var statuses []struct {
		ArtifactType string `orm:"artifact_type_code"`
		StatusCode   string `orm:"status_code"`
		ReasonCode   string `orm:"last_reason_code"`
	}
	if err = database.GetScan(ctx, &statuses, `
SELECT artifact_type_code, status_code, last_reason_code
FROM plugin_idc_compliance_contract_archive_job
ORDER BY artifact_type_code`); err != nil {
		t.Fatalf("read archive jobs: %v", err)
	}
	want := map[string]string{
		"blockchain_metadata": "archived", "final_pdf": "archived", "sign_report": "skipped",
	}
	for _, status := range statuses {
		if want[status.ArtifactType] != status.StatusCode {
			t.Fatalf("unexpected archive status: %+v", status)
		}
	}
	fileCount, err := database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_compliance_contract_file
WHERE flow_no = 'flow-1' AND file_type_code = 'final_pdf'
  AND status_code = 'archived' AND immutable = TRUE
  AND supplier_observed_at IS NOT NULL`)
	if err != nil || fileCount.Int() != 1 {
		t.Fatalf("archived final PDF count=%d err=%v", fileCount.Int(), err)
	}
	evidenceCount, err := database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_compliance_external_evidence
WHERE aggregate_no = 'contract-1' AND evidence_type_code = 'blockchain_short_lived_metadata'
  AND private_object_ref IS NULL`)
	if err != nil || evidenceCount.Int() != 1 {
		t.Fatalf("short-lived metadata count=%d err=%v", evidenceCount.Int(), err)
	}
	if gateway.finalCalls != 1 || gateway.reportCalls != 1 || len(storage.objects) != 1 ||
		len(audit.transitions) != 3 {
		t.Fatalf("calls final=%d report=%d objects=%d audits=%d",
			gateway.finalCalls, gateway.reportCalls, len(storage.objects), len(audit.transitions))
	}
	replayed, err := serviceValue.Run(ctx, MaximumBatchSize)
	if err != nil || replayed.Enqueued != 0 || replayed.Processed != 0 ||
		gateway.finalCalls != 1 || gateway.reportCalls != 1 {
		t.Fatalf("archive replay was not closed: result=%+v err=%v", replayed, err)
	}
}

func openArchivePostgreSQL(t *testing.T) (gdb.DB, func()) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_COMPLIANCE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_COMPLIANCE_TEST_DSN to run contract archive PostgreSQL tests")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open contract archive PostgreSQL admin connection: %v", err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatalf("generate contract archive schema: %v", err)
	}
	schema := "idc_contract_archive_test_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create contract archive schema: %v", err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatalf("open scoped contract archive database: %v", err)
	}
	if _, err = database.Exec(ctx, archiveFoundationSQL); err != nil {
		t.Fatalf("create contract archive foundation: %v", err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve contract archive migration path")
	}
	migrationPath := filepath.Join(
		filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "018-contract-archive.sql",
	)
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read contract archive migration: %v", err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("execute contract archive migration attempt %d: %v", attempt, err)
		}
	}
	return database, func() {
		_ = database.Close(ctx)
		_, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
		_ = admin.Close(ctx)
	}
}

const archiveFoundationSQL = `
CREATE TABLE plugin_idc_compliance_contract (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    contract_no VARCHAR(64) NOT NULL UNIQUE,
    customer_no VARCHAR(64) NOT NULL,
    status_code VARCHAR(24) NOT NULL
);
CREATE TABLE plugin_idc_compliance_contract_flow (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    flow_no VARCHAR(64) NOT NULL UNIQUE,
    contract_no VARCHAR(64) NOT NULL REFERENCES plugin_idc_compliance_contract(contract_no),
    purpose_code VARCHAR(16) NOT NULL,
    provider_code VARCHAR(32) NOT NULL,
    status_code VARCHAR(24) NOT NULL,
    gateway_route_no VARCHAR(64),
    completed_at TIMESTAMPTZ,
    UNIQUE (flow_no, contract_no)
);
CREATE TABLE plugin_idc_compliance_contract_file (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    file_no VARCHAR(64) NOT NULL UNIQUE,
    contract_no VARCHAR(64) NOT NULL REFERENCES plugin_idc_compliance_contract(contract_no),
    flow_no VARCHAR(64),
    file_type_code VARCHAR(32) NOT NULL,
    private_object_ref VARCHAR(512) NOT NULL,
    content_sha256 CHAR(64) NOT NULL,
    media_type VARCHAR(128) NOT NULL,
    byte_size BIGINT NOT NULL,
    status_code VARCHAR(24) NOT NULL DEFAULT 'staging',
    immutable BOOLEAN NOT NULL DEFAULT FALSE,
    archived_at TIMESTAMPTZ,
    retention_until TIMESTAMPTZ,
    legal_hold BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (flow_no, contract_no)
        REFERENCES plugin_idc_compliance_contract_flow(flow_no, contract_no)
);
CREATE TABLE plugin_idc_compliance_external_evidence (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    evidence_no VARCHAR(64) NOT NULL UNIQUE,
    aggregate_type_code VARCHAR(24) NOT NULL,
    aggregate_no VARCHAR(64) NOT NULL,
    source_system_code VARCHAR(64) NOT NULL,
    source_event_ref VARCHAR(256) NOT NULL,
    evidence_type_code VARCHAR(64) NOT NULL,
    payload_sha256 CHAR(64) NOT NULL,
    private_object_ref VARCHAR(512),
    media_type VARCHAR(128),
    byte_size BIGINT,
    disposition_code VARCHAR(16) NOT NULL DEFAULT 'accepted',
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, source_system_code, source_event_ref)
);`
