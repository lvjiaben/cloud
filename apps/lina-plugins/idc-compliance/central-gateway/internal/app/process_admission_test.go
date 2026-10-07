// This file verifies the PostgreSQL transaction boundary that keeps process
// nonce replay and write idempotency as independent persistent fences.

package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

const admissionTestInstallationID = "linst_3333333333333333333333333333333333333333333333333333333333333333"

func TestAuthenticateReadPersistsOnlyNonceAndRecoversExactFrame(t *testing.T) {
	runtime, mock := newAdmissionTestRuntime(t)
	now := time.Unix(1_800_000_000, 0).UTC()
	credentials := signAdmissionTestFrame(
		t, runtime, "installation.read", "GET", localInstallationPath,
		processauth.PayloadModeEmpty, nil, admissionBusinessKey(t, 31), now, 32,
	)
	verified := verifyAdmissionTestFrame(
		t, runtime, "installation.read", "GET", localInstallationPath,
		processauth.PayloadModeEmpty, nil, credentials, now,
	)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT CURRENT_TIMESTAMP`).WillReturnRows(
		sqlmock.NewRows([]string{"current_timestamp"}).AddRow(now),
	)
	mock.ExpectExec(`(?s)INSERT INTO idc_gateway_process_nonce`).
		WithArgs(
			admissionTestInstallationID, sqlmock.AnyArg(), sqlmock.AnyArg(), nil,
			credentials.KeyID, credentialsExpiry(credentials),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	_, recovered, err := runtime.authenticateRead(
		context.Background(), "installation.read", "GET", localInstallationPath,
		processauth.PayloadModeEmpty, nil, credentials,
	)
	if err != nil || recovered {
		t.Fatalf("first admission recovered=%t err=%v", recovered, err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT CURRENT_TIMESTAMP`).WillReturnRows(
		sqlmock.NewRows([]string{"current_timestamp"}).AddRow(now),
	)
	mock.ExpectExec(`(?s)INSERT INTO idc_gateway_process_nonce`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`(?s)SELECT frame_sha256, request_no`).
		WithArgs(admissionTestInstallationID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"frame_sha256", "request_no"}).
			AddRow(verified.FrameSHA256[:], nil))
	mock.ExpectCommit()
	_, recovered, err = runtime.authenticateRead(
		context.Background(), "installation.read", "GET", localInstallationPath,
		processauth.PayloadModeEmpty, nil, credentials,
	)
	if err != nil || !recovered {
		t.Fatalf("replay recovered=%t err=%v", recovered, err)
	}
	assertAdmissionExpectations(t, runtime, mock)
}

func TestAuthenticateReadRejectsSameNonceForDifferentFrame(t *testing.T) {
	runtime, mock := newAdmissionTestRuntime(t)
	now := time.Unix(1_800_000_000, 0).UTC()
	first := signAdmissionTestFrame(
		t, runtime, "items.read", "GET", "/internal/v1/items?a=1",
		processauth.PayloadModeEmpty, nil, admissionBusinessKey(t, 33), now, 34,
	)
	firstVerified := verifyAdmissionTestFrame(
		t, runtime, "items.read", "GET", "/internal/v1/items?a=1",
		processauth.PayloadModeEmpty, nil, first, now,
	)
	second := signAdmissionTestFrame(
		t, runtime, "items.read", "GET", "/internal/v1/items?a=2",
		processauth.PayloadModeEmpty, nil, admissionBusinessKey(t, 35), now, 34,
	)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT CURRENT_TIMESTAMP`).WillReturnRows(
		sqlmock.NewRows([]string{"current_timestamp"}).AddRow(now),
	)
	mock.ExpectExec(`(?s)INSERT INTO idc_gateway_process_nonce`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`(?s)SELECT frame_sha256, request_no`).
		WillReturnRows(sqlmock.NewRows([]string{"frame_sha256", "request_no"}).
			AddRow(firstVerified.FrameSHA256[:], nil))
	mock.ExpectRollback()
	_, _, err := runtime.authenticateRead(
		context.Background(), "items.read", "GET", "/internal/v1/items?a=2",
		processauth.PayloadModeEmpty, nil, second,
	)
	if err != errProcessNonceConflict {
		t.Fatalf("conflicting nonce err=%v", err)
	}
	assertAdmissionExpectations(t, runtime, mock)
}

func TestAuthenticateWriteRecoversSameContentAndRejectsChangedContent(t *testing.T) {
	runtime, mock := newAdmissionTestRuntime(t)
	now := time.Unix(1_800_000_000, 0).UTC()
	businessKey := admissionBusinessKey(t, 36)
	firstBody := []byte(`{"a":1,"b":2}`)
	first := signAdmissionTestFrame(
		t, runtime, "contract.create", "POST", "/internal/v1/contracts",
		processauth.PayloadModeJSON, firstBody, businessKey, now, 37,
	)
	firstVerified := verifyAdmissionTestFrame(
		t, runtime, "contract.create", "POST", "/internal/v1/contracts",
		processauth.PayloadModeJSON, firstBody, first, now,
	)
	targetDigest := sha256.Sum256([]byte(firstVerified.RequestTarget))

	expectBusinessInsert(mock, 1)
	expectNonceInsert(mock, first.KeyID, credentialsExpiry(first))
	mock.ExpectCommit()
	_, firstAdmission, _, err := runtime.authenticateRequest(
		context.Background(), "contract.create", "POST", "/internal/v1/contracts",
		processauth.PayloadModeJSON, firstBody, first, true,
	)
	if err != nil || firstAdmission.recovered || firstAdmission.requestNo == "" {
		t.Fatalf("first admission=%+v err=%v", firstAdmission, err)
	}

	secondBody := []byte(`{ "b": 2.0, "a": 1 }`)
	second := signAdmissionTestFrame(
		t, runtime, "contract.create", "POST", "/internal/v1/contracts",
		processauth.PayloadModeJSON, secondBody, businessKey, now.Add(time.Second), 38,
	)
	expectBusinessRecovery(
		mock, firstAdmission.requestNo, firstVerified, targetDigest,
	)
	expectNonceInsert(mock, second.KeyID, credentialsExpiry(second))
	mock.ExpectCommit()
	_, secondAdmission, _, err := runtime.authenticateRequest(
		context.Background(), "contract.create", "POST", "/internal/v1/contracts",
		processauth.PayloadModeJSON, secondBody, second, true,
	)
	if err != nil || !secondAdmission.recovered ||
		secondAdmission.requestNo != firstAdmission.requestNo {
		t.Fatalf("second admission=%+v err=%v", secondAdmission, err)
	}

	changedBody := []byte(`{"a":1,"b":3}`)
	changed := signAdmissionTestFrame(
		t, runtime, "contract.create", "POST", "/internal/v1/contracts",
		processauth.PayloadModeJSON, changedBody, businessKey, now.Add(2*time.Second), 39,
	)
	expectBusinessRecovery(
		mock, firstAdmission.requestNo, firstVerified, targetDigest,
	)
	mock.ExpectRollback()
	_, _, _, err = runtime.authenticateRequest(
		context.Background(), "contract.create", "POST", "/internal/v1/contracts",
		processauth.PayloadModeJSON, changedBody, changed, true,
	)
	if err != errBusinessConflict {
		t.Fatalf("changed-content err=%v", err)
	}
	assertAdmissionExpectations(t, runtime, mock)
}

func newAdmissionTestRuntime(t *testing.T) (*localRuntime, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("new SQL mock: %v", err)
	}
	file, err := processauth.Generate(bytes.NewReader(bytes.Repeat([]byte{40}, 512)))
	if err != nil {
		t.Fatalf("generate keyring: %v", err)
	}
	ring, err := processauth.New(file)
	if err != nil {
		t.Fatalf("new keyring: %v", err)
	}
	t.Cleanup(ring.Clear)
	return &localRuntime{
		database: database, keyring: ring, installationID: admissionTestInstallationID,
		processAuthWindow: 45 * time.Second,
	}, mock
}

func assertAdmissionExpectations(
	t *testing.T,
	runtime *localRuntime,
	mock sqlmock.Sqlmock,
) {
	t.Helper()
	mock.ExpectClose()
	if err := runtime.database.Close(); err != nil {
		t.Fatalf("close SQL mock: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("database expectations: %v", err)
	}
}

func signAdmissionTestFrame(
	t *testing.T,
	runtime *localRuntime,
	scope string,
	method string,
	target string,
	payloadMode processauth.PayloadMode,
	body []byte,
	businessKey string,
	now time.Time,
	randomByte byte,
) processauth.Credentials {
	t.Helper()
	credentials, err := runtime.keyring.Sign(
		runtime.installationID, scope, method, target, payloadMode, body, businessKey,
		now, runtime.processAuthWindow,
		bytes.NewReader(bytes.Repeat([]byte{randomByte}, 128)),
	)
	if err != nil {
		t.Fatalf("sign frame: %v", err)
	}
	return credentials
}

func verifyAdmissionTestFrame(
	t *testing.T,
	runtime *localRuntime,
	scope string,
	method string,
	target string,
	payloadMode processauth.PayloadMode,
	body []byte,
	credentials processauth.Credentials,
	now time.Time,
) processauth.Verified {
	t.Helper()
	verified, err := runtime.keyring.Verify(
		runtime.installationID, scope, method, target, payloadMode, body, credentials,
		now, runtime.processAuthWindow,
	)
	if err != nil {
		t.Fatalf("verify frame: %v", err)
	}
	return verified
}

func admissionBusinessKey(t *testing.T, value byte) string {
	t.Helper()
	key, err := processauth.GenerateBusinessIdempotencyKey(
		bytes.NewReader(bytes.Repeat([]byte{value}, 64)),
	)
	if err != nil {
		t.Fatalf("generate business key: %v", err)
	}
	return key
}

func credentialsExpiry(credentials processauth.Credentials) time.Time {
	return time.UnixMilli(credentials.ExpiresMS).UTC()
}

func expectBusinessInsert(mock sqlmock.Sqlmock, affected int64) {
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT CURRENT_TIMESTAMP`).WillReturnRows(
		sqlmock.NewRows([]string{"current_timestamp"}).AddRow(time.Unix(1_800_000_000, 0).UTC()),
	)
	mock.ExpectExec(`(?s)INSERT INTO idc_gateway_local_business_request`).
		WillReturnResult(sqlmock.NewResult(0, affected))
}

func expectBusinessRecovery(
	mock sqlmock.Sqlmock,
	requestNo string,
	verified processauth.Verified,
	targetDigest [sha256.Size]byte,
) {
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT CURRENT_TIMESTAMP`).WillReturnRows(
		sqlmock.NewRows([]string{"current_timestamp"}).AddRow(
			time.Unix(1_800_000_000, 0).UTC().Add(time.Second),
		),
	)
	mock.ExpectExec(`(?s)INSERT INTO idc_gateway_local_business_request`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`(?s)SELECT request_no, operation_scope, method`).
		WillReturnRows(sqlmock.NewRows([]string{
			"request_no", "operation_scope", "method", "request_target_sha256",
			"business_content_sha256", "body_sha256",
		}).AddRow(
			requestNo, verified.OperationScope, verified.Method, targetDigest[:],
			verified.BusinessContentSHA256[:], verified.BodySHA256[:],
		))
}

func expectNonceInsert(mock sqlmock.Sqlmock, keyID string, expiresAt time.Time) {
	mock.ExpectExec(`(?s)INSERT INTO idc_gateway_process_nonce`).
		WithArgs(
			admissionTestInstallationID, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			keyID, expiresAt,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
}
