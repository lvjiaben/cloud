// This file verifies callback byte bounds, encrypted evidence, deduplication,
// monotonic convergence, and composite ownership against real PostgreSQL.

package suppliercallback

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"lina-platform/idc-compliance-gateway/internal/callbacksecurity"
	"lina-platform/idc-compliance-gateway/internal/migration"
	"lina-platform/idc-compliance-gateway/internal/supplierguard"
)

const supplierCallbackTestDSNEnv = "LINAPRO_IDC_COMPLIANCE_GATEWAY_TEST_DSN"

type verifierStub struct {
	mu       sync.Mutex
	template VerifiedEvent
	calls    int
}

func (v *verifierStub) Verify(_ context.Context, _ RawCallback) (VerifiedEvent, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.calls++
	result := v.template
	result.ExternalEventID = append([]byte(nil), v.template.ExternalEventID...)
	result.EvidenceFrame = append([]byte(nil), v.template.EvidenceFrame...)
	return result, nil
}

func (v *verifierStub) set(event VerifiedEvent) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.template = event
}

func TestSupplierCallbackPostgreSQLMonotonicConvergence(t *testing.T) {
	database, installationID, cleanup := openSupplierCallbackDatabase(t)
	defer cleanup()
	guard, boundary, verifier := prepareCallbackServices(t, database, installationID)
	ctx := context.Background()
	lease := acquireCallbackRoute(t, database, guard, installationID, 1, "contract.sign")

	verifier.set(callbackEvent(lease, StatusRunning, "event-running", "raw-running"))
	stale, err := acceptStubCallback(ctx, boundary)
	if err != nil || stale.DecisionCode != "stale" || stale.RouteVersion != 1 ||
		stale.DeliveryNo != "" {
		t.Fatalf("running callback=%+v err=%v", stale, err)
	}

	unknownEvent := callbackEvent(
		lease, StatusResultUnknown, "event-unknown", "raw-unknown-evidence",
	)
	verifier.set(unknownEvent)
	unknown, err := acceptStubCallback(ctx, boundary)
	if err != nil || unknown.DecisionCode != "applied" || unknown.RouteVersion != 2 ||
		unknown.ProjectedStatus != StatusResultUnknown || unknown.DeliveryNo == "" {
		t.Fatalf("unknown callback=%+v err=%v", unknown, err)
	}
	verifier.set(unknownEvent)
	replayed, err := acceptStubCallback(ctx, boundary)
	if err != nil || !replayed.Replayed || replayed.EvidenceNo != unknown.EvidenceNo ||
		replayed.DeliveryNo != unknown.DeliveryNo {
		t.Fatalf("replayed callback=%+v err=%v", replayed, err)
	}
	conflictingDuplicate := unknownEvent
	conflictingDuplicate.EvidenceFrame = []byte("changed-raw-evidence")
	verifier.set(conflictingDuplicate)
	if _, err = acceptStubCallback(ctx, boundary); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed duplicate error=%v", err)
	}

	verifier.set(callbackEvent(lease, StatusSucceeded, "event-success", "raw-success"))
	succeeded, err := acceptStubCallback(ctx, boundary)
	if err != nil || succeeded.RouteVersion != 3 ||
		succeeded.ProjectedStatus != StatusSucceeded || succeeded.ReconciliationStatus != "none" {
		t.Fatalf("success callback=%+v err=%v", succeeded, err)
	}
	verifier.set(unknownEvent)
	historicalReplay, err := acceptStubCallback(ctx, boundary)
	if err != nil || !historicalReplay.Replayed ||
		historicalReplay.EvidenceNo != unknown.EvidenceNo ||
		historicalReplay.DeliveryNo != unknown.DeliveryNo ||
		historicalReplay.RouteVersion != unknown.RouteVersion ||
		historicalReplay.ProjectedStatus != unknown.ProjectedStatus ||
		historicalReplay.ReconciliationStatus != unknown.ReconciliationStatus {
		t.Fatalf("historical replay=%+v original=%+v err=%v", historicalReplay, unknown, err)
	}
	verifier.set(callbackEvent(lease, StatusRunning, "event-late", "raw-late"))
	late, err := acceptStubCallback(ctx, boundary)
	if err != nil || late.DecisionCode != "stale" || late.RouteVersion != 3 ||
		late.ProjectedStatus != StatusSucceeded || late.DeliveryNo != "" {
		t.Fatalf("late callback=%+v err=%v", late, err)
	}
	verifier.set(callbackEvent(lease, StatusFailed, "event-conflict", "raw-conflict"))
	conflict, err := acceptStubCallback(ctx, boundary)
	if err != nil || conflict.DecisionCode != "conflict" || conflict.RouteVersion != 4 ||
		conflict.ProjectedStatus != StatusSucceeded ||
		conflict.ReconciliationStatus != "reconciliation_required" {
		t.Fatalf("terminal conflict=%+v err=%v", conflict, err)
	}

	var businessStatus string
	if err = database.QueryRowContext(ctx, `
SELECT status_code FROM idc_gateway_local_business_request WHERE request_no = $1`,
		leaseRequestNo(t, database, lease.RouteNo)).Scan(&businessStatus); err != nil ||
		businessStatus != "succeeded" {
		t.Fatalf("business status=%s err=%v", businessStatus, err)
	}
	var evidenceCount, deliveryCount int
	if err = database.QueryRowContext(ctx, `
SELECT COUNT(*) FROM idc_gateway_local_callback_evidence WHERE route_no = $1`,
		lease.RouteNo).Scan(&evidenceCount); err != nil || evidenceCount != 5 {
		t.Fatalf("evidence count=%d want=5 err=%v", evidenceCount, err)
	}
	if err = database.QueryRowContext(ctx, `
SELECT COUNT(*) FROM idc_gateway_local_route_delivery WHERE route_no = $1`,
		lease.RouteNo).Scan(&deliveryCount); err != nil || deliveryCount != 3 {
		t.Fatalf("delivery count=%d want=3 err=%v", deliveryCount, err)
	}
	var ciphertext []byte
	if err = database.QueryRowContext(ctx, `
SELECT payload_ciphertext FROM idc_gateway_local_callback_evidence
WHERE evidence_no = $1`, unknown.EvidenceNo).Scan(&ciphertext); err != nil ||
		bytes.Contains(ciphertext, []byte("raw-unknown-evidence")) {
		t.Fatalf("evidence ciphertext leaked plaintext err=%v", err)
	}
	if _, err = database.ExecContext(ctx, `
UPDATE idc_gateway_local_callback_evidence SET decision_code = 'stale'
WHERE evidence_no = $1`, unknown.EvidenceNo); err == nil {
		t.Fatal("database allowed callback evidence mutation")
	}
	if _, err = database.ExecContext(ctx, `
INSERT INTO idc_gateway_local_callback_evidence
    (evidence_no, route_no, request_no, installation_id, provider_code,
     operation_scope, event_key_version, external_event_hmac,
     payload_ciphertext, payload_nonce, payload_sha256,
     cipher_format_version, encryption_key_version, normalized_status,
     projected_status, reconciliation_status, decision_code, route_version)
VALUES ($1, $2, $3, $4, $5, $6, 1, $7, $8, $9, $10, 1, 1,
		'running', 'running', 'none', 'stale', 2)`,
		"lev_"+strings.Repeat("9", 32), lease.RouteNo,
		leaseRequestNo(t, database, lease.RouteNo), "linst_"+strings.Repeat("9", 64),
		ProviderTencentESSBasic, "contract.sign", bytes.Repeat([]byte{9}, 32),
		bytes.Repeat([]byte{9}, 17), bytes.Repeat([]byte{9}, 12),
		bytes.Repeat([]byte{9}, 32)); err == nil {
		t.Fatal("database allowed callback evidence with a mismatched route owner")
	}

	secondLease := acquireCallbackRoute(t, database, guard, installationID, 6, "contract.sign")
	secondRequestNo := leaseRequestNo(t, database, secondLease.RouteNo)
	if _, err = database.ExecContext(ctx, `
UPDATE idc_gateway_local_supplier_route
SET projected_status = 'result_unknown', reconciliation_status = 'query_required',
    version = 2, updated_at = CURRENT_TIMESTAMP
WHERE route_no = $1`, secondLease.RouteNo); err != nil {
		t.Fatalf("prepare composite delivery fixture: %v", err)
	}
	var evidenceSHA []byte
	if err = database.QueryRowContext(ctx, `
SELECT payload_sha256
FROM idc_gateway_local_callback_evidence
WHERE evidence_no = $1`, unknown.EvidenceNo).Scan(&evidenceSHA); err != nil {
		t.Fatalf("read callback evidence digest: %v", err)
	}
	if _, err = database.ExecContext(ctx, `
INSERT INTO idc_gateway_local_route_delivery
    (delivery_no, route_no, request_no, installation_id, provider_code,
     operation_scope, route_version, projected_status, reconciliation_status,
     evidence_no, evidence_sha256)
VALUES ($1, $2, $3, $4, $5, $6, 2, 'result_unknown', 'query_required', $7, $8)`,
		"ldv_"+strings.Repeat("9", 32), secondLease.RouteNo, secondRequestNo,
		installationID, ProviderTencentESSBasic, "contract.sign",
		unknown.EvidenceNo, evidenceSHA); err == nil {
		t.Fatal("database allowed delivery to reference another route's evidence")
	}
	if verifier.calls != 8 {
		t.Fatalf("verifier calls=%d want=8", verifier.calls)
	}
}

func TestTencentExactStatusDeliveriesPostgreSQL(t *testing.T) {
	database, installationID, cleanup := openSupplierCallbackDatabase(t)
	defer cleanup()
	guard, boundary, verifier := prepareCallbackServices(t, database, installationID)
	ctx := context.Background()
	lease := acquireCallbackRoute(t, database, guard, installationID, 71, "contract.sign")

	initialized := callbackEvent(lease, StatusRunning, "event-init", "raw-init")
	initialized.ProviderStatus = "INIT"
	initialized.SupplierObservedAt = time.Now().UTC().Add(-2 * time.Second)
	verifier.set(initialized)
	first, err := acceptStubCallback(ctx, boundary)
	if err != nil || first.RouteVersion != 2 || first.DeliveryNo == "" || first.DecisionCode != "applied" ||
		first.ProviderStatus != "INIT" {
		t.Fatalf("exact INIT callback=%+v err=%v", first, err)
	}

	partial := callbackEvent(lease, StatusRunning, "event-part", "raw-part")
	partial.ProviderStatus = "PART"
	partial.SupplierObservedAt = time.Now().UTC().Add(-time.Second)
	verifier.set(partial)
	second, err := acceptStubCallback(ctx, boundary)
	if err != nil || second.RouteVersion != 3 || second.DeliveryNo == "" || second.DecisionCode != "applied" ||
		second.ProviderStatus != "PART" {
		t.Fatalf("exact PART callback=%+v err=%v", second, err)
	}
	lateInitialized := callbackEvent(lease, StatusRunning, "event-late-init", "raw-late-init")
	lateInitialized.ProviderStatus = "INIT"
	lateInitialized.SupplierObservedAt = time.Now().UTC().Add(-3 * time.Second)
	verifier.set(lateInitialized)
	late, err := acceptStubCallback(ctx, boundary)
	if err != nil || late.RouteVersion != 4 || late.DeliveryNo == "" ||
		late.DecisionCode != "stale" || late.ProviderStatus != "INIT" {
		t.Fatalf("late exact INIT callback=%+v err=%v", late, err)
	}

	keyFile, err := callbacksecurity.Generate(rand.Reader)
	if err != nil {
		t.Fatalf("generate exact delivery keyring: %v", err)
	}
	ring, err := callbacksecurity.New(keyFile)
	if err != nil {
		t.Fatalf("construct exact delivery keyring: %v", err)
	}
	defer ring.Clear()
	query, err := NewQueryService(
		database, installationID, ring, map[string]ReadOnlyQuerier{},
	)
	if err != nil {
		t.Fatalf("construct exact delivery query service: %v", err)
	}
	deliveries, err := query.PendingDeliveries(
		ctx, ProviderTencentESSBasic, "contract.sign", MaximumDeliveryBatchSize,
	)
	if err != nil || len(deliveries) != 3 || deliveries[0].ProviderStatus != "INIT" ||
		deliveries[1].ProviderStatus != "PART" || deliveries[2].DecisionCode != "stale" {
		t.Fatalf("pending exact deliveries=%+v err=%v", deliveries, err)
	}
	numbers := []string{deliveries[2].DeliveryNo, deliveries[1].DeliveryNo, deliveries[0].DeliveryNo}
	if err = query.AcknowledgeDeliveries(
		ctx, ProviderTencentESSBasic, "contract.sign", numbers,
	); err != nil {
		t.Fatalf("acknowledge exact deliveries: %v", err)
	}
	if err = query.AcknowledgeDeliveries(
		ctx, ProviderTencentESSBasic, "contract.sign", numbers,
	); err != nil {
		t.Fatalf("repeat exact delivery acknowledgement: %v", err)
	}
	deliveries, err = query.PendingDeliveries(
		ctx, ProviderTencentESSBasic, "contract.sign", MaximumDeliveryBatchSize,
	)
	if err != nil || len(deliveries) != 0 {
		t.Fatalf("acknowledged deliveries remained pending=%+v err=%v", deliveries, err)
	}
	if err = query.AcknowledgeDeliveries(
		ctx, ProviderTencentESSBasic, "contract.other", numbers,
	); !errors.Is(err, ErrConflict) {
		t.Fatalf("cross-scope acknowledgement error=%v want conflict", err)
	}
}

func TestResolveExternalReferenceUsesInstallationLocalProtectedBinding(t *testing.T) {
	database, installationID, cleanup := openSupplierCallbackDatabase(t)
	defer cleanup()
	guard, err := supplierguard.NewService(database, installationID, supplierguard.DefaultPolicy())
	if err != nil {
		t.Fatalf("construct supplier guard: %v", err)
	}
	lease := acquireCallbackRoute(t, database, guard, installationID, 90, "tencent.contract.create")
	file, err := callbacksecurity.Generate(rand.Reader)
	if err != nil {
		t.Fatalf("generate callback keyring: %v", err)
	}
	ring, err := callbacksecurity.New(file)
	if err != nil {
		t.Fatalf("construct callback keyring: %v", err)
	}
	defer ring.Clear()
	queries, err := NewQueryService(database, installationID, ring, map[string]ReadOnlyQuerier{})
	if err != nil {
		t.Fatalf("construct query service: %v", err)
	}
	reference := []byte("AbCdEfGhIjKlMnOpQrStUvWxYz012345")
	if _, err = queries.BindExternalReference(context.Background(), ReferenceBinding{
		RouteNo: lease.RouteNo, Provider: ProviderTencentESSBasic,
		OperationScope: lease.OperationScope, ExternalReference: append([]byte(nil), reference...),
	}); err != nil {
		t.Fatalf("bind external reference: %v", err)
	}
	resolved, err := queries.ResolveExternalReference(
		context.Background(), ProviderTencentESSBasic, append([]byte(nil), reference...),
	)
	if err != nil || resolved.RouteNo != lease.RouteNo || resolved.OperationScope != lease.OperationScope {
		t.Fatalf("resolved reference=%+v err=%v", resolved, err)
	}
	if _, err = queries.ResolveExternalReference(
		context.Background(), ProviderTencentESSBasic, []byte("00000000000000000000000000000000"),
	); !errors.Is(err, ErrConflict) {
		t.Fatalf("unknown reference error=%v", err)
	}
}

func TestSupplierCallbackPostgreSQLConcurrentDeduplicationAndBounds(t *testing.T) {
	database, installationID, cleanup := openSupplierCallbackDatabase(t)
	defer cleanup()
	guard, boundary, verifier := prepareCallbackServices(t, database, installationID)
	ctx := context.Background()
	lease := acquireCallbackRoute(t, database, guard, installationID, 2, "invoice.issue")
	verifier.set(callbackEvent(lease, StatusSucceeded, "same-event", "same-raw-evidence"))

	const contenders = 8
	results := make(chan struct {
		result Result
		err    error
	}, contenders)
	var wait sync.WaitGroup
	for range contenders {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := acceptStubCallback(ctx, boundary)
			results <- struct {
				result Result
				err    error
			}{result: result, err: err}
		}()
	}
	wait.Wait()
	close(results)
	created := 0
	recovered := 0
	var evidenceNo string
	for item := range results {
		if item.err != nil {
			t.Fatalf("concurrent callback: %v", item.err)
		}
		if evidenceNo == "" {
			evidenceNo = item.result.EvidenceNo
		} else if item.result.EvidenceNo != evidenceNo {
			t.Fatalf("evidence=%s want=%s", item.result.EvidenceNo, evidenceNo)
		}
		if item.result.Replayed {
			recovered++
		} else {
			created++
		}
	}
	if created != 1 || recovered != contenders-1 {
		t.Fatalf("created=%d recovered=%d", created, recovered)
	}

	oversizedBody := bytes.Repeat([]byte{1}, MaximumRawBodyBytes+1)
	signature := []byte("signature")
	if _, err := boundary.Accept(ctx, RawCallback{
		Provider: ProviderNuonuoInvoice, Body: oversizedBody, Signature: signature,
	}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("oversized callback error=%v", err)
	}
	if !allZero(oversizedBody) || !allZero(signature) {
		t.Fatal("oversized callback buffers were not cleared")
	}
	disabledBoundary, err := NewBoundary(map[string]Verifier{}, boundary.service)
	if err != nil {
		t.Fatalf("construct disabled boundary: %v", err)
	}
	body := []byte("callback")
	signature = []byte("signature")
	if _, err = disabledBoundary.Accept(ctx, RawCallback{
		Provider: ProviderTencentESSBasic, Body: body, Signature: signature,
	}); !errors.Is(err, ErrVerifierDisabled) {
		t.Fatalf("disabled verifier error=%v", err)
	}
	if !allZero(body) || !allZero(signature) {
		t.Fatal("disabled callback buffers were not cleared")
	}

	maximumLease := acquireCallbackRoute(t, database, guard, installationID, 3, "contract.sign")
	maximumEvent := callbackEvent(maximumLease, StatusSucceeded, "maximum-event", "unused")
	maximumEvent.EvidenceFrame = bytes.Repeat([]byte{2}, MaximumEvidenceFrameBytes)
	verifier.set(maximumEvent)
	body = bytes.Repeat([]byte{3}, MaximumRawBodyBytes)
	signature = bytes.Repeat([]byte{4}, MaximumSignatureBytes)
	headers := map[string][]byte{
		"x1": bytes.Repeat([]byte{5}, 4094),
		"x2": bytes.Repeat([]byte{6}, 4094),
		"x3": bytes.Repeat([]byte{7}, 4094),
		"x4": bytes.Repeat([]byte{8}, 4094),
	}
	maximum, err := boundary.Accept(ctx, RawCallback{
		Provider: ProviderTencentESSBasic, Body: body,
		Signature: signature, SignedHeaders: headers,
	})
	if err != nil || maximum.DecisionCode != "applied" || maximum.DeliveryNo == "" {
		t.Fatalf("maximum bounded callback=%+v err=%v", maximum, err)
	}
	if !allZero(body) || !allZero(signature) || len(headers) != 0 {
		t.Fatal("maximum bounded callback buffers were not cleared")
	}
	var ciphertextBytes int
	if err = database.QueryRowContext(ctx, `
SELECT octet_length(payload_ciphertext)
FROM idc_gateway_local_callback_evidence
WHERE evidence_no = $1`, maximum.EvidenceNo).Scan(&ciphertextBytes); err != nil ||
		ciphertextBytes != MaximumEvidenceCiphertextBytes {
		t.Fatalf("maximum evidence ciphertext bytes=%d err=%v", ciphertextBytes, err)
	}
}

func TestSupplierCallbackPostgreSQLConcurrentCrossRouteEventConflict(t *testing.T) {
	database, installationID, cleanup := openSupplierCallbackDatabase(t)
	defer cleanup()
	guard, boundary, _ := prepareCallbackServices(t, database, installationID)
	firstLease := acquireCallbackRoute(t, database, guard, installationID, 4, "contract.sign")
	secondLease := acquireCallbackRoute(t, database, guard, installationID, 5, "contract.sign")
	firstVerifier := &verifierStub{template: callbackEvent(
		firstLease, StatusSucceeded, "cross-route-event", "same-provider-evidence",
	)}
	secondVerifier := &verifierStub{template: callbackEvent(
		secondLease, StatusSucceeded, "cross-route-event", "same-provider-evidence",
	)}
	firstBoundary, err := NewBoundary(map[string]Verifier{
		ProviderTencentESSBasic: firstVerifier,
	}, boundary.service)
	if err != nil {
		t.Fatalf("construct first callback boundary: %v", err)
	}
	secondBoundary, err := NewBoundary(map[string]Verifier{
		ProviderTencentESSBasic: secondVerifier,
	}, boundary.service)
	if err != nil {
		t.Fatalf("construct second callback boundary: %v", err)
	}

	start := make(chan struct{})
	errorsByRoute := make(chan error, 2)
	for _, candidate := range []*Boundary{firstBoundary, secondBoundary} {
		go func(current *Boundary) {
			<-start
			_, acceptErr := acceptStubCallback(context.Background(), current)
			errorsByRoute <- acceptErr
		}(candidate)
	}
	close(start)
	firstError := <-errorsByRoute
	secondError := <-errorsByRoute
	accepted := 0
	conflicted := 0
	for _, acceptError := range []error{firstError, secondError} {
		switch {
		case acceptError == nil:
			accepted++
		case errors.Is(acceptError, ErrConflict):
			conflicted++
		default:
			t.Fatalf("cross-route duplicate returned %v", acceptError)
		}
	}
	if accepted != 1 || conflicted != 1 {
		t.Fatalf("accepted=%d conflicted=%d", accepted, conflicted)
	}
	var evidenceCount int
	if err = database.QueryRowContext(context.Background(), `
SELECT COUNT(*)
FROM idc_gateway_local_callback_evidence
WHERE provider_code = $1`, ProviderTencentESSBasic).Scan(&evidenceCount); err != nil ||
		evidenceCount != 1 {
		t.Fatalf("cross-route evidence count=%d err=%v", evidenceCount, err)
	}
}

func prepareCallbackServices(
	t *testing.T,
	database *sql.DB,
	installationID string,
) (*supplierguard.Service, *Boundary, *verifierStub) {
	t.Helper()
	guard, err := supplierguard.NewService(database, installationID, supplierguard.DefaultPolicy())
	if err != nil {
		t.Fatalf("construct supplier guard: %v", err)
	}
	file, err := callbacksecurity.Generate(rand.Reader)
	if err != nil {
		t.Fatalf("generate callback keyring: %v", err)
	}
	ring, err := callbacksecurity.New(file)
	if err != nil {
		t.Fatalf("construct callback keyring: %v", err)
	}
	t.Cleanup(ring.Clear)
	service, err := NewService(database, installationID, ring)
	if err != nil {
		t.Fatalf("construct callback service: %v", err)
	}
	verifier := &verifierStub{}
	boundary, err := NewBoundary(map[string]Verifier{
		ProviderTencentESSBasic: verifier, ProviderNuonuoInvoice: verifier,
	}, service)
	if err != nil {
		t.Fatalf("construct callback boundary: %v", err)
	}
	return guard, boundary, verifier
}

func callbackEvent(
	lease supplierguard.Lease,
	status Status,
	eventID string,
	evidence string,
) VerifiedEvent {
	return VerifiedEvent{
		RouteNo: lease.RouteNo, Provider: lease.Provider,
		OperationScope: lease.OperationScope, ExternalEventID: []byte(eventID),
		Status: status, EvidenceFrame: []byte(evidence), SupplierObservedAt: time.Now().UTC(),
	}
}

func acceptStubCallback(ctx context.Context, boundary *Boundary) (Result, error) {
	return boundary.Accept(ctx, RawCallback{
		Provider: ProviderTencentESSBasic,
		Body:     []byte("callback-body"), Signature: []byte("callback-signature"),
		SignedHeaders: map[string][]byte{"x-signed-at": []byte("1")},
	})
}

func acquireCallbackRoute(
	t *testing.T,
	database *sql.DB,
	guard *supplierguard.Service,
	installationID string,
	index int,
	scope string,
) supplierguard.Lease {
	t.Helper()
	requestNo := insertCallbackBusinessRequest(t, database, installationID, index, scope)
	provider := ProviderTencentESSBasic
	lease, err := guard.Acquire(context.Background(), supplierguard.Admission{
		RequestNo: requestNo, Provider: provider, OperationScope: scope,
	})
	if err != nil || !lease.CallSupplier || lease.RouteNo == "" {
		t.Fatalf("acquire callback route=%+v err=%v", lease, err)
	}
	return lease
}

func insertCallbackBusinessRequest(
	t *testing.T,
	database *sql.DB,
	installationID string,
	index int,
	scope string,
) string {
	t.Helper()
	requestNo := fmt.Sprintf("lbr_%032x", index)
	idempotency := sha256.Sum256([]byte("callback-idempotency\x00" + requestNo))
	target := sha256.Sum256([]byte("callback-target\x00" + requestNo))
	content := sha256.Sum256([]byte("callback-content\x00" + requestNo))
	body := sha256.Sum256([]byte("callback-body\x00" + requestNo))
	_, err := database.ExecContext(context.Background(), `
INSERT INTO idc_gateway_local_business_request
    (request_no, installation_id, operation_scope, method, request_target_sha256,
     idempotency_hmac, business_content_sha256, body_sha256)
VALUES ($1, $2, $3, 'POST', $4, $5, $6, $7)`,
		requestNo, installationID, scope, target[:], idempotency[:], content[:], body[:])
	if err != nil {
		t.Fatalf("insert callback business request: %v", err)
	}
	return requestNo
}

func leaseRequestNo(t *testing.T, database *sql.DB, routeNo string) string {
	t.Helper()
	var requestNo string
	if err := database.QueryRowContext(context.Background(), `
SELECT request_no FROM idc_gateway_local_supplier_route WHERE route_no = $1`,
		routeNo).Scan(&requestNo); err != nil {
		t.Fatalf("read route request: %v", err)
	}
	return requestNo
}

func openSupplierCallbackDatabase(t *testing.T) (*sql.DB, string, func()) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(supplierCallbackTestDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run supplier callback PostgreSQL tests", supplierCallbackTestDSNEnv)
	}
	bootstrap, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	ctx := context.Background()
	suffix := make([]byte, 8)
	if _, err = rand.Read(suffix); err != nil {
		t.Fatalf("generate schema suffix: %v", err)
	}
	schema := "idc_supplier_callback_test_" + hex.EncodeToString(suffix)
	if _, err = bootstrap.ExecContext(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse PostgreSQL DSN: %v", err)
	}
	config.RuntimeParams["search_path"] = schema
	database := stdlib.OpenDB(*config)
	database.SetMaxOpenConns(16)
	database.SetMaxIdleConns(16)
	if err = migration.Apply(ctx, database); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	installationID := "linst_" + strings.Repeat("2", 64)
	if _, err = database.ExecContext(ctx, `
INSERT INTO idc_gateway_local_supplier_instance (installation_id) VALUES ($1)`,
		installationID); err != nil {
		t.Fatalf("insert callback installation: %v", err)
	}
	return database, installationID, func() {
		if !strings.HasPrefix(schema, "idc_supplier_callback_test_") {
			t.Errorf("refusing to drop unexpected schema %q", schema)
			return
		}
		if closeErr := database.Close(); closeErr != nil {
			t.Errorf("close callback database: %v", closeErr)
		}
		if _, dropErr := bootstrap.ExecContext(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Errorf("drop schema: %v", dropErr)
		}
		if closeErr := bootstrap.Close(); closeErr != nil {
			t.Errorf("close callback bootstrap database: %v", closeErr)
		}
	}
}

func allZero(value []byte) bool {
	for _, item := range value {
		if item != 0 {
			return false
		}
	}
	return true
}
