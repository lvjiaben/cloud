// This file verifies refund-to-red-letter behavior against isolated PostgreSQL schemas.

package taxinvoiceredletter

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

const redLetterPostgreSQLDSNEnvironment = "LINAPRO_IDC_COMPLIANCE_TEST_DSN"

type redLetterSource struct {
	operations map[string]*taxinvoicecap.RefundOperation
	references []taxinvoicecap.PostedRefundReference
}

func (*redLetterSource) ListCurrentCustomerOptions(
	context.Context,
	taxinvoicecap.AllocationUsageReader,
) (*taxinvoicecap.RequestOptions, error) {
	return nil, errors.New("request options are outside the red-letter test")
}

func (*redLetterSource) PrepareCurrentCustomerRequest(
	context.Context,
	taxinvoicecap.AllocationUsageReader,
	string,
	[]taxinvoicecap.Selection,
) (*taxinvoicecap.RequestSnapshot, error) {
	return nil, errors.New("request preparation is outside the red-letter test")
}

func (s *redLetterSource) GetPostedRefund(
	_ context.Context,
	operationNo string,
) (*taxinvoicecap.RefundOperation, error) {
	operation := s.operations[operationNo]
	if operation == nil {
		return nil, errors.New("posted refund not found")
	}
	copyValue := *operation
	copyValue.Allocations = append([]taxinvoicecap.RefundAllocation(nil), operation.Allocations...)
	return &copyValue, nil
}

func (s *redLetterSource) ListPostedRefundOperations(
	_ context.Context,
	afterCursor string,
	limit int,
) ([]taxinvoicecap.PostedRefundReference, error) {
	start := 0
	if afterCursor != "" {
		start = -1
		for index, reference := range s.references {
			if reference.Cursor == afterCursor {
				start = index + 1
				break
			}
		}
		if start < 0 {
			return nil, errors.New("unknown posted refund cursor")
		}
	}
	end := min(start+limit, len(s.references))
	return append([]taxinvoicecap.PostedRefundReference(nil), s.references[start:end]...), nil
}

type redLetterCustomer struct {
	mu         sync.Mutex
	customerNo string
}

func (c *redLetterCustomer) CurrentCustomer(context.Context) (*identitycap.CustomerSubject, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return &identitycap.CustomerSubject{
		CustomerNo: c.customerNo, CustomerTypeCode: "enterprise",
		PrincipalReference: "principal-1", ActorUserID: 81,
	}, nil
}

func (c *redLetterCustomer) setCustomerNo(customerNo string) {
	c.mu.Lock()
	c.customerNo = customerNo
	c.mu.Unlock()
}

type redLetterAuthorizer struct {
	mu          sync.Mutex
	permissions []string
}

func (a *redLetterAuthorizer) Authorize(
	_ context.Context,
	request managementcap.AuthorizationRequest,
) (int, error) {
	if err := managementcap.ValidateAuthorizationRequest(request); err != nil {
		return 0, err
	}
	a.mu.Lock()
	a.permissions = append(a.permissions, request.Permission)
	a.mu.Unlock()
	return 73, nil
}

type redLetterAudit struct {
	mu          sync.Mutex
	transitions []idccap.BusinessAuditTransition
	fail        bool
}

func (a *redLetterAudit) AppendBusinessTransition(
	_ context.Context,
	transition idccap.BusinessAuditTransition,
) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.fail {
		return gerror.New("forced red-letter audit failure")
	}
	a.transitions = append(a.transitions, transition)
	return nil
}

func (*redLetterAudit) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

type redLetterGateway struct {
	mu     sync.Mutex
	result AuthoritativeResult
	calls  int
}

func (*redLetterGateway) CustomerAction(context.Context, string, string) (string, error) {
	return "https://provider.example/red-letter/action", nil
}

func (g *redLetterGateway) Recheck(context.Context, string, string) (AuthoritativeResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	return g.result, nil
}

func (g *redLetterGateway) callCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls
}

func TestRedLetterLifecyclePostgreSQL(t *testing.T) {
	database := openRedLetterTestDatabase(t)
	ctx := context.Background()
	source := &redLetterSource{operations: make(map[string]*taxinvoicecap.RefundOperation)}
	customer := &redLetterCustomer{customerNo: "customer-1"}
	authorizer := new(redLetterAuthorizer)
	audit := new(redLetterAudit)
	gateway := new(redLetterGateway)
	serviceValue, err := New(database, source, customer, managementcap.Runtime{
		Authorizer: authorizer, Audit: audit,
	}, audit, audit, gateway)
	if err != nil {
		t.Fatalf("construct red-letter service: %v", err)
	}
	service := serviceValue.(*serviceImpl)
	service.now = func() time.Time { return time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC) }

	seedIssuedTaxInvoice(t, database, "tax-unique", "billing-unique", "billing-item-unique", 1000, 900)
	source.operations["refund-unique-1"] = postedRefund(
		"refund-unique-1", "customer-1", "refund-allocation-unique-1", "billing-unique", "billing-item-unique", 400,
	)
	created, err := service.RecordPostedRefund(ctx, "refund-unique-1", "record-refund-unique-key-0001")
	if err != nil || created.Task == nil || created.Reused || !created.Task.AllocationResolved ||
		created.Task.GrossAmountMinor != 400 || created.Task.AllocationCount != 1 ||
		created.Task.StatusCode != StatusManualRequired {
		t.Fatalf("record unique refund = %#v error=%v", created, err)
	}
	replayed, err := service.RecordPostedRefund(ctx, "refund-unique-1", "record-refund-unique-key-0001")
	if err != nil || replayed.Task == nil || !replayed.Reused || replayed.Task.TaskNo != created.Task.TaskNo {
		t.Fatalf("replay unique refund = %#v error=%v", replayed, err)
	}
	if _, err = service.RecordPostedRefund(ctx, "refund-unique-1", "record-refund-changed-key-0001"); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("changed refund idempotency error=%v want conflict", err)
	}
	if _, err = service.CompleteManual(ctx, ManualCompletionInput{
		TaskNo: created.Task.TaskNo, ExpectedVersion: created.Task.Version,
		IdempotencyKey: "complete-unique-key-0001", EvidenceReference: "proof-unique-1",
		EvidenceSHA256: strings.Repeat("a", 64), OccurredAt: service.now(),
	}); err != nil {
		t.Fatalf("complete unique red-letter: %v", err)
	}
	assertTaxInvoiceState(t, database, "tax-unique", "partially_red_lettered", 400, 400)
	completedReplay, err := service.CompleteManual(ctx, ManualCompletionInput{
		TaskNo: created.Task.TaskNo, ExpectedVersion: created.Task.Version,
		IdempotencyKey: "complete-unique-key-0001", EvidenceReference: "proof-unique-1",
		EvidenceSHA256: strings.Repeat("a", 64), OccurredAt: service.now(),
	})
	if err != nil || completedReplay.StatusCode != StatusCompleted {
		t.Fatalf("replay manual completion = %#v error=%v", completedReplay, err)
	}

	source.operations["refund-unique-2"] = postedRefund(
		"refund-unique-2", "customer-1", "refund-allocation-unique-2", "billing-unique", "billing-item-unique", 600,
	)
	second, err := service.RecordPostedRefund(ctx, "refund-unique-2", "record-refund-unique-key-0002")
	if err != nil || second.Task == nil || second.Task.TaskNo == created.Task.TaskNo {
		t.Fatalf("record second refund = %#v error=%v", second, err)
	}
	if _, err = service.CompleteManual(ctx, ManualCompletionInput{
		TaskNo: second.Task.TaskNo, ExpectedVersion: second.Task.Version,
		IdempotencyKey: "complete-unique-key-0002", EvidenceReference: "proof-unique-2",
		EvidenceSHA256: strings.Repeat("b", 64), OccurredAt: service.now(),
	}); err != nil {
		t.Fatalf("complete full red-letter: %v", err)
	}
	assertTaxInvoiceState(t, database, "tax-unique", "red_lettered", 1000, 1000)
	assertAppendOnlyRedLetterHistory(t, database, created.Task.TaskNo)

	seedIssuedTaxInvoice(t, database, "tax-ambiguous-a", "billing-ambiguous-a", "billing-item-shared", 300, 270)
	seedIssuedTaxInvoice(t, database, "tax-ambiguous-b", "billing-ambiguous-b", "billing-item-shared", 700, 630)
	source.operations["refund-ambiguous"] = postedRefund(
		"refund-ambiguous", "customer-1", "refund-allocation-ambiguous", "billing-origin", "billing-item-shared", 500,
	)
	ambiguous, err := service.RecordPostedRefund(ctx, "refund-ambiguous", "record-refund-ambiguous-key")
	if err != nil || ambiguous.Task == nil || ambiguous.Task.AllocationResolved ||
		ambiguous.Task.ManualReasonCode != ManualReasonAllocationSelection || ambiguous.Task.CandidateCount != 2 {
		t.Fatalf("record ambiguous refund = %#v error=%v", ambiguous, err)
	}
	resolved, err := service.ResolveAllocation(ctx, ResolveInput{
		TaskNo: ambiguous.Task.TaskNo, ExpectedVersion: ambiguous.Task.Version,
		IdempotencyKey: "resolve-ambiguous-key-0001",
		Selections: []AllocationSelection{
			{RefundAllocationNo: "refund-allocation-ambiguous", TaxInvoiceAllocationNo: "allocation-tax-ambiguous-a", GrossAmountMinor: 300},
			{RefundAllocationNo: "refund-allocation-ambiguous", TaxInvoiceAllocationNo: "allocation-tax-ambiguous-b", GrossAmountMinor: 200},
		},
	})
	if err != nil || !resolved.AllocationResolved || resolved.AllocationCount != 2 || resolved.Version != 2 {
		t.Fatalf("resolve ambiguous refund = %#v error=%v", resolved, err)
	}
	changedSelection := ResolveInput{
		TaskNo: ambiguous.Task.TaskNo, ExpectedVersion: ambiguous.Task.Version,
		IdempotencyKey: "resolve-ambiguous-key-0001",
		Selections: []AllocationSelection{{
			RefundAllocationNo:     "refund-allocation-ambiguous",
			TaxInvoiceAllocationNo: "allocation-tax-ambiguous-a", GrossAmountMinor: 500,
		}},
	}
	if _, err = service.ResolveAllocation(ctx, changedSelection); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("changed allocation replay error=%v want conflict", err)
	}
	if _, err = service.CompleteManual(ctx, ManualCompletionInput{
		TaskNo: resolved.TaskNo, ExpectedVersion: resolved.Version,
		IdempotencyKey: "complete-ambiguous-key", EvidenceReference: "proof-ambiguous",
		EvidenceSHA256:    strings.Repeat("c", 64),
		UploadedObjectRef: "private://red-letter/proof-ambiguous", MediaType: "application/pdf",
		ByteSize: 1200, OccurredAt: service.now(),
	}); err != nil {
		t.Fatalf("complete ambiguous red-letter: %v", err)
	}
	assertTaxInvoiceState(t, database, "tax-ambiguous-a", "red_lettered", 300, 300)
	assertTaxInvoiceState(t, database, "tax-ambiguous-b", "partially_red_lettered", 200, 200)

	seedIssuedTaxInvoice(t, database, "tax-sequence", "billing-sequence", "billing-item-sequence", 1000, 900)
	source.operations["refund-sequence-1"] = postedRefund(
		"refund-sequence-1", "customer-1", "refund-allocation-sequence-1", "billing-sequence", "billing-item-sequence", 300,
	)
	source.operations["refund-sequence-2"] = postedRefund(
		"refund-sequence-2", "customer-1", "refund-allocation-sequence-2", "billing-sequence", "billing-item-sequence", 200,
	)
	sequenceFirst, err := service.RecordPostedRefund(ctx, "refund-sequence-1", "record-sequence-key-0001")
	if err != nil {
		t.Fatalf("record first sequence refund: %v", err)
	}
	sequenceSecond, err := service.RecordPostedRefund(ctx, "refund-sequence-2", "record-sequence-key-0002")
	if err != nil {
		t.Fatalf("record second sequence refund: %v", err)
	}
	if _, err = service.CompleteManual(ctx, completionInput(sequenceSecond.Task, "sequence-2", service.now())); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("out-of-order completion error=%v want conflict", err)
	}
	if _, err = service.CompleteManual(ctx, completionInput(sequenceFirst.Task, "sequence-1", service.now())); err != nil {
		t.Fatalf("complete first sequence task: %v", err)
	}
	if _, err = service.CompleteManual(ctx, completionInput(sequenceSecond.Task, "sequence-2", service.now())); err != nil {
		t.Fatalf("complete second sequence task: %v", err)
	}

	seedIssuedTaxInvoice(t, database, "tax-customer", "billing-customer", "billing-item-customer", 100, 90)
	source.operations["refund-customer"] = postedRefund(
		"refund-customer", "customer-1", "refund-allocation-customer", "billing-customer", "billing-item-customer", 100,
	)
	customerTask, err := service.RecordPostedRefund(ctx, "refund-customer", "record-customer-key-0001")
	if err != nil {
		t.Fatalf("record customer recheck task: %v", err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_compliance_red_letter_task
SET status_code = 'awaiting_confirmation', manual_reason_code = '',
    customer_confirmation_required = TRUE, provider_code = 'nuonuo',
    external_task_ref = 'provider-red-letter-1', version = 2
WHERE tenant_id = 0 AND task_no = ?`, customerTask.Task.TaskNo); err != nil {
		t.Fatalf("prepare customer confirmation task: %v", err)
	}
	owned, err := service.GetCustomer(ctx, customerTask.Task.TaskNo)
	if err != nil || owned.CustomerActionURL == "" {
		t.Fatalf("get owned customer task = %#v error=%v", owned, err)
	}
	rechecked, err := service.RecheckCustomer(ctx, RecheckInput{
		TaskNo: customerTask.Task.TaskNo, ExpectedVersion: 2,
		IdempotencyKey: "customer-recheck-key-0001",
	})
	if err != nil || rechecked.StatusCode != StatusAwaitingConfirmation || rechecked.Version != 3 {
		t.Fatalf("customer authoritative recheck = %#v error=%v", rechecked, err)
	}
	if _, err = service.RecheckCustomer(ctx, RecheckInput{
		TaskNo: customerTask.Task.TaskNo, ExpectedVersion: 2,
		IdempotencyKey: "customer-recheck-key-0001",
	}); err != nil || gateway.callCount() != 1 {
		t.Fatalf("customer recheck replay error=%v provider calls=%d", err, gateway.callCount())
	}
	customer.setCustomerNo("customer-other")
	if _, err = service.GetCustomer(ctx, customerTask.Task.TaskNo); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("other customer task access error=%v", err)
	}
	customer.setCustomerNo("customer-1")
	if _, err = service.GetAdmin(ctx, customerTask.Task.TaskNo); err != nil {
		t.Fatalf("admin query task: %v", err)
	}
	assertRedLetterPermissions(t, authorizer)

	seedIssuedTaxInvoice(t, database, "tax-audit-rollback", "billing-audit-rollback", "billing-item-audit-rollback", 100, 90)
	source.operations["refund-audit-rollback"] = postedRefund(
		"refund-audit-rollback", "customer-1", "refund-allocation-audit-rollback",
		"billing-audit-rollback", "billing-item-audit-rollback", 100,
	)
	audit.mu.Lock()
	audit.fail = true
	audit.mu.Unlock()
	if _, err = service.RecordPostedRefund(ctx, "refund-audit-rollback", "record-audit-rollback-key"); !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("audit rollback error=%v want unavailable", err)
	}
	rollbackValue, rollbackErr := database.GetValue(ctx, `
SELECT COUNT(*) FROM plugin_idc_compliance_red_letter_task
WHERE tenant_id = 0 AND refund_operation_no = 'refund-audit-rollback'`)
	if rollbackErr != nil || rollbackValue.Int() != 0 {
		t.Fatalf("audit rollback task count=%d error=%v", rollbackValue.Int(), rollbackErr)
	}
}

func TestRecordPostedRefundConcurrentReplayPostgreSQL(t *testing.T) {
	database := openRedLetterTestDatabase(t)
	seedIssuedTaxInvoice(t, database, "tax-concurrent", "billing-concurrent", "billing-item-concurrent", 1000, 900)
	source := &redLetterSource{operations: map[string]*taxinvoicecap.RefundOperation{
		"refund-concurrent": postedRefund(
			"refund-concurrent", "customer-1", "refund-allocation-concurrent",
			"billing-concurrent", "billing-item-concurrent", 500,
		),
	}}
	audit := new(redLetterAudit)
	service, err := New(database, source, &redLetterCustomer{customerNo: "customer-1"},
		managementcap.Runtime{Authorizer: new(redLetterAuthorizer), Audit: audit},
		audit, audit, new(redLetterGateway))
	if err != nil {
		t.Fatalf("construct concurrent red-letter service: %v", err)
	}
	start := make(chan struct{})
	results := make(chan *RecordResult, 8)
	errorsByCall := make(chan error, 8)
	for range 8 {
		go func() {
			<-start
			result, callErr := service.RecordPostedRefund(
				context.Background(), "refund-concurrent", "record-concurrent-key-0001",
			)
			results <- result
			errorsByCall <- callErr
		}()
	}
	close(start)
	var first, replay int
	for range 8 {
		if callErr := <-errorsByCall; callErr != nil {
			t.Fatalf("concurrent refund error: %v", callErr)
		}
		result := <-results
		if result == nil || result.Task == nil {
			t.Fatalf("concurrent refund result: %#v", result)
		}
		if result.Reused {
			replay++
		} else {
			first++
		}
	}
	if first != 1 || replay != 7 {
		t.Fatalf("concurrent refund first=%d replay=%d", first, replay)
	}
}

func TestRedLetterIntakeDurableCursorPostgreSQL(t *testing.T) {
	database := openRedLetterTestDatabase(t)
	ctx := context.Background()
	seedIssuedTaxInvoice(t, database, "tax-intake-1", "billing-intake-1", "billing-item-intake-1", 100, 90)
	seedIssuedTaxInvoice(t, database, "tax-intake-2", "billing-intake-2", "billing-item-intake-2", 200, 180)
	source := &redLetterSource{
		operations: map[string]*taxinvoicecap.RefundOperation{
			"refund-intake-1": postedRefund(
				"refund-intake-1", "customer-1", "refund-allocation-intake-1",
				"billing-intake-1", "billing-item-intake-1", 100,
			),
			"refund-intake-2": postedRefund(
				"refund-intake-2", "customer-1", "refund-allocation-intake-2",
				"billing-intake-2", "billing-item-intake-2", 200,
			),
		},
		references: []taxinvoicecap.PostedRefundReference{
			{RefundOperationNo: "refund-intake-1", Cursor: "rfc_0000000000000001"},
			{RefundOperationNo: "refund-intake-2", Cursor: "rfc_0000000000000002"},
		},
	}
	audit := new(redLetterAudit)
	service, err := New(
		database, source, &redLetterCustomer{customerNo: "customer-1"},
		managementcap.Runtime{Authorizer: new(redLetterAuthorizer), Audit: audit},
		audit, audit, new(redLetterGateway),
	)
	if err != nil {
		t.Fatalf("construct red-letter intake service: %v", err)
	}
	worker, err := NewIntakeWorker(database, source, service)
	if err != nil {
		t.Fatalf("construct red-letter intake worker: %v", err)
	}
	processed, err := worker.Run(ctx, 2)
	if err != nil || processed != 2 {
		t.Fatalf("run red-letter intake processed=%d error=%v", processed, err)
	}
	taskCount, err := database.GetValue(ctx, `
SELECT COUNT(*)
FROM plugin_idc_compliance_red_letter_task
WHERE tenant_id = 0 AND refund_operation_no IN ('refund-intake-1', 'refund-intake-2')`)
	if err != nil || taskCount.Int() != 2 {
		t.Fatalf("red-letter intake task count=%d error=%v", taskCount.Int(), err)
	}
	processed, err = worker.Run(ctx, 2)
	if err != nil || processed != 0 {
		t.Fatalf("replay red-letter intake processed=%d error=%v", processed, err)
	}

	source.references = append(source.references, taxinvoicecap.PostedRefundReference{
		RefundOperationNo: "refund-intake-3", Cursor: "rfc_0000000000000003",
	})
	if processed, err = worker.Run(ctx, 1); err == nil || processed != 0 {
		t.Fatalf("failing intake processed=%d error=%v", processed, err)
	}
	cursor, err := database.GetValue(ctx, `
SELECT source_cursor
FROM plugin_idc_compliance_red_letter_intake_cursor
WHERE tenant_id = 0 AND feed_code = 'posted_refund'`)
	if err != nil || cursor.String() != "rfc_0000000000000002" {
		t.Fatalf("red-letter intake cursor=%q error=%v", cursor.String(), err)
	}
}

func postedRefund(
	operationNo, customerNo, allocationNo, billingInvoiceNo, billingItemNo string,
	amount int64,
) *taxinvoicecap.RefundOperation {
	return &taxinvoicecap.RefundOperation{
		RefundOperationNo: operationNo, CustomerNo: customerNo, CurrencyCode: "CNY",
		PostedAt: time.Date(2026, 8, 17, 11, 0, 0, 0, time.UTC),
		Allocations: []taxinvoicecap.RefundAllocation{{
			RefundAllocationNo: allocationNo, BillingInvoiceNo: billingInvoiceNo,
			BillingInvoiceItemNo: billingItemNo, AmountMinor: amount,
			ContentDigest: strings.Repeat("f", 64),
		}},
	}
}

func completionInput(task *Task, suffix string, occurredAt time.Time) ManualCompletionInput {
	return ManualCompletionInput{
		TaskNo: task.TaskNo, ExpectedVersion: task.Version,
		IdempotencyKey:    "complete-" + suffix + "-key-0001",
		EvidenceReference: "proof-" + suffix,
		EvidenceSHA256:    strings.Repeat("d", 64), OccurredAt: occurredAt,
	}
}

func seedIssuedTaxInvoice(
	t *testing.T,
	database gdb.DB,
	taxInvoiceNo, billingInvoiceNo, billingItemNo string,
	gross, net int64,
) {
	t.Helper()
	allocationNo := "allocation-" + taxInvoiceNo
	itemNo := "item-" + taxInvoiceNo
	ctx := context.Background()
	_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_tax_invoice
    (tax_invoice_no, request_sha256, customer_no, principal_no,
     seller_snapshot_ref, seller_snapshot_sha256, buyer_snapshot_ref, buyer_snapshot_sha256,
     invoice_type_code, status_code, tax_currency_code, gross_amount_minor,
     net_amount_minor, tax_amount_minor, billing_currency_code, provider_code,
     external_invoice_ref, submitted_at, issued_at, version)
VALUES (?, ?, 'customer-1', 'principal-1', ?, ?, ?, ?, 'vat_electronic_normal',
        'issued', 'CNY', ?, ?, ?, 'CNY', 'nuonuo', ?, CURRENT_TIMESTAMP,
        CURRENT_TIMESTAMP, 1)`,
		taxInvoiceNo, redLetterDigest(taxInvoiceNo+"-request"),
		"private://"+taxInvoiceNo+"/seller", redLetterDigest(taxInvoiceNo+"-seller"),
		"private://"+taxInvoiceNo+"/buyer", redLetterDigest(taxInvoiceNo+"-buyer"),
		gross, net, gross-net, "external-"+taxInvoiceNo)
	if err != nil {
		t.Fatalf("seed issued tax invoice %q: %v", taxInvoiceNo, err)
	}
	_, err = database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_tax_invoice_allocation
    (allocation_no, tax_invoice_no, billing_invoice_no, order_no,
     allocated_gross_minor, allocated_net_minor, allocated_tax_minor)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		allocationNo, taxInvoiceNo, billingInvoiceNo, "order-"+taxInvoiceNo,
		gross, net, gross-net)
	if err != nil {
		t.Fatalf("seed tax invoice allocation %q: %v", taxInvoiceNo, err)
	}
	_, err = database.Exec(ctx, `
INSERT INTO plugin_idc_compliance_tax_invoice_item
    (item_no, tax_invoice_no, line_no, item_snapshot_ref, item_snapshot_sha256,
     tax_classification_code, tax_rate_ppm, gross_amount_minor, net_amount_minor,
     tax_amount_minor, allocation_no, billing_invoice_item_no)
VALUES (?, ?, 1, ?, ?, 'server_service', 111111, ?, ?, ?, ?, ?)`,
		itemNo, taxInvoiceNo, "private://"+taxInvoiceNo+"/item", redLetterDigest(taxInvoiceNo+"-item"),
		gross, net, gross-net, allocationNo, billingItemNo,
	)
	if err != nil {
		t.Fatalf("seed tax invoice item %q: %v", taxInvoiceNo, err)
	}
}

func assertTaxInvoiceState(
	t *testing.T,
	database gdb.DB,
	taxInvoiceNo, status string,
	refunded, redLettered int64,
) {
	t.Helper()
	var row *struct {
		Status      string `json:"status_code"`
		Refunded    int64  `json:"refunded_gross_minor"`
		RedLettered int64  `json:"red_lettered_gross_minor"`
	}
	err := database.GetScan(context.Background(), &row, `
SELECT invoice.status_code, allocation.refunded_gross_minor, allocation.red_lettered_gross_minor
FROM plugin_idc_compliance_tax_invoice invoice
JOIN plugin_idc_compliance_tax_invoice_allocation allocation
  ON allocation.tenant_id = invoice.tenant_id AND allocation.tax_invoice_no = invoice.tax_invoice_no
WHERE invoice.tenant_id = 0 AND invoice.tax_invoice_no = ?`, taxInvoiceNo)
	if err != nil || row == nil || row.Status != status || row.Refunded != refunded || row.RedLettered != redLettered {
		t.Fatalf("tax invoice %s state=%#v error=%v want status=%s refunded=%d red-lettered=%d",
			taxInvoiceNo, row, err, status, refunded, redLettered)
	}
}

func assertAppendOnlyRedLetterHistory(t *testing.T, database gdb.DB, taskNo string) {
	t.Helper()
	for _, statement := range []string{
		"UPDATE plugin_idc_compliance_red_letter_candidate SET requested_gross_minor = requested_gross_minor + 1 WHERE task_no = ?",
		"DELETE FROM plugin_idc_compliance_red_letter_action WHERE task_no = ?",
		"UPDATE plugin_idc_compliance_external_evidence SET disposition_code = 'duplicate' WHERE aggregate_no = ?",
	} {
		if _, err := database.Exec(context.Background(), statement, taskNo); err == nil {
			t.Fatalf("append-only history mutation succeeded: %s", statement)
		}
	}
}

func assertRedLetterPermissions(t *testing.T, authorizer *redLetterAuthorizer) {
	t.Helper()
	authorizer.mu.Lock()
	defer authorizer.mu.Unlock()
	seen := make(map[string]bool, len(authorizer.permissions))
	for _, permission := range authorizer.permissions {
		seen[permission] = true
	}
	for _, permission := range []string{PermissionQuery, PermissionHandle} {
		if !seen[permission] {
			t.Errorf("permission %q was not enforced", permission)
		}
	}
}

func redLetterDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func openRedLetterTestDatabase(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(redLetterPostgreSQLDSNEnvironment))
	if dsn == "" {
		t.Skipf("set %s to run red-letter PostgreSQL tests", redLetterPostgreSQLDSNEnvironment)
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open red-letter admin database: %v", err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatalf("create red-letter schema suffix: %v", err)
	}
	schema := "idc_red_letter_test_" + hex.EncodeToString(suffix[:])
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create red-letter schema: %v", err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "idc_red_letter_test_") {
			if _, dropErr := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); dropErr != nil {
				t.Errorf("drop red-letter schema: %v", dropErr)
			}
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close red-letter admin database: %v", closeErr)
		}
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatalf("open scoped red-letter database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close red-letter database: %v", closeErr)
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
		t.Fatalf("create red-letter dictionary fixtures: %v", err)
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve red-letter test path")
	}
	pluginRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	for _, name := range []string{
		"001-compliance-foundation.sql", "019-tax-invoice-request.sql", "020-tax-invoice-red-letter.sql",
	} {
		payload, readErr := os.ReadFile(filepath.Join(pluginRoot, "manifest", "sql", name))
		if readErr != nil {
			t.Fatalf("read red-letter migration %s: %v", name, readErr)
		}
		if _, execErr := database.Exec(ctx, string(payload)); execErr != nil {
			t.Fatalf("execute red-letter migration %s: %v", name, execErr)
		}
	}
	return database
}
