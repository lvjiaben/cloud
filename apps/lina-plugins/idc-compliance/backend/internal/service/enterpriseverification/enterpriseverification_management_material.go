// This file releases one private original only through a recent-login,
// target-bound, single-use proof and a mandatory per-view audit event.

package enterpriseverification

import (
	"context"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

// Clear wipes the bounded decrypted original after the HTTP response finishes.
func (c *MaterialContent) Clear() {
	if c == nil {
		return
	}
	clearBytes(c.Content)
	c.Content = nil
}

// IssueMaterialAccess authorizes the exact material before requesting a recent-login proof.
func (s *managementServiceImpl) IssueMaterialAccess(
	ctx context.Context,
	caseNo string,
	materialNo string,
) (*MaterialAccessContext, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if _, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: permissionEnterpriseMaterialRead,
		TargetNo:   managementTarget(caseNo, materialNo),
	}); err != nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	caseRow, material, err := loadAccessMaterial(ctx, caseNo, materialNo, false)
	if err != nil || caseRow == nil || material == nil ||
		!materialAccessibleAt(material, s.now()) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	generation, blocked, err := s.privacyGate.CurrentSensitiveAccessGeneration(ctx)
	if err != nil || blocked || generation <= 0 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	binding := sensitiveMaterialBinding(caseRow, material, generation)
	issued, err := s.sensitiveAccess.IssueSensitiveAccess(ctx, binding)
	if err != nil || issued == nil || issued.Token == "" || issued.ExpiresAt.IsZero() {
		if err != nil {
			return nil, err
		}
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &MaterialAccessContext{
		AccessContext: issued.Token, ExpiresAt: issued.ExpiresAt.UTC(),
	}, nil
}

// OpenMaterial consumes one proof and releases bytes only after a durable per-view audit.
func (s *managementServiceImpl) OpenMaterial(
	ctx context.Context,
	caseNo string,
	materialNo string,
	accessContext string,
) (*MaterialContent, error) {
	if ctx == nil || ctx.Err() != nil || accessContext == "" ||
		accessContext != strings.TrimSpace(accessContext) || len(accessContext) > 16*1024 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	actor, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: permissionEnterpriseMaterialRead,
		TargetNo:   managementTarget(caseNo, materialNo),
	})
	if err != nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	caseRow, material, err := loadAccessMaterial(ctx, caseNo, materialNo, false)
	if err != nil || caseRow == nil || material == nil ||
		!materialAccessibleAt(material, s.now()) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	generation, blocked, err := s.privacyGate.CurrentSensitiveAccessGeneration(ctx)
	if err != nil || blocked || generation <= 0 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	binding := sensitiveMaterialBinding(caseRow, material, generation)
	proof, err := s.sensitiveAccess.ConsumeSensitiveAccess(ctx, accessContext, binding)
	if err != nil || proof == nil || proof.ActorUserID != actor {
		if err != nil {
			return nil, err
		}
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	content, err := s.decryptMaterial(ctx, material)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	result := &MaterialContent{
		MediaType:    material.MediaType,
		MaterialType: MaterialType(material.MaterialTypeCode), Content: content,
	}
	accessDigest := sha256Hex([]byte(accessContext))
	err = dao.VerificationCase.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		lockedCase, lockedMaterial, lockErr := loadAccessMaterial(ctx, caseNo, materialNo, true)
		if lockErr != nil || lockedCase == nil || lockedMaterial == nil ||
			sensitiveMaterialBinding(lockedCase, lockedMaterial, generation) != binding ||
			!materialAccessibleAt(lockedMaterial, s.now()) {
			return gerror.New("enterprise verification material changed before release")
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.enterprise-material.view",
			IdempotencyKey:   accessDigest,
			OperationCode:    "compliance.enterprise-material.view",
			TargetType:       "enterprise_verification_material", TargetNo: materialNo,
			CustomerNo: lockedCase.CustomerNo, ResultCode: "succeeded",
			ReasonCode: "private_original_released", RiskCode: "critical",
			RequiredPermission: permissionEnterpriseMaterialRead,
			CorrelationNo:      caseNo,
			BeforeSnapshot: map[string]any{
				"case_status":     lockedCase.StatusCode,
				"material_type":   lockedMaterial.MaterialTypeCode,
				"material_status": lockedMaterial.StatusCode,
			},
			AfterSnapshot: map[string]any{
				"access": "one_time_released", "media_type": lockedMaterial.MediaType,
				"byte_size": lockedMaterial.ByteSize,
			},
		})
	})
	if err != nil {
		result.Clear()
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return result, nil
}

// materialAccessibleAt enforces availability, retention expiry, and legal-hold consistency.
func materialAccessibleAt(row *entity.EnterpriseMaterial, now time.Time) bool {
	if row == nil || now.IsZero() {
		return false
	}
	if row.StatusCode == materialStatusLegalHold && row.LegalHold {
		return true
	}
	return row.StatusCode == materialStatusAvailable && !row.LegalHold &&
		(row.RetentionUntil == nil || row.RetentionUntil.After(now.UTC()))
}

// loadAccessMaterial reads and optionally locks one exact case/material binding.
func loadAccessMaterial(
	ctx context.Context,
	caseNo string,
	materialNo string,
	lock bool,
) (*entity.VerificationCase, *entity.EnterpriseMaterial, error) {
	if !validBusinessNo(caseNo, 64) || !validBusinessNo(materialNo, 64) {
		return nil, nil, nil
	}
	caseModel := dao.VerificationCase.Ctx(ctx).Where(do.VerificationCase{
		TenantId: idccap.DefaultTenantID, CaseNo: caseNo,
		SubjectTypeCode: enterpriseSubjectType,
	}).Where("submitted_at IS NOT NULL")
	if lock {
		caseModel = caseModel.LockUpdate()
	}
	var caseRow *entity.VerificationCase
	if err := caseModel.Limit(1).Scan(&caseRow); err != nil || caseRow == nil {
		return nil, nil, err
	}
	materialModel := dao.EnterpriseMaterial.Ctx(ctx).Where(do.EnterpriseMaterial{
		TenantId: idccap.DefaultTenantID, CaseNo: caseNo, MaterialNo: materialNo,
		CaseSubjectTypeCode: enterpriseSubjectType,
	}).WhereIn("status_code", []string{materialStatusAvailable, materialStatusLegalHold})
	if lock {
		materialModel = materialModel.LockUpdate()
	}
	var material *entity.EnterpriseMaterial
	if err := materialModel.Limit(1).Scan(&material); err != nil || material == nil {
		return nil, nil, err
	}
	if !validateMaterialType(MaterialType(material.MaterialTypeCode)) ||
		material.EncryptionStateCode != materialEncryptionState ||
		material.EncryptionKeyNo == "" || material.PrivateObjectRef == "" ||
		material.ContentSha256 == "" || material.ByteSize <= 0 ||
		material.ByteSize > maximumMaterialBytes || material.StoredByteSize <= material.ByteSize {
		return nil, nil, gerror.New("enterprise verification material metadata is invalid")
	}
	return caseRow, material, nil
}

// sensitiveMaterialBinding creates a domain-separated proof target without raw values.
func sensitiveMaterialBinding(
	caseRow *entity.VerificationCase,
	material *entity.EnterpriseMaterial,
	privacyGeneration int64,
) managementcap.SensitiveAccessBinding {
	canonical := strings.Join([]string{
		"idc/enterprise-material-access/v1", caseRow.CaseNo,
		material.MaterialNo, material.ContentSha256, material.StatusCode,
		strconv.FormatInt(privacyGeneration, 10),
	}, "\x00")
	return managementcap.SensitiveAccessBinding{
		TargetDigest: managementcap.DigestSensitiveAccessTarget([]byte(canonical)),
	}
}

// decryptMaterial authenticates storage metadata and returns one bounded plaintext buffer.
func (s *managementServiceImpl) decryptMaterial(
	ctx context.Context,
	row *entity.EnterpriseMaterial,
) ([]byte, error) {
	result, err := s.storage.Get(ctx, storagecap.GetInput{Path: row.PrivateObjectRef})
	if err != nil || result == nil || !result.Found || result.Object == nil || result.Body == nil {
		return nil, gerror.New("enterprise verification material is unavailable")
	}
	if result.Object.Visibility != storagecap.VisibilityPrivate ||
		result.Object.Size != row.StoredByteSize ||
		result.Object.ContentType != "application/octet-stream" {
		if closeErr := result.Body.Close(); closeErr != nil {
			return nil, gerror.Wrap(closeErr, "close inconsistent enterprise material")
		}
		return nil, gerror.New("enterprise verification material metadata is inconsistent")
	}
	envelope, readErr := io.ReadAll(io.LimitReader(result.Body, row.StoredByteSize+1))
	closeErr := result.Body.Close()
	if readErr != nil || closeErr != nil || int64(len(envelope)) != row.StoredByteSize ||
		sha256Hex(envelope) != row.ContentSha256 {
		clearBytes(envelope)
		return nil, gerror.New("enterprise verification material integrity check failed")
	}
	defer clearBytes(envelope)
	var plaintext []byte
	err = accessEnterpriseKey(ctx, s.keyAccessor, row.EncryptionKeyNo, func(key []byte) error {
		var decryptErr error
		plaintext, decryptErr = decryptEnvelope(
			key, envelope, "material:"+row.MaterialNo, maximumMaterialBytes,
		)
		return decryptErr
	})
	if err != nil || int64(len(plaintext)) != row.ByteSize {
		clearBytes(plaintext)
		return nil, gerror.New("enterprise verification material decryption failed")
	}
	detected, err := detectMaterialMediaType(plaintext, row.MediaType)
	if err != nil || detected != row.MediaType {
		clearBytes(plaintext)
		return nil, gerror.New("enterprise verification material media type changed")
	}
	return plaintext, nil
}
