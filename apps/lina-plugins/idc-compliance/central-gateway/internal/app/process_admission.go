// This file commits database-authoritative process nonce and business
// idempotency fences for authenticated local gateway requests.

package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

// databaseAdmissionTimeout bounds the complete database-authoritative admission.
const databaseAdmissionTimeout = 2 * time.Second

var (
	// errProcessNonceConflict identifies one nonce reused for a different signed frame.
	errProcessNonceConflict = errors.New("local process nonce conflicts with another frame")
	// errBusinessConflict identifies one business key reused for different content.
	errBusinessConflict = errors.New("local business idempotency key conflicts with another request")
)

// businessAdmission identifies the stable write request recovered or created.
type businessAdmission struct {
	requestNo string
	recovered bool
}

// authenticateRead verifies a read and persists only its nonce fence.
func (r *localRuntime) authenticateRead(
	ctx context.Context,
	expectedScope string,
	method string,
	requestTarget string,
	payloadMode processauth.PayloadMode,
	body []byte,
	credentials processauth.Credentials,
) (verified processauth.Verified, recovered bool, err error) {
	verified, _, recovered, err = r.authenticateRequest(
		ctx, expectedScope, method, requestTarget, payloadMode, body, credentials, false,
	)
	return verified, recovered, err
}

// authenticateRequest verifies one frame and commits its required persistent fences.
func (r *localRuntime) authenticateRequest(
	ctx context.Context,
	expectedScope string,
	method string,
	requestTarget string,
	payloadMode processauth.PayloadMode,
	body []byte,
	credentials processauth.Credentials,
	persistBusinessFence bool,
) (
	verified processauth.Verified,
	admission businessAdmission,
	nonceRecovered bool,
	err error,
) {
	if err = r.validateAdmissionInput(ctx); err != nil {
		return processauth.Verified{}, businessAdmission{}, false, err
	}
	databaseContext, cancel := context.WithTimeout(ctx, databaseAdmissionTimeout)
	defer cancel()
	transaction, err := r.database.BeginTx(databaseContext, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return processauth.Verified{}, businessAdmission{}, false, processauth.ErrRejected
	}
	defer finishAdmissionTransaction(transaction, &err)
	databaseNow, err := databaseTime(databaseContext, transaction)
	if err != nil {
		return processauth.Verified{}, businessAdmission{}, false, processauth.ErrRejected
	}
	verified, err = r.keyring.Verify(
		r.installationID, expectedScope, method, requestTarget, payloadMode, body,
		credentials, databaseNow, r.processAuthWindow,
	)
	if err != nil {
		return processauth.Verified{}, businessAdmission{}, false, err
	}
	var requestNo *string
	if persistBusinessFence {
		admission, err = registerBusinessRequest(databaseContext, transaction, verified)
		if err != nil {
			return processauth.Verified{}, businessAdmission{}, false, err
		}
		requestNo = &admission.requestNo
	}
	nonceRecovered, err = consumeNonce(databaseContext, transaction, verified, requestNo)
	if err != nil {
		return processauth.Verified{}, businessAdmission{}, false, err
	}
	if err = transaction.Commit(); err != nil {
		return processauth.Verified{}, businessAdmission{}, false, processauth.ErrRejected
	}
	return verified, admission, nonceRecovered, nil
}

// validateAdmissionInput rejects incomplete or canceled runtime state.
func (r *localRuntime) validateAdmissionInput(ctx context.Context) error {
	if r == nil || r.database == nil || r.keyring == nil || r.installationID == "" ||
		r.processAuthWindow <= 0 || r.processAuthWindow > processauth.MaximumAuthenticationWindow ||
		ctx == nil || ctx.Err() != nil {
		return processauth.ErrRejected
	}
	return nil
}

// databaseTime obtains the transaction's authoritative UTC comparison time.
func databaseTime(ctx context.Context, transaction *sql.Tx) (time.Time, error) {
	var databaseNow time.Time
	if err := transaction.QueryRowContext(ctx, `SELECT CURRENT_TIMESTAMP`).Scan(&databaseNow); err != nil {
		return time.Time{}, err
	}
	return databaseNow.UTC(), nil
}

// consumeNonce inserts a one-way nonce fence or recovers the identical frame.
func consumeNonce(
	ctx context.Context,
	transaction *sql.Tx,
	verified processauth.Verified,
	requestNo *string,
) (bool, error) {
	result, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_process_nonce
    (installation_id, nonce_sha256, frame_sha256, request_no, key_id, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (installation_id, nonce_sha256) DO NOTHING`,
		verified.InstallationID, verified.NonceSHA256[:], verified.FrameSHA256[:], requestNo,
		verified.KeyID, verified.ExpiresAt.UTC())
	if err != nil {
		return false, processauth.ErrRejected
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, processauth.ErrRejected
	}
	if affected == 1 {
		return false, nil
	}
	var (
		existingFrame     []byte
		existingRequestNo sql.NullString
	)
	err = transaction.QueryRowContext(ctx, `
SELECT frame_sha256, request_no
FROM idc_gateway_process_nonce
WHERE installation_id = $1 AND nonce_sha256 = $2`,
		verified.InstallationID, verified.NonceSHA256[:]).Scan(&existingFrame, &existingRequestNo)
	if err != nil || !bytes.Equal(existingFrame, verified.FrameSHA256[:]) ||
		!sameNullableRequest(existingRequestNo, requestNo) {
		return false, errProcessNonceConflict
	}
	return true, nil
}

// registerBusinessRequest creates or recovers one stable write admission.
func registerBusinessRequest(
	ctx context.Context,
	transaction *sql.Tx,
	verified processauth.Verified,
) (businessAdmission, error) {
	requestNo, err := generateBusinessRequestNo()
	if err != nil {
		return businessAdmission{}, processauth.ErrRejected
	}
	targetDigest := sha256.Sum256([]byte(verified.RequestTarget))
	result, err := transaction.ExecContext(ctx, `
INSERT INTO idc_gateway_local_business_request
    (request_no, installation_id, operation_scope, method, request_target_sha256,
     idempotency_hmac, business_content_sha256, body_sha256)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (installation_id, idempotency_hmac) DO NOTHING`,
		requestNo, verified.InstallationID, verified.OperationScope, verified.Method,
		targetDigest[:], verified.IdempotencyHMAC[:], verified.BusinessContentSHA256[:],
		verified.BodySHA256[:])
	if err != nil {
		return businessAdmission{}, processauth.ErrRejected
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return businessAdmission{}, processauth.ErrRejected
	}
	if affected == 1 {
		return businessAdmission{requestNo: requestNo}, nil
	}
	var existing businessRequestRecord
	err = transaction.QueryRowContext(ctx, `
SELECT request_no, operation_scope, method, request_target_sha256,
       business_content_sha256, body_sha256
FROM idc_gateway_local_business_request
WHERE installation_id = $1 AND idempotency_hmac = $2`,
		verified.InstallationID, verified.IdempotencyHMAC[:]).Scan(
		&existing.requestNo, &existing.operationScope, &existing.method,
		&existing.targetSHA256, &existing.businessContentSHA256, &existing.bodySHA256,
	)
	if err != nil {
		return businessAdmission{}, processauth.ErrRejected
	}
	if !existing.matches(verified, targetDigest) {
		return businessAdmission{}, errBusinessConflict
	}
	return businessAdmission{requestNo: existing.requestNo, recovered: true}, nil
}

// businessRequestRecord is the persistence projection used for conflict checks.
type businessRequestRecord struct {
	requestNo             string
	operationScope        string
	method                string
	targetSHA256          []byte
	businessContentSHA256 []byte
	bodySHA256            []byte
}

// matches reports whether a stored admission represents the same canonical intent.
func (r businessRequestRecord) matches(
	verified processauth.Verified,
	targetDigest [sha256.Size]byte,
) bool {
	return r.requestNo != "" && r.operationScope == verified.OperationScope &&
		r.method == verified.Method && bytes.Equal(r.targetSHA256, targetDigest[:]) &&
		bytes.Equal(r.businessContentSHA256, verified.BusinessContentSHA256[:]) &&
		bytes.Equal(r.bodySHA256, verified.BodySHA256[:])
}

// generateBusinessRequestNo creates a non-secret random local write identifier.
func generateBusinessRequestNo() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		clearRuntimeBytes(value)
		return "", err
	}
	defer clearRuntimeBytes(value)
	return "lbr_" + hex.EncodeToString(value), nil
}

// sameNullableRequest compares the optional write association on a nonce fence.
func sameNullableRequest(existing sql.NullString, requestNo *string) bool {
	if requestNo == nil {
		return !existing.Valid
	}
	return existing.Valid && existing.String == *requestNo
}

// finishAdmissionTransaction rolls back unfinished work without hiding prior errors.
func finishAdmissionTransaction(transaction *sql.Tx, returnError *error) {
	rollbackError := transaction.Rollback()
	if rollbackError != nil && !errors.Is(rollbackError, sql.ErrTxDone) && *returnError == nil {
		*returnError = fmt.Errorf("rollback local process admission: %w", rollbackError)
	}
}
