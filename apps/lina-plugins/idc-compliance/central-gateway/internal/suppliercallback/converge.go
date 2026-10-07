// This file commits callback deduplication, encrypted evidence, monotonic route
// projection, and local delivery in one PostgreSQL transaction.

package suppliercallback

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"lina-platform/idc-compliance-gateway/internal/callbacksecurity"
)

const callbackDatabaseTimeout = 3 * time.Second

type routeRecord struct {
	RouteNo              string
	AttemptNo            string
	RequestNo            string
	InstallationID       string
	Provider             string
	OperationScope       string
	ProjectedStatus      Status
	ProviderStatus       string
	ReconciliationStatus string
	Version              int64
}

type transition struct {
	ProjectedStatus      Status
	ReconciliationStatus string
	DecisionCode         string
	Changed              bool
}

// converge accepts an already verified event and is intentionally unavailable
// outside the provider-verifier boundary.
func (s *Service) converge(ctx context.Context, event VerifiedEvent) (result Result, err error) {
	if !s.canConverge(ctx, event) {
		return Result{}, ErrInvalid
	}
	databaseContext, cancel := context.WithTimeout(ctx, callbackDatabaseTimeout)
	defer cancel()
	identity, err := s.readRouteIdentity(databaseContext, event.RouteNo)
	if err != nil {
		return Result{}, err
	}
	transaction, err := s.database.BeginTx(
		databaseContext, &sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		return Result{}, ErrRejected
	}
	defer finishTransaction(transaction, &err)
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
	route, err := lockRoute(databaseContext, transaction, event.RouteNo)
	if err != nil || !sameRouteOwner(route, identity) || route.InstallationID != s.installationID ||
		route.Provider != event.Provider || route.OperationScope != event.OperationScope {
		return Result{}, ErrConflict
	}
	eventDigests, err := s.keyring.EventDigests(event.Provider, event.ExternalEventID)
	if err != nil {
		return Result{}, ErrRejected
	}
	payloadDigest := digestBytes(event.EvidenceFrame)
	if recovered, found, recoverErr := findExistingEvent(
		databaseContext, transaction, route, event, eventDigests, payloadDigest,
	); recoverErr != nil {
		return Result{}, recoverErr
	} else if found {
		if err = transaction.Commit(); err != nil {
			return Result{}, ErrRejected
		}
		return recovered, nil
	}
	change, providerChanged := decideObservedTransition(route, businessStatus, event.Status, event.ProviderStatus)
	evidenceNo, err := randomIdentifier(s.random, "lev_")
	if err != nil {
		return Result{}, ErrRejected
	}
	sealed, err := s.keyring.Seal(
		s.random, evidenceNo, route.RouteNo, route.Provider, "callback", event.EvidenceFrame,
	)
	if err != nil || len(sealed.Ciphertext) > MaximumEvidenceCiphertextBytes {
		clear(sealed.Nonce)
		clear(sealed.Ciphertext)
		return Result{}, ErrRejected
	}
	defer clear(sealed.Nonce)
	defer clear(sealed.Ciphertext)
	activeDigest, ok := activeEventDigest(s.keyring.ActiveEventVersion(), eventDigests)
	if !ok {
		return Result{}, ErrRejected
	}
	routeVersion := route.Version
	deliveryNo := ""
	if change.Changed || providerChanged {
		routeVersion++
		if change.Changed {
			err = applyBusinessProjection(
				databaseContext, transaction, route.RequestNo, s.installationID,
				businessStatus, change.ProjectedStatus, change.DecisionCode,
			)
			if err != nil {
				return Result{}, err
			}
		}
		updateResult, updateErr := transaction.ExecContext(databaseContext, `
UPDATE idc_gateway_local_supplier_route
SET projected_status = $2::VARCHAR(24), reconciliation_status = $3::VARCHAR(32),
	provider_status = CASE WHEN $4::VARCHAR(32) = '' THEN provider_status ELSE $4::VARCHAR(32) END,
    next_query_at = CASE
        WHEN external_ref_hmac IS NOT NULL
             AND ($2::VARCHAR(24) IN ('running', 'result_unknown')
                  OR $3::VARCHAR(32) = 'reconciliation_required')
			THEN $5::TIMESTAMPTZ
        ELSE NULL::TIMESTAMPTZ
    END,
    query_lease_sha256 = NULL, query_lease_started_at = NULL,
	query_lease_expires_at = NULL, updated_at = $5, version = $6
WHERE route_no = $1 AND version = $7`, route.RouteNo,
			string(change.ProjectedStatus), change.ReconciliationStatus,
			event.ProviderStatus, databaseNow, routeVersion, route.Version)
		if updateErr != nil || rowsAffected(updateResult) != 1 {
			return Result{}, fmt.Errorf("%w: update callback route projection", ErrRejected)
		}
	}
	insertResult, err := transaction.ExecContext(databaseContext, `
INSERT INTO idc_gateway_local_callback_evidence
    (evidence_no, route_no, request_no, installation_id, provider_code,
     operation_scope, event_key_version, external_event_hmac,
     payload_ciphertext, payload_nonce, payload_sha256,
	 cipher_format_version, encryption_key_version, normalized_status, provider_status,
     projected_status, reconciliation_status, decision_code, route_version,
     supplier_observed_at, observed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
		$14, $15, $16, $17, $18, $19, $20, $21)`,
		evidenceNo, route.RouteNo, route.RequestNo, route.InstallationID,
		route.Provider, route.OperationScope, activeDigest.KeyVersion, activeDigest.Sum[:],
		sealed.Ciphertext, sealed.Nonce, sealed.PlainSHA256[:], sealed.FormatVersion,
		sealed.KeyVersion, string(event.Status), nullableProviderStatus(event.ProviderStatus),
		string(change.ProjectedStatus),
		change.ReconciliationStatus, change.DecisionCode, routeVersion,
		nullableSupplierTime(event.SupplierObservedAt), databaseNow)
	if err != nil {
		if externalEventUniqueConflict(err) {
			return Result{}, ErrConflict
		}
		return Result{}, fmt.Errorf("%w: append callback evidence", ErrRejected)
	}
	if rowsAffected(insertResult) != 1 {
		return Result{}, ErrRejected
	}
	if change.Changed || providerChanged {
		deliveryNo, err = randomIdentifier(s.random, "ldv_")
		if err != nil {
			return Result{}, ErrRejected
		}
		insertResult, err = transaction.ExecContext(databaseContext, `
INSERT INTO idc_gateway_local_route_delivery
    (delivery_no, route_no, request_no, installation_id, provider_code,
     operation_scope, route_version, projected_status, reconciliation_status,
     evidence_no, evidence_sha256)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			deliveryNo, route.RouteNo, route.RequestNo, route.InstallationID,
			route.Provider, route.OperationScope, routeVersion,
			string(change.ProjectedStatus), change.ReconciliationStatus,
			evidenceNo, sealed.PlainSHA256[:])
		if err != nil || rowsAffected(insertResult) != 1 {
			return Result{}, fmt.Errorf("%w: append callback delivery", ErrRejected)
		}
	}
	if err = transaction.Commit(); err != nil {
		return Result{}, ErrRejected
	}
	return Result{
		RouteNo: route.RouteNo, ProjectedStatus: change.ProjectedStatus,
		ProviderStatus:       event.ProviderStatus,
		ReconciliationStatus: change.ReconciliationStatus, RouteVersion: routeVersion,
		EvidenceNo: evidenceNo, DeliveryNo: deliveryNo, DecisionCode: change.DecisionCode,
		SupplierObservedAt: event.SupplierObservedAt,
	}, nil
}

func decideObservedTransition(
	route routeRecord,
	businessStatus string,
	incoming Status,
	providerStatus string,
) (transition, bool) {
	change := decideTransition(route, businessStatus, incoming)
	providerChanged := providerStatus != "" && providerStatus != route.ProviderStatus
	if providerChanged && !change.Changed {
		change.DecisionCode = decideProviderStatusObservation(route.ProviderStatus, providerStatus)
	}
	return change, providerChanged
}

func decideProviderStatusObservation(current string, incoming string) string {
	if current == "" {
		return "applied"
	}
	currentRank := providerStatusRank(current)
	incomingRank := providerStatusRank(incoming)
	if currentRank > 0 && incomingRank > 0 {
		if incomingRank > currentRank {
			return "applied"
		}
		return "stale"
	}
	if currentRank > 0 {
		return "applied"
	}
	if incomingRank > 0 {
		return "stale"
	}
	if current == "ALL" && incoming == "RELIEVED" {
		return "applied"
	}
	if current == "RELIEVED" && incoming == "ALL" {
		return "stale"
	}
	return "conflict"
}

func providerStatusRank(value string) int {
	switch value {
	case "INIT":
		return 1
	case "PART":
		return 2
	case "WILLEXPIRE":
		return 3
	default:
		return 0
	}
}

func (s *Service) canConverge(ctx context.Context, event VerifiedEvent) bool {
	return s != nil && s.database != nil && s.keyring != nil && s.random != nil &&
		ctx != nil && ctx.Err() == nil && validVerifiedEvent(event)
}

func (s *Service) readRouteIdentity(ctx context.Context, routeNo string) (routeRecord, error) {
	var result routeRecord
	err := s.database.QueryRowContext(ctx, `
SELECT route_no, attempt_no, request_no, installation_id, provider_code,
	   operation_scope, projected_status, COALESCE(provider_status, ''),
	   reconciliation_status, version
FROM idc_gateway_local_supplier_route
WHERE route_no = $1`, routeNo).Scan(
		&result.RouteNo, &result.AttemptNo, &result.RequestNo, &result.InstallationID,
		&result.Provider, &result.OperationScope, &result.ProjectedStatus,
		&result.ProviderStatus, &result.ReconciliationStatus, &result.Version,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return routeRecord{}, ErrConflict
	}
	if err != nil || result.InstallationID != s.installationID {
		return routeRecord{}, ErrRejected
	}
	return result, nil
}

func lockRoute(ctx context.Context, transaction *sql.Tx, routeNo string) (routeRecord, error) {
	var result routeRecord
	err := transaction.QueryRowContext(ctx, `
SELECT route_no, attempt_no, request_no, installation_id, provider_code,
	   operation_scope, projected_status, COALESCE(provider_status, ''),
	   reconciliation_status, version
FROM idc_gateway_local_supplier_route
WHERE route_no = $1
FOR UPDATE`, routeNo).Scan(
		&result.RouteNo, &result.AttemptNo, &result.RequestNo, &result.InstallationID,
		&result.Provider, &result.OperationScope, &result.ProjectedStatus,
		&result.ProviderStatus, &result.ReconciliationStatus, &result.Version,
	)
	return result, err
}

func lockBusinessRequest(
	ctx context.Context,
	transaction *sql.Tx,
	requestNo string,
	installationID string,
) (string, error) {
	var status string
	err := transaction.QueryRowContext(ctx, `
SELECT status_code
FROM idc_gateway_local_business_request
WHERE request_no = $1 AND installation_id = $2
FOR UPDATE`, requestNo, installationID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrConflict
	}
	if err != nil {
		return "", ErrRejected
	}
	return status, nil
}

func findExistingEvent(
	ctx context.Context,
	transaction *sql.Tx,
	route routeRecord,
	event VerifiedEvent,
	digests []callbacksecurity.EventDigest,
	payloadDigest [sha256.Size]byte,
) (Result, bool, error) {
	if len(digests) == 0 || len(digests) > callbacksecurity.MaximumVersionsPerFamily {
		return Result{}, false, ErrRejected
	}
	arguments := make([]any, 0, 1+len(digests)*2)
	arguments = append(arguments, event.Provider)
	predicates := make([]string, 0, len(digests))
	for _, digest := range digests {
		versionParameter := len(arguments) + 1
		digestParameter := versionParameter + 1
		predicates = append(predicates, fmt.Sprintf(
			"(e.event_key_version = $%d AND e.external_event_hmac = $%d)",
			versionParameter,
			digestParameter,
		))
		arguments = append(arguments, digest.KeyVersion, digest.Sum[:])
	}
	var (
		evidenceNo             string
		existingRouteNo        string
		existingStatus         Status
		existingProviderStatus sql.NullString
		existingProjection     Status
		existingReconciliation string
		existingPayload        []byte
		decisionCode           string
		routeVersion           int64
		deliveryNo             sql.NullString
		supplierObservedAt     sql.NullTime
		matchCount             int
	)
	err := transaction.QueryRowContext(ctx, `
SELECT e.evidence_no, e.route_no, e.normalized_status, e.provider_status, e.projected_status,
       e.reconciliation_status, e.payload_sha256, e.decision_code,
	   e.route_version, d.delivery_no, e.supplier_observed_at, COUNT(*) OVER ()
FROM idc_gateway_local_callback_evidence e
LEFT JOIN idc_gateway_local_route_delivery d ON d.evidence_no = e.evidence_no
WHERE e.provider_code = $1 AND (`+strings.Join(predicates, " OR ")+`)
ORDER BY e.observed_at, e.evidence_no
LIMIT 1`, arguments...).Scan(
		&evidenceNo, &existingRouteNo, &existingStatus, &existingProviderStatus, &existingProjection,
		&existingReconciliation, &existingPayload, &decisionCode, &routeVersion,
		&deliveryNo, &supplierObservedAt, &matchCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, ErrRejected
	}
	if matchCount != 1 || existingRouteNo != route.RouteNo || existingStatus != event.Status ||
		existingProviderStatus.String != event.ProviderStatus ||
		!equalDigest(existingPayload, payloadDigest) {
		return Result{}, false, ErrConflict
	}
	return Result{
		RouteNo: route.RouteNo, ProjectedStatus: existingProjection,
		ProviderStatus:       existingProviderStatus.String,
		ReconciliationStatus: existingReconciliation, RouteVersion: routeVersion,
		EvidenceNo: evidenceNo, DeliveryNo: deliveryNo.String,
		DecisionCode: decisionCode, SupplierObservedAt: supplierObservedAt.Time,
		Replayed: true,
	}, true, nil
}

func sameRouteOwner(left routeRecord, right routeRecord) bool {
	return left.RouteNo == right.RouteNo && left.AttemptNo == right.AttemptNo &&
		left.RequestNo == right.RequestNo && left.InstallationID == right.InstallationID &&
		left.Provider == right.Provider && left.OperationScope == right.OperationScope
}

func externalEventUniqueConflict(err error) bool {
	return postgresConstraintConflict(err, "uq_gateway_local_evidence_event")
}

func postgresConstraintConflict(err error, constraint string) bool {
	var postgresError *pgconn.PgError
	return errors.As(err, &postgresError) && postgresError.Code == "23505" &&
		postgresError.ConstraintName == constraint
}

func decideTransition(route routeRecord, businessStatus string, incoming Status) transition {
	current := route.ProjectedStatus
	if route.ReconciliationStatus == "reconciliation_required" {
		decision := "stale"
		if terminal(current) && terminal(incoming) && current != incoming {
			decision = "conflict"
		}
		return transition{
			ProjectedStatus: current, ReconciliationStatus: route.ReconciliationStatus,
			DecisionCode: decision,
		}
	}
	if businessConflict(businessStatus, incoming) ||
		(terminal(current) && terminal(incoming) && current != incoming) {
		return transition{
			ProjectedStatus: current, ReconciliationStatus: "reconciliation_required",
			DecisionCode: "conflict", Changed: route.ReconciliationStatus != "reconciliation_required",
		}
	}
	switch current {
	case StatusRunning:
		switch incoming {
		case StatusRunning:
			return unchangedTransition(route, "stale")
		case StatusResultUnknown:
			return transition{incoming, "query_required", "applied", true}
		default:
			return transition{incoming, "none", "applied", true}
		}
	case StatusResultUnknown:
		if incoming == StatusSucceeded || incoming == StatusFailed {
			return transition{incoming, "none", "applied", true}
		}
		return unchangedTransition(route, "stale")
	case StatusSucceeded, StatusFailed:
		return unchangedTransition(route, "stale")
	default:
		return unchangedTransition(route, "stale")
	}
}

func unchangedTransition(route routeRecord, decision string) transition {
	return transition{
		ProjectedStatus:      route.ProjectedStatus,
		ReconciliationStatus: route.ReconciliationStatus,
		DecisionCode:         decision,
	}
}

func terminal(status Status) bool {
	return status == StatusSucceeded || status == StatusFailed
}

func businessConflict(status string, incoming Status) bool {
	return (status == "succeeded" && incoming == StatusFailed) ||
		(status == "failed" && incoming == StatusSucceeded)
}

func applyBusinessProjection(
	ctx context.Context,
	transaction *sql.Tx,
	requestNo string,
	installationID string,
	current string,
	target Status,
	decision string,
) error {
	if decision == "conflict" || target == StatusRunning || current == string(target) {
		return nil
	}
	if target == StatusResultUnknown {
		if current != "accepted" && current != "result_unknown" {
			return nil
		}
	} else if current != "accepted" && current != "result_unknown" {
		return ErrConflict
	}
	result, err := transaction.ExecContext(ctx, `
UPDATE idc_gateway_local_business_request
SET status_code = $3
WHERE request_no = $1 AND installation_id = $2 AND status_code = $4`,
		requestNo, installationID, string(target), current)
	if err != nil || rowsAffected(result) != 1 {
		return ErrRejected
	}
	return nil
}

func activeEventDigest(
	version uint32,
	digests []callbacksecurity.EventDigest,
) (callbacksecurity.EventDigest, bool) {
	for _, digest := range digests {
		if digest.KeyVersion == version {
			return digest, true
		}
	}
	return callbacksecurity.EventDigest{}, false
}

func callbackDatabaseTime(ctx context.Context, transaction *sql.Tx) (time.Time, error) {
	var value time.Time
	if err := transaction.QueryRowContext(ctx, `SELECT CURRENT_TIMESTAMP`).Scan(&value); err != nil {
		return time.Time{}, err
	}
	return value.UTC(), nil
}

func nullableSupplierTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}

func nullableProviderStatus(value string) any {
	if value == "" {
		return nil
	}
	return value
}
