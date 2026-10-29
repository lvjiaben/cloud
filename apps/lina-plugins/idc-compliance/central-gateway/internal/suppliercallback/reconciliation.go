// This file owns installation-local external-reference protection, read-only
// query claiming, and lease-fenced authoritative convergence.

package suppliercallback

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"lina-platform/idc-compliance-gateway/internal/callbacksecurity"
)

const (
	// MaximumExternalReferenceBytes bounds one decrypted provider lookup key.
	MaximumExternalReferenceBytes = 512
	// MaximumQueryBatchSize bounds one PostgreSQL SKIP LOCKED claim.
	MaximumQueryBatchSize = 50
	// QueryLeaseDuration is the database lease lifetime for one read-only query.
	QueryLeaseDuration = 60 * time.Second
	// QueryWorkerPollInterval controls the idle polling cadence.
	QueryWorkerPollInterval = 10 * time.Second
	// MaximumConcurrentQueries bounds concurrent provider reads per worker.
	MaximumConcurrentQueries = 4
	querySuccessPollInterval = time.Minute
	queryExecutionTimeout    = 50 * time.Second
)

var (
	// ErrQueryDisabled reports that no verified provider read client is installed.
	ErrQueryDisabled = errors.New("supplier read-only query client is disabled")
	// ErrLeaseLost reports that a newer callback or worker owns the route.
	ErrLeaseLost = errors.New("local supplier query lease was lost")
)

// QueryFailureCode is the only persisted query-failure vocabulary.
type QueryFailureCode string

const (
	// QueryFailureTransportUnavailable records a provider transport failure.
	QueryFailureTransportUnavailable QueryFailureCode = "transport_unavailable"
	// QueryFailureTimeout records a bounded provider-read timeout.
	QueryFailureTimeout QueryFailureCode = "timeout"
	// QueryFailureMalformedResponse records an invalid provider response.
	QueryFailureMalformedResponse QueryFailureCode = "malformed_response"
	// QueryFailureReferenceMismatch records an authoritative object mismatch.
	QueryFailureReferenceMismatch QueryFailureCode = "reference_mismatch"
	// QueryFailureProviderRejected records an explicit provider refusal.
	QueryFailureProviderRejected QueryFailureCode = "provider_rejected"
)

// ReferenceBinding is emitted only by a trusted provider write adapter.
type ReferenceBinding struct {
	RouteNo           string
	Provider          string
	OperationScope    string
	ExternalReference []byte
}

// ResolveExternalReference finds exactly one installation-local route without
// exposing the provider reference outside this synchronous call.
func (s *QueryService) ResolveExternalReference(
	ctx context.Context,
	provider string,
	externalReference []byte,
) (ResolvedReference, error) {
	defer clear(externalReference)
	if s == nil || s.database == nil || s.keyring == nil || ctx == nil || ctx.Err() != nil ||
		!validProvider(provider) || len(externalReference) == 0 ||
		len(externalReference) > MaximumExternalReferenceBytes {
		return ResolvedReference{}, ErrInvalid
	}
	digests, err := s.keyring.ReferenceDigests(provider, externalReference)
	if err != nil || len(digests) == 0 || len(digests) > callbacksecurity.MaximumVersionsPerFamily {
		return ResolvedReference{}, ErrRejected
	}
	arguments := []any{s.installationID, provider}
	predicates := make([]string, 0, len(digests))
	for _, digest := range digests {
		arguments = append(arguments, digest.KeyVersion, digest.Sum[:])
		predicates = append(predicates, fmt.Sprintf(
			"(external_ref_hmac_key_version = $%d AND external_ref_hmac = $%d)",
			len(arguments)-1, len(arguments),
		))
	}
	databaseContext, cancel := context.WithTimeout(ctx, callbackDatabaseTimeout)
	defer cancel()
	var result ResolvedReference
	var matchCount int
	err = s.database.QueryRowContext(databaseContext, `
SELECT route_no, operation_scope, COUNT(*) OVER ()
FROM idc_gateway_local_supplier_route
WHERE installation_id = $1 AND provider_code = $2 AND (`+
		strings.Join(predicates, " OR ")+`)
ORDER BY route_no
LIMIT 1`, arguments...).Scan(&result.RouteNo, &result.OperationScope, &matchCount)
	if errors.Is(err, sql.ErrNoRows) || matchCount != 1 {
		return ResolvedReference{}, ErrConflict
	}
	if err != nil || !routeNoPattern.MatchString(result.RouteNo) ||
		!operationScopePattern.MatchString(result.OperationScope) {
		return ResolvedReference{}, ErrRejected
	}
	return result, nil
}

// QueryLease contains a one-use plaintext token. The database stores only its digest.
type QueryLease struct {
	RouteNo        string
	Provider       string
	OperationScope string
	RouteVersion   int64
	Token          []byte
}

// QueryRequest exposes the external reference only during one synchronous call.
type QueryRequest struct {
	RouteNo           string
	OperationScope    string
	ExternalReference []byte
}

// QueryResponse is normalized by one provider-specific read-only client.
type QueryResponse struct {
	Status             Status
	ProviderStatus     string
	FailureCode        QueryFailureCode
	EvidenceFrame      []byte
	SupplierObservedAt time.Time
}

// ReadOnlyQuerier performs one provider-specific lookup and cannot issue writes.
type ReadOnlyQuerier interface {
	Query(context.Context, QueryRequest) (QueryResponse, error)
}

// QueryService keeps provider clients startup-frozen and has no write-client fallback.
type QueryService struct {
	database       *sql.DB
	installationID string
	keyring        *callbacksecurity.Ring
	queriers       map[string]ReadOnlyQuerier
	random         io.Reader
	queryTimeout   time.Duration
}

// ReferenceConsumer receives one short-lived plaintext provider reference.
// The caller must not retain, persist, or log the supplied bytes.
type ReferenceConsumer func([]byte) error

// TerminalReferenceRule is one exact terminal supplier state that may expose a
// short-lived reference to a reviewed in-process read-only operation.
type TerminalReferenceRule struct {
	OperationScope string
	ProviderStatus string
}

// NewQueryService freezes verified provider readers into one local query service.
func NewQueryService(
	database *sql.DB,
	installationID string,
	keyring *callbacksecurity.Ring,
	queriers map[string]ReadOnlyQuerier,
) (*QueryService, error) {
	if database == nil || !installationIDPattern.MatchString(installationID) || keyring == nil ||
		keyring.ActiveEvidenceVersion() == 0 || keyring.ActiveReferenceVersion() == 0 ||
		keyring.ActiveReferenceLookupVersion() == 0 {
		return nil, ErrInvalid
	}
	cloned := make(map[string]ReadOnlyQuerier, len(queriers))
	for provider, querier := range queriers {
		if !validProvider(provider) || nilDependency(querier) {
			return nil, ErrInvalid
		}
		cloned[provider] = querier
	}
	return &QueryService{
		database: database, installationID: installationID, keyring: keyring,
		queriers: cloned, random: rand.Reader, queryTimeout: queryExecutionTimeout,
	}, nil
}

// HasQueriers reports whether production startup may enable the query worker.
func (s *QueryService) HasQueriers() bool {
	return s != nil && len(s.queriers) > 0
}

// RunRoute acquires an immediate one-route lease and runs the same verified
// read-only query path used by the background worker.
func (s *QueryService) RunRoute(
	ctx context.Context,
	routeNo string,
	provider string,
	operationScope string,
) (result Result, returnError error) {
	if s == nil || ctx == nil || ctx.Err() != nil || !routeNoPattern.MatchString(routeNo) ||
		!validProvider(provider) || !operationScopePattern.MatchString(operationScope) {
		return Result{}, ErrInvalid
	}
	if nilDependency(s.queriers[provider]) {
		return Result{}, ErrQueryDisabled
	}
	databaseContext, cancel := context.WithTimeout(ctx, callbackDatabaseTimeout)
	defer cancel()
	transaction, err := s.database.BeginTx(
		databaseContext, &sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		return Result{}, ErrRejected
	}
	defer finishTransaction(transaction, &returnError)
	databaseNow, err := callbackDatabaseTime(databaseContext, transaction)
	if err != nil {
		return Result{}, ErrRejected
	}
	var (
		status            Status
		providerStatus    string
		reconciliation    string
		version           int64
		externalReference []byte
		leaseDigest       []byte
		leaseExpires      sql.NullTime
		supplierObserved  sql.NullTime
	)
	err = transaction.QueryRowContext(databaseContext, `
SELECT route.projected_status, COALESCE(route.provider_status, ''),
       route.reconciliation_status, route.version, route.external_ref_hmac,
       route.query_lease_sha256, route.query_lease_expires_at,
       evidence.supplier_observed_at
FROM idc_gateway_local_supplier_route AS route
LEFT JOIN LATERAL (
    SELECT supplier_observed_at
    FROM idc_gateway_local_callback_evidence
    WHERE route_no = route.route_no AND route_version = route.version
      AND provider_status IS NOT NULL
    ORDER BY observed_at DESC, evidence_no DESC
    LIMIT 1
) AS evidence ON TRUE
WHERE route.route_no = $1 AND route.installation_id = $2 AND route.provider_code = $3
  AND route.operation_scope = $4
FOR UPDATE OF route`, routeNo, s.installationID, provider, operationScope).Scan(
		&status, &providerStatus, &reconciliation, &version, &externalReference,
		&leaseDigest, &leaseExpires, &supplierObserved,
	)
	if errors.Is(err, sql.ErrNoRows) || len(externalReference) == 0 {
		return Result{}, ErrConflict
	}
	if err != nil {
		return Result{}, ErrRejected
	}
	if terminal(status) && reconciliation != "reconciliation_required" {
		if err = transaction.Commit(); err != nil {
			return Result{}, ErrRejected
		}
		return Result{RouteNo: routeNo, ProjectedStatus: status,
			ProviderStatus: providerStatus, ReconciliationStatus: reconciliation,
			RouteVersion: version, DecisionCode: "stale",
			SupplierObservedAt: supplierObserved.Time}, nil
	}
	if len(leaseDigest) > 0 && leaseExpires.Valid && leaseExpires.Time.After(databaseNow) {
		return Result{}, ErrConflict
	}
	token := make([]byte, callbacksecurity.SecretBytes)
	if _, err = io.ReadFull(s.random, token); err != nil {
		clear(token)
		return Result{}, ErrRejected
	}
	digest := sha256.Sum256(token)
	update, err := transaction.ExecContext(databaseContext, `
UPDATE idc_gateway_local_supplier_route
SET query_lease_sha256 = $2, query_lease_started_at = $3,
    query_lease_expires_at = $4, updated_at = $3
WHERE route_no = $1 AND version = $5`, routeNo, digest[:], databaseNow,
		databaseNow.Add(QueryLeaseDuration), version)
	if err != nil || rowsAffected(update) != 1 {
		clear(token)
		return Result{}, ErrRejected
	}
	if err = transaction.Commit(); err != nil {
		clear(token)
		return Result{}, ErrRejected
	}
	return s.Run(ctx, QueryLease{RouteNo: routeNo, Provider: provider,
		OperationScope: operationScope, RouteVersion: version, Token: token})
}

// WithRunningReference decrypts an existing non-terminal route reference only
// for one synchronous read-side provider call.
func (s *QueryService) WithRunningReference(
	ctx context.Context,
	routeNo string,
	provider string,
	operationScope string,
	consume ReferenceConsumer,
) error {
	if s == nil || ctx == nil || ctx.Err() != nil || consume == nil ||
		!routeNoPattern.MatchString(routeNo) || !validProvider(provider) ||
		!operationScopePattern.MatchString(operationScope) {
		return ErrInvalid
	}
	var sealed callbacksecurity.SealedEvidence
	var status Status
	var reconciliation string
	err := s.database.QueryRowContext(ctx, `
SELECT projected_status, reconciliation_status,
       external_ref_cipher_format_version, external_ref_encryption_key_version,
       external_ref_nonce, external_ref_ciphertext
FROM idc_gateway_local_supplier_route
WHERE route_no = $1 AND installation_id = $2 AND provider_code = $3
  AND operation_scope = $4 AND external_ref_hmac IS NOT NULL`,
		routeNo, s.installationID, provider, operationScope).Scan(
		&status, &reconciliation, &sealed.FormatVersion, &sealed.KeyVersion,
		&sealed.Nonce, &sealed.Ciphertext,
	)
	if errors.Is(err, sql.ErrNoRows) || status != StatusRunning || reconciliation != "none" {
		return ErrConflict
	}
	if err != nil {
		return ErrRejected
	}
	defer clear(sealed.Nonce)
	defer clear(sealed.Ciphertext)
	reference, err := s.keyring.OpenReference(routeNo, provider, sealed)
	if err != nil {
		return ErrRejected
	}
	defer clear(reference)
	return consume(reference)
}

// WithTerminalReference decrypts a succeeded route reference only when its
// operation and provider status match one of the caller's explicit rules.
func (s *QueryService) WithTerminalReference(
	ctx context.Context,
	routeNo string,
	provider string,
	rules []TerminalReferenceRule,
	consume ReferenceConsumer,
) error {
	if s == nil || ctx == nil || ctx.Err() != nil || consume == nil ||
		!routeNoPattern.MatchString(routeNo) || !validProvider(provider) ||
		len(rules) == 0 || len(rules) > 8 {
		return ErrInvalid
	}
	allowed := make(map[string]map[string]struct{}, len(rules))
	for _, rule := range rules {
		if !operationScopePattern.MatchString(rule.OperationScope) ||
			!validProviderStatus(provider, rule.ProviderStatus) {
			return ErrInvalid
		}
		statuses := allowed[rule.OperationScope]
		if statuses == nil {
			statuses = map[string]struct{}{}
			allowed[rule.OperationScope] = statuses
		}
		statuses[rule.ProviderStatus] = struct{}{}
	}
	var sealed callbacksecurity.SealedEvidence
	var status Status
	var operationScope string
	var providerStatus string
	var reconciliation string
	err := s.database.QueryRowContext(ctx, `
SELECT operation_scope, projected_status, COALESCE(provider_status, ''),
       reconciliation_status, external_ref_cipher_format_version,
       external_ref_encryption_key_version, external_ref_nonce,
       external_ref_ciphertext
FROM idc_gateway_local_supplier_route
WHERE route_no = $1 AND installation_id = $2 AND provider_code = $3
  AND external_ref_hmac IS NOT NULL`, routeNo, s.installationID, provider).Scan(
		&operationScope, &status, &providerStatus, &reconciliation,
		&sealed.FormatVersion, &sealed.KeyVersion, &sealed.Nonce, &sealed.Ciphertext,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrConflict
	}
	if err != nil {
		return ErrRejected
	}
	_, permitted := allowed[operationScope][providerStatus]
	if status != StatusSucceeded || reconciliation != "none" || !permitted {
		return ErrConflict
	}
	defer clear(sealed.Nonce)
	defer clear(sealed.Ciphertext)
	reference, err := s.keyring.OpenReference(routeNo, provider, sealed)
	if err != nil {
		return ErrRejected
	}
	defer clear(reference)
	return consume(reference)
}

// RunWorker executes immediate and periodic bounded query batches.
func (s *QueryService) RunWorker(ctx context.Context) error {
	if s == nil || ctx == nil {
		return ErrInvalid
	}
	if !s.HasQueriers() {
		return ErrQueryDisabled
	}
	for {
		if err := s.runQueryBatch(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		timer := time.NewTimer(QueryWorkerPollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil
		case <-timer.C:
		}
	}
}

func (s *QueryService) runQueryBatch(ctx context.Context) error {
	leases, err := s.Claim(ctx, MaximumQueryBatchSize)
	if err != nil || len(leases) == 0 {
		return err
	}
	workerCount := min(MaximumConcurrentQueries, len(leases))
	jobs := make(chan QueryLease)
	errorsFound := make(chan error, len(leases))
	var wait sync.WaitGroup
	for range workerCount {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for lease := range jobs {
				if _, runErr := s.Run(ctx, lease); runErr != nil &&
					!errors.Is(runErr, ErrLeaseLost) && ctx.Err() == nil {
					errorsFound <- runErr
				}
			}
		}()
	}
	for _, lease := range leases {
		jobs <- lease
	}
	close(jobs)
	wait.Wait()
	close(errorsFound)
	for runErr := range errorsFound {
		return fmt.Errorf("run local supplier query batch: %w", runErr)
	}
	return nil
}

// BindExternalReference protects one provider reference and makes the route queryable.
func (s *QueryService) BindExternalReference(
	ctx context.Context,
	input ReferenceBinding,
) (routeVersion int64, returnError error) {
	defer clear(input.ExternalReference)
	if s == nil || ctx == nil || ctx.Err() != nil ||
		!routeNoPattern.MatchString(input.RouteNo) || !validProvider(input.Provider) ||
		!operationScopePattern.MatchString(input.OperationScope) ||
		len(input.ExternalReference) == 0 ||
		len(input.ExternalReference) > MaximumExternalReferenceBytes {
		return 0, ErrInvalid
	}
	digests, err := s.keyring.ReferenceDigests(input.Provider, input.ExternalReference)
	if err != nil {
		return 0, ErrRejected
	}
	activeDigest, ok := activeEventDigest(s.keyring.ActiveReferenceLookupVersion(), digests)
	if !ok {
		return 0, ErrRejected
	}
	sealed, err := s.keyring.SealReference(
		s.random, input.RouteNo, input.Provider, input.ExternalReference,
	)
	if err != nil {
		return 0, ErrRejected
	}
	defer clear(sealed.Nonce)
	defer clear(sealed.Ciphertext)
	defer clear(sealed.PlainSHA256[:])
	databaseContext, cancel := context.WithTimeout(ctx, callbackDatabaseTimeout)
	defer cancel()
	transaction, err := s.database.BeginTx(
		databaseContext, &sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		return 0, ErrRejected
	}
	defer finishTransaction(transaction, &returnError)
	databaseNow, err := callbackDatabaseTime(databaseContext, transaction)
	if err != nil {
		return 0, ErrRejected
	}
	boundRouteNo, bound, err := findBoundReferenceRoute(
		databaseContext, transaction, input.Provider, digests,
	)
	if err != nil {
		return 0, err
	}
	if bound && boundRouteNo != input.RouteNo {
		return 0, ErrConflict
	}
	var (
		provider, operationScope string
		version                  int64
		existingKeyVersion       sql.NullInt64
		existingHMAC             []byte
	)
	err = transaction.QueryRowContext(databaseContext, `
SELECT provider_code, operation_scope, version, external_ref_hmac_key_version,
       external_ref_hmac
FROM idc_gateway_local_supplier_route
WHERE route_no = $1 AND installation_id = $2
FOR UPDATE`, input.RouteNo, s.installationID).Scan(
		&provider, &operationScope, &version, &existingKeyVersion, &existingHMAC,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrConflict
	}
	if err != nil || provider != input.Provider || operationScope != input.OperationScope {
		return 0, ErrConflict
	}
	if existingKeyVersion.Valid {
		matches := false
		for _, digest := range digests {
			if int64(digest.KeyVersion) == existingKeyVersion.Int64 &&
				equalDigest(existingHMAC, digest.Sum) {
				matches = true
				break
			}
		}
		if !matches {
			return 0, ErrConflict
		}
		if err = transaction.Commit(); err != nil {
			return 0, ErrRejected
		}
		return version, nil
	}
	result, err := transaction.ExecContext(databaseContext, `
UPDATE idc_gateway_local_supplier_route
SET external_ref_ciphertext = $3, external_ref_nonce = $4,
    external_ref_cipher_format_version = $5,
    external_ref_encryption_key_version = $6,
    external_ref_hmac_key_version = $7, external_ref_hmac = $8,
    next_query_at = $9, query_failure_count = 0, updated_at = $9,
    version = version + 1
WHERE route_no = $1 AND installation_id = $2 AND version = $10
  AND external_ref_hmac IS NULL`,
		input.RouteNo, s.installationID, sealed.Ciphertext, sealed.Nonce,
		sealed.FormatVersion, sealed.KeyVersion, activeDigest.KeyVersion,
		activeDigest.Sum[:], databaseNow, version)
	if err != nil || rowsAffected(result) != 1 {
		if externalReferenceUniqueConflict(err) {
			return 0, ErrConflict
		}
		return 0, ErrRejected
	}
	if err = transaction.Commit(); err != nil {
		return 0, ErrRejected
	}
	return version + 1, nil
}

// Claim uses one bounded SKIP LOCKED read and one set-based lease update.
func (s *QueryService) Claim(ctx context.Context, limit int) (leases []QueryLease, returnError error) {
	if s == nil || ctx == nil || ctx.Err() != nil || limit < 1 || limit > MaximumQueryBatchSize {
		return nil, ErrInvalid
	}
	providers := make([]string, 0, len(s.queriers))
	for provider := range s.queriers {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	if len(providers) == 0 {
		return []QueryLease{}, nil
	}
	databaseContext, cancel := context.WithTimeout(ctx, callbackDatabaseTimeout)
	defer cancel()
	transaction, err := s.database.BeginTx(
		databaseContext, &sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		return nil, ErrRejected
	}
	defer finishTransaction(transaction, &returnError)
	databaseNow, err := callbackDatabaseTime(databaseContext, transaction)
	if err != nil {
		return nil, ErrRejected
	}
	arguments := []any{limit, s.installationID}
	providerParameters := make([]string, 0, len(providers))
	for _, provider := range providers {
		arguments = append(arguments, provider)
		providerParameters = append(providerParameters, fmt.Sprintf("$%d", len(arguments)))
	}
	rows, err := transaction.QueryContext(databaseContext, `
SELECT route_no, provider_code, operation_scope, version
FROM idc_gateway_local_supplier_route
WHERE installation_id = $2
  AND provider_code IN (`+strings.Join(providerParameters, ", ")+`)
  AND external_ref_hmac IS NOT NULL
  AND next_query_at <= CURRENT_TIMESTAMP
  AND (query_lease_sha256 IS NULL OR query_lease_expires_at <= CURRENT_TIMESTAMP)
  AND (
      projected_status IN ('running', 'result_unknown')
      OR reconciliation_status = 'reconciliation_required'
  )
ORDER BY next_query_at, route_no
LIMIT $1
FOR UPDATE SKIP LOCKED`, arguments...)
	if err != nil {
		return nil, fmt.Errorf("%w: select due supplier queries", ErrRejected)
	}
	defer func() {
		if closeError := rows.Close(); closeError != nil && returnError == nil {
			clearQueryLeases(leases)
			leases = nil
			returnError = ErrRejected
		}
	}()
	for rows.Next() {
		var lease QueryLease
		if err = rows.Scan(
			&lease.RouteNo, &lease.Provider, &lease.OperationScope, &lease.RouteVersion,
		); err != nil {
			clearQueryLeases(leases)
			return nil, ErrRejected
		}
		lease.Token = make([]byte, callbacksecurity.SecretBytes)
		if _, err = io.ReadFull(s.random, lease.Token); err != nil {
			clear(lease.Token)
			clearQueryLeases(leases)
			return nil, ErrRejected
		}
		leases = append(leases, lease)
	}
	if err = rows.Close(); err != nil || rows.Err() != nil {
		clearQueryLeases(leases)
		return nil, ErrRejected
	}
	if len(leases) == 0 {
		if err = transaction.Commit(); err != nil {
			return nil, ErrRejected
		}
		return []QueryLease{}, nil
	}
	updateArguments := make([]any, 0, len(leases)*3+2)
	valueRows := make([]string, 0, len(leases))
	for _, lease := range leases {
		tokenDigest := sha256.Sum256(lease.Token)
		updateArguments = append(
			updateArguments, lease.RouteNo, lease.RouteVersion, tokenDigest[:],
		)
		base := len(updateArguments) - 2
		valueRows = append(valueRows, fmt.Sprintf(
			"($%d::VARCHAR(40), $%d::BIGINT, $%d::BYTEA)", base, base+1, base+2,
		))
	}
	updateArguments = append(
		updateArguments, databaseNow, databaseNow.Add(QueryLeaseDuration),
	)
	startedParameter := len(updateArguments) - 1
	expiresParameter := len(updateArguments)
	result, err := transaction.ExecContext(databaseContext, `
UPDATE idc_gateway_local_supplier_route route
SET query_lease_sha256 = claimed.lease_sha256,
    query_lease_started_at = $`+fmt.Sprint(startedParameter)+`,
    query_lease_expires_at = $`+fmt.Sprint(expiresParameter)+`,
    updated_at = $`+fmt.Sprint(startedParameter)+`
FROM (VALUES `+strings.Join(valueRows, ", ")+`) AS claimed(
    route_no, route_version, lease_sha256
)
WHERE route.route_no = claimed.route_no
  AND route.version = claimed.route_version`, updateArguments...)
	if err != nil || rowsAffected(result) != int64(len(leases)) {
		clearQueryLeases(leases)
		return nil, fmt.Errorf("%w: persist supplier query leases", ErrRejected)
	}
	if err = transaction.Commit(); err != nil {
		clearQueryLeases(leases)
		return nil, ErrRejected
	}
	return leases, nil
}

// Run decrypts a reference only around one provider-specific read-only call.
func (s *QueryService) Run(ctx context.Context, lease QueryLease) (Result, error) {
	defer clear(lease.Token)
	if !validQueryLease(s, ctx, lease) || s.queryTimeout <= 0 ||
		s.queryTimeout >= QueryLeaseDuration {
		return Result{}, ErrInvalid
	}
	querier := s.queriers[lease.Provider]
	if nilDependency(querier) {
		return Result{}, ErrQueryDisabled
	}
	var response QueryResponse
	queryContext, cancelQuery := context.WithTimeout(ctx, s.queryTimeout)
	err := s.withExternalReference(queryContext, lease, func(reference []byte) error {
		var queryError error
		response, queryError = querier.Query(queryContext, QueryRequest{
			RouteNo: lease.RouteNo, OperationScope: lease.OperationScope,
			ExternalReference: reference,
		})
		return queryError
	})
	cancelQuery()
	defer clear(response.EvidenceFrame)
	if err != nil {
		if errors.Is(err, ErrLeaseLost) || errors.Is(err, ErrRejected) {
			return Result{}, err
		}
		if ctx.Err() != nil {
			return Result{}, ctx.Err()
		}
		failure := QueryFailureTransportUnavailable
		if errors.Is(err, context.DeadlineExceeded) {
			failure = QueryFailureTimeout
		}
		return s.reportQueryFailure(
			ctx, lease, failure, []byte("query_failure:"+failure), time.Time{},
		)
	}
	if !validQueryResponse(lease.Provider, response) {
		return s.reportQueryFailure(
			ctx, lease, QueryFailureMalformedResponse,
			[]byte("query_failure:malformed_response"), time.Time{},
		)
	}
	if response.FailureCode != "" {
		evidence := response.EvidenceFrame
		if len(evidence) == 0 {
			evidence = []byte("query_failure:" + response.FailureCode)
			defer clear(evidence)
		}
		return s.reportQueryFailure(
			ctx, lease, response.FailureCode, evidence, response.SupplierObservedAt,
		)
	}
	return s.reportQueryObservation(ctx, lease, response)
}

func (s *QueryService) withExternalReference(
	ctx context.Context,
	lease QueryLease,
	callback func([]byte) error,
) error {
	digest := sha256.Sum256(lease.Token)
	var sealed callbacksecurity.SealedEvidence
	err := s.database.QueryRowContext(ctx, `
SELECT external_ref_cipher_format_version, external_ref_encryption_key_version,
       external_ref_nonce, external_ref_ciphertext
FROM idc_gateway_local_supplier_route
WHERE route_no = $1 AND installation_id = $2 AND provider_code = $3
  AND operation_scope = $4 AND version = $5
  AND query_lease_sha256 = $6
  AND query_lease_expires_at > CURRENT_TIMESTAMP`,
		lease.RouteNo, s.installationID, lease.Provider, lease.OperationScope,
		lease.RouteVersion, digest[:]).Scan(
		&sealed.FormatVersion, &sealed.KeyVersion, &sealed.Nonce,
		&sealed.Ciphertext,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrLeaseLost
	}
	if err != nil {
		return ErrRejected
	}
	defer clear(sealed.Nonce)
	defer clear(sealed.Ciphertext)
	reference, err := s.keyring.OpenReference(lease.RouteNo, lease.Provider, sealed)
	if err != nil {
		return ErrRejected
	}
	defer clear(reference)
	return callback(reference)
}

type queryRouteRecord struct {
	routeRecord
	QueryFailureCount int
	QueryLeaseSHA256  []byte
	QueryLeaseExpires time.Time
}

func lockQueryRoute(
	ctx context.Context,
	transaction *sql.Tx,
	routeNo string,
) (queryRouteRecord, error) {
	var result queryRouteRecord
	err := transaction.QueryRowContext(ctx, `
SELECT route_no, attempt_no, request_no, installation_id, provider_code,
	   operation_scope, projected_status, COALESCE(provider_status, ''),
	   reconciliation_status, version,
       query_failure_count, query_lease_sha256, query_lease_expires_at
FROM idc_gateway_local_supplier_route
WHERE route_no = $1
FOR UPDATE`, routeNo).Scan(
		&result.RouteNo, &result.AttemptNo, &result.RequestNo, &result.InstallationID,
		&result.Provider, &result.OperationScope, &result.ProjectedStatus,
		&result.ProviderStatus, &result.ReconciliationStatus, &result.Version,
		&result.QueryFailureCount,
		&result.QueryLeaseSHA256, &result.QueryLeaseExpires,
	)
	return result, err
}

func (s *QueryService) reportQueryFailure(
	ctx context.Context,
	lease QueryLease,
	failure QueryFailureCode,
	evidenceFrame []byte,
	supplierObservedAt time.Time,
) (result Result, returnError error) {
	if !validQueryFailure(failure) || len(evidenceFrame) == 0 ||
		len(evidenceFrame) > MaximumEvidenceFrameBytes {
		return Result{}, ErrInvalid
	}
	identity, err := readTakeoverRouteIdentity(
		ctx, s.database, s.installationID, lease.RouteNo,
	)
	if err != nil {
		return Result{}, err
	}
	evidenceNo, err := randomIdentifier(s.random, "lev_")
	if err != nil {
		return Result{}, ErrRejected
	}
	sealed, err := s.keyring.Seal(
		s.random, evidenceNo, lease.RouteNo, lease.Provider, "active_query", evidenceFrame,
	)
	if err != nil || len(sealed.Ciphertext) > MaximumEvidenceCiphertextBytes {
		return Result{}, ErrRejected
	}
	defer clear(sealed.Nonce)
	defer clear(sealed.Ciphertext)
	databaseContext, cancel := context.WithTimeout(ctx, callbackDatabaseTimeout)
	defer cancel()
	transaction, err := s.database.BeginTx(
		databaseContext, &sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		return Result{}, ErrRejected
	}
	defer finishTransaction(transaction, &returnError)
	databaseNow, err := callbackDatabaseTime(databaseContext, transaction)
	if err != nil {
		return Result{}, ErrRejected
	}
	if _, err = lockBusinessRequest(
		databaseContext, transaction, identity.RequestNo, s.installationID,
	); err != nil {
		return Result{}, err
	}
	route, err := lockQueryRoute(databaseContext, transaction, lease.RouteNo)
	if err != nil || !sameRouteOwner(route.routeRecord, identity) ||
		!validLeaseFence(route, lease, databaseNow) {
		return Result{}, ErrLeaseLost
	}
	failureCount := route.QueryFailureCount + 1
	if failureCount > 1000000 {
		failureCount = 1000000
	}
	nextQueryAt := databaseNow.Add(queryBackoff(failureCount))
	updateResult, err := transaction.ExecContext(databaseContext, `
UPDATE idc_gateway_local_supplier_route
SET query_failure_count = $2, next_query_at = $3,
    query_lease_sha256 = NULL, query_lease_started_at = NULL,
    query_lease_expires_at = NULL, updated_at = $4
WHERE route_no = $1 AND version = $5 AND query_lease_sha256 = $6`,
		route.RouteNo, failureCount, nextQueryAt, databaseNow, route.Version,
		route.QueryLeaseSHA256)
	if err != nil || rowsAffected(updateResult) != 1 {
		return Result{}, ErrLeaseLost
	}
	if err = insertQueryEvidence(
		databaseContext, transaction, route.routeRecord, evidenceNo, sealed,
		route.ProjectedStatus, "", route.ProjectedStatus, route.ReconciliationStatus,
		"query_failed", failure, route.Version, supplierObservedAt, databaseNow,
	); err != nil {
		return Result{}, err
	}
	if err = transaction.Commit(); err != nil {
		return Result{}, ErrRejected
	}
	return Result{
		RouteNo: route.RouteNo, ProjectedStatus: route.ProjectedStatus,
		ProviderStatus:       route.ProviderStatus,
		ReconciliationStatus: route.ReconciliationStatus, RouteVersion: route.Version,
		EvidenceNo: evidenceNo, DecisionCode: "query_failed",
	}, nil
}

func (s *QueryService) reportQueryObservation(
	ctx context.Context,
	lease QueryLease,
	response QueryResponse,
) (result Result, returnError error) {
	identity, err := readTakeoverRouteIdentity(
		ctx, s.database, s.installationID, lease.RouteNo,
	)
	if err != nil {
		return Result{}, err
	}
	evidenceNo, err := randomIdentifier(s.random, "lev_")
	if err != nil {
		return Result{}, ErrRejected
	}
	sealed, err := s.keyring.Seal(
		s.random, evidenceNo, lease.RouteNo, lease.Provider, "active_query",
		response.EvidenceFrame,
	)
	if err != nil || len(sealed.Ciphertext) > MaximumEvidenceCiphertextBytes {
		return Result{}, ErrRejected
	}
	defer clear(sealed.Nonce)
	defer clear(sealed.Ciphertext)
	databaseContext, cancel := context.WithTimeout(ctx, callbackDatabaseTimeout)
	defer cancel()
	transaction, err := s.database.BeginTx(
		databaseContext, &sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		return Result{}, ErrRejected
	}
	defer finishTransaction(transaction, &returnError)
	databaseNow, err := callbackDatabaseTime(databaseContext, transaction)
	if err != nil {
		return Result{}, ErrRejected
	}
	businessStatus, err := lockBusinessRequest(
		databaseContext, transaction, identity.RequestNo, s.installationID,
	)
	if err != nil {
		return Result{}, err
	}
	route, err := lockQueryRoute(databaseContext, transaction, lease.RouteNo)
	if err != nil || !sameRouteOwner(route.routeRecord, identity) ||
		!validLeaseFence(route, lease, databaseNow) {
		return Result{}, ErrLeaseLost
	}
	change, providerChanged := decideObservedQueryTransition(
		route.routeRecord, businessStatus, response.Status, response.ProviderStatus,
	)
	routeVersion := route.Version
	if change.Changed || providerChanged {
		routeVersion++
	}
	if change.Changed {
		if err = applyBusinessProjection(
			databaseContext, transaction, route.RequestNo, s.installationID,
			businessStatus, change.ProjectedStatus, change.DecisionCode,
		); err != nil {
			return Result{}, err
		}
	}
	var nextQueryAt any
	if !terminal(change.ProjectedStatus) &&
		(change.ProjectedStatus == StatusRunning ||
			change.ProjectedStatus == StatusResultUnknown ||
			change.ReconciliationStatus == "reconciliation_required") {
		nextQueryAt = databaseNow.Add(querySuccessPollInterval)
	}
	updateResult, err := transaction.ExecContext(databaseContext, `
UPDATE idc_gateway_local_supplier_route
SET projected_status = $2, reconciliation_status = $3,
	provider_status = CASE WHEN $4::VARCHAR(32) = '' THEN provider_status ELSE $4::VARCHAR(32) END,
	query_failure_count = 0, next_query_at = $5,
    query_lease_sha256 = NULL, query_lease_started_at = NULL,
	query_lease_expires_at = NULL, updated_at = $6, version = $7
WHERE route_no = $1 AND version = $8 AND query_lease_sha256 = $9`,
		route.RouteNo, string(change.ProjectedStatus), change.ReconciliationStatus,
		response.ProviderStatus, nextQueryAt, databaseNow, routeVersion, route.Version,
		route.QueryLeaseSHA256)
	if err != nil || rowsAffected(updateResult) != 1 {
		return Result{}, ErrLeaseLost
	}
	if err = insertQueryEvidence(
		databaseContext, transaction, route.routeRecord, evidenceNo, sealed,
		response.Status, response.ProviderStatus, change.ProjectedStatus, change.ReconciliationStatus,
		change.DecisionCode, "", routeVersion, response.SupplierObservedAt, databaseNow,
	); err != nil {
		return Result{}, err
	}
	deliveryNo := ""
	if change.Changed || providerChanged {
		deliveryNo, err = randomIdentifier(s.random, "ldv_")
		if err != nil {
			return Result{}, ErrRejected
		}
		insertResult, insertErr := transaction.ExecContext(databaseContext, `
INSERT INTO idc_gateway_local_route_delivery
    (delivery_no, route_no, request_no, installation_id, provider_code,
     operation_scope, route_version, projected_status, reconciliation_status,
     evidence_no, evidence_sha256)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			deliveryNo, route.RouteNo, route.RequestNo, route.InstallationID,
			route.Provider, route.OperationScope, routeVersion,
			string(change.ProjectedStatus), change.ReconciliationStatus,
			evidenceNo, sealed.PlainSHA256[:])
		if insertErr != nil || rowsAffected(insertResult) != 1 {
			return Result{}, ErrRejected
		}
	}
	if err = transaction.Commit(); err != nil {
		return Result{}, ErrRejected
	}
	return Result{
		RouteNo: route.RouteNo, ProjectedStatus: change.ProjectedStatus,
		ProviderStatus:       response.ProviderStatus,
		ReconciliationStatus: change.ReconciliationStatus, RouteVersion: routeVersion,
		EvidenceNo: evidenceNo, DeliveryNo: deliveryNo, DecisionCode: change.DecisionCode,
		SupplierObservedAt: response.SupplierObservedAt,
	}, nil
}

func decideObservedQueryTransition(
	route routeRecord,
	businessStatus string,
	incoming Status,
	providerStatus string,
) (transition, bool) {
	change := decideQueryTransition(route, businessStatus, incoming)
	providerChanged := providerStatus != "" && providerStatus != route.ProviderStatus
	if providerChanged && !change.Changed {
		change.DecisionCode = decideProviderStatusObservation(route.ProviderStatus, providerStatus)
	}
	return change, providerChanged
}

func insertQueryEvidence(
	ctx context.Context,
	transaction *sql.Tx,
	route routeRecord,
	evidenceNo string,
	sealed callbacksecurity.SealedEvidence,
	normalizedStatus Status,
	providerStatus string,
	projectedStatus Status,
	reconciliationStatus string,
	decisionCode string,
	stableCode QueryFailureCode,
	routeVersion int64,
	supplierObservedAt time.Time,
	databaseNow time.Time,
) error {
	result, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_local_callback_evidence
    (evidence_no, route_no, request_no, installation_id, provider_code,
     operation_scope, source_kind, payload_ciphertext, payload_nonce,
     payload_sha256, cipher_format_version, encryption_key_version,
	 normalized_status, provider_status, projected_status, reconciliation_status,
     decision_code, stable_code, route_version, supplier_observed_at, observed_at)
VALUES ($1, $2, $3, $4, $5, $6, 'active_query', $7, $8, $9, $10, $11,
		$12, $13, $14, $15, $16, $17, $18, $19, $20)`,
		evidenceNo, route.RouteNo, route.RequestNo, route.InstallationID,
		route.Provider, route.OperationScope, sealed.Ciphertext, sealed.Nonce,
		sealed.PlainSHA256[:], sealed.FormatVersion, sealed.KeyVersion,
		string(normalizedStatus), nullableProviderStatus(providerStatus),
		string(projectedStatus), reconciliationStatus,
		decisionCode, nullableQueryFailure(stableCode), routeVersion,
		nullableSupplierTime(supplierObservedAt), databaseNow)
	if err != nil || rowsAffected(result) != 1 {
		return ErrRejected
	}
	return nil
}

func decideQueryTransition(
	route routeRecord,
	businessStatus string,
	incoming Status,
) transition {
	if route.ReconciliationStatus == "reconciliation_required" && terminal(route.ProjectedStatus) {
		if incoming == route.ProjectedStatus {
			return transition{
				ProjectedStatus: route.ProjectedStatus, ReconciliationStatus: "none",
				DecisionCode: "applied", Changed: true,
			}
		}
		decision := "stale"
		if terminal(incoming) {
			decision = "conflict"
		}
		return unchangedTransition(route, decision)
	}
	if route.ReconciliationStatus == "reconciliation_required" {
		if !terminal(incoming) {
			return unchangedTransition(route, "stale")
		}
		route.ReconciliationStatus = "query_required"
	}
	return decideTransition(route, businessStatus, incoming)
}

func validQueryLease(service *QueryService, ctx context.Context, lease QueryLease) bool {
	return service != nil && service.database != nil && service.keyring != nil &&
		ctx != nil && ctx.Err() == nil && routeNoPattern.MatchString(lease.RouteNo) &&
		validProvider(lease.Provider) && operationScopePattern.MatchString(lease.OperationScope) &&
		lease.RouteVersion > 0 && len(lease.Token) == callbacksecurity.SecretBytes
}

func validQueryResponse(provider string, response QueryResponse) bool {
	if len(response.EvidenceFrame) > MaximumEvidenceFrameBytes {
		return false
	}
	if response.FailureCode != "" {
		return response.Status == "" && validQueryFailure(response.FailureCode)
	}
	return validStatus(response.Status) &&
		(response.ProviderStatus == "" || validProviderStatus(provider, response.ProviderStatus)) &&
		len(response.EvidenceFrame) > 0 &&
		!response.SupplierObservedAt.IsZero()
}

func validQueryFailure(value QueryFailureCode) bool {
	switch value {
	case QueryFailureTransportUnavailable, QueryFailureTimeout,
		QueryFailureMalformedResponse, QueryFailureReferenceMismatch,
		QueryFailureProviderRejected:
		return true
	default:
		return false
	}
}

func validLeaseFence(route queryRouteRecord, lease QueryLease, databaseNow time.Time) bool {
	digest := sha256.Sum256(lease.Token)
	return route.InstallationID != "" && route.Provider == lease.Provider &&
		route.OperationScope == lease.OperationScope && route.Version == lease.RouteVersion &&
		equalDigest(route.QueryLeaseSHA256, digest) && route.QueryLeaseExpires.After(databaseNow)
}

func queryBackoff(failureCount int) time.Duration {
	if failureCount <= 1 {
		return time.Minute
	}
	if failureCount >= 6 {
		return 30 * time.Minute
	}
	return time.Minute * time.Duration(1<<(failureCount-1))
}

func clearQueryLeases(leases []QueryLease) {
	for index := range leases {
		clear(leases[index].Token)
		leases[index].Token = nil
	}
}

func findBoundReferenceRoute(
	ctx context.Context,
	transaction *sql.Tx,
	provider string,
	digests []callbacksecurity.EventDigest,
) (string, bool, error) {
	if len(digests) == 0 || len(digests) > callbacksecurity.MaximumVersionsPerFamily {
		return "", false, ErrRejected
	}
	arguments := []any{provider}
	predicates := make([]string, 0, len(digests))
	for _, digest := range digests {
		arguments = append(arguments, digest.KeyVersion, digest.Sum[:])
		predicates = append(predicates, fmt.Sprintf(
			"(external_ref_hmac_key_version = $%d AND external_ref_hmac = $%d)",
			len(arguments)-1, len(arguments),
		))
	}
	var routeNo string
	var matchCount int
	err := transaction.QueryRowContext(ctx, `
SELECT route_no, COUNT(*) OVER ()
FROM idc_gateway_local_supplier_route
WHERE provider_code = $1 AND (`+strings.Join(predicates, " OR ")+`)
ORDER BY route_no
LIMIT 1`, arguments...).Scan(&routeNo, &matchCount)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, ErrRejected
	}
	if matchCount != 1 {
		return "", false, ErrConflict
	}
	return routeNo, true, nil
}

func nullableQueryFailure(value QueryFailureCode) any {
	if value == "" {
		return nil
	}
	return string(value)
}

func externalReferenceUniqueConflict(err error) bool {
	return postgresConstraintConflict(err, "uq_gateway_local_route_external_ref")
}
