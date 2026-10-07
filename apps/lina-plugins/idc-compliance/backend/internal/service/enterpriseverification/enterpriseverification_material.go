// This file performs bounded server-mediated validation, encryption, private
// storage confirmation, and material metadata persistence.

package enterpriseverification

import (
	"bytes"
	"context"
	"crypto/subtle"
	"io"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const (
	// materialStatusQuarantined means metadata exists but storage is not committed.
	materialStatusQuarantined = "quarantined"
	// materialStatusAvailable means ciphertext was read back and verified.
	materialStatusAvailable = "available"
	// materialEncryptionState is the only accepted storage state in this slice.
	materialEncryptionState = "encrypted"
)

// UploadMaterial encrypts, privately stores, confirms, and safely replays one original.
func (s *serviceImpl) UploadMaterial(
	ctx context.Context,
	input UploadMaterialInput,
) (*Material, error) {
	if ctx == nil || ctx.Err() != nil || input.Body == nil ||
		!validBusinessNo(input.CaseNo, 64) || !validBusinessNo(input.MaterialNo, 64) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	retentionUntil, err := s.privacyGate.RequireSensitiveCollection(
		ctx, "enterprise_verification", "enterprise_verification_material",
	)
	if err != nil || retentionUntil.IsZero() {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	subject, err := s.identities.CurrentEnterprise(ctx)
	if err != nil || subject == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	caseRow, err := loadOwnedDraft(ctx, subject.CustomerNo, input.CaseNo)
	if err != nil || caseRow == nil || caseRow.EncryptionKeyNo == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	reservation, err := loadOwnedMaterialReservation(ctx, input.CaseNo, input.MaterialNo)
	if err != nil || reservation == nil ||
		(reservation.StatusCode != materialUploadStatusReserved &&
			reservation.StatusCode != materialUploadStatusCompleted) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	body, err := io.ReadAll(io.LimitReader(input.Body, maximumMaterialBytes+1))
	if err != nil {
		clearBytes(body)
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	defer clearBytes(body)
	mediaType, err := detectMaterialMediaType(body, input.DeclaredMediaType)
	if err != nil {
		return nil, err
	}
	var fingerprint string
	err = s.withKey(ctx, *caseRow.EncryptionKeyNo, func(key []byte) error {
		var fingerprintErr error
		fingerprint, fingerprintErr = keyedFingerprint(
			key, "material-upload", []byte(reservation.MaterialTypeCode), []byte(mediaType), body,
		)
		return fingerprintErr
	})
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	existing, err := loadMaterialByNumber(ctx, caseRow.CaseNo, reservation.MaterialNo)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if existing != nil {
		return s.resumeMaterial(ctx, caseRow, reservation, existing, mediaType, body, fingerprint)
	}
	if reservation.StatusCode == materialUploadStatusCompleted {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return s.createMaterial(ctx, caseRow, reservation, mediaType, body, fingerprint, retentionUntil)
}

func (s *serviceImpl) createMaterial(
	ctx context.Context,
	caseRow *entity.VerificationCase,
	reservation *entity.EnterpriseMaterialUpload,
	mediaType string,
	body []byte,
	fingerprint string,
	retentionUntil time.Time,
) (*Material, error) {
	path := materialObjectPath(caseRow.CaseNo, reservation.MaterialNo)
	var envelope []byte
	err := s.withKey(ctx, *caseRow.EncryptionKeyNo, func(key []byte) error {
		var encryptionErr error
		envelope, encryptionErr = encryptEnvelope(key, body, "material:"+reservation.MaterialNo)
		return encryptionErr
	})
	if err != nil {
		clearBytes(envelope)
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	defer clearBytes(envelope)
	digest := sha256Hex(envelope)
	sequence, err := s.persistQuarantinedMaterial(
		ctx, caseRow, reservation, mediaType, path, int64(len(body)), envelope, digest, fingerprint,
		retentionUntil,
	)
	if err != nil {
		replay, loadErr := loadMaterialByNumber(ctx, caseRow.CaseNo, reservation.MaterialNo)
		if loadErr == nil && replay != nil {
			return s.resumeMaterial(ctx, caseRow, reservation, replay, mediaType, body, fingerprint)
		}
		return nil, bizerr.NewCode(CodeConflict)
	}
	if err = s.putOrConfirmEnvelope(ctx, path, envelope, digest); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	row := &entity.EnterpriseMaterial{
		MaterialNo: reservation.MaterialNo, CaseNo: caseRow.CaseNo,
		MaterialTypeCode: reservation.MaterialTypeCode,
		SequenceNo:       sequence, MediaType: mediaType, ByteSize: int64(len(body)),
		StoredByteSize: int64(len(envelope)), ContentSha256: digest,
		PrivateObjectRef: path, EncryptionKeyNo: *caseRow.EncryptionKeyNo,
		StatusCode: materialStatusQuarantined, RequestFingerprint: fingerprint,
	}
	return s.finalizeMaterial(ctx, caseRow, row, false)
}

func (s *serviceImpl) persistQuarantinedMaterial(
	ctx context.Context,
	caseRow *entity.VerificationCase,
	reservation *entity.EnterpriseMaterialUpload,
	mediaType string,
	path string,
	byteSize int64,
	envelope []byte,
	digest string,
	fingerprint string,
	retentionUntil time.Time,
) (int, error) {
	sequence := 0
	err := dao.EnterpriseMaterial.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var locked *entity.VerificationCase
		if lockErr := dao.VerificationCase.Ctx(ctx).Fields("id", "case_no", "status_code", "version").
			Where(do.VerificationCase{
				TenantId: idccap.DefaultTenantID, CaseNo: caseRow.CaseNo,
				CustomerNo: caseRow.CustomerNo, SubjectTypeCode: enterpriseSubjectType,
				StatusCode: caseStatusDraft,
			}).LockUpdate().Limit(1).Scan(&locked); lockErr != nil {
			return lockErr
		}
		if locked == nil {
			return gerror.New("enterprise verification draft is no longer writable")
		}
		var latest *entity.EnterpriseMaterial
		if loadErr := dao.EnterpriseMaterial.Ctx(ctx).Fields("sequence_no").
			Where(do.EnterpriseMaterial{
				TenantId: idccap.DefaultTenantID, CaseNo: caseRow.CaseNo,
			}).OrderDesc("sequence_no").Limit(1).Scan(&latest); loadErr != nil {
			return loadErr
		}
		sequence = 1
		if latest != nil {
			sequence = latest.SequenceNo + 1
		}
		_, insertErr := dao.EnterpriseMaterial.Ctx(ctx).Data(do.EnterpriseMaterial{
			TenantId: idccap.DefaultTenantID, MaterialNo: reservation.MaterialNo,
			CaseNo: caseRow.CaseNo, CaseSubjectTypeCode: enterpriseSubjectType,
			MaterialTypeCode: reservation.MaterialTypeCode, SequenceNo: sequence,
			PrivateObjectRef: path, ContentSha256: digest, MediaType: mediaType,
			ByteSize: byteSize, EncryptionStateCode: materialEncryptionState,
			StatusCode: materialStatusQuarantined, EncryptionKeyNo: *caseRow.EncryptionKeyNo,
			StoredByteSize: len(envelope), IdempotencyKeyDigest: reservation.IdempotencyKeyDigest,
			RequestFingerprint: fingerprint,
		}).Insert()
		if insertErr == nil {
			_, insertErr = s.database.Exec(ctx, `UPDATE plugin_idc_compliance_enterprise_material SET retention_until = ? WHERE tenant_id = ? AND material_no = ?`, retentionUntil, idccap.DefaultTenantID, reservation.MaterialNo)
		}
		return insertErr
	})
	return sequence, err
}

func (s *serviceImpl) resumeMaterial(
	ctx context.Context,
	caseRow *entity.VerificationCase,
	reservation *entity.EnterpriseMaterialUpload,
	row *entity.EnterpriseMaterial,
	mediaType string,
	body []byte,
	fingerprint string,
) (*Material, error) {
	if reservation == nil || row.MaterialNo != reservation.MaterialNo ||
		row.IdempotencyKeyDigest != reservation.IdempotencyKeyDigest ||
		row.RequestFingerprint != fingerprint || row.MaterialTypeCode != reservation.MaterialTypeCode ||
		row.MediaType != mediaType || row.ByteSize != int64(len(body)) ||
		row.EncryptionKeyNo != *caseRow.EncryptionKeyNo ||
		(row.StatusCode != materialStatusQuarantined && row.StatusCode != materialStatusAvailable) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	var envelope []byte
	err := s.withKey(ctx, row.EncryptionKeyNo, func(key []byte) error {
		var encryptionErr error
		envelope, encryptionErr = encryptEnvelope(key, body, "material:"+row.MaterialNo)
		return encryptionErr
	})
	if err != nil {
		clearBytes(envelope)
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	defer clearBytes(envelope)
	if int64(len(envelope)) != row.StoredByteSize || sha256Hex(envelope) != row.ContentSha256 ||
		row.PrivateObjectRef != materialObjectPath(caseRow.CaseNo, row.MaterialNo) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	if err = s.putOrConfirmEnvelope(ctx, row.PrivateObjectRef, envelope, row.ContentSha256); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return s.finalizeMaterial(ctx, caseRow, row, true)
}

func (s *serviceImpl) putOrConfirmEnvelope(
	ctx context.Context,
	path string,
	envelope []byte,
	digest string,
) error {
	found, err := s.confirmEnvelope(ctx, path, envelope, digest)
	if err != nil {
		return err
	}
	if found {
		return nil
	}
	if _, err = s.storage.Put(ctx, storagecap.PutInput{
		Path: path, Body: bytes.NewReader(envelope), Size: int64(len(envelope)),
		ContentType: "application/octet-stream", Overwrite: false,
	}); err != nil {
		found, confirmErr := s.confirmEnvelope(ctx, path, envelope, digest)
		if confirmErr != nil {
			return confirmErr
		}
		if found {
			return nil
		}
		return err
	}
	found, err = s.confirmEnvelope(ctx, path, envelope, digest)
	if err != nil || !found {
		return gerror.New("enterprise material private object confirmation failed")
	}
	return nil
}

func (s *serviceImpl) confirmEnvelope(
	ctx context.Context,
	path string,
	expected []byte,
	digest string,
) (bool, error) {
	result, err := s.storage.Get(ctx, storagecap.GetInput{Path: path})
	if err != nil {
		return false, err
	}
	if result == nil || !result.Found {
		return false, nil
	}
	if result.Object == nil || result.Body == nil || result.Object.Visibility != storagecap.VisibilityPrivate ||
		result.Object.Size != int64(len(expected)) || result.Object.ContentType != "application/octet-stream" {
		if result.Body != nil {
			if closeErr := result.Body.Close(); closeErr != nil {
				return false, gerror.Wrap(closeErr, "close inconsistent enterprise material object")
			}
		}
		return false, gerror.New("enterprise material private object metadata is inconsistent")
	}
	actual, readErr := io.ReadAll(io.LimitReader(result.Body, int64(len(expected))+1))
	closeErr := result.Body.Close()
	defer clearBytes(actual)
	if readErr != nil || closeErr != nil || len(actual) != len(expected) ||
		subtle.ConstantTimeCompare(actual, expected) != 1 || sha256Hex(actual) != digest {
		return false, gerror.New("enterprise material private object integrity check failed")
	}
	return true, nil
}

func (s *serviceImpl) finalizeMaterial(
	ctx context.Context,
	caseRow *entity.VerificationCase,
	row *entity.EnterpriseMaterial,
	replayed bool,
) (*Material, error) {
	var confirmed *entity.EnterpriseMaterial
	err := dao.EnterpriseMaterial.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var lockedMaterial *entity.EnterpriseMaterial
		if lockErr := dao.EnterpriseMaterial.Ctx(ctx).Where(do.EnterpriseMaterial{
			TenantId: idccap.DefaultTenantID, MaterialNo: row.MaterialNo, CaseNo: caseRow.CaseNo,
		}).LockUpdate().Limit(1).Scan(&lockedMaterial); lockErr != nil {
			return lockErr
		}
		var lockedReservation *entity.EnterpriseMaterialUpload
		if lockErr := dao.EnterpriseMaterialUpload.Ctx(ctx).Where(do.EnterpriseMaterialUpload{
			TenantId: idccap.DefaultTenantID, MaterialNo: row.MaterialNo, CaseNo: caseRow.CaseNo,
			CaseSubjectTypeCode: enterpriseSubjectType,
		}).LockUpdate().Limit(1).Scan(&lockedReservation); lockErr != nil {
			return lockErr
		}
		if lockedMaterial == nil || lockedReservation == nil ||
			lockedMaterial.MaterialTypeCode != lockedReservation.MaterialTypeCode ||
			lockedMaterial.IdempotencyKeyDigest != lockedReservation.IdempotencyKeyDigest {
			return gerror.New("enterprise verification material reservation is inconsistent")
		}
		if lockedMaterial.StatusCode == materialStatusAvailable &&
			lockedReservation.StatusCode == materialUploadStatusCompleted {
			confirmed = lockedMaterial
			replayed = true
			return nil
		}
		if lockedMaterial.StatusCode != materialStatusQuarantined ||
			lockedReservation.StatusCode != materialUploadStatusReserved {
			return gerror.New("enterprise verification material state is inconsistent")
		}
		updated, updateErr := dao.EnterpriseMaterial.Ctx(ctx).Where(do.EnterpriseMaterial{
			TenantId: idccap.DefaultTenantID, MaterialNo: row.MaterialNo,
			CaseNo: caseRow.CaseNo, StatusCode: materialStatusQuarantined,
		}).Data(do.EnterpriseMaterial{StatusCode: materialStatusAvailable}).Update()
		if updateErr != nil {
			return updateErr
		}
		if updateErr = requireOneRow(updated, "confirm enterprise verification material"); updateErr != nil {
			return updateErr
		}
		updated, updateErr = dao.EnterpriseMaterialUpload.Ctx(ctx).Where(do.EnterpriseMaterialUpload{
			TenantId: idccap.DefaultTenantID, MaterialNo: row.MaterialNo,
			CaseNo: caseRow.CaseNo, StatusCode: materialUploadStatusReserved,
		}).Data(do.EnterpriseMaterialUpload{
			StatusCode: materialUploadStatusCompleted,
			Version:    lockedReservation.Version + 1,
		}).Update()
		if updateErr != nil {
			return updateErr
		}
		if updateErr = requireOneRow(updated, "complete enterprise material reservation"); updateErr != nil {
			return updateErr
		}
		if auditErr := s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.enterprise-verification.material",
			IdempotencyKey:   row.MaterialNo,
			OperationCode:    "compliance.enterprise-verification.upload-material",
			TargetType:       "enterprise_verification_material", TargetNo: row.MaterialNo,
			CustomerNo: caseRow.CustomerNo, ResultCode: "succeeded",
			ReasonCode: "encrypted_private_upload", RiskCode: "high",
			CorrelationNo:  caseRow.CaseNo,
			BeforeSnapshot: map[string]any{"material_status": materialStatusQuarantined},
			AfterSnapshot: map[string]any{
				"material_status": materialStatusAvailable,
				"material_type":   row.MaterialTypeCode, "media_type": row.MediaType,
				"byte_size": row.ByteSize,
			},
		}); auditErr != nil {
			return auditErr
		}
		// GoFrame auto-maintains updated_at; this local projection only records
		// the state committed by the transaction.
		confirmed = lockedMaterial
		confirmed.StatusCode = materialStatusAvailable
		return nil
	})
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return materialProjection(confirmed, replayed), nil
}

func loadOwnedDraft(
	ctx context.Context,
	customerNo string,
	caseNo string,
) (*entity.VerificationCase, error) {
	var row *entity.VerificationCase
	err := dao.VerificationCase.Ctx(ctx).Fields(
		"case_no", "customer_no", "subject_type_code", "status_code", "consent_no",
		"legal_change_request_no", "encryption_key_no", "version",
	).Where(do.VerificationCase{
		TenantId: idccap.DefaultTenantID, CaseNo: caseNo, CustomerNo: customerNo,
		SubjectTypeCode: enterpriseSubjectType, StatusCode: caseStatusDraft,
	}).Limit(1).Scan(&row)
	return row, err
}

func loadMaterialByNumber(
	ctx context.Context,
	caseNo string,
	materialNo string,
) (*entity.EnterpriseMaterial, error) {
	var row *entity.EnterpriseMaterial
	err := dao.EnterpriseMaterial.Ctx(ctx).Where(do.EnterpriseMaterial{
		TenantId: idccap.DefaultTenantID, CaseNo: caseNo, MaterialNo: materialNo,
	}).Limit(1).Scan(&row)
	return row, err
}

func materialProjection(row *entity.EnterpriseMaterial, reused bool) *Material {
	return &Material{
		MaterialNo: row.MaterialNo, MaterialType: MaterialType(row.MaterialTypeCode),
		SequenceNo: row.SequenceNo, MediaType: row.MediaType, ByteSize: row.ByteSize,
		StatusCode: materialStatusAvailable, Reused: reused,
	}
}

func materialObjectPath(caseNo, materialNo string) string {
	return "enterprise/cases/" + caseNo + "/materials/" + materialNo + ".bin"
}
