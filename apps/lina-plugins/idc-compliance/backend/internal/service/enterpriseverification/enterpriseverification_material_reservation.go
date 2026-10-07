// This file creates bounded, idempotent upload reservations before original
// enterprise evidence bytes enter the raw content endpoint.

package enterpriseverification

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const (
	// materialUploadStatusReserved allows one original upload for the reservation.
	materialUploadStatusReserved = "reserved"
	// materialUploadStatusCompleted binds the reservation to confirmed ciphertext.
	materialUploadStatusCompleted = "completed"
	// maximumReservationAttempts bounds random-number collision recovery.
	maximumReservationAttempts = 8
)

// ReserveMaterial allocates or replays one bounded server-side upload number.
func (s *serviceImpl) ReserveMaterial(
	ctx context.Context,
	input ReserveMaterialInput,
) (*MaterialReservation, error) {
	if ctx == nil || ctx.Err() != nil || !validBusinessNo(input.CaseNo, 64) ||
		!validateMaterialType(input.MaterialType) || !validIdempotencyKey(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	subject, err := s.identities.CurrentEnterprise(ctx)
	if err != nil || subject == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	caseRow, err := loadOwnedDraft(ctx, subject.CustomerNo, input.CaseNo)
	if err != nil || caseRow == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	idempotencyDigest := sha256Hex([]byte(input.IdempotencyKey))
	fingerprint := sha256Hex([]byte(
		input.CaseNo + "\x00" + string(input.MaterialType) + "\x00enterprise-material-reservation-v1",
	))
	if replay, loadErr := loadMaterialReservationReplay(ctx, input.CaseNo, idempotencyDigest); loadErr != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	} else if replay != nil {
		return validateMaterialReservationReplay(replay, input.MaterialType, fingerprint)
	}
	for attempt := 0; attempt < maximumReservationAttempts; attempt++ {
		entropy, randomErr := randomHex(s.random, 16)
		if randomErr != nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		reservation, reserveErr := s.reserveMaterialNumber(
			ctx, caseRow, input.MaterialType, "mat_"+entropy, idempotencyDigest, fingerprint,
		)
		if reserveErr == nil {
			return reservation, nil
		}
		if strings.Contains(reserveErr.Error(), "enterprise material reservation limit") {
			return nil, bizerr.NewCode(CodeConflict)
		}
		if bizerr.Is(reserveErr, CodeConflict) {
			return nil, reserveErr
		}
		if !strings.Contains(reserveErr.Error(), "ent_upload_") {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		if replay, loadErr := loadMaterialReservationReplay(ctx, input.CaseNo, idempotencyDigest); loadErr == nil && replay != nil {
			return validateMaterialReservationReplay(replay, input.MaterialType, fingerprint)
		}
	}
	return nil, bizerr.NewCode(CodeConflict)
}

func (s *serviceImpl) reserveMaterialNumber(
	ctx context.Context,
	caseRow *entity.VerificationCase,
	materialType MaterialType,
	materialNo string,
	idempotencyDigest string,
	fingerprint string,
) (*MaterialReservation, error) {
	var reservation *MaterialReservation
	err := dao.EnterpriseMaterialUpload.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var locked *entity.VerificationCase
		if lockErr := dao.VerificationCase.Ctx(ctx).Fields("id", "case_no", "status_code").
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
		replay, loadErr := loadMaterialReservationReplay(ctx, caseRow.CaseNo, idempotencyDigest)
		if loadErr != nil {
			return loadErr
		}
		if replay != nil {
			validated, validationErr := validateMaterialReservationReplay(replay, materialType, fingerprint)
			if validationErr != nil {
				return validationErr
			}
			reservation = validated
			return nil
		}
		count, countErr := dao.EnterpriseMaterialUpload.Ctx(ctx).Where(do.EnterpriseMaterialUpload{
			TenantId: idccap.DefaultTenantID, CaseNo: caseRow.CaseNo,
		}).Count()
		if countErr != nil {
			return countErr
		}
		if count >= maximumSelectedCount {
			return gerror.New("enterprise material reservation limit reached")
		}
		if _, insertErr := dao.EnterpriseMaterialUpload.Ctx(ctx).Data(do.EnterpriseMaterialUpload{
			TenantId: idccap.DefaultTenantID, MaterialNo: materialNo,
			CaseNo: caseRow.CaseNo, CaseSubjectTypeCode: enterpriseSubjectType,
			MaterialTypeCode: string(materialType), IdempotencyKeyDigest: idempotencyDigest,
			RequestFingerprint: fingerprint, StatusCode: materialUploadStatusReserved, Version: 1,
		}).Insert(); insertErr != nil {
			return insertErr
		}
		if auditErr := s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.enterprise-verification.material-reservation",
			IdempotencyKey:   materialNo,
			OperationCode:    "compliance.enterprise-verification.reserve-material",
			TargetType:       "enterprise_verification_material", TargetNo: materialNo,
			CustomerNo: caseRow.CustomerNo, ResultCode: "succeeded",
			ReasonCode: "customer_upload_preparation", RiskCode: "high",
			CorrelationNo:  caseRow.CaseNo,
			BeforeSnapshot: map[string]any{"reservation_status": "absent"},
			AfterSnapshot: map[string]any{
				"reservation_status": materialUploadStatusReserved,
				"material_type":      string(materialType),
			},
		}); auditErr != nil {
			return auditErr
		}
		reservation = &MaterialReservation{
			MaterialNo: materialNo, MaterialType: materialType,
			StatusCode: materialUploadStatusReserved,
		}
		return nil
	})
	return reservation, err
}

func loadMaterialReservationReplay(
	ctx context.Context,
	caseNo string,
	idempotencyDigest string,
) (*entity.EnterpriseMaterialUpload, error) {
	var row *entity.EnterpriseMaterialUpload
	err := dao.EnterpriseMaterialUpload.Ctx(ctx).Where(do.EnterpriseMaterialUpload{
		TenantId: idccap.DefaultTenantID, CaseNo: caseNo,
		IdempotencyKeyDigest: idempotencyDigest,
	}).Limit(1).Scan(&row)
	return row, err
}

func loadOwnedMaterialReservation(
	ctx context.Context,
	caseNo string,
	materialNo string,
) (*entity.EnterpriseMaterialUpload, error) {
	var row *entity.EnterpriseMaterialUpload
	err := dao.EnterpriseMaterialUpload.Ctx(ctx).Where(do.EnterpriseMaterialUpload{
		TenantId: idccap.DefaultTenantID, CaseNo: caseNo, MaterialNo: materialNo,
		CaseSubjectTypeCode: enterpriseSubjectType,
	}).Limit(1).Scan(&row)
	return row, err
}

func validateMaterialReservationReplay(
	row *entity.EnterpriseMaterialUpload,
	materialType MaterialType,
	fingerprint string,
) (*MaterialReservation, error) {
	if row == nil || row.MaterialTypeCode != string(materialType) ||
		row.RequestFingerprint != fingerprint ||
		(row.StatusCode != materialUploadStatusReserved && row.StatusCode != materialUploadStatusCompleted) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return &MaterialReservation{
		MaterialNo: row.MaterialNo, MaterialType: materialType,
		StatusCode: row.StatusCode, Reused: true,
	}, nil
}
