// This file verifies local read-only query leases and employee takeover against PostgreSQL.

package suppliercallback

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"sync"
	"testing"
	"time"

	"lina-platform/idc-compliance-gateway/internal/callbacksecurity"
	"lina-platform/idc-compliance-gateway/internal/supplierguard"
)

type queryStub struct {
	mu        sync.Mutex
	response  QueryResponse
	err       error
	calls     int
	reference []byte
	called    chan struct{}
	callOnce  sync.Once
}

func (q *queryStub) Query(_ context.Context, request QueryRequest) (QueryResponse, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.calls++
	q.reference = append(q.reference[:0], request.ExternalReference...)
	if q.called != nil {
		q.callOnce.Do(func() { close(q.called) })
	}
	result := q.response
	result.EvidenceFrame = append([]byte(nil), q.response.EvidenceFrame...)
	return result, q.err
}

func (q *queryStub) set(response QueryResponse, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.response = response
	q.err = err
}

type blockingQueryStub struct {
	started  chan struct{}
	release  chan struct{}
	response QueryResponse
}

func (q *blockingQueryStub) Query(_ context.Context, _ QueryRequest) (QueryResponse, error) {
	close(q.started)
	<-q.release
	result := q.response
	result.EvidenceFrame = append([]byte(nil), q.response.EvidenceFrame...)
	return result, nil
}

type timeoutQueryStub struct{}

func (*timeoutQueryStub) Query(ctx context.Context, _ QueryRequest) (QueryResponse, error) {
	<-ctx.Done()
	return QueryResponse{}, ctx.Err()
}

type takeoverAuthorityStub struct {
	authorization TakeoverAuthorization
	err           error
	action        string
	routeNo       string
}

func TestLocalInteractiveContractReadPostgreSQLReusesRouteWithoutProviderWrite(t *testing.T) {
	database, installationID, cleanup := openSupplierCallbackDatabase(t)
	defer cleanup()
	guard, _, _, ring := prepareReconciliationServices(t, database, installationID)
	querier := &queryStub{response: QueryResponse{
		Status: StatusSucceeded, ProviderStatus: "ALL", EvidenceFrame: []byte("interactive-query-success"),
		SupplierObservedAt: time.Now().UTC(),
	}}
	service, err := NewQueryService(database, installationID, ring, map[string]ReadOnlyQuerier{
		ProviderTencentESSBasic: querier,
	})
	if err != nil {
		t.Fatal(err)
	}
	lease := acquireCallbackRoute(t, database, guard, installationID, 53, "tencent.contract.create")
	if _, err = service.BindExternalReference(context.Background(), ReferenceBinding{
		RouteNo: lease.RouteNo, Provider: lease.Provider, OperationScope: lease.OperationScope,
		ExternalReference: []byte("provider-flow-interactive-53"),
	}); err != nil {
		t.Fatalf("bind interactive reference: %v", err)
	}
	var consumed []byte
	err = service.WithRunningReference(
		context.Background(), lease.RouteNo, lease.Provider, lease.OperationScope,
		func(reference []byte) error {
			if string(reference) != "provider-flow-interactive-53" {
				t.Fatalf("reference=%q", reference)
			}
			consumed = reference
			return nil
		},
	)
	if err != nil || !allZero(consumed) {
		t.Fatalf("consume error=%v cleared=%t", err, allZero(consumed))
	}
	result, err := service.RunRoute(
		context.Background(), lease.RouteNo, lease.Provider, lease.OperationScope,
	)
	if err != nil || result.ProjectedStatus != StatusSucceeded || result.ProviderStatus != "ALL" ||
		result.RouteVersion != 3 || result.DeliveryNo == "" {
		t.Fatalf("interactive result=%+v err=%v", result, err)
	}
	deliveries, err := service.PendingDeliveries(
		context.Background(), lease.Provider, lease.OperationScope, MaximumDeliveryBatchSize,
	)
	if err != nil || len(deliveries) != 1 || deliveries[0].ProviderStatus != "ALL" ||
		deliveries[0].SourceKind != "active_query" {
		t.Fatalf("interactive exact deliveries=%+v err=%v", deliveries, err)
	}
	terminalSnapshot, err := service.RunRoute(
		context.Background(), lease.RouteNo, lease.Provider, lease.OperationScope,
	)
	if err != nil || terminalSnapshot.DecisionCode != "stale" ||
		terminalSnapshot.ProviderStatus != "ALL" || terminalSnapshot.SupplierObservedAt.IsZero() ||
		terminalSnapshot.RouteVersion != result.RouteVersion {
		t.Fatalf("terminal exact snapshot=%+v err=%v", terminalSnapshot, err)
	}
	querier.mu.Lock()
	calls := querier.calls
	querier.mu.Unlock()
	if calls != 1 {
		t.Fatalf("interactive query calls=%d want=1", calls)
	}
	if err = service.WithRunningReference(
		context.Background(), lease.RouteNo, lease.Provider, lease.OperationScope,
		func([]byte) error { return nil },
	); !errors.Is(err, ErrConflict) {
		t.Fatalf("terminal route produced signing reference: %v", err)
	}
	replayed, err := service.RunRoute(
		context.Background(), lease.RouteNo, lease.Provider, lease.OperationScope,
	)
	querier.mu.Lock()
	calls = querier.calls
	querier.mu.Unlock()
	if err != nil || replayed.ProjectedStatus != StatusSucceeded || calls != 1 {
		t.Fatalf("terminal replay=%+v calls=%d err=%v", replayed, calls, err)
	}
	var terminalReference []byte
	err = service.WithTerminalReference(
		context.Background(), lease.RouteNo, lease.Provider,
		[]TerminalReferenceRule{{OperationScope: lease.OperationScope, ProviderStatus: "ALL"}},
		func(reference []byte) error {
			if string(reference) != "provider-flow-interactive-53" {
				t.Fatalf("terminal reference=%q", reference)
			}
			terminalReference = reference
			return nil
		},
	)
	if err != nil || !allZero(terminalReference) {
		t.Fatalf("terminal reference error=%v cleared=%t", err, allZero(terminalReference))
	}
	if err = service.WithTerminalReference(
		context.Background(), lease.RouteNo, lease.Provider,
		[]TerminalReferenceRule{{OperationScope: lease.OperationScope, ProviderStatus: "RELIEVED"}},
		func([]byte) error { return nil },
	); !errors.Is(err, ErrConflict) {
		t.Fatalf("mismatched terminal rule exposed a reference: %v", err)
	}
}

func TestLocalReadOnlyQueryPostgreSQLExecutionTimeoutIsStableFailure(t *testing.T) {
	database, installationID, cleanup := openSupplierCallbackDatabase(t)
	defer cleanup()
	guard, _, _, ring := prepareReconciliationServices(t, database, installationID)
	queryService, err := NewQueryService(
		database,
		installationID,
		ring,
		map[string]ReadOnlyQuerier{ProviderTencentESSBasic: &timeoutQueryStub{}},
	)
	if err != nil {
		t.Fatalf("construct timeout query service: %v", err)
	}
	queryService.queryTimeout = 20 * time.Millisecond
	lease := acquireCallbackRoute(t, database, guard, installationID, 38, "contract.sign")
	if _, err = queryService.BindExternalReference(context.Background(), ReferenceBinding{
		RouteNo: lease.RouteNo, Provider: lease.Provider,
		OperationScope:    lease.OperationScope,
		ExternalReference: []byte("provider-flow-timeout-38"),
	}); err != nil {
		t.Fatalf("bind timeout route: %v", err)
	}
	claims, err := queryService.Claim(context.Background(), 1)
	if err != nil || len(claims) != 1 {
		t.Fatalf("timeout claims=%+v err=%v", claims, err)
	}
	result, err := queryService.Run(context.Background(), claims[0])
	if err != nil || result.DecisionCode != "query_failed" {
		t.Fatalf("timeout result=%+v err=%v", result, err)
	}
	var (
		stableCode   string
		failureCount int
		leaseCleared bool
	)
	if err = database.QueryRowContext(context.Background(), `
SELECT evidence.stable_code, route.query_failure_count,
       route.query_lease_sha256 IS NULL
FROM idc_gateway_local_callback_evidence evidence
JOIN idc_gateway_local_supplier_route route ON route.route_no = evidence.route_no
WHERE evidence.evidence_no = $1`, result.EvidenceNo).Scan(
		&stableCode, &failureCount, &leaseCleared,
	); err != nil || stableCode != string(QueryFailureTimeout) ||
		failureCount != 1 || !leaseCleared {
		t.Fatalf(
			"stableCode=%s failureCount=%d leaseCleared=%t err=%v",
			stableCode, failureCount, leaseCleared, err,
		)
	}
}

func (a *takeoverAuthorityStub) AuthorizeTakeover(
	_ context.Context,
	routeNo string,
	action string,
) (TakeoverAuthorization, error) {
	a.routeNo = routeNo
	a.action = action
	return a.authorization, a.err
}

func TestLocalReadOnlyQueryPostgreSQLLeaseConvergenceAndFailure(t *testing.T) {
	database, installationID, cleanup := openSupplierCallbackDatabase(t)
	defer cleanup()
	guard, _, _, ring := prepareReconciliationServices(t, database, installationID)
	querier := &queryStub{}
	queryService, err := NewQueryService(database, installationID, ring, map[string]ReadOnlyQuerier{
		ProviderTencentESSBasic: querier,
	})
	if err != nil {
		t.Fatalf("construct query service: %v", err)
	}

	lease := acquireCallbackRoute(t, database, guard, installationID, 31, "contract.sign")
	externalReference := []byte("provider-flow-sensitive-31")
	boundVersion, err := queryService.BindExternalReference(context.Background(), ReferenceBinding{
		RouteNo: lease.RouteNo, Provider: lease.Provider,
		OperationScope: lease.OperationScope, ExternalReference: externalReference,
	})
	if err != nil || boundVersion != 2 || !allZero(externalReference) {
		t.Fatalf("bind version=%d cleared=%t err=%v", boundVersion, allZero(externalReference), err)
	}
	var storedReference []byte
	if err = database.QueryRowContext(context.Background(), `
SELECT external_ref_ciphertext
FROM idc_gateway_local_supplier_route
WHERE route_no = $1`, lease.RouteNo).Scan(&storedReference); err != nil ||
		bytes.Contains(storedReference, []byte("provider-flow-sensitive-31")) {
		t.Fatalf("protected reference leaked plaintext err=%v", err)
	}
	claims, err := queryService.Claim(context.Background(), MaximumQueryBatchSize)
	if err != nil || len(claims) != 1 || claims[0].RouteVersion != boundVersion ||
		len(claims[0].Token) != callbacksecurity.SecretBytes {
		t.Fatalf("claims=%+v err=%v", claims, err)
	}
	secondClaims, err := queryService.Claim(context.Background(), MaximumQueryBatchSize)
	if err != nil || len(secondClaims) != 0 {
		t.Fatalf("duplicate claims=%+v err=%v", secondClaims, err)
	}
	var storedLease []byte
	if err = database.QueryRowContext(context.Background(), `
SELECT query_lease_sha256
FROM idc_gateway_local_supplier_route
WHERE route_no = $1`, lease.RouteNo).Scan(&storedLease); err != nil ||
		bytes.Equal(storedLease, claims[0].Token) {
		t.Fatalf("database stored plaintext lease token err=%v", err)
	}
	querier.set(QueryResponse{
		Status: StatusSucceeded, EvidenceFrame: []byte("authoritative-query-success"),
		SupplierObservedAt: time.Now().UTC(),
	}, nil)
	queryResult, err := queryService.Run(context.Background(), claims[0])
	if err != nil || queryResult.ProjectedStatus != StatusSucceeded ||
		queryResult.RouteVersion != 3 || queryResult.DeliveryNo == "" {
		t.Fatalf("query result=%+v err=%v", queryResult, err)
	}
	if string(querier.reference) != "provider-flow-sensitive-31" {
		t.Fatalf("querier reference=%q", querier.reference)
	}

	failureLease := acquireCallbackRoute(t, database, guard, installationID, 32, "invoice.issue")
	if _, err = queryService.BindExternalReference(context.Background(), ReferenceBinding{
		RouteNo: failureLease.RouteNo, Provider: failureLease.Provider,
		OperationScope:    failureLease.OperationScope,
		ExternalReference: []byte("provider-flow-sensitive-32"),
	}); err != nil {
		t.Fatalf("bind failure route: %v", err)
	}
	failureClaims, err := queryService.Claim(context.Background(), MaximumQueryBatchSize)
	if err != nil || len(failureClaims) != 1 {
		t.Fatalf("failure claims=%+v err=%v", failureClaims, err)
	}
	querier.set(QueryResponse{
		FailureCode:        QueryFailureTimeout,
		EvidenceFrame:      []byte("provider-timeout-response-sensitive"),
		SupplierObservedAt: time.Now().UTC(),
	}, nil)
	failureResult, err := queryService.Run(context.Background(), failureClaims[0])
	if err != nil || failureResult.DecisionCode != "query_failed" ||
		failureResult.RouteVersion != 2 {
		t.Fatalf("failure result=%+v err=%v", failureResult, err)
	}
	var (
		failureCount int
		nextDelay    float64
		leaseCleared bool
		ciphertext   []byte
		stableCode   string
	)
	if err = database.QueryRowContext(context.Background(), `
SELECT query_failure_count,
       EXTRACT(EPOCH FROM (next_query_at - updated_at)),
       query_lease_sha256 IS NULL
FROM idc_gateway_local_supplier_route
WHERE route_no = $1`, failureLease.RouteNo).Scan(
		&failureCount, &nextDelay, &leaseCleared,
	); err != nil || failureCount != 1 || nextDelay != 60 || !leaseCleared {
		t.Fatalf("failure count=%d delay=%v leaseCleared=%t err=%v",
			failureCount, nextDelay, leaseCleared, err)
	}
	if err = database.QueryRowContext(context.Background(), `
SELECT payload_ciphertext, stable_code
FROM idc_gateway_local_callback_evidence
WHERE evidence_no = $1`, failureResult.EvidenceNo).Scan(&ciphertext, &stableCode); err != nil ||
		bytes.Contains(ciphertext, []byte("provider-timeout-response-sensitive")) ||
		stableCode != string(QueryFailureTimeout) {
		t.Fatalf("query failure stableCode=%s leaked plaintext=%t err=%v",
			stableCode, bytes.Contains(ciphertext, []byte("provider-timeout-response-sensitive")), err)
	}

	disabled, err := NewQueryService(
		database, installationID, ring, map[string]ReadOnlyQuerier{},
	)
	if err != nil {
		t.Fatalf("construct disabled query service: %v", err)
	}
	disabledClaims, err := disabled.Claim(context.Background(), MaximumQueryBatchSize)
	if err != nil || len(disabledClaims) != 0 {
		t.Fatalf("disabled claims=%+v err=%v", disabledClaims, err)
	}
	if err = disabled.RunWorker(context.Background()); !errors.Is(err, ErrQueryDisabled) {
		t.Fatalf("disabled worker error=%v", err)
	}
}

func TestLocalReadOnlyQueryPostgreSQLLateResultCannotOverwriteCallback(t *testing.T) {
	database, installationID, cleanup := openSupplierCallbackDatabase(t)
	defer cleanup()
	guard, boundary, verifier, ring := prepareReconciliationServices(t, database, installationID)
	blocking := &blockingQueryStub{
		started: make(chan struct{}), release: make(chan struct{}),
		response: QueryResponse{
			Status: StatusFailed, EvidenceFrame: []byte("late-query-failure"),
			SupplierObservedAt: time.Now().UTC(),
		},
	}
	queryService, err := NewQueryService(database, installationID, ring, map[string]ReadOnlyQuerier{
		ProviderTencentESSBasic: blocking,
	})
	if err != nil {
		t.Fatalf("construct query service: %v", err)
	}
	lease := acquireCallbackRoute(t, database, guard, installationID, 33, "contract.sign")
	if _, err = queryService.BindExternalReference(context.Background(), ReferenceBinding{
		RouteNo: lease.RouteNo, Provider: lease.Provider,
		OperationScope: lease.OperationScope, ExternalReference: []byte("provider-flow-33"),
	}); err != nil {
		t.Fatalf("bind reference: %v", err)
	}
	claims, err := queryService.Claim(context.Background(), 1)
	if err != nil || len(claims) != 1 {
		t.Fatalf("claims=%+v err=%v", claims, err)
	}
	runResult := make(chan error, 1)
	go func() {
		_, runErr := queryService.Run(context.Background(), claims[0])
		runResult <- runErr
	}()
	<-blocking.started
	verifier.set(callbackEvent(lease, StatusSucceeded, "event-callback-wins", "callback-wins"))
	callbackResult, err := acceptStubCallback(context.Background(), boundary)
	if err != nil || callbackResult.ProjectedStatus != StatusSucceeded ||
		callbackResult.RouteVersion != 3 {
		t.Fatalf("callback result=%+v err=%v", callbackResult, err)
	}
	close(blocking.release)
	if runErr := <-runResult; !errors.Is(runErr, ErrLeaseLost) {
		t.Fatalf("late query error=%v", runErr)
	}
	var projected string
	if err = database.QueryRowContext(context.Background(), `
SELECT projected_status
FROM idc_gateway_local_supplier_route
WHERE route_no = $1`, lease.RouteNo).Scan(&projected); err != nil || projected != "succeeded" {
		t.Fatalf("projected=%s err=%v", projected, err)
	}
}

func TestLocalReadOnlyQueryPostgreSQLConcurrentClaimAndExpiredLeaseReclaim(t *testing.T) {
	database, installationID, cleanup := openSupplierCallbackDatabase(t)
	defer cleanup()
	guard, _, _, ring := prepareReconciliationServices(t, database, installationID)
	querier := &queryStub{}
	firstService, err := NewQueryService(database, installationID, ring, map[string]ReadOnlyQuerier{
		ProviderTencentESSBasic: querier,
	})
	if err != nil {
		t.Fatalf("construct first query service: %v", err)
	}
	secondService, err := NewQueryService(database, installationID, ring, map[string]ReadOnlyQuerier{
		ProviderTencentESSBasic: querier,
	})
	if err != nil {
		t.Fatalf("construct second query service: %v", err)
	}
	for index := 41; index <= 48; index++ {
		lease := acquireCallbackRoute(t, database, guard, installationID, index, "contract.sign")
		if _, err = firstService.BindExternalReference(context.Background(), ReferenceBinding{
			RouteNo: lease.RouteNo, Provider: lease.Provider,
			OperationScope:    lease.OperationScope,
			ExternalReference: []byte("concurrent-provider-flow-" + lease.RouteNo),
		}); err != nil {
			t.Fatalf("bind route %d: %v", index, err)
		}
	}
	start := make(chan struct{})
	type claimResult struct {
		leases []QueryLease
		err    error
	}
	results := make(chan claimResult, 2)
	for _, service := range []*QueryService{firstService, secondService} {
		go func(current *QueryService) {
			<-start
			leases, claimErr := current.Claim(context.Background(), MaximumQueryBatchSize)
			results <- claimResult{leases: leases, err: claimErr}
		}(service)
	}
	close(start)
	claimed := make(map[string]QueryLease, 8)
	for range 2 {
		item := <-results
		if item.err != nil {
			t.Fatalf("concurrent claim: %v", item.err)
		}
		for _, lease := range item.leases {
			if _, duplicate := claimed[lease.RouteNo]; duplicate {
				t.Fatalf("route %s was leased twice", lease.RouteNo)
			}
			claimed[lease.RouteNo] = lease
		}
	}
	if len(claimed) != 8 {
		t.Fatalf("claimed routes=%d want=8", len(claimed))
	}
	var expired QueryLease
	for _, lease := range claimed {
		expired = lease
		break
	}
	if _, err = database.ExecContext(context.Background(), `
UPDATE idc_gateway_local_supplier_route
SET query_lease_started_at = CURRENT_TIMESTAMP - INTERVAL '61 seconds',
    query_lease_expires_at = CURRENT_TIMESTAMP - INTERVAL '1 second'
WHERE route_no = $1`, expired.RouteNo); err != nil {
		t.Fatalf("expire test lease: %v", err)
	}
	reclaimed, err := firstService.Claim(context.Background(), 1)
	if err != nil || len(reclaimed) != 1 || reclaimed[0].RouteNo != expired.RouteNo ||
		bytes.Equal(reclaimed[0].Token, expired.Token) {
		t.Fatalf("reclaimed=%+v expired=%+v err=%v", reclaimed, expired, err)
	}
	querier.set(QueryResponse{
		Status: StatusSucceeded, EvidenceFrame: []byte("stale-old-lease"),
		SupplierObservedAt: time.Now().UTC(),
	}, nil)
	if _, err = firstService.Run(context.Background(), expired); !errors.Is(err, ErrLeaseLost) {
		t.Fatalf("expired lease error=%v", err)
	}
}

func TestLocalExternalReferencePostgreSQLRetainedDigestPreventsCrossRouteBinding(t *testing.T) {
	database, installationID, cleanup := openSupplierCallbackDatabase(t)
	defer cleanup()
	guard, err := supplierguard.NewService(database, installationID, supplierguard.DefaultPolicy())
	if err != nil {
		t.Fatalf("construct guard: %v", err)
	}
	file, err := callbacksecurity.Generate(rand.Reader)
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	firstRing, err := callbacksecurity.New(file)
	if err != nil {
		t.Fatalf("construct first ring: %v", err)
	}
	defer firstRing.Clear()
	firstService, err := NewQueryService(database, installationID, firstRing, map[string]ReadOnlyQuerier{})
	if err != nil {
		t.Fatalf("construct first service: %v", err)
	}
	firstRoute := acquireCallbackRoute(t, database, guard, installationID, 49, "contract.sign")
	if _, err = firstService.BindExternalReference(context.Background(), ReferenceBinding{
		RouteNo: firstRoute.RouteNo, Provider: firstRoute.Provider,
		OperationScope:    firstRoute.OperationScope,
		ExternalReference: []byte("retained-reference"),
	}); err != nil {
		t.Fatalf("bind first route: %v", err)
	}
	rotateTestKeyFamily(t, &file.ReferenceLookup)
	secondRing, err := callbacksecurity.New(file)
	if err != nil {
		t.Fatalf("construct rotated ring: %v", err)
	}
	defer secondRing.Clear()
	secondService, err := NewQueryService(database, installationID, secondRing, map[string]ReadOnlyQuerier{})
	if err != nil {
		t.Fatalf("construct rotated service: %v", err)
	}
	if version, bindErr := secondService.BindExternalReference(context.Background(), ReferenceBinding{
		RouteNo: firstRoute.RouteNo, Provider: firstRoute.Provider,
		OperationScope:    firstRoute.OperationScope,
		ExternalReference: []byte("retained-reference"),
	}); bindErr != nil || version != 2 {
		t.Fatalf("retained replay version=%d err=%v", version, bindErr)
	}
	secondRoute := acquireCallbackRoute(t, database, guard, installationID, 50, "contract.sign")
	if _, err = secondService.BindExternalReference(context.Background(), ReferenceBinding{
		RouteNo: secondRoute.RouteNo, Provider: secondRoute.Provider,
		OperationScope:    secondRoute.OperationScope,
		ExternalReference: []byte("retained-reference"),
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("cross-version duplicate reference error=%v", err)
	}
}

func TestLocalReadOnlyQueryPostgreSQLWorkerRunsImmediateBatch(t *testing.T) {
	database, installationID, cleanup := openSupplierCallbackDatabase(t)
	defer cleanup()
	guard, _, _, ring := prepareReconciliationServices(t, database, installationID)
	querier := &queryStub{
		called: make(chan struct{}),
		response: QueryResponse{
			Status: StatusSucceeded, EvidenceFrame: []byte("worker-success"),
			SupplierObservedAt: time.Now().UTC(),
		},
	}
	service, err := NewQueryService(database, installationID, ring, map[string]ReadOnlyQuerier{
		ProviderTencentESSBasic: querier,
	})
	if err != nil {
		t.Fatalf("construct query service: %v", err)
	}
	lease := acquireCallbackRoute(t, database, guard, installationID, 52, "contract.sign")
	if _, err = service.BindExternalReference(context.Background(), ReferenceBinding{
		RouteNo: lease.RouteNo, Provider: lease.Provider,
		OperationScope:    lease.OperationScope,
		ExternalReference: []byte("worker-provider-flow"),
	}); err != nil {
		t.Fatalf("bind worker route: %v", err)
	}
	workerContext, cancel := context.WithCancel(context.Background())
	workerResult := make(chan error, 1)
	go func() { workerResult <- service.RunWorker(workerContext) }()
	select {
	case <-querier.called:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("query worker did not run its immediate batch")
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		var status string
		err = database.QueryRowContext(context.Background(), `
SELECT projected_status
FROM idc_gateway_local_supplier_route
WHERE route_no = $1`, lease.RouteNo).Scan(&status)
		if err == nil && status == "succeeded" {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("worker route status=%s err=%v", status, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err = <-workerResult; err != nil {
		t.Fatalf("worker shutdown: %v", err)
	}
}

func TestLocalManualTakeoverPostgreSQLEvidenceVersionAndReplay(t *testing.T) {
	database, installationID, cleanup := openSupplierCallbackDatabase(t)
	defer cleanup()
	guard, boundary, verifier, ring := prepareReconciliationServices(t, database, installationID)
	lease := acquireCallbackRoute(t, database, guard, installationID, 34, "contract.sign")
	verifier.set(callbackEvent(lease, StatusResultUnknown, "event-takeover", "unknown-evidence"))
	unknown, err := acceptStubCallback(context.Background(), boundary)
	if err != nil || unknown.RouteVersion != 2 {
		t.Fatalf("unknown callback=%+v err=%v", unknown, err)
	}
	authority := &takeoverAuthorityStub{authorization: TakeoverAuthorization{
		PrincipalNo: "employee_34", CurrentMerchantEmployee: true,
		PermissionGranted: true, DataScopeMatched: true, FullAuthAt: time.Now().UTC(),
	}}
	takeoverService, err := NewTakeoverService(database, installationID, ring, authority)
	if err != nil {
		t.Fatalf("construct takeover service: %v", err)
	}
	operationKey := bytes.Repeat([]byte{34}, callbacksecurity.SecretBytes)
	takeover, err := takeoverService.Takeover(context.Background(), TakeoverInput{
		RouteNo: lease.RouteNo, ExpectedVersion: unknown.RouteVersion,
		Reason: TakeoverResultRemainsUnknown, EvidenceNos: []string{unknown.EvidenceNo},
		OperationKey: operationKey,
	})
	if err != nil || takeover.RouteVersion != 3 || takeover.DeliveryNo == "" ||
		takeover.DecisionEvidenceNo == "" || !allZero(operationKey) {
		t.Fatalf("takeover=%+v cleared=%t err=%v", takeover, allZero(operationKey), err)
	}
	if authority.action != TakeoverPermission || authority.routeNo != lease.RouteNo {
		t.Fatalf("authority action=%q route=%q", authority.action, authority.routeNo)
	}
	var (
		routeStatus, reconciliationStatus, businessStatus string
		relationCount, takeoverCount                      int
		decisionCiphertext                                []byte
	)
	if err = database.QueryRowContext(context.Background(), `
SELECT projected_status, reconciliation_status
FROM idc_gateway_local_supplier_route
WHERE route_no = $1`, lease.RouteNo).Scan(&routeStatus, &reconciliationStatus); err != nil ||
		routeStatus != "manual_takeover" || reconciliationStatus != "manual_takeover" {
		t.Fatalf("route=%s/%s err=%v", routeStatus, reconciliationStatus, err)
	}
	if err = database.QueryRowContext(context.Background(), `
SELECT status_code
FROM idc_gateway_local_business_request
WHERE request_no = $1`, leaseRequestNo(t, database, lease.RouteNo)).Scan(&businessStatus); err != nil ||
		businessStatus != "manual_takeover" {
		t.Fatalf("business status=%s err=%v", businessStatus, err)
	}
	if err = database.QueryRowContext(context.Background(), `
SELECT COUNT(*)
FROM idc_gateway_local_supplier_takeover
WHERE route_no = $1`, lease.RouteNo).Scan(&takeoverCount); err != nil || takeoverCount != 1 {
		t.Fatalf("takeover count=%d err=%v", takeoverCount, err)
	}
	if err = database.QueryRowContext(context.Background(), `
SELECT COUNT(*)
FROM idc_gateway_local_takeover_evidence
WHERE takeover_no = $1`, takeover.TakeoverNo).Scan(&relationCount); err != nil || relationCount != 1 {
		t.Fatalf("relation count=%d err=%v", relationCount, err)
	}
	if err = database.QueryRowContext(context.Background(), `
SELECT payload_ciphertext
FROM idc_gateway_local_callback_evidence
WHERE evidence_no = $1`, takeover.DecisionEvidenceNo).Scan(&decisionCiphertext); err != nil ||
		bytes.Contains(decisionCiphertext, []byte("employee_34")) {
		t.Fatalf("decision evidence leaked plaintext err=%v", err)
	}
	replayKey := bytes.Repeat([]byte{34}, callbacksecurity.SecretBytes)
	replayed, err := takeoverService.Takeover(context.Background(), TakeoverInput{
		RouteNo: lease.RouteNo, ExpectedVersion: unknown.RouteVersion,
		Reason: TakeoverResultRemainsUnknown, EvidenceNos: []string{unknown.EvidenceNo},
		OperationKey: replayKey,
	})
	if err != nil || !replayed.Replayed || replayed.TakeoverNo != takeover.TakeoverNo ||
		replayed.DeliveryNo != takeover.DeliveryNo || !allZero(replayKey) {
		t.Fatalf("replay=%+v cleared=%t err=%v", replayed, allZero(replayKey), err)
	}
	changedKey := bytes.Repeat([]byte{34}, callbacksecurity.SecretBytes)
	if _, err = takeoverService.Takeover(context.Background(), TakeoverInput{
		RouteNo: lease.RouteNo, ExpectedVersion: unknown.RouteVersion,
		Reason: TakeoverComplianceSecurityReview, EvidenceNos: []string{unknown.EvidenceNo},
		OperationKey: changedKey,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed replay error=%v", err)
	}
	if _, err = takeoverService.Takeover(context.Background(), TakeoverInput{
		RouteNo: lease.RouteNo, ExpectedVersion: unknown.RouteVersion,
		Reason: TakeoverReason("free_text"), EvidenceNos: []string{unknown.EvidenceNo},
		OperationKey: bytes.Repeat([]byte{35}, callbacksecurity.SecretBytes),
	}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("free reason error=%v", err)
	}
}

func TestLocalManualTakeoverPostgreSQLRejectsOldAuthAndCrossRouteEvidence(t *testing.T) {
	database, installationID, cleanup := openSupplierCallbackDatabase(t)
	defer cleanup()
	guard, boundary, verifier, ring := prepareReconciliationServices(t, database, installationID)
	first := acquireCallbackRoute(t, database, guard, installationID, 35, "contract.sign")
	verifier.set(callbackEvent(first, StatusResultUnknown, "event-first", "first-evidence"))
	firstUnknown, err := acceptStubCallback(context.Background(), boundary)
	if err != nil {
		t.Fatalf("first callback: %v", err)
	}
	second := acquireCallbackRoute(t, database, guard, installationID, 36, "contract.sign")
	verifier.set(callbackEvent(second, StatusResultUnknown, "event-second", "second-evidence"))
	secondUnknown, err := acceptStubCallback(context.Background(), boundary)
	if err != nil {
		t.Fatalf("second callback: %v", err)
	}
	oldAuthority := &takeoverAuthorityStub{authorization: TakeoverAuthorization{
		PrincipalNo: "employee_35", CurrentMerchantEmployee: true,
		PermissionGranted: true, DataScopeMatched: true,
		FullAuthAt: time.Now().UTC().Add(-recentTakeoverAuthAge - time.Second),
	}}
	service, err := NewTakeoverService(database, installationID, ring, oldAuthority)
	if err != nil {
		t.Fatalf("construct takeover service: %v", err)
	}
	if _, err = service.Takeover(context.Background(), TakeoverInput{
		RouteNo: first.RouteNo, ExpectedVersion: firstUnknown.RouteVersion,
		Reason: TakeoverResultRemainsUnknown, EvidenceNos: []string{firstUnknown.EvidenceNo},
		OperationKey: bytes.Repeat([]byte{36}, callbacksecurity.SecretBytes),
	}); !errors.Is(err, ErrTakeoverRecentAuth) {
		t.Fatalf("old auth error=%v", err)
	}
	oldAuthority.authorization.FullAuthAt = time.Now().UTC()
	if _, err = service.Takeover(context.Background(), TakeoverInput{
		RouteNo: first.RouteNo, ExpectedVersion: firstUnknown.RouteVersion,
		Reason: TakeoverResultRemainsUnknown, EvidenceNos: []string{secondUnknown.EvidenceNo},
		OperationKey: bytes.Repeat([]byte{37}, callbacksecurity.SecretBytes),
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("cross-route evidence error=%v", err)
	}
}

func TestLocalManualTakeoverPostgreSQLTerminalConflictPreservesBusinessResult(t *testing.T) {
	database, installationID, cleanup := openSupplierCallbackDatabase(t)
	defer cleanup()
	guard, boundary, verifier, ring := prepareReconciliationServices(t, database, installationID)
	lease := acquireCallbackRoute(t, database, guard, installationID, 51, "contract.sign")
	verifier.set(callbackEvent(lease, StatusSucceeded, "event-terminal-success", "success-evidence"))
	succeeded, err := acceptStubCallback(context.Background(), boundary)
	if err != nil {
		t.Fatalf("success callback: %v", err)
	}
	verifier.set(callbackEvent(lease, StatusFailed, "event-terminal-conflict", "failure-evidence"))
	conflict, err := acceptStubCallback(context.Background(), boundary)
	if err != nil || conflict.ReconciliationStatus != "reconciliation_required" {
		t.Fatalf("conflict callback=%+v err=%v", conflict, err)
	}
	authority := &takeoverAuthorityStub{authorization: TakeoverAuthorization{
		PrincipalNo: "employee_51", CurrentMerchantEmployee: true,
		PermissionGranted: true, DataScopeMatched: true, FullAuthAt: time.Now().UTC(),
	}}
	service, err := NewTakeoverService(database, installationID, ring, authority)
	if err != nil {
		t.Fatalf("construct takeover service: %v", err)
	}
	result, err := service.Takeover(context.Background(), TakeoverInput{
		RouteNo: lease.RouteNo, ExpectedVersion: conflict.RouteVersion,
		Reason:       TakeoverTerminalResultConflict,
		EvidenceNos:  []string{succeeded.EvidenceNo, conflict.EvidenceNo},
		OperationKey: bytes.Repeat([]byte{51}, callbacksecurity.SecretBytes),
	})
	if err != nil || result.RouteVersion != conflict.RouteVersion+1 {
		t.Fatalf("takeover=%+v err=%v", result, err)
	}
	var businessStatus string
	if err = database.QueryRowContext(context.Background(), `
SELECT status_code
FROM idc_gateway_local_business_request
WHERE request_no = $1`, leaseRequestNo(t, database, lease.RouteNo)).Scan(&businessStatus); err != nil ||
		businessStatus != "succeeded" {
		t.Fatalf("terminal business status=%s err=%v", businessStatus, err)
	}
}

func prepareReconciliationServices(
	t *testing.T,
	database *sql.DB,
	installationID string,
) (*supplierguard.Service, *Boundary, *verifierStub, *callbacksecurity.Ring) {
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
	callbackService, err := NewService(database, installationID, ring)
	if err != nil {
		t.Fatalf("construct callback service: %v", err)
	}
	verifier := &verifierStub{}
	boundary, err := NewBoundary(map[string]Verifier{
		ProviderTencentESSBasic: verifier,
	}, callbackService)
	if err != nil {
		t.Fatalf("construct callback boundary: %v", err)
	}
	return guard, boundary, verifier, ring
}

func rotateTestKeyFamily(t *testing.T, family *callbacksecurity.FileFamily) {
	t.Helper()
	if family == nil || len(family.Keys) != 1 {
		t.Fatal("unexpected test key family")
	}
	family.Keys[0].State = callbacksecurity.KeyStateRetained
	secret := make([]byte, callbacksecurity.SecretBytes)
	if _, err := rand.Read(secret); err != nil {
		t.Fatalf("generate rotated test key: %v", err)
	}
	defer clear(secret)
	family.ActiveVersion = 2
	family.Keys = append(family.Keys, callbacksecurity.FileKey{
		Version: 2, State: callbacksecurity.KeyStateActive,
		Secret: base64.RawURLEncoding.EncodeToString(secret),
	})
}
