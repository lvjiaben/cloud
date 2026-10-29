// This file implements the installation-local supplier-subject binding,
// revocation, reopening, and plaintext callback lifecycle.

package supplieridentity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	ProviderTencentESSBasic = "tencent_essbasic"
	ProviderNuonuoInvoice   = "nuonuo_invoice"
	maximumCanonicalBytes   = 4096

	OperationSubjectBind       = "subject_bind"
	OperationSubjectRevoke     = "subject_revoke"
	OperationSubjectReopen     = "subject_reopen"
	OperationInstanceRevoke    = "instance_revoke"
	ReasonSubjectReplaced      = "subject_replaced"
	ReasonAuthorizationLost    = "supplier_authorization_revoked"
	ReasonSecurityCompromise   = "security_compromise"
	ReasonInstanceRevoked      = "instance_revoked"
	ReasonReplacementApproved  = "replacement_approved"
	ReasonSupplierReauthorized = "supplier_reauthorized"
	ReasonSecurityRecovered    = "security_recovered"
)

var (
	ErrInvalid          = errors.New("supplier identity input is invalid")
	ErrUnavailable      = errors.New("supplier identity service is unavailable")
	ErrCanonicalizerOff = errors.New("supplier identity canonicalizer is disabled")
	ErrConflict         = errors.New("supplier identity operation conflicts with current state")
	ErrNotFound         = errors.New("supplier identity binding is unavailable")
	ErrInstanceRevoked  = errors.New("supplier identity instance is revoked")
	ErrReopenRequired   = errors.New("supplier identity requires a separate reopen operation")
	installationPattern = regexp.MustCompile(`^linst_[0-9a-f]{64}$`)
	bindingPattern      = regexp.MustCompile(`^lsb_[0-9a-f]{32}$`)
)

// Canonicalizer is the only component allowed to interpret one provider's
// exact subject syntax. Production maps remain empty until contracts are frozen.
type Canonicalizer interface {
	Canonicalize(context.Context, []byte) ([]byte, error)
}

// BindInput consumes and clears Subject and IdempotencyKey before returning.
type BindInput struct {
	Provider       string
	Subject        []byte
	IdempotencyKey []byte
}

// RevokeInput consumes and clears IdempotencyKey before returning.
type RevokeInput struct {
	BindingNo      string
	ReasonCode     string
	IdempotencyKey []byte
}

// ReopenInput is a new action against one exact revoked binding. It consumes
// and clears Subject and IdempotencyKey before returning.
type ReopenInput struct {
	PreviousBindingNo string
	Provider          string
	Subject           []byte
	ReasonCode        string
	IdempotencyKey    []byte
}

// InstanceRevokeInput consumes and clears IdempotencyKey before returning.
type InstanceRevokeInput struct {
	IdempotencyKey []byte
}

// Result contains only stable non-secret references.
type Result struct {
	OperationNo string
	BindingNo   string
	Replayed    bool
}

// Service owns one installation's internal-only supplier identity lifecycle.
type Service struct {
	database       *sql.DB
	installationID string
	keyring        *Ring
	canonicalizers map[string]Canonicalizer
	random         io.Reader
}

// NewService builds an internal service. An empty canonicalizer map is the
// required production state before Tencent and NuoNuo contracts are confirmed.
func NewService(
	database *sql.DB,
	installationID string,
	keyring *Ring,
	canonicalizers map[string]Canonicalizer,
) (*Service, error) {
	if database == nil || keyring == nil || !installationPattern.MatchString(installationID) {
		return nil, ErrUnavailable
	}
	if keyring.ActiveVersion(PurposeSubjectEncryption) == 0 ||
		keyring.ActiveVersion(PurposeSubjectLookup) == 0 ||
		keyring.ActiveVersion(PurposeIdentityOperation) == 0 {
		return nil, ErrUnavailable
	}
	result := &Service{
		database: database, installationID: installationID, keyring: keyring,
		canonicalizers: make(map[string]Canonicalizer, len(canonicalizers)), random: rand.Reader,
	}
	for provider, canonicalizer := range canonicalizers {
		if !validProvider(provider) || isNilCanonicalizer(canonicalizer) {
			return nil, ErrUnavailable
		}
		result.canonicalizers[provider] = canonicalizer
	}
	return result, nil
}

// EnsureReady creates only the current local instance projection and backfills
// retained lookup versions for at most the two supported active subjects.
func (s *Service) EnsureReady(ctx context.Context) error {
	if !s.validContext(ctx) {
		return ErrUnavailable
	}
	transaction, err := s.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return ErrUnavailable
	}
	defer rollback(transaction)
	if _, err = transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_local_supplier_instance (installation_id)
VALUES ($1)
ON CONFLICT (installation_id) DO NOTHING`, s.installationID); err != nil {
		return ErrUnavailable
	}
	status, err := lockInstance(ctx, transaction, s.installationID)
	if err != nil {
		return ErrUnavailable
	}
	if status != "active" {
		return ErrInstanceRevoked
	}
	rows, err := transaction.QueryContext(ctx, `
SELECT binding_no, provider_code, subject_ciphertext, subject_nonce,
       cipher_format_version, encryption_key_version
FROM idc_gateway_local_supplier_subject
WHERE installation_id = $1 AND status_code = 'active'
ORDER BY provider_code
LIMIT 3
FOR UPDATE`, s.installationID)
	if err != nil {
		return ErrUnavailable
	}
	defer rows.Close()
	type activeSubject struct {
		bindingNo string
		provider  string
		sealed    Ciphertext
	}
	active := make([]activeSubject, 0, 3)
	for rows.Next() {
		var item activeSubject
		if err = rows.Scan(
			&item.bindingNo, &item.provider, &item.sealed.Payload, &item.sealed.Nonce,
			&item.sealed.FormatVersion, &item.sealed.KeyVersion,
		); err != nil {
			_ = rows.Close()
			return ErrUnavailable
		}
		active = append(active, item)
	}
	iterationErr := rows.Err()
	closeErr := rows.Close()
	if iterationErr != nil || closeErr != nil || len(active) > 2 {
		return ErrUnavailable
	}
	for _, item := range active {
		plaintext, openErr := s.keyring.Open(
			s.installationID, item.bindingNo, item.provider, item.sealed,
		)
		if openErr != nil {
			return ErrUnavailable
		}
		digests, digestErr := s.keyring.LookupDigests(item.provider, plaintext)
		clearBytes(plaintext)
		if digestErr != nil {
			return ErrUnavailable
		}
		if err = insertLookupDigests(ctx, transaction, item.bindingNo, item.provider, digests); err != nil {
			return classifyDatabaseError(err)
		}
	}
	if err = transaction.Commit(); err != nil {
		return classifyDatabaseError(err)
	}
	return nil
}

// Bind creates the first binding for a subject that has no retained local history.
func (s *Service) Bind(ctx context.Context, input BindInput) (Result, error) {
	return s.writeSubject(ctx, input.Provider, "", "", input.Subject, input.IdempotencyKey)
}

// Reopen creates a new binding after an exact previous binding was revoked.
func (s *Service) Reopen(ctx context.Context, input ReopenInput) (Result, error) {
	if !validReopenReason(input.ReasonCode) || !bindingPattern.MatchString(input.PreviousBindingNo) {
		clearBytes(input.Subject)
		clearBytes(input.IdempotencyKey)
		return Result{}, ErrInvalid
	}
	return s.writeSubject(
		ctx, input.Provider, input.PreviousBindingNo, input.ReasonCode,
		input.Subject, input.IdempotencyKey,
	)
}

// Revoke closes one active binding with a fixed reason and never deletes history.
func (s *Service) Revoke(ctx context.Context, input RevokeInput) (Result, error) {
	operationKey := consume(input.IdempotencyKey)
	defer clearBytes(operationKey)
	if !s.validContext(ctx) || !bindingPattern.MatchString(input.BindingNo) ||
		!validRevokeReason(input.ReasonCode) || len(operationKey) != SecretBytes {
		return Result{}, ErrInvalid
	}
	requestDigest := digestRequest(
		OperationSubjectRevoke, input.BindingNo, input.ReasonCode,
	)
	candidates, err := s.identityDigests(OperationSubjectRevoke, operationKey)
	if err != nil {
		return Result{}, ErrUnavailable
	}
	transaction, err := s.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return Result{}, ErrUnavailable
	}
	defer rollback(transaction)
	if recovered, found, recoverErr := recoverOperation(
		ctx, transaction, OperationSubjectRevoke, candidates, requestDigest,
	); found || recoverErr != nil {
		return recovered, recoverErr
	}
	status, err := lockInstance(ctx, transaction, s.installationID)
	if err != nil || status != "active" {
		return Result{}, ErrInstanceRevoked
	}
	var provider, bindingStatus string
	if err = transaction.QueryRowContext(ctx, `
SELECT provider_code, status_code
FROM idc_gateway_local_supplier_subject
WHERE binding_no = $1 AND installation_id = $2
FOR UPDATE`, input.BindingNo, s.installationID).Scan(&provider, &bindingStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Result{}, ErrNotFound
		}
		return Result{}, ErrUnavailable
	}
	if bindingStatus != "active" {
		return Result{}, ErrConflict
	}
	if _, err = transaction.ExecContext(ctx, `
UPDATE idc_gateway_local_supplier_subject
SET status_code = 'revoked', revoked_at = CURRENT_TIMESTAMP,
    revoke_reason_code = $2, version = version + 1
WHERE binding_no = $1`, input.BindingNo, input.ReasonCode); err != nil {
		return Result{}, classifyDatabaseError(err)
	}
	if _, err = transaction.ExecContext(ctx, `
UPDATE idc_gateway_local_subject_lookup SET status_code = 'revoked'
WHERE binding_no = $1 AND status_code = 'active'`, input.BindingNo); err != nil {
		return Result{}, ErrUnavailable
	}
	operationNo, err := randomIdentifier(s.random, "lsi_")
	if err != nil {
		return Result{}, ErrUnavailable
	}
	active := activeDigest(candidates, s.keyring.ActiveVersion(PurposeIdentityOperation))
	if active == nil {
		return Result{}, ErrUnavailable
	}
	if _, err = transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_local_identity_operation
    (operation_no, installation_id, operation_kind, provider_code, binding_no,
     idempotency_key_version, idempotency_hmac, request_sha256, reason_code)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		operationNo, s.installationID, OperationSubjectRevoke, provider, input.BindingNo,
		active.KeyVersion, active.Sum[:], requestDigest[:], input.ReasonCode,
	); err != nil {
		return Result{}, classifyDatabaseError(err)
	}
	if err = transaction.Commit(); err != nil {
		return Result{}, classifyDatabaseError(err)
	}
	return Result{OperationNo: operationNo, BindingNo: input.BindingNo}, nil
}

// RevokeInstance atomically closes the instance and every active subject. It
// never deletes ciphertext or key versions.
func (s *Service) RevokeInstance(
	ctx context.Context,
	input InstanceRevokeInput,
) (Result, error) {
	operationKey := consume(input.IdempotencyKey)
	defer clearBytes(operationKey)
	if !s.validContext(ctx) || len(operationKey) != SecretBytes {
		return Result{}, ErrInvalid
	}
	requestDigest := digestRequest(OperationInstanceRevoke, s.installationID, ReasonInstanceRevoked)
	candidates, err := s.identityDigests(OperationInstanceRevoke, operationKey)
	if err != nil {
		return Result{}, ErrUnavailable
	}
	transaction, err := s.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return Result{}, ErrUnavailable
	}
	defer rollback(transaction)
	if recovered, found, recoverErr := recoverOperation(
		ctx, transaction, OperationInstanceRevoke, candidates, requestDigest,
	); found || recoverErr != nil {
		return recovered, recoverErr
	}
	status, err := lockInstance(ctx, transaction, s.installationID)
	if err != nil {
		return Result{}, ErrUnavailable
	}
	if status != "active" {
		return Result{}, ErrInstanceRevoked
	}
	if _, err = transaction.ExecContext(ctx, `
UPDATE idc_gateway_local_supplier_instance
SET status_code = 'revoked', revoked_at = CURRENT_TIMESTAMP,
    revoke_reason_code = 'instance_revoked', updated_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE installation_id = $1`, s.installationID); err != nil {
		return Result{}, classifyDatabaseError(err)
	}
	operationNo, err := randomIdentifier(s.random, "lsi_")
	if err != nil {
		return Result{}, ErrUnavailable
	}
	active := activeDigest(candidates, s.keyring.ActiveVersion(PurposeIdentityOperation))
	if active == nil {
		return Result{}, ErrUnavailable
	}
	if _, err = transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_local_identity_operation
    (operation_no, installation_id, operation_kind, idempotency_key_version,
     idempotency_hmac, request_sha256, reason_code)
VALUES ($1, $2, $3, $4, $5, $6, 'instance_revoked')`,
		operationNo, s.installationID, OperationInstanceRevoke,
		active.KeyVersion, active.Sum[:], requestDigest[:],
	); err != nil {
		return Result{}, classifyDatabaseError(err)
	}
	if err = transaction.Commit(); err != nil {
		return Result{}, classifyDatabaseError(err)
	}
	return Result{OperationNo: operationNo}, nil
}

// WithActiveSubject decrypts one exact current subject only for the synchronous
// callback and clears the plaintext whether the callback succeeds or panics.
// The callback must not copy, retain, log, or asynchronously use the plaintext.
func (s *Service) WithActiveSubject(
	ctx context.Context,
	provider string,
	callback func([]byte) error,
) (returnError error) {
	if !s.validContext(ctx) || !validProvider(provider) || callback == nil {
		return ErrInvalid
	}
	var bindingNo string
	var sealed Ciphertext
	err := s.database.QueryRowContext(ctx, `
SELECT subject.binding_no, subject.subject_ciphertext, subject.subject_nonce,
       subject.cipher_format_version, subject.encryption_key_version
FROM idc_gateway_local_supplier_subject subject
JOIN idc_gateway_local_supplier_instance instance
  ON instance.installation_id = subject.installation_id
WHERE subject.installation_id = $1 AND subject.provider_code = $2
  AND subject.status_code = 'active' AND instance.status_code = 'active'`,
		s.installationID, provider,
	).Scan(
		&bindingNo, &sealed.Payload, &sealed.Nonce,
		&sealed.FormatVersion, &sealed.KeyVersion,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return ErrUnavailable
	}
	plaintext, err := s.keyring.Open(s.installationID, bindingNo, provider, sealed)
	if err != nil {
		return ErrUnavailable
	}
	defer clearBytes(plaintext)
	return callback(plaintext)
}

func (s *Service) writeSubject(
	ctx context.Context,
	provider string,
	previousBindingNo string,
	reasonCode string,
	subjectInput []byte,
	idempotencyInput []byte,
) (Result, error) {
	subject := consume(subjectInput)
	operationKey := consume(idempotencyInput)
	defer clearBytes(subject)
	defer clearBytes(operationKey)
	isReopen := previousBindingNo != ""
	operationKind := OperationSubjectBind
	if isReopen {
		operationKind = OperationSubjectReopen
	}
	if !s.validContext(ctx) || !validProvider(provider) || len(subject) == 0 ||
		len(operationKey) != SecretBytes {
		return Result{}, ErrInvalid
	}
	canonicalizer := s.canonicalizers[provider]
	if canonicalizer == nil {
		return Result{}, ErrCanonicalizerOff
	}
	canonical, err := canonicalizer.Canonicalize(ctx, subject)
	if err != nil || len(canonical) == 0 || len(canonical) > maximumCanonicalBytes {
		clearBytes(canonical)
		return Result{}, ErrInvalid
	}
	defer clearBytes(canonical)
	lookupDigests, err := s.keyring.LookupDigests(provider, canonical)
	if err != nil {
		return Result{}, ErrUnavailable
	}
	if len(lookupDigests) == 0 {
		return Result{}, ErrUnavailable
	}
	// The oldest still-retained lookup version keeps the logical request digest
	// stable across rotation. It may be removed only after idempotency retention verification.
	requestLookup := &lookupDigests[0]
	requestDigest := digestRequest(
		operationKind, provider, previousBindingNo, reasonCode,
		hex.EncodeToString(requestLookup.Sum[:]),
	)
	idempotencyDigests, err := s.identityDigests(operationKind, operationKey)
	if err != nil {
		return Result{}, ErrUnavailable
	}
	transaction, err := s.database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return Result{}, ErrUnavailable
	}
	defer rollback(transaction)
	if recovered, found, recoverErr := recoverOperation(
		ctx, transaction, operationKind, idempotencyDigests, requestDigest,
	); found || recoverErr != nil {
		return recovered, recoverErr
	}
	status, err := lockInstance(ctx, transaction, s.installationID)
	if err != nil {
		return Result{}, ErrUnavailable
	}
	if status != "active" {
		return Result{}, ErrInstanceRevoked
	}
	if found, findErr := findMatchingBinding(
		ctx, transaction, provider, lookupDigests, "active", "", "",
	); findErr != nil {
		return Result{}, ErrUnavailable
	} else if found {
		return Result{}, ErrConflict
	}
	if isReopen {
		found, findErr := findMatchingBinding(
			ctx, transaction, provider, lookupDigests, "revoked", previousBindingNo,
			s.installationID,
		)
		if findErr != nil {
			return Result{}, ErrUnavailable
		}
		if !found {
			return Result{}, ErrNotFound
		}
	} else if found, findErr := findMatchingBinding(
		ctx, transaction, provider, lookupDigests, "", "", s.installationID,
	); findErr != nil {
		return Result{}, ErrUnavailable
	} else if found {
		return Result{}, ErrReopenRequired
	}
	bindingNo, err := randomIdentifier(s.random, "lsb_")
	if err != nil {
		return Result{}, ErrUnavailable
	}
	sealed, err := s.keyring.Seal(s.random, s.installationID, bindingNo, provider, canonical)
	if err != nil {
		return Result{}, ErrUnavailable
	}
	defer clearBytes(sealed.Nonce)
	defer clearBytes(sealed.Payload)
	var reopened any
	if isReopen {
		reopened = previousBindingNo
	}
	if _, err = transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_local_supplier_subject
    (binding_no, installation_id, provider_code, subject_ciphertext, subject_nonce,
     cipher_format_version, encryption_key_version, reopened_from_binding_no)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		bindingNo, s.installationID, provider, sealed.Payload, sealed.Nonce,
		sealed.FormatVersion, sealed.KeyVersion, reopened,
	); err != nil {
		return Result{}, classifyDatabaseError(err)
	}
	if err = insertLookupDigests(ctx, transaction, bindingNo, provider, lookupDigests); err != nil {
		return Result{}, classifyDatabaseError(err)
	}
	operationNo, err := randomIdentifier(s.random, "lsi_")
	if err != nil {
		return Result{}, ErrUnavailable
	}
	activeIdempotency := activeDigest(
		idempotencyDigests, s.keyring.ActiveVersion(PurposeIdentityOperation),
	)
	if activeIdempotency == nil {
		return Result{}, ErrUnavailable
	}
	var reason any
	if reasonCode != "" {
		reason = reasonCode
	}
	if _, err = transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_local_identity_operation
    (operation_no, installation_id, operation_kind, provider_code, binding_no,
     previous_binding_no, idempotency_key_version, idempotency_hmac,
     request_sha256, reason_code)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		operationNo, s.installationID, operationKind, provider, bindingNo, reopened,
		activeIdempotency.KeyVersion, activeIdempotency.Sum[:], requestDigest[:], reason,
	); err != nil {
		return Result{}, classifyDatabaseError(err)
	}
	if err = transaction.Commit(); err != nil {
		return Result{}, classifyDatabaseError(err)
	}
	return Result{OperationNo: operationNo, BindingNo: bindingNo}, nil
}

func recoverOperation(
	ctx context.Context,
	transaction *sql.Tx,
	operationKind string,
	candidates []Digest,
	requestDigest [sha256.Size]byte,
) (Result, bool, error) {
	clause, arguments := digestPredicate(candidates, 1, "idempotency_key_version", "idempotency_hmac")
	if clause == "" {
		return Result{}, false, ErrUnavailable
	}
	query := `SELECT operation_no, operation_kind, request_sha256, COALESCE(binding_no, '')
FROM idc_gateway_local_identity_operation WHERE ` + clause + ` LIMIT 2`
	rows, err := transaction.QueryContext(ctx, query, arguments...)
	if err != nil {
		return Result{}, false, ErrUnavailable
	}
	defer rows.Close()
	var result Result
	var storedKind string
	var storedDigest []byte
	count := 0
	for rows.Next() {
		count++
		if err = rows.Scan(&result.OperationNo, &storedKind, &storedDigest, &result.BindingNo); err != nil {
			return Result{}, false, ErrUnavailable
		}
	}
	if err = rows.Err(); err != nil {
		return Result{}, false, ErrUnavailable
	}
	if count == 0 {
		return Result{}, false, nil
	}
	if count != 1 || storedKind != operationKind || len(storedDigest) != sha256.Size ||
		subtle.ConstantTimeCompare(storedDigest, requestDigest[:]) != 1 {
		return Result{}, true, ErrConflict
	}
	result.Replayed = true
	return result, true, nil
}

func findMatchingBinding(
	ctx context.Context,
	transaction *sql.Tx,
	provider string,
	digests []Digest,
	status string,
	bindingNo string,
	installationID string,
) (bool, error) {
	clause, digestArguments := digestPredicate(
		digests, 2, "lookup.lookup_key_version", "lookup.subject_lookup_hmac",
	)
	arguments := []any{provider}
	arguments = append(arguments, digestArguments...)
	query := `SELECT lookup.binding_no
FROM idc_gateway_local_subject_lookup lookup
JOIN idc_gateway_local_supplier_subject subject ON subject.binding_no = lookup.binding_no
WHERE lookup.provider_code = $1 AND (` + clause + `)`
	if status != "" {
		arguments = append(arguments, status)
		query += fmt.Sprintf(" AND subject.status_code = $%d", len(arguments))
	}
	if bindingNo != "" {
		arguments = append(arguments, bindingNo)
		query += fmt.Sprintf(" AND subject.binding_no = $%d", len(arguments))
	}
	if installationID != "" {
		arguments = append(arguments, installationID)
		query += fmt.Sprintf(" AND subject.installation_id = $%d", len(arguments))
	}
	query += " LIMIT 1 FOR UPDATE OF subject"
	var found string
	err := transaction.QueryRowContext(ctx, query, arguments...).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func insertLookupDigests(
	ctx context.Context,
	transaction *sql.Tx,
	bindingNo string,
	provider string,
	digests []Digest,
) error {
	for _, digest := range digests {
		if _, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_local_subject_lookup
    (binding_no, provider_code, lookup_key_version, subject_lookup_hmac)
VALUES ($1, $2, $3, $4)
ON CONFLICT (binding_no, lookup_key_version) DO NOTHING`,
			bindingNo, provider, digest.KeyVersion, digest.Sum[:],
		); err != nil {
			return err
		}
	}
	return nil
}

func digestPredicate(
	digests []Digest,
	start int,
	versionColumn string,
	digestColumn string,
) (string, []any) {
	if len(digests) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(digests))
	arguments := make([]any, 0, len(digests)*2)
	parameter := start
	for _, digest := range digests {
		parts = append(parts, fmt.Sprintf(
			"(%s = $%d AND %s = $%d)", versionColumn, parameter, digestColumn, parameter+1,
		))
		arguments = append(arguments, digest.KeyVersion, digest.Sum[:])
		parameter += 2
	}
	return strings.Join(parts, " OR "), arguments
}

func lockInstance(ctx context.Context, transaction *sql.Tx, installationID string) (string, error) {
	var status string
	err := transaction.QueryRowContext(ctx, `
SELECT status_code FROM idc_gateway_local_supplier_instance
WHERE installation_id = $1 FOR UPDATE`, installationID).Scan(&status)
	return status, err
}

func activeDigest(digests []Digest, version uint32) *Digest {
	for index := range digests {
		if digests[index].KeyVersion == version {
			return &digests[index]
		}
	}
	return nil
}

func digestRequest(values ...string) [sha256.Size]byte {
	frame := appendFrame(nil, []byte("idc-compliance/supplier-identity-request/v1"))
	for _, value := range values {
		frame = appendFrame(frame, []byte(value))
	}
	digest := sha256.Sum256(frame)
	clearBytes(frame)
	return digest
}

func randomIdentifier(random io.Reader, prefix string) (string, error) {
	value, err := randomBytes(random, 16)
	if err != nil {
		return "", err
	}
	defer clearBytes(value)
	return prefix + hex.EncodeToString(value), nil
}

func consume(value []byte) []byte {
	result := append([]byte(nil), value...)
	clearBytes(value)
	return result
}

func validProvider(provider string) bool {
	return provider == ProviderTencentESSBasic || provider == ProviderNuonuoInvoice
}

func validRevokeReason(reason string) bool {
	switch reason {
	case ReasonSubjectReplaced, ReasonAuthorizationLost, ReasonSecurityCompromise, ReasonInstanceRevoked:
		return true
	default:
		return false
	}
}

func validReopenReason(reason string) bool {
	switch reason {
	case ReasonReplacementApproved, ReasonSupplierReauthorized, ReasonSecurityRecovered:
		return true
	default:
		return false
	}
}

func (s *Service) validContext(ctx context.Context) bool {
	return s != nil && s.database != nil && s.keyring != nil &&
		ctx != nil && ctx.Err() == nil && installationPattern.MatchString(s.installationID) &&
		s.keyring.ActiveVersion(PurposeSubjectEncryption) != 0 &&
		s.keyring.ActiveVersion(PurposeSubjectLookup) != 0 &&
		s.keyring.ActiveVersion(PurposeIdentityOperation) != 0
}

func (s *Service) identityDigests(_ string, operationKey []byte) ([]Digest, error) {
	return s.keyring.IdempotencyDigests(s.installationID, operationKey)
}

func isNilCanonicalizer(canonicalizer Canonicalizer) bool {
	if canonicalizer == nil {
		return true
	}
	value := reflect.ValueOf(canonicalizer)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func classifyDatabaseError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) &&
		(postgresError.Code == "23505" || postgresError.Code == "40001") {
		return ErrConflict
	}
	return ErrUnavailable
}

func rollback(transaction *sql.Tx) {
	if transaction != nil {
		_ = transaction.Rollback()
	}
}
