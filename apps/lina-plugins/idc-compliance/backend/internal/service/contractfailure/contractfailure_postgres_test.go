package contractfailure

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/contractcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

type compensationFake struct {
	inputs []contractcap.FailureCompensationInput
}

type failingCompensationFake struct {
	calls int
}

func (f *failingCompensationFake) CompensateContractFailure(
	context.Context,
	contractcap.FailureCompensationInput,
) (*contractcap.FailureCompensationResult, error) {
	f.calls++
	return nil, errors.New("payment result is unknown")
}

func (f *compensationFake) CompensateContractFailure(
	_ context.Context,
	input contractcap.FailureCompensationInput,
) (*contractcap.FailureCompensationResult, error) {
	f.inputs = append(f.inputs, input)
	return &contractcap.FailureCompensationResult{
		CompensationNo:  "ccp_0123456789abcdef0123456789abcdef",
		RefundRequestNo: "rfr_0123456789abcdef0123456789abcdef",
		OrderNo:         input.OrderNo, StatusCode: "refund_pending",
		ReasonCode: "required_contract_terminal",
	}, nil
}

type auditFake struct {
	transitions []idccap.BusinessAuditTransition
}

type authorizerFake struct{}

func (authorizerFake) Authorize(
	context.Context,
	managementcap.AuthorizationRequest,
) (int, error) {
	return 1, nil
}

func (f *auditFake) AppendBusinessTransition(_ context.Context, input idccap.BusinessAuditTransition) error {
	f.transitions = append(f.transitions, input)
	return nil
}

func (*auditFake) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

func TestDueAndImmediateContractFailureClaimsPostgreSQL(t *testing.T) {
	database, cleanup := openContractFailurePostgreSQL(t)
	defer cleanup()
	ctx := context.Background()
	insertFailureFixture(t, ctx, database, "contract-due", "case-due", "order-due", -time.Hour)
	insertFailureFixture(t, ctx, database, "contract-immediate", "case-immediate", "order-immediate", 24*time.Hour)
	compensations := &compensationFake{}
	audit := &auditFake{}
	serviceValue, err := NewManagement(database, compensations, audit, authorizerFake{})
	if err != nil {
		t.Fatal(err)
	}
	processed, err := serviceValue.ProcessDue(ctx, MaximumBatchSize)
	if err != nil || processed.Claimed != 1 || processed.RefundPending != 1 {
		t.Fatalf("process due contract failure: result=%+v err=%v", processed, err)
	}
	if _, err = serviceValue.CancelNow(ctx, "contract-immediate"); err != nil {
		t.Fatalf("cancel contract immediately: %v", err)
	}
	if len(compensations.inputs) != 2 || compensations.inputs[0].TriggerCode != "deadline" ||
		compensations.inputs[1].TriggerCode != "admin_cancel" || len(audit.transitions) != 2 {
		t.Fatalf("unexpected compensation calls=%+v audits=%d", compensations.inputs, len(audit.transitions))
	}
	var states []struct {
		ContractNo  string  `orm:"contract_no"`
		StatusCode  string  `orm:"status_code"`
		TriggerCode string  `orm:"trigger_code"`
		LeaseToken  *string `orm:"lease_token"`
	}
	if err = database.GetScan(ctx, &states, `
SELECT contract_no, status_code, trigger_code, lease_token
FROM plugin_idc_compliance_contract_failure_case
ORDER BY contract_no`); err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 || states[0].StatusCode != "refund_pending" ||
		states[1].StatusCode != "refund_pending" || states[0].LeaseToken != nil ||
		states[1].LeaseToken != nil {
		t.Fatalf("unexpected failure case states: %+v", states)
	}
	policy, err := serviceValue.GetPolicy(ctx)
	if err != nil || policy.PolicyVersion != 1 || policy.HandlingDays != 3 {
		t.Fatalf("load default contract failure policy: policy=%+v err=%v", policy, err)
	}
	updated, err := serviceValue.UpdatePolicy(ctx, PolicyUpdateInput{
		ExpectedVersion: 1, HandlingDays: 0,
		IdempotencyKey: "contract-failure-policy-0001",
	})
	if err != nil || updated.PolicyVersion != 2 || updated.HandlingDays != 0 {
		t.Fatalf("update contract failure policy: policy=%+v err=%v", updated, err)
	}
	replay, err := serviceValue.UpdatePolicy(ctx, PolicyUpdateInput{
		ExpectedVersion: 1, HandlingDays: 0,
		IdempotencyKey: "contract-failure-policy-0001",
	})
	if err != nil || !replay.Reused || replay.PolicyNo != updated.PolicyNo {
		t.Fatalf("replay contract failure policy: policy=%+v err=%v", replay, err)
	}
}

func TestUnknownCompensationStopsAfterBoundedRetriesPostgreSQL(t *testing.T) {
	database, cleanup := openContractFailurePostgreSQL(t)
	defer cleanup()
	ctx := context.Background()
	insertFailureFixture(t, ctx, database, "contract-unknown", "case-unknown", "order-unknown", -time.Hour)
	compensations := &failingCompensationFake{}
	serviceValue, err := New(database, compensations, &auditFake{})
	if err != nil {
		t.Fatal(err)
	}
	processed, err := serviceValue.ProcessDue(ctx, MaximumBatchSize)
	if err != nil || processed.Claimed != maximumAttempts || processed.Retried != maximumAttempts-1 ||
		processed.ManualReview != 1 || compensations.calls != maximumAttempts {
		t.Fatalf("bounded unknown result=%+v calls=%d error=%v", processed, compensations.calls, err)
	}
	var state struct {
		AttemptCount int     `orm:"attempt_count"`
		StatusCode   string  `orm:"status_code"`
		LeaseToken   *string `orm:"lease_token"`
	}
	if err = database.GetScan(ctx, &state, `
SELECT attempt_count, status_code, lease_token
FROM plugin_idc_compliance_contract_failure_case
WHERE contract_no = 'contract-unknown'`); err != nil {
		t.Fatal(err)
	}
	if state.AttemptCount != maximumAttempts || state.StatusCode != "manual_review" || state.LeaseToken != nil {
		t.Fatalf("unexpected unknown compensation state: %+v", state)
	}
}

func insertFailureFixture(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	contractNo string,
	caseNo string,
	orderNo string,
	deadlineOffset time.Duration,
) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract
    (contract_no, order_no, required_for_fulfillment)
VALUES (?, ?, TRUE)`, contractNo, orderNo); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_contract_failure_case
    (case_no, contract_no, order_no, terminal_event_no, terminal_status_code,
     policy_no, policy_version, handling_days, handling_deadline, created_at)
VALUES (?, ?, ?, ?, 'rejected', 'cfp_default_v1', 1, 3, ?, ?)`,
		caseNo, contractNo, orderNo, "event-"+caseNo,
		time.Now().UTC().Add(deadlineOffset), time.Now().UTC().Add(-72*time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func openContractFailurePostgreSQL(t *testing.T) (gdb.DB, func()) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_COMPLIANCE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_COMPLIANCE_TEST_DSN to run contract failure PostgreSQL tests")
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	var randomBytes [8]byte
	if _, err = rand.Read(randomBytes[:]); err != nil {
		t.Fatal(err)
	}
	schema := "idc_contract_failure_test_" + hex.EncodeToString(randomBytes[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `
CREATE TABLE plugin_idc_compliance_contract (
    contract_no VARCHAR(64) PRIMARY KEY,
    order_no VARCHAR(64) NOT NULL,
    required_for_fulfillment BOOLEAN NOT NULL
)`); err != nil {
		t.Fatal(err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve contract failure migration")
	}
	migrationPath := filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "025-contract-failure-compensation.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("execute contract failure migration attempt %d: %v", attempt, err)
		}
	}
	return database, func() {
		_ = database.Close(ctx)
		_, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
		_ = admin.Close(ctx)
	}
}
