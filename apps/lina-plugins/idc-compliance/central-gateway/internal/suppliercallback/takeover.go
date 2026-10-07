// This file owns current-merchant employee authorization and immutable,
// evidence-driven manual takeover.

package suppliercallback

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"

	"lina-platform/idc-compliance-gateway/internal/callbacksecurity"
)

const (
	// TakeoverPermission is the exact local employee permission for takeover.
	TakeoverPermission    = "idc:admin:compliance:route:manual-takeover"
	recentTakeoverAuthAge = 5 * time.Minute
)

var (
	// ErrTakeoverUnauthorized reports a missing local employee authority decision.
	ErrTakeoverUnauthorized = errors.New("current merchant employee takeover authorization is required")
	// ErrTakeoverRecentAuth reports that full authentication is older than five minutes.
	ErrTakeoverRecentAuth = errors.New("recent full authentication is required for takeover")

	evidenceNoPattern  = regexp.MustCompile(`^lev_[0-9a-f]{32}$`)
	principalNoPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
)

// TakeoverReason is one of the five fixed, non-terminal takeover reasons.
type TakeoverReason string

const (
	// TakeoverResultRemainsUnknown records a result that remains indeterminate.
	TakeoverResultRemainsUnknown TakeoverReason = "result_remains_unknown"
	// TakeoverTerminalResultConflict records conflicting authoritative evidence.
	TakeoverTerminalResultConflict TakeoverReason = "terminal_result_conflict"
	// TakeoverQueryReferenceMissing records that no safe provider lookup key exists.
	TakeoverQueryReferenceMissing TakeoverReason = "query_reference_missing"
	// TakeoverSupplierQueryUnavailable records that no verified reader is available.
	TakeoverSupplierQueryUnavailable TakeoverReason = "supplier_query_unavailable"
	// TakeoverComplianceSecurityReview records a local compliance or security hold.
	TakeoverComplianceSecurityReview TakeoverReason = "compliance_security_review"
)

// TakeoverAuthorization is resolved from the current installation's trusted session.
type TakeoverAuthorization struct {
	PrincipalNo             string
	CurrentMerchantEmployee bool
	PermissionGranted       bool
	DataScopeMatched        bool
	FullAuthAt              time.Time
}

// TakeoverAuthority resolves current-session local employee authorization.
type TakeoverAuthority interface {
	AuthorizeTakeover(context.Context, string, string) (TakeoverAuthorization, error)
}

// TakeoverInput contains only the fenced route decision and existing evidence IDs.
type TakeoverInput struct {
	RouteNo         string
	ExpectedVersion int64
	Reason          TakeoverReason
	EvidenceNos     []string
	OperationKey    []byte
}

// TakeoverResult identifies the immutable local decision and versioned delivery.
type TakeoverResult struct {
	TakeoverNo          string
	RouteNo             string
	RouteVersion        int64
	DecisionEvidenceNo  string
	DeliveryNo          string
	Reason              TakeoverReason
	EmployeePrincipalNo string
	EvidenceSetSHA256   [sha256.Size]byte
	Replayed            bool
}

// TakeoverService commits authorized evidence-driven local automation stops.
type TakeoverService struct {
	database       *sql.DB
	installationID string
	keyring        *callbacksecurity.Ring
	authority      TakeoverAuthority
	random         io.Reader
}

// NewTakeoverService constructs a local takeover service with a trusted authority.
func NewTakeoverService(
	database *sql.DB,
	installationID string,
	keyring *callbacksecurity.Ring,
	authority TakeoverAuthority,
) (*TakeoverService, error) {
	if database == nil || !installationIDPattern.MatchString(installationID) ||
		keyring == nil || keyring.ActiveEvidenceVersion() == 0 ||
		keyring.ActiveTakeoverOperationVersion() == 0 || nilDependency(authority) {
		return nil, ErrInvalid
	}
	return &TakeoverService{
		database: database, installationID: installationID, keyring: keyring,
		authority: authority, random: rand.Reader,
	}, nil
}

// Takeover stops automation without declaring supplier success or failure.
func (s *TakeoverService) Takeover(
	ctx context.Context,
	input TakeoverInput,
) (result TakeoverResult, returnError error) {
	defer clear(input.OperationKey)
	evidenceNos, valid := canonicalEvidenceNos(input.EvidenceNos)
	if s == nil || ctx == nil || ctx.Err() != nil ||
		!routeNoPattern.MatchString(input.RouteNo) || input.ExpectedVersion < 1 ||
		!validTakeoverReason(input.Reason) || !valid ||
		len(input.OperationKey) != callbacksecurity.SecretBytes {
		return TakeoverResult{}, ErrInvalid
	}
	authorization, err := s.authority.AuthorizeTakeover(
		ctx, input.RouteNo, TakeoverPermission,
	)
	if err != nil || !authorization.CurrentMerchantEmployee ||
		!authorization.PermissionGranted || !authorization.DataScopeMatched ||
		!principalNoPattern.MatchString(authorization.PrincipalNo) {
		return TakeoverResult{}, ErrTakeoverUnauthorized
	}
	identity, err := readTakeoverRouteIdentity(ctx, s.database, s.installationID, input.RouteNo)
	if err != nil {
		return TakeoverResult{}, err
	}
	operationDigests, err := s.keyring.TakeoverOperationDigests(
		s.installationID, input.OperationKey,
	)
	if err != nil {
		return TakeoverResult{}, ErrRejected
	}
	activeOperationDigest, ok := activeEventDigest(
		s.keyring.ActiveTakeoverOperationVersion(), operationDigests,
	)
	if !ok {
		return TakeoverResult{}, ErrRejected
	}
	databaseContext, cancel := context.WithTimeout(ctx, callbackDatabaseTimeout)
	defer cancel()
	transaction, err := s.database.BeginTx(
		databaseContext, &sql.TxOptions{Isolation: sql.LevelReadCommitted},
	)
	if err != nil {
		return TakeoverResult{}, ErrRejected
	}
	defer finishTransaction(transaction, &returnError)
	databaseNow, err := callbackDatabaseTime(databaseContext, transaction)
	if err != nil {
		return TakeoverResult{}, ErrRejected
	}
	fullAuthAt := authorization.FullAuthAt.UTC()
	if fullAuthAt.IsZero() || fullAuthAt.After(databaseNow.Add(15*time.Second)) ||
		databaseNow.Sub(fullAuthAt) > recentTakeoverAuthAge {
		return TakeoverResult{}, ErrTakeoverRecentAuth
	}
	businessStatus, err := lockBusinessRequest(
		databaseContext, transaction, identity.RequestNo, s.installationID,
	)
	if err != nil {
		return TakeoverResult{}, err
	}
	route, err := lockRoute(databaseContext, transaction, input.RouteNo)
	if err != nil || !sameRouteOwner(route, identity) || route.InstallationID != s.installationID {
		return TakeoverResult{}, ErrConflict
	}
	evidenceSetDigest, err := lockAndDigestTakeoverEvidence(
		databaseContext, transaction, route.RouteNo, evidenceNos,
	)
	if err != nil {
		return TakeoverResult{}, err
	}
	requestDigest := takeoverRequestDigest(
		input.RouteNo, input.ExpectedVersion, input.Reason, authorization.PrincipalNo,
		evidenceSetDigest,
	)
	if recovered, found, recoverErr := findExistingTakeover(
		databaseContext, transaction, operationDigests, requestDigest,
	); recoverErr != nil {
		return TakeoverResult{}, recoverErr
	} else if found {
		if err = transaction.Commit(); err != nil {
			return TakeoverResult{}, ErrRejected
		}
		return recovered, nil
	}
	if route.Version != input.ExpectedVersion || !routeAllowsTakeover(route) {
		return TakeoverResult{}, ErrConflict
	}
	result, err = s.persistNewTakeover(databaseContext, transaction, newTakeover{
		input: input, authorization: authorization, evidenceNos: evidenceNos,
		evidenceSetDigest: evidenceSetDigest, requestDigest: requestDigest,
		activeOperationDigest: activeOperationDigest, databaseNow: databaseNow,
		fullAuthAt: fullAuthAt, businessStatus: businessStatus, route: route,
	})
	if err != nil {
		return TakeoverResult{}, err
	}
	if err = transaction.Commit(); err != nil {
		return TakeoverResult{}, ErrRejected
	}
	return result, nil
}

type newTakeover struct {
	input                 TakeoverInput
	authorization         TakeoverAuthorization
	evidenceNos           []string
	evidenceSetDigest     [sha256.Size]byte
	requestDigest         [sha256.Size]byte
	activeOperationDigest callbacksecurity.EventDigest
	databaseNow           time.Time
	fullAuthAt            time.Time
	businessStatus        string
	route                 routeRecord
}

func (s *TakeoverService) persistNewTakeover(
	ctx context.Context,
	transaction *sql.Tx,
	command newTakeover,
) (TakeoverResult, error) {
	takeoverNo, err := randomIdentifier(s.random, "lto_")
	if err != nil {
		return TakeoverResult{}, ErrRejected
	}
	decisionEvidenceNo, err := randomIdentifier(s.random, "lev_")
	if err != nil {
		return TakeoverResult{}, ErrRejected
	}
	deliveryNo, err := randomIdentifier(s.random, "ldv_")
	if err != nil {
		return TakeoverResult{}, ErrRejected
	}
	committedVersion := command.route.Version + 1
	decisionFrame, err := json.Marshal(struct {
		RouteNo             string         `json:"routeNo"`
		ExpectedVersion     int64          `json:"expectedVersion"`
		CommittedVersion    int64          `json:"committedVersion"`
		Reason              TakeoverReason `json:"reason"`
		EmployeePrincipalNo string         `json:"employeePrincipalNo"`
		FullAuthAt          string         `json:"fullAuthAt"`
		EvidenceSetSHA256   string         `json:"evidenceSetSha256"`
	}{
		RouteNo: command.route.RouteNo, ExpectedVersion: command.route.Version,
		CommittedVersion: committedVersion, Reason: command.input.Reason,
		EmployeePrincipalNo: command.authorization.PrincipalNo,
		FullAuthAt:          command.fullAuthAt.Format(time.RFC3339Nano),
		EvidenceSetSHA256:   hex.EncodeToString(command.evidenceSetDigest[:]),
	})
	if err != nil {
		return TakeoverResult{}, ErrRejected
	}
	defer clear(decisionFrame)
	sealed, err := s.keyring.Seal(
		s.random, decisionEvidenceNo, command.route.RouteNo, command.route.Provider,
		"manual_takeover", decisionFrame,
	)
	if err != nil || len(sealed.Ciphertext) > MaximumEvidenceCiphertextBytes {
		return TakeoverResult{}, ErrRejected
	}
	defer clear(sealed.Nonce)
	defer clear(sealed.Ciphertext)
	defer clear(sealed.PlainSHA256[:])
	if err = applyManualBusinessProjection(
		ctx, transaction, command.route.RequestNo, s.installationID, command.businessStatus,
	); err != nil {
		return TakeoverResult{}, err
	}
	updateResult, err := transaction.ExecContext(ctx, `
UPDATE idc_gateway_local_supplier_route
SET projected_status = 'manual_takeover', reconciliation_status = 'manual_takeover',
    next_query_at = NULL, query_lease_sha256 = NULL,
    query_lease_started_at = NULL, query_lease_expires_at = NULL,
    updated_at = $2, version = $3
WHERE route_no = $1 AND version = $4`,
		command.route.RouteNo, command.databaseNow, committedVersion, command.route.Version)
	if err != nil || rowsAffected(updateResult) != 1 {
		return TakeoverResult{}, ErrConflict
	}
	evidenceResult, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_local_callback_evidence
    (evidence_no, route_no, request_no, installation_id, provider_code,
     operation_scope, source_kind, payload_ciphertext, payload_nonce,
     payload_sha256, cipher_format_version, encryption_key_version,
     normalized_status, projected_status, reconciliation_status,
     decision_code, route_version, observed_at)
VALUES ($1, $2, $3, $4, $5, $6, 'manual_takeover', $7, $8, $9, $10, $11,
        'manual_takeover', 'manual_takeover', 'manual_takeover',
        'manual_takeover', $12, $13)`,
		decisionEvidenceNo, command.route.RouteNo, command.route.RequestNo,
		command.route.InstallationID, command.route.Provider, command.route.OperationScope,
		sealed.Ciphertext, sealed.Nonce,
		sealed.PlainSHA256[:], sealed.FormatVersion, sealed.KeyVersion,
		committedVersion, command.databaseNow)
	if err != nil || rowsAffected(evidenceResult) != 1 {
		return TakeoverResult{}, ErrRejected
	}
	takeoverResult, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_local_supplier_takeover
    (takeover_no, route_no, request_no, installation_id, provider_code,
     operation_scope, operation_key_version, operation_hmac, request_sha256,
     expected_route_version, committed_route_version, from_projected_status,
     from_reconciliation_status, reason_code, employee_principal_no,
     full_auth_at, evidence_set_sha256, evidence_count, decision_evidence_no,
     decision_evidence_sha256, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14,
        $15, $16, $17, $18, $19, $20, $21)`,
		takeoverNo, command.route.RouteNo, command.route.RequestNo,
		command.route.InstallationID, command.route.Provider, command.route.OperationScope,
		command.activeOperationDigest.KeyVersion, command.activeOperationDigest.Sum[:],
		command.requestDigest[:], command.route.Version, committedVersion,
		string(command.route.ProjectedStatus), command.route.ReconciliationStatus,
		string(command.input.Reason), command.authorization.PrincipalNo,
		command.fullAuthAt, command.evidenceSetDigest[:], len(command.evidenceNos),
		decisionEvidenceNo, sealed.PlainSHA256[:], command.databaseNow)
	if err != nil || rowsAffected(takeoverResult) != 1 {
		if postgresConstraintConflict(err, "uq_gateway_local_takeover_operation") {
			return TakeoverResult{}, ErrConflict
		}
		return TakeoverResult{}, ErrRejected
	}
	if err = insertTakeoverEvidenceRelations(
		ctx, transaction, takeoverNo, command.route.RouteNo, command.evidenceNos,
	); err != nil {
		return TakeoverResult{}, err
	}
	deliveryResult, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_local_route_delivery
    (delivery_no, route_no, request_no, installation_id, provider_code,
     operation_scope, route_version, projected_status, reconciliation_status,
     evidence_no, evidence_sha256)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'manual_takeover', 'manual_takeover', $8, $9)`,
		deliveryNo, command.route.RouteNo, command.route.RequestNo,
		command.route.InstallationID, command.route.Provider,
		command.route.OperationScope, committedVersion,
		decisionEvidenceNo, sealed.PlainSHA256[:])
	if err != nil || rowsAffected(deliveryResult) != 1 {
		return TakeoverResult{}, ErrRejected
	}
	return TakeoverResult{
		TakeoverNo: takeoverNo, RouteNo: command.route.RouteNo, RouteVersion: committedVersion,
		DecisionEvidenceNo: decisionEvidenceNo, DeliveryNo: deliveryNo,
		Reason:              command.input.Reason,
		EmployeePrincipalNo: command.authorization.PrincipalNo,
		EvidenceSetSHA256:   command.evidenceSetDigest,
	}, nil
}

func readTakeoverRouteIdentity(
	ctx context.Context,
	database *sql.DB,
	installationID string,
	routeNo string,
) (routeRecord, error) {
	var result routeRecord
	err := database.QueryRowContext(ctx, `
SELECT route_no, attempt_no, request_no, installation_id, provider_code,
	   operation_scope, projected_status, COALESCE(provider_status, ''),
	   reconciliation_status, version
FROM idc_gateway_local_supplier_route
WHERE route_no = $1 AND installation_id = $2`, routeNo, installationID).Scan(
		&result.RouteNo, &result.AttemptNo, &result.RequestNo, &result.InstallationID,
		&result.Provider, &result.OperationScope, &result.ProjectedStatus,
		&result.ProviderStatus, &result.ReconciliationStatus, &result.Version,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return routeRecord{}, ErrConflict
	}
	if err != nil {
		return routeRecord{}, ErrRejected
	}
	return result, nil
}

func lockAndDigestTakeoverEvidence(
	ctx context.Context,
	transaction *sql.Tx,
	routeNo string,
	evidenceNos []string,
) (result [sha256.Size]byte, returnError error) {
	arguments := []any{routeNo}
	parameters := make([]string, 0, len(evidenceNos))
	for _, evidenceNo := range evidenceNos {
		arguments = append(arguments, evidenceNo)
		parameters = append(parameters, fmt.Sprintf("$%d", len(arguments)))
	}
	rows, err := transaction.QueryContext(ctx, `
SELECT evidence_no, payload_sha256
FROM idc_gateway_local_callback_evidence
WHERE route_no = $1 AND evidence_no IN (`+strings.Join(parameters, ", ")+`)
ORDER BY evidence_no
FOR SHARE`, arguments...)
	if err != nil {
		return [sha256.Size]byte{}, ErrRejected
	}
	defer func() {
		if closeError := rows.Close(); closeError != nil && returnError == nil {
			result = [sha256.Size]byte{}
			returnError = ErrRejected
		}
	}()
	frame := make([]byte, 0, len(evidenceNos)*(sha256.Size+48))
	defer clear(frame)
	count := 0
	for rows.Next() {
		var evidenceNo string
		var payloadDigest []byte
		if err = rows.Scan(&evidenceNo, &payloadDigest); err != nil ||
			len(payloadDigest) != sha256.Size {
			return [sha256.Size]byte{}, ErrRejected
		}
		frame = appendTakeoverDigestField(frame, []byte(evidenceNo))
		frame = appendTakeoverDigestField(frame, payloadDigest)
		count++
	}
	if err = rows.Close(); err != nil || rows.Err() != nil || count != len(evidenceNos) {
		return [sha256.Size]byte{}, ErrConflict
	}
	return sha256.Sum256(frame), nil
}

func findExistingTakeover(
	ctx context.Context,
	transaction *sql.Tx,
	digests []callbacksecurity.EventDigest,
	requestDigest [sha256.Size]byte,
) (TakeoverResult, bool, error) {
	arguments := make([]any, 0, len(digests)*2)
	predicates := make([]string, 0, len(digests))
	for _, digest := range digests {
		arguments = append(arguments, digest.KeyVersion, digest.Sum[:])
		predicates = append(predicates, fmt.Sprintf(
			"(operation_key_version = $%d AND operation_hmac = $%d)",
			len(arguments)-1, len(arguments),
		))
	}
	var (
		result       TakeoverResult
		storedDigest []byte
		reason       string
		evidenceSet  []byte
		matchCount   int
	)
	err := transaction.QueryRowContext(ctx, `
SELECT takeover_no, route_no, committed_route_version, decision_evidence_no,
       reason_code, employee_principal_no, evidence_set_sha256, request_sha256,
       COUNT(*) OVER ()
FROM idc_gateway_local_supplier_takeover
WHERE `+strings.Join(predicates, " OR ")+`
ORDER BY created_at, takeover_no
LIMIT 1`, arguments...).Scan(
		&result.TakeoverNo, &result.RouteNo, &result.RouteVersion,
		&result.DecisionEvidenceNo, &reason, &result.EmployeePrincipalNo,
		&evidenceSet, &storedDigest, &matchCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return TakeoverResult{}, false, nil
	}
	if err != nil {
		return TakeoverResult{}, false, ErrRejected
	}
	if matchCount != 1 || !equalDigest(storedDigest, requestDigest) ||
		len(evidenceSet) != sha256.Size {
		return TakeoverResult{}, false, ErrConflict
	}
	result.Reason = TakeoverReason(reason)
	copy(result.EvidenceSetSHA256[:], evidenceSet)
	if err = transaction.QueryRowContext(ctx, `
SELECT delivery_no
FROM idc_gateway_local_route_delivery
WHERE evidence_no = $1`, result.DecisionEvidenceNo).Scan(&result.DeliveryNo); err != nil {
		return TakeoverResult{}, false, ErrRejected
	}
	result.Replayed = true
	return result, true, nil
}

func insertTakeoverEvidenceRelations(
	ctx context.Context,
	transaction *sql.Tx,
	takeoverNo string,
	routeNo string,
	evidenceNos []string,
) error {
	arguments := make([]any, 0, len(evidenceNos)*4)
	values := make([]string, 0, len(evidenceNos))
	for index, evidenceNo := range evidenceNos {
		arguments = append(arguments, takeoverNo, routeNo, evidenceNo, index+1)
		base := len(arguments) - 3
		values = append(values, fmt.Sprintf(
			"($%d, $%d, $%d, $%d)", base, base+1, base+2, base+3,
		))
	}
	result, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_local_takeover_evidence
    (takeover_no, route_no, evidence_no, ordinal)
VALUES `+strings.Join(values, ", "), arguments...)
	if err != nil || rowsAffected(result) != int64(len(evidenceNos)) {
		return ErrRejected
	}
	return nil
}

func applyManualBusinessProjection(
	ctx context.Context,
	transaction *sql.Tx,
	requestNo string,
	installationID string,
	current string,
) error {
	switch current {
	case "accepted", "result_unknown":
		result, err := transaction.ExecContext(ctx, `
UPDATE idc_gateway_local_business_request
SET status_code = 'manual_takeover'
WHERE request_no = $1 AND installation_id = $2 AND status_code = $3`,
			requestNo, installationID, current)
		if err != nil || rowsAffected(result) != 1 {
			return ErrRejected
		}
	case "succeeded", "failed":
		return nil
	default:
		return ErrConflict
	}
	return nil
}

func canonicalEvidenceNos(values []string) ([]string, bool) {
	if len(values) < 1 || len(values) > 16 {
		return nil, false
	}
	result := append([]string(nil), values...)
	for _, value := range result {
		if !evidenceNoPattern.MatchString(value) {
			return nil, false
		}
	}
	sort.Strings(result)
	for index := 1; index < len(result); index++ {
		if result[index] == result[index-1] {
			return nil, false
		}
	}
	return result, true
}

func validTakeoverReason(value TakeoverReason) bool {
	switch value {
	case TakeoverResultRemainsUnknown, TakeoverTerminalResultConflict,
		TakeoverQueryReferenceMissing, TakeoverSupplierQueryUnavailable,
		TakeoverComplianceSecurityReview:
		return true
	default:
		return false
	}
}

func routeAllowsTakeover(route routeRecord) bool {
	return route.ProjectedStatus == StatusRunning ||
		route.ProjectedStatus == StatusResultUnknown ||
		route.ReconciliationStatus == "reconciliation_required"
}

func takeoverRequestDigest(
	routeNo string,
	expectedVersion int64,
	reason TakeoverReason,
	principalNo string,
	evidenceSetDigest [sha256.Size]byte,
) [sha256.Size]byte {
	frame := make([]byte, 0, 256)
	defer clear(frame)
	frame = appendTakeoverDigestField(
		frame, []byte("idc-compliance/manual-takeover-request/v1"),
	)
	frame = appendTakeoverDigestField(frame, []byte(routeNo))
	frame = appendTakeoverDigestField(frame, []byte(fmt.Sprint(expectedVersion)))
	frame = appendTakeoverDigestField(frame, []byte(reason))
	frame = appendTakeoverDigestField(frame, []byte(principalNo))
	frame = appendTakeoverDigestField(frame, evidenceSetDigest[:])
	return sha256.Sum256(frame)
}

func appendTakeoverDigestField(frame []byte, value []byte) []byte {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	frame = append(frame, length[:]...)
	return append(frame, value...)
}
