package taxinvoicemanagement

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

const managementTestDSNEnv = "LINAPRO_IDC_COMPLIANCE_TEST_DSN"

type managementAuthorizer struct {
	mu       sync.Mutex
	requests []managementcap.AuthorizationRequest
}

func (a *managementAuthorizer) Authorize(
	_ context.Context,
	request managementcap.AuthorizationRequest,
) (int, error) {
	if err := managementcap.ValidateAuthorizationRequest(request); err != nil {
		return 0, err
	}
	a.mu.Lock()
	a.requests = append(a.requests, request)
	a.mu.Unlock()
	return 73, nil
}

type managementAudit struct {
	mu          sync.Mutex
	transitions []idccap.BusinessAuditTransition
	fail        bool
}

func (a *managementAudit) AppendBusinessTransition(
	_ context.Context,
	transition idccap.BusinessAuditTransition,
) error {
	if a.fail {
		return gerror.New("forced audit failure")
	}
	a.mu.Lock()
	a.transitions = append(a.transitions, transition)
	a.mu.Unlock()
	return nil
}

func (a *managementAudit) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

type fixedGate struct{ capability Capability }

func (g fixedGate) Current(context.Context) (Capability, error) { return g.capability, nil }

type managementSnapshotAccessor struct{}

func (managementSnapshotAccessor) Access(
	_ context.Context,
	request secretcap.AccessRequest,
	consume secretcap.Consumer,
) error {
	if err := secretcap.ValidateAccess(request, consume); err != nil {
		return err
	}
	reference := request.Handle.OpaqueRef()
	taxInvoiceNo := strings.TrimSuffix(strings.TrimPrefix(reference, "private://"), "/buyer")
	if taxInvoiceNo == reference || taxInvoiceNo == "" {
		return gerror.New("unexpected management snapshot reference")
	}
	payload, err := json.Marshal(storedSnapshotBundle{
		TaxInvoiceNo: taxInvoiceNo,
		Request:      taxinvoicecap.RequestSnapshot{Buyer: managementBuyerProfile(taxInvoiceNo)},
	})
	if err != nil {
		return err
	}
	defer clear(payload)
	return consume(payload)
}

func TestTaxInvoiceManagementPostgreSQL(t *testing.T) {
	database := openManagementTestDatabase(t)
	ctx := context.Background()
	prefix := managementTestPrefix(t)
	authorizer := new(managementAuthorizer)
	audit := new(managementAudit)
	ready := Capability{
		ProviderCode: ProviderNuoNuo, Ready: true,
		SupportedInvoiceTypes: []string{
			InvoiceTypeVATElectronicNormal, InvoiceTypeDigitalNormal,
		},
	}
	serviceValue, err := New(database, managementcap.Runtime{
		Authorizer: authorizer, Audit: audit,
	}, fixedGate{capability: ready}, managementSnapshotAccessor{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	blockedNo := prefix + "blocked"
	insertManagementRequest(t, database, blockedNo, StatusPendingReview, 1, "", false, false)
	blockedService, err := New(database, managementcap.Runtime{
		Authorizer: authorizer, Audit: audit,
	}, ClosedNuoNuoGate{}, managementSnapshotAccessor{})
	if err != nil {
		t.Fatalf("New(closed) error = %v", err)
	}
	blocked, err := blockedService.Approve(ctx, ReviewInput{
		TaxInvoiceNo: blockedNo, InvoiceTypeCode: InvoiceTypeVATElectronicNormal,
		ExpectedVersion: 1, IdempotencyKey: prefix + "blocked-key",
	})
	if err != nil || blocked.Applied || blocked.Request.StatusCode != StatusPendingReview ||
		blocked.Request.Version != 1 || blocked.Request.CapabilityReasonCode != CapabilityExternalConfirmation {
		t.Fatalf("blocked Approve() = %#v error=%v", blocked, err)
	}
	if _, err = blockedService.Approve(ctx, ReviewInput{
		TaxInvoiceNo: blockedNo, InvoiceTypeCode: InvoiceTypeVATElectronicNormal,
		ExpectedVersion: 2, IdempotencyKey: prefix + "blocked-stale-key",
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("blocked stale Approve() error=%v", err)
	}
	if _, err = blockedService.Submit(ctx, SubmitInput{
		TaxInvoiceNo: blockedNo, ExpectedVersion: 1,
		IdempotencyKey: prefix + "blocked-invalid-submit-key",
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("blocked invalid Submit() error=%v", err)
	}

	approvedNo := prefix + "approved"
	insertManagementRequest(t, database, approvedNo, StatusPendingReview, 1, "", false, false)
	approvalInput := ReviewInput{
		TaxInvoiceNo: approvedNo, InvoiceTypeCode: InvoiceTypeVATElectronicNormal,
		ExpectedVersion: 1, IdempotencyKey: prefix + "approve-key",
	}
	approved, err := serviceValue.Approve(ctx, approvalInput)
	if err != nil || !approved.Applied || approved.Reused ||
		approved.Request.StatusCode != StatusApprovedPendingSubmission || approved.Request.Version != 2 ||
		approved.Request.InvoiceTypeCode != InvoiceTypeVATElectronicNormal {
		t.Fatalf("Approve() = %#v error=%v", approved, err)
	}
	replay, err := serviceValue.Approve(ctx, approvalInput)
	if err != nil || !replay.Applied || !replay.Reused || replay.Request.Version != 2 {
		t.Fatalf("Approve(replay) = %#v error=%v", replay, err)
	}
	changed := approvalInput
	changed.InvoiceTypeCode = InvoiceTypeDigitalNormal
	if _, err = serviceValue.Approve(ctx, changed); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("Approve(changed replay) error=%v", err)
	}
	concurrentNo := prefix + "concurrent"
	insertManagementRequest(t, database, concurrentNo, StatusPendingReview, 1, "", false, false)
	concurrentInput := ReviewInput{
		TaxInvoiceNo: concurrentNo, InvoiceTypeCode: InvoiceTypeVATElectronicNormal,
		ExpectedVersion: 1, IdempotencyKey: prefix + "concurrent-key",
	}
	var wait sync.WaitGroup
	errorsByCall := make(chan error, 8)
	resultsByCall := make(chan *Result, 8)
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, callErr := serviceValue.Approve(ctx, concurrentInput)
			resultsByCall <- result
			errorsByCall <- callErr
		}()
	}
	wait.Wait()
	close(resultsByCall)
	close(errorsByCall)
	for callErr := range errorsByCall {
		if callErr != nil {
			t.Fatalf("concurrent exact replay error=%v", callErr)
		}
	}
	firstCount := 0
	replayCount := 0
	for result := range resultsByCall {
		if result == nil || !result.Applied || result.Request.Version != 2 {
			t.Fatalf("concurrent exact replay result=%#v", result)
		}
		if result.Reused {
			replayCount++
		} else {
			firstCount++
		}
	}
	if firstCount != 1 || replayCount != 7 {
		t.Fatalf("concurrent exact replay first=%d replay=%d", firstCount, replayCount)
	}
	submitted, err := serviceValue.Submit(ctx, SubmitInput{
		TaxInvoiceNo: approvedNo, ExpectedVersion: 2, IdempotencyKey: prefix + "submit-key",
	})
	if err != nil || !submitted.Applied || submitted.Request.StatusCode != StatusQueued ||
		submitted.Request.Version != 3 {
		t.Fatalf("Submit() = %#v error=%v", submitted, err)
	}

	queried, err := serviceValue.Get(ctx, approvedNo)
	if err != nil || queried.StatusCode != StatusQueued {
		t.Fatalf("Get() = %#v error=%v", queried, err)
	}
	detail, err := serviceValue.GetDetail(ctx, approvedNo)
	if err != nil || detail.Buyer.InvoiceTitle != "Buyer "+approvedNo ||
		detail.Buyer.TaxRegistrationNo != "91310000123456789X" ||
		detail.Buyer.BankAccountMasked != "****8890" {
		t.Fatalf("GetDetail() = %#v error=%v", detail, err)
	}

	rejectedNo := prefix + "rejected"
	insertManagementRequest(t, database, rejectedNo, StatusPendingReview, 1, "", false, false)
	rejected, err := serviceValue.Reject(ctx, CloseInput{
		TaxInvoiceNo: rejectedNo, ReasonCode: ReasonDuplicateRequest,
		InternalNote: "后台内部说明", ExpectedVersion: 1,
		IdempotencyKey: prefix + "reject-key",
	})
	if err != nil || rejected.Request.StatusCode != StatusRejected ||
		rejected.Request.DecisionReasonCode != ReasonDuplicateRequest ||
		rejected.Request.DecisionInternalNote != "后台内部说明" {
		t.Fatalf("Reject() = %#v error=%v", rejected, err)
	}

	for _, test := range []struct {
		name      string
		status    string
		version   int64
		external  bool
		confirmed bool
		wantAllow bool
	}{
		{name: "pending", status: StatusPendingReview, version: 1, wantAllow: true},
		{name: "approved", status: StatusApprovedPendingSubmission, version: 2, wantAllow: true},
		{name: "failed confirmed", status: StatusFailed, version: 3, external: true, confirmed: true, wantAllow: true},
		{name: "failed unconfirmed", status: StatusFailed, version: 3, external: true},
		{name: "queued", status: StatusQueued, version: 3},
		{name: "submitting", status: "submitting", version: 4},
		{name: "unknown", status: "result_unknown", version: 5},
		{name: "issued", status: "issued", version: 6},
		{name: "delivered", status: "delivered", version: 7},
		{name: "red pending", status: "red_letter_pending", version: 8},
		{name: "red lettered", status: "red_lettered", version: 9},
	} {
		t.Run(test.name, func(t *testing.T) {
			number := prefix + "matrix-" + strings.ReplaceAll(test.name, " ", "-")
			keySuffix := strings.ReplaceAll(test.name, " ", "-")
			issued := test.status == "issued" || test.status == "delivered" ||
				test.status == "red_letter_pending" || test.status == "red_lettered"
			insertManagementRequest(
				t, database, number, test.status, test.version, "",
				issued || test.external, test.confirmed,
			)
			result, cancelErr := serviceValue.Cancel(ctx, CloseInput{
				TaxInvoiceNo: number, ReasonCode: ReasonCustomerRequestedCancellation,
				ExpectedVersion: test.version, IdempotencyKey: prefix + "cancel-" + keySuffix,
			})
			if test.wantAllow {
				if cancelErr != nil || !result.Applied || result.Request.StatusCode != StatusCanceled {
					t.Fatalf("Cancel() = %#v error=%v", result, cancelErr)
				}
			} else if !bizerr.Is(cancelErr, CodeConflict) {
				t.Fatalf("Cancel() error=%v, want conflict", cancelErr)
			}
		})
	}

	rollbackNo := prefix + "rollback"
	insertManagementRequest(t, database, rollbackNo, StatusPendingReview, 1, "", false, false)
	failingAudit := &managementAudit{fail: true}
	failingService, err := New(database, managementcap.Runtime{
		Authorizer: authorizer, Audit: failingAudit,
	}, fixedGate{capability: ready}, managementSnapshotAccessor{})
	if err != nil {
		t.Fatalf("New(failing audit) error = %v", err)
	}
	if _, err = failingService.Approve(ctx, ReviewInput{
		TaxInvoiceNo: rollbackNo, InvoiceTypeCode: InvoiceTypeVATElectronicNormal,
		ExpectedVersion: 1, IdempotencyKey: prefix + "rollback-key",
	}); !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("Approve(audit failure) error=%v", err)
	}
	assertManagementState(t, database, rollbackNo, StatusPendingReview, 1)
	actionCount, err := database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_compliance_tax_invoice_action
WHERE tenant_id = 0 AND tax_invoice_no = ?`, rollbackNo)
	if err != nil || actionCount.Int() != 0 {
		t.Fatalf("rolled-back action count=%d error=%v", actionCount.Int(), err)
	}

	authorizer.mu.Lock()
	permissions := make(map[string]bool, len(authorizer.requests))
	for _, request := range authorizer.requests {
		permissions[request.Permission] = true
	}
	authorizer.mu.Unlock()
	for _, permission := range []string{PermissionQuery, PermissionReview, PermissionSubmit, PermissionCancel} {
		if !permissions[permission] {
			t.Errorf("permission %q was not enforced", permission)
		}
	}
	for _, transition := range audit.transitions {
		if strings.Contains(strings.ToLower(strings.TrimSpace(transition.ReasonCode)), "后台内部说明") {
			t.Fatal("administrator-only note leaked into audit reason")
		}
	}
}

func openManagementTestDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(managementTestDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run tax invoice management PostgreSQL tests", managementTestDSNEnv)
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open management test admin database failed: %v", err)
	}
	suffix := make([]byte, 8)
	if _, err = rand.Read(suffix); err != nil {
		t.Fatalf("create management test schema suffix failed: %v", err)
	}
	schema := "idc_tax_invoice_mgmt_test_" + hex.EncodeToString(suffix)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create management test schema failed: %v", err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "idc_tax_invoice_mgmt_test_") {
			if _, dropErr := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); dropErr != nil {
				t.Errorf("drop management test schema failed: %v", dropErr)
			}
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close management test admin database failed: %v", closeErr)
		}
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatalf("open scoped management test database failed: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close management test database failed: %v", closeErr)
		}
	})
	if _, err = database.Exec(ctx, `
CREATE TABLE sys_dict_type (
    tenant_id INT NOT NULL, name VARCHAR(128) NOT NULL, type VARCHAR(128) NOT NULL,
    status INT NOT NULL, is_builtin INT NOT NULL, allow_tenant_override BOOLEAN NOT NULL,
    remark VARCHAR(512) NOT NULL, created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL, UNIQUE (tenant_id, type)
);
CREATE TABLE sys_dict_data (
    tenant_id INT NOT NULL, dict_type VARCHAR(128) NOT NULL, label VARCHAR(256) NOT NULL,
    value VARCHAR(128) NOT NULL, sort INT NOT NULL, tag_style VARCHAR(32) NOT NULL,
    status INT NOT NULL, is_builtin INT NOT NULL, created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL, UNIQUE (tenant_id, dict_type, value)
);`); err != nil {
		t.Fatalf("create management dictionary fixtures failed: %v", err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve management test path failed")
	}
	pluginRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	for _, name := range []string{"001-compliance-foundation.sql", "019-tax-invoice-request.sql"} {
		payload, readErr := os.ReadFile(filepath.Join(pluginRoot, "manifest", "sql", name))
		if readErr != nil {
			t.Fatalf("read management migration %s failed: %v", name, readErr)
		}
		if _, execErr := database.Exec(ctx, string(payload)); execErr != nil {
			t.Fatalf("execute management migration %s failed: %v", name, execErr)
		}
	}
	return database
}

func managementTestPrefix(t *testing.T) string {
	t.Helper()
	value := make([]byte, 6)
	if _, err := rand.Read(value); err != nil {
		t.Fatalf("create test prefix failed: %v", err)
	}
	return "tim_" + hex.EncodeToString(value) + "_"
}

func insertManagementRequest(
	t *testing.T,
	database gdb.DB,
	taxInvoiceNo, status string,
	version int64,
	decisionReason string,
	hasProviderEvidence bool,
	notIssuedConfirmed bool,
) {
	t.Helper()
	buyerPayload, err := json.Marshal(managementBuyerProfile(taxInvoiceNo))
	if err != nil {
		t.Fatalf("encode buyer snapshot failed: %v", err)
	}
	defer clear(buyerPayload)
	buyerDigest := buyerSnapshotDigest(buyerPayload)
	invoiceType := InvoiceTypePendingConfirmation
	provider := any(nil)
	external := any(nil)
	issuedAt := any(nil)
	notIssuedConfirmedAt := any(nil)
	if status != StatusPendingReview {
		invoiceType = InvoiceTypeVATElectronicNormal
		provider = ProviderNuoNuo
	}
	if hasProviderEvidence {
		external = taxInvoiceNo + "-external"
		if status == "issued" || status == "delivered" ||
			status == "red_letter_pending" || status == "red_lettered" {
			issuedAt = "2026-01-01T00:00:00Z"
		}
	}
	if notIssuedConfirmed {
		notIssuedConfirmedAt = "2026-01-01T00:00:00Z"
	}
	_, err = database.Exec(context.Background(), `
INSERT INTO plugin_idc_compliance_tax_invoice
    (tax_invoice_no, request_sha256, customer_no, principal_no,
     seller_snapshot_ref, seller_snapshot_sha256, buyer_snapshot_ref, buyer_snapshot_sha256,
     invoice_type_code, status_code, tax_currency_code, gross_amount_minor,
     net_amount_minor, tax_amount_minor, billing_currency_code, provider_code,
     external_invoice_ref, decision_reason_code, submitted_at, issued_at,
     provider_not_issued_confirmed_at, version)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'CNY', 1000, 900, 100, 'CNY', ?, ?, ?, CURRENT_TIMESTAMP, ?, ?, ?)`,
		taxInvoiceNo, sha256Hex([]byte(taxInvoiceNo+"request")), taxInvoiceNo+"customer",
		taxInvoiceNo+"principal", "private://"+taxInvoiceNo+"/seller",
		sha256Hex([]byte(taxInvoiceNo+"seller")), "private://"+taxInvoiceNo+"/buyer",
		buyerDigest, invoiceType, status, provider, external,
		decisionReason, issuedAt, notIssuedConfirmedAt, version)
	if err != nil {
		t.Fatalf("insert request %q failed: %v", taxInvoiceNo, err)
	}
}

func managementBuyerProfile(taxInvoiceNo string) taxinvoicecap.ProfileSnapshot {
	return taxinvoicecap.ProfileSnapshot{
		ProfileSummary: taxinvoicecap.ProfileSummary{
			ProfileNo: taxInvoiceNo + "profile", TitleTypeCode: "enterprise",
			InvoiceTitle: "Buyer " + taxInvoiceNo, TaxRegistrationNo: "91310000123456789X",
			RegisteredAddress: "Shanghai", RegisteredPhone: "021-12345678",
			BankName: "Example Bank", BankAccountMasked: "****8890", Version: 2,
		},
		BankAccountSecretRef: "secret-must-never-leave-backend",
	}
}

func assertManagementState(
	t *testing.T,
	database gdb.DB,
	taxInvoiceNo, status string,
	version int64,
) {
	t.Helper()
	var row *struct {
		StatusCode string `json:"status_code"`
		Version    int64  `json:"version"`
	}
	if err := database.GetScan(context.Background(), &row, `
SELECT status_code, version FROM plugin_idc_compliance_tax_invoice
WHERE tenant_id = 0 AND tax_invoice_no = ?`, taxInvoiceNo); err != nil || row == nil {
		t.Fatalf("load request state failed: row=%#v error=%v", row, err)
	}
	if row.StatusCode != status || row.Version != version {
		t.Fatalf("state=(%s,%d), want=(%s,%d)", row.StatusCode, row.Version, status, version)
	}
}
