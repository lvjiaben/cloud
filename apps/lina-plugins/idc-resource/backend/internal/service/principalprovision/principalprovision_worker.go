package principalprovision

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const maximumVerificationAttempts = 16
const maximumExpiredRecoveryBatch = 32

type providerConstructor func(
	context.Context,
	string,
) (resourcecap.Provider, func(), error)

// Worker advances at most one durable principal task per RunOne call.
type Worker struct {
	database  gdb.DB
	binder    ManagerBinder
	construct providerConstructor
	random    io.Reader
}

// NewWorker constructs the managed-job worker.
func NewWorker(
	database gdb.DB,
	binder ManagerBinder,
	construct providerConstructor,
) (*Worker, error) {
	if database == nil || binder == nil || construct == nil {
		return nil, gerror.New("resource principal worker dependencies are required")
	}
	return &Worker{database: database, binder: binder, construct: construct, random: rand.Reader}, nil
}

type claimedTask struct {
	TaskNo                       string    `orm:"task_no"`
	CustomerNo                   string    `orm:"customer_no"`
	ChannelNo                    string    `orm:"channel_no"`
	UsernameSecretRef            string    `orm:"username_secret_ref"`
	PasswordSecretRef            string    `orm:"password_secret_ref"`
	ClaimedStatus                string    `orm:"claimed_status"`
	ExpectedExternalPrincipalRef string    `orm:"expected_external_principal_ref"`
	ResultUnknown                bool      `orm:"result_unknown"`
	ObservationAttemptCount      int       `orm:"observation_attempt_count"`
	AttemptCount                 int       `orm:"attempt_count"`
	FailureCode                  string    `orm:"failure_code"`
	LeaseDigest                  string    `orm:"lease_token_digest"`
	LeaseExpiresAt               time.Time `orm:"lease_expires_at"`
	Version                      int64     `orm:"version"`
}

// RunOne returns false when no claimable task exists.
func (w *Worker) RunOne(ctx context.Context) (bool, error) {
	if w == nil || w.database == nil || w.binder == nil || w.construct == nil || w.random == nil ||
		ctx == nil || ctx.Err() != nil {
		return false, gerror.New("resource principal worker is unavailable")
	}
	claim, err := w.claimOne(ctx)
	if err != nil || claim == nil {
		return false, err
	}
	switch claim.ClaimedStatus {
	case "lookup_leased", "verify_leased":
		err = w.lookup(ctx, claim)
	case "create_leased":
		err = w.create(ctx, claim)
	case "cleanup_leased", "manual_cleanup_leased":
		err = w.cleanup(ctx, claim)
	default:
		err = gerror.New("resource principal claim status is invalid")
	}
	return true, err
}

func (w *Worker) claimOne(ctx context.Context) (*claimedTask, error) {
	var claimed *claimedTask
	err := w.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := principalDatabaseTime(tx)
		if err != nil {
			return err
		}
		if err = recoverExpiredClaims(ctx, tx, now); err != nil {
			return err
		}
		var row *struct {
			ID                           int64  `orm:"id"`
			TaskNo                       string `orm:"task_no"`
			CustomerNo                   string `orm:"customer_no"`
			ChannelNo                    string `orm:"channel_no"`
			UsernameSecretRef            string `orm:"username_secret_ref"`
			PasswordSecretRef            string `orm:"password_secret_ref"`
			StatusCode                   string `orm:"status_code"`
			ExpectedExternalPrincipalRef string `orm:"expected_external_principal_ref"`
			ResultUnknown                bool   `orm:"result_unknown"`
			ObservationAttemptCount      int    `orm:"observation_attempt_count"`
			AttemptCount                 int    `orm:"attempt_count"`
			FailureCode                  string `orm:"failure_code"`
			Version                      int64  `orm:"version"`
		}
		err = tx.GetScan(&row, `
SELECT id, task_no, customer_no, channel_no, username_secret_ref, password_secret_ref,
       status_code, expected_external_principal_ref, result_unknown,
       observation_attempt_count, attempt_count, failure_code, version
FROM plugin_idc_resource_principal_task
WHERE tenant_id = ? AND status_code IN (
    'pending_lookup', 'ready_create', 'verify_wait', 'cleanup_pending',
    'manual_cleanup_pending'
) AND available_at <= ?
  AND (status_code <> 'ready_create' OR attempt_count < 3)
ORDER BY available_at ASC, id ASC
LIMIT 1 FOR UPDATE SKIP LOCKED`, idccap.DefaultTenantID, now)
		if err != nil || row == nil {
			return err
		}
		leaseToken := make([]byte, 32)
		if _, err = io.ReadFull(w.random, leaseToken); err != nil {
			return gerror.Wrap(err, "generate resource principal lease")
		}
		digest := sha256.Sum256(leaseToken)
		clear(leaseToken)
		leaseDigest := hex.EncodeToString(digest[:])
		leasedStatus := map[string]string{
			"pending_lookup": "lookup_leased", "ready_create": "create_leased",
			"verify_wait": "verify_leased", "cleanup_pending": "cleanup_leased",
			"manual_cleanup_pending": "manual_cleanup_leased",
		}[row.StatusCode]
		leaseExpiresAt := now.Add(2 * time.Minute)
		result, err := tx.Exec(`
UPDATE plugin_idc_resource_principal_task
SET status_code = ?, lease_token_digest = ?, lease_expires_at = ?,
    attempt_count = attempt_count + CASE WHEN ? = 'create_leased' THEN 1 ELSE 0 END,
    observation_attempt_count = observation_attempt_count +
        CASE WHEN ? IN ('lookup_leased', 'verify_leased') THEN 1 ELSE 0 END,
    version = version + 1, updated_at = ?
WHERE id = ? AND version = ?`,
			leasedStatus, leaseDigest, leaseExpiresAt, leasedStatus, leasedStatus,
			now, row.ID, row.Version,
		)
		if err != nil {
			return gerror.Wrap(err, "lease resource principal task")
		}
		if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
			return gerror.New("resource principal task lease was lost")
		}
		claimed = &claimedTask{
			TaskNo: row.TaskNo, CustomerNo: row.CustomerNo, ChannelNo: row.ChannelNo,
			UsernameSecretRef: row.UsernameSecretRef, PasswordSecretRef: row.PasswordSecretRef,
			ClaimedStatus: leasedStatus, ExpectedExternalPrincipalRef: row.ExpectedExternalPrincipalRef,
			ResultUnknown:           row.ResultUnknown,
			ObservationAttemptCount: row.ObservationAttemptCount + boolInt(leasedStatus == "lookup_leased" || leasedStatus == "verify_leased"),
			AttemptCount:            row.AttemptCount + boolInt(leasedStatus == "create_leased"),
			FailureCode:             row.FailureCode,
			LeaseDigest:             leaseDigest, LeaseExpiresAt: leaseExpiresAt, Version: row.Version + 1,
		}
		return nil
	})
	if err != nil {
		return nil, gerror.Wrap(err, "claim resource principal task")
	}
	return claimed, nil
}

func recoverExpiredClaims(ctx context.Context, tx gdb.TX, now time.Time) error {
	_, err := tx.Exec(`
WITH expired AS (
    SELECT id
    FROM plugin_idc_resource_principal_task
    WHERE tenant_id = ? AND status_code IN (
        'lookup_leased', 'create_leased', 'verify_leased', 'cleanup_leased',
        'manual_cleanup_leased'
    ) AND lease_expires_at <= ?
    ORDER BY lease_expires_at ASC, id ASC
    LIMIT ? FOR UPDATE SKIP LOCKED
)
UPDATE plugin_idc_resource_principal_task
SET status_code = CASE status_code
        WHEN 'lookup_leased' THEN 'pending_lookup'
        WHEN 'create_leased' THEN 'verify_wait'
        WHEN 'verify_leased' THEN 'verify_wait'
        WHEN 'cleanup_leased' THEN 'cleanup_pending'
		WHEN 'manual_cleanup_leased' THEN 'manual_cleanup_pending'
    END,
    result_unknown = result_unknown OR status_code = 'create_leased',
    failure_code = CASE WHEN status_code = 'create_leased' THEN 'PRINCIPAL_WRITE_LEASE_EXPIRED' ELSE failure_code END,
    available_at = ?, lease_token_digest = '', lease_expires_at = NULL,
    version = version + 1, updated_at = ?
WHERE id IN (SELECT id FROM expired)`,
		idccap.DefaultTenantID, now, maximumExpiredRecoveryBatch, now, now,
	)
	return gerror.Wrap(err, "recover expired resource principal claims")
}

func (w *Worker) lookup(ctx context.Context, claim *claimedTask) error {
	provider, cleanup, err := w.construct(ctx, claim.ChannelNo)
	if cleanup != nil {
		defer cleanup()
	}
	principalProvider, ok := provider.(resourcecap.PrincipalProvider)
	if err != nil || !ok || principalProvider == nil || cleanup == nil {
		return w.rescheduleLookup(ctx, claim, "PRINCIPAL_PROVIDER_UNAVAILABLE")
	}
	username, err := secretcap.NewHandle(claim.UsernameSecretRef)
	if err != nil {
		return w.beginManualCleanup(ctx, claim, "PRINCIPAL_SECRET_INVALID")
	}
	result, callErr := principalProvider.LookupPrincipal(ctx, resourcecap.PrincipalLookupRequest{
		RequestID: claim.TaskNo, Username: username,
	})
	if callErr != nil || result == nil || result.Validate() != nil {
		return w.rescheduleLookup(ctx, claim, "PRINCIPAL_LOOKUP_UNAVAILABLE")
	}
	if !result.Exists {
		if claim.ClaimedStatus == "lookup_leased" && !claim.ResultUnknown {
			return w.transition(ctx, claim, "ready_create", "", "", false, time.Time{})
		}
		// A documented non-acceptance is replayable only after a fresh exact
		// lookup again proves absence. Successful or ambiguous create attempts
		// have no failure code that can restore write eligibility.
		if claim.ClaimedStatus == "verify_leased" && !claim.ResultUnknown &&
			claim.ExpectedExternalPrincipalRef == "" && claim.FailureCode != "" {
			if claim.AttemptCount >= 3 {
				return w.beginManualCleanup(ctx, claim, "PRINCIPAL_CREATE_RETRY_EXHAUSTED")
			}
			return w.transition(ctx, claim, "ready_create", "", claim.FailureCode, false, time.Time{})
		}
		if claim.ObservationAttemptCount >= maximumVerificationAttempts {
			return w.beginManualCleanup(ctx, claim, "PRINCIPAL_RESULT_UNRESOLVED")
		}
		return w.rescheduleLookup(ctx, claim, "")
	}
	if claim.ExpectedExternalPrincipalRef != "" &&
		claim.ExpectedExternalPrincipalRef != result.ExternalPrincipalRef {
		return w.beginManualCleanup(ctx, claim, "PRINCIPAL_REFERENCE_MISMATCH")
	}
	return w.confirmMapping(ctx, claim, *result)
}

func (w *Worker) create(ctx context.Context, claim *claimedTask) error {
	provider, cleanup, err := w.construct(ctx, claim.ChannelNo)
	if cleanup != nil {
		defer cleanup()
	}
	principalProvider, ok := provider.(resourcecap.PrincipalProvider)
	if err != nil || !ok || principalProvider == nil || cleanup == nil {
		return w.retrySafeCreate(ctx, claim, "PRINCIPAL_PROVIDER_UNAVAILABLE")
	}
	username, usernameErr := secretcap.NewHandle(claim.UsernameSecretRef)
	password, passwordErr := secretcap.NewHandle(claim.PasswordSecretRef)
	if usernameErr != nil || passwordErr != nil {
		return w.beginManualCleanup(ctx, claim, "PRINCIPAL_SECRET_INVALID")
	}
	result, callErr := principalProvider.CreatePrincipal(ctx, resourcecap.PrincipalCreateRequest{
		RequestID: claim.TaskNo,
		SecretBindings: []resourcecap.SecretBinding{
			{Name: resourcecap.PrincipalBindingUsername, Handle: username},
			{Name: resourcecap.PrincipalBindingPassword, Handle: password},
		},
	})
	if callErr != nil {
		failure := resourcecap.NormalizeProviderError(callErr)
		if failure.ResultUnknown {
			return w.transition(ctx, claim, "verify_wait", "", string(failure.Code), true, claim.LeaseExpiresAt)
		}
		return w.retrySafeCreate(ctx, claim, string(failure.Code))
	}
	if result == nil || result.Validate() != nil {
		return w.transition(ctx, claim, "verify_wait", "", "PRINCIPAL_RESPONSE_INVALID", true, claim.LeaseExpiresAt)
	}
	return w.transition(
		ctx, claim, "verify_wait", result.ExternalPrincipalRef, "", false, claim.LeaseExpiresAt,
	)
}

func (w *Worker) retrySafeCreate(ctx context.Context, claim *claimedTask, code string) error {
	if claim.ResultUnknown {
		return w.transition(ctx, claim, "verify_wait", "", code, true, claim.LeaseExpiresAt)
	}
	if claim.AttemptCount >= 3 {
		return w.beginManualCleanup(ctx, claim, "PRINCIPAL_CREATE_RETRY_EXHAUSTED")
	}
	return w.transition(ctx, claim, "verify_wait", "", code, false, claim.LeaseExpiresAt)
}

func (w *Worker) confirmMapping(
	ctx context.Context,
	claim *claimedTask,
	result resourcecap.PrincipalLookupResult,
) error {
	evidenceDigest := principalEvidenceDigest(claim.TaskNo, claim.CustomerNo, claim.ChannelNo, result.ExternalPrincipalRef)
	mappingNo := principalMappingNo(claim.CustomerNo, claim.ChannelNo)
	return w.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := principalDatabaseTime(tx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`
INSERT INTO plugin_idc_resource_principal_mapping
    (tenant_id, mapping_no, customer_no, channel_no, external_principal_ref,
     status_code, evidence_digest, observed_at)
VALUES (?, ?, ?, ?, ?, 'active', ?, ?)
ON CONFLICT DO NOTHING`, idccap.DefaultTenantID, mappingNo, claim.CustomerNo,
			claim.ChannelNo, result.ExternalPrincipalRef, evidenceDigest, result.ObservedAt,
		); err != nil {
			return gerror.Wrap(err, "persist resource principal mapping")
		}
		var mapping struct {
			MappingNo, ExternalPrincipalRef, EvidenceDigest string
		}
		if err = tx.GetScan(&mapping, `
SELECT mapping_no, external_principal_ref, evidence_digest
FROM plugin_idc_resource_principal_mapping
WHERE tenant_id = ? AND channel_no = ? AND customer_no = ? AND status_code = 'active'`,
			idccap.DefaultTenantID, claim.ChannelNo, claim.CustomerNo,
		); err != nil {
			return gerror.Wrap(err, "verify resource principal mapping")
		}
		if mapping.MappingNo != mappingNo || mapping.ExternalPrincipalRef != result.ExternalPrincipalRef ||
			mapping.EvidenceDigest != evidenceDigest {
			return updateClaimedTask(
				ctx, tx, claim, now, "manual_cleanup_pending", "",
				"PRINCIPAL_MAPPING_CONFLICT", claim.ResultUnknown, "", now,
			)
		}
		return updateClaimedTask(
			ctx, tx, claim, now, "cleanup_pending", result.ExternalPrincipalRef, "",
			claim.ResultUnknown, evidenceDigest, now,
		)
	})
}

func (w *Worker) cleanup(ctx context.Context, claim *claimedTask) error {
	manager, err := w.binder(claim.ChannelNo)
	if err != nil {
		return w.releaseCleanup(ctx, claim)
	}
	references := []string{claim.PasswordSecretRef, claim.UsernameSecretRef}
	terminalStatus := "succeeded"
	terminalCode := ""
	if claim.ClaimedStatus == "manual_cleanup_leased" {
		references = references[:1]
		terminalStatus = "manual_review"
		terminalCode = claim.FailureCode
	}
	for _, reference := range references {
		handle, handleErr := secretcap.NewHandle(reference)
		if handleErr != nil {
			if claim.ClaimedStatus == "manual_cleanup_leased" {
				return w.finish(ctx, claim, terminalStatus, terminalCode)
			}
			return w.releaseCleanup(ctx, claim)
		}
		metadata, metadataErr := manager.Metadata(ctx, handle)
		if metadataErr != nil {
			return w.releaseCleanup(ctx, claim)
		}
		if metadata.Configured {
			if _, metadataErr = manager.Revoke(ctx, handle, metadata.UpdatedAt); metadataErr != nil {
				return w.releaseCleanup(ctx, claim)
			}
		}
	}
	return w.finish(ctx, claim, terminalStatus, terminalCode)
}

func (w *Worker) releaseCleanup(ctx context.Context, claim *claimedTask) error {
	if claim.ClaimedStatus == "manual_cleanup_leased" {
		return w.transition(
			ctx, claim, "manual_cleanup_pending", "", claim.FailureCode,
			claim.ResultUnknown, claim.LeaseExpiresAt,
		)
	}
	return w.transition(ctx, claim, "cleanup_pending", "", "PRINCIPAL_SECRET_CLEANUP_PENDING", claim.ResultUnknown, claim.LeaseExpiresAt)
}

func (w *Worker) rescheduleLookup(ctx context.Context, claim *claimedTask, code string) error {
	to := "pending_lookup"
	if claim.ClaimedStatus == "verify_leased" {
		to = "verify_wait"
	}
	return w.transition(ctx, claim, to, "", code, claim.ResultUnknown,
		nextReadAt(claim.TaskNo, claim.ObservationAttemptCount, claim.LeaseExpiresAt))
}

func (w *Worker) beginManualCleanup(ctx context.Context, claim *claimedTask, code string) error {
	// The password is durably revoked first. The encrypted username handle is
	// retained because an operator may need the exact value to reconcile an
	// unknown write without exposing it through task storage or diagnostics.
	return w.transition(ctx, claim, "manual_cleanup_pending", "", code, claim.ResultUnknown, time.Time{})
}

func (w *Worker) transition(
	ctx context.Context,
	claim *claimedTask,
	to string,
	externalRef string,
	failureCode string,
	resultUnknown bool,
	availableAt time.Time,
) error {
	return w.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := principalDatabaseTime(tx)
		if err != nil {
			return err
		}
		if availableAt.IsZero() {
			availableAt = now
		}
		return updateClaimedTask(ctx, tx, claim, availableAt, to, externalRef, failureCode, resultUnknown, "", now)
	})
}

func (w *Worker) finish(ctx context.Context, claim *claimedTask, to string, code string) error {
	return w.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		now, err := principalDatabaseTime(tx)
		if err != nil {
			return err
		}
		result, err := tx.Exec(`
UPDATE plugin_idc_resource_principal_task
SET status_code = ?, failure_code = ?, lease_token_digest = '', lease_expires_at = NULL,
    finished_at = ?, version = version + 1, updated_at = ?
WHERE tenant_id = ? AND task_no = ? AND status_code = ?
  AND lease_token_digest = ? AND version = ?`,
			to, code, now, now, idccap.DefaultTenantID, claim.TaskNo,
			claim.ClaimedStatus, claim.LeaseDigest, claim.Version,
		)
		if err != nil {
			return gerror.Wrap(err, "finish resource principal task")
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			return gerror.New("resource principal task terminal lease was lost")
		}
		return nil
	})
}

func updateClaimedTask(
	ctx context.Context,
	tx gdb.TX,
	claim *claimedTask,
	availableAt time.Time,
	to string,
	externalRef string,
	failureCode string,
	resultUnknown bool,
	evidenceDigest string,
	updatedAt time.Time,
) error {
	result, err := tx.Exec(`
UPDATE plugin_idc_resource_principal_task
SET status_code = ?,
    expected_external_principal_ref = CASE WHEN ? <> '' THEN ? ELSE expected_external_principal_ref END,
    result_unknown = ?, failure_code = ?,
    evidence_digest = CASE WHEN ? <> '' THEN ? ELSE evidence_digest END,
    available_at = ?, lease_token_digest = '', lease_expires_at = NULL,
    version = version + 1, updated_at = ?
WHERE tenant_id = ? AND task_no = ? AND status_code = ?
  AND lease_token_digest = ? AND version = ?`,
		to, externalRef, externalRef, resultUnknown, failureCode,
		evidenceDigest, evidenceDigest, availableAt, updatedAt,
		idccap.DefaultTenantID, claim.TaskNo, claim.ClaimedStatus, claim.LeaseDigest, claim.Version,
	)
	if err != nil {
		return gerror.Wrap(err, "transition resource principal task")
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		return gerror.New("resource principal task transition lease was lost")
	}
	return nil
}

func principalDatabaseTime(tx gdb.TX) (time.Time, error) {
	value, err := tx.GetValue("SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return time.Time{}, gerror.Wrap(err, "read resource principal database time")
	}
	now := value.Time()
	if now.IsZero() {
		return time.Time{}, gerror.New("resource principal database time is unavailable")
	}
	return now.UTC(), nil
}

func principalEvidenceDigest(taskNo, customerNo, channelNo, externalRef string) string {
	digest := sha256.Sum256([]byte(
		"resource-principal-evidence/v1\x00" + taskNo + "\x00" + customerNo + "\x00" + channelNo + "\x00" + externalRef,
	))
	return hex.EncodeToString(digest[:])
}

func principalMappingNo(customerNo, channelNo string) string {
	digest := sha256.Sum256([]byte("resource-principal-mapping/v1\x00" + channelNo + "\x00" + customerNo))
	return "prm_" + hex.EncodeToString(digest[:16])
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
