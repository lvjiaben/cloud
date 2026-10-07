// This file validates a complete enterprise package and atomically publishes
// its pending manual-review projection without persisting legal plaintext.

package enterpriseverification

import (
	"context"
	"io"
	"sort"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
)

// Submit validates one complete package and transitions it to pending review.
func (s *serviceImpl) Submit(
	ctx context.Context,
	input SubmitInput,
) (*Submission, error) {
	defer clearBytes(input.LegalName)
	defer clearBytes(input.RegistrationIdentifier)
	defer clearBytes(input.LegalRepresentativeName)
	defer clearBytes(input.LegalRepresentativeIdentityNumber)
	defer clearBytes(input.AgentName)
	defer clearBytes(input.AgentIdentityNumber)
	if ctx == nil || ctx.Err() != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	legal, err := validateLegalProfile(input)
	if err != nil {
		return nil, err
	}
	defer clearBytes(legal.legalName)
	defer clearBytes(legal.registrationIdentifier)
	defer clearBytes(legal.legalRepresentativeName)
	defer clearBytes(legal.legalRepresentativeIdentityNumber)
	defer clearBytes(legal.agentName)
	defer clearBytes(legal.agentIdentityNumber)
	materialNos, err := normalizeMaterialNos(input.MaterialNos)
	if err != nil {
		return nil, err
	}
	subject, err := s.identities.CurrentEnterprise(ctx)
	if err != nil || subject == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	caseRow, err := loadOwnedSubmissionCase(ctx, subject.CustomerNo, input.CaseNo)
	if err != nil || caseRow == nil || caseRow.EncryptionKeyNo == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	preparation, err := s.identities.PrepareEnterprise(
		ctx, caseRow.ConsentNo, caseRow.LegalChangeRequestNo,
	)
	if err != nil || preparation == nil || preparation.Subject.CustomerNo != caseRow.CustomerNo ||
		(preparation.ReviewCaseNo != "" && preparation.ReviewCaseNo != caseRow.CaseNo) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	materials, materialTypes, err := loadSelectedMaterials(
		ctx, caseRow, materialNos, input.ApplicantRole,
	)
	if err != nil {
		return nil, err
	}
	for _, material := range materials {
		if err = s.confirmPersistedMaterial(ctx, material); err != nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
	}
	snapshot := buildLegalSnapshot(input, legal, materialNos)
	defer clearBytes(snapshot)
	idempotencyDigest := sha256Hex([]byte(input.IdempotencyKey))
	var fingerprint string
	var envelope []byte
	err = s.withKey(ctx, *caseRow.EncryptionKeyNo, func(key []byte) error {
		var fingerprintErr error
		fingerprint, fingerprintErr = keyedFingerprint(key, "case-submission", snapshot)
		if fingerprintErr != nil {
			return fingerprintErr
		}
		var encryptionErr error
		envelope, encryptionErr = encryptEnvelope(key, snapshot, "snapshot:"+caseRow.CaseNo)
		return encryptionErr
	})
	if err != nil {
		clearBytes(envelope)
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	defer clearBytes(envelope)
	snapshotPath := legalSnapshotObjectPath(caseRow.CaseNo)
	snapshotDigest := sha256Hex(envelope)
	if caseRow.StatusCode == caseStatusPending {
		return s.replaySubmission(
			ctx, caseRow, preparation.Reverification, idempotencyDigest, fingerprint,
			snapshotPath, snapshotDigest, envelope,
		)
	}
	if err = s.putOrConfirmEnvelope(ctx, snapshotPath, envelope, snapshotDigest); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return s.commitSubmission(
		ctx, caseRow, preparation.Reverification, legal, input, materialTypes,
		idempotencyDigest, fingerprint, snapshotPath, snapshotDigest, int64(len(envelope)),
	)
}

func (s *serviceImpl) commitSubmission(
	ctx context.Context,
	caseRow *entity.VerificationCase,
	reverification bool,
	legal validatedLegalProfile,
	input SubmitInput,
	materialTypes []string,
	idempotencyDigest string,
	fingerprint string,
	snapshotPath string,
	snapshotDigest string,
	storedSize int64,
) (*Submission, error) {
	submittedAt := s.now().UTC().Truncate(time.Microsecond)
	result := &Submission{
		CaseNo: caseRow.CaseNo, StatusCode: caseStatusPending, Reverification: reverification,
	}
	err := dao.VerificationCase.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var locked *entity.VerificationCase
		if lockErr := dao.VerificationCase.Ctx(ctx).Where(do.VerificationCase{
			TenantId: idccap.DefaultTenantID, CaseNo: caseRow.CaseNo,
			CustomerNo: caseRow.CustomerNo, SubjectTypeCode: enterpriseSubjectType,
		}).LockUpdate().Limit(1).Scan(&locked); lockErr != nil {
			return lockErr
		}
		if locked == nil || locked.StatusCode != caseStatusDraft || locked.Version != caseRow.Version {
			return gerror.New("enterprise verification submission lost its case fence")
		}
		projection, projectionErr := s.identities.ProjectEnterprise(ctx, identitycap.EnterpriseProjectionInput{
			CustomerNo: caseRow.CustomerNo, CaseNo: caseRow.CaseNo,
			Status:               identitycap.StatusPending,
			LegalChangeRequestNo: caseRow.LegalChangeRequestNo,
		})
		if projectionErr != nil || projection == nil ||
			(!reverification && projection.Status != identitycap.StatusPending) ||
			(reverification && (!projection.PreservedCurrent || projection.Status != identitycap.StatusVerified)) {
			return gerror.New("enterprise identity pending projection is inconsistent")
		}
		path := snapshotPath
		digest := snapshotDigest
		updated, updateErr := dao.VerificationCase.Ctx(ctx).Where(do.VerificationCase{
			Id: locked.Id, TenantId: idccap.DefaultTenantID,
			StatusCode: caseStatusDraft, Version: locked.Version,
		}).Data(do.VerificationCase{
			StatusCode: caseStatusPending, LegalProfileSnapshotRef: &path,
			LegalProfileSha256: &digest, MaskedName: legal.maskedName,
			MaskedIdentityNo:               legal.maskedIdentifier,
			SubmissionIdempotencyKeyDigest: idempotencyDigest,
			SubmissionFingerprint:          fingerprint,
			SnapshotStoredByteSize:         storedSize,
			ApplicantRoleCode:              string(input.ApplicantRole),
			SubmittedAt:                    &submittedAt, Version: locked.Version + 1,
		}).Update()
		if updateErr != nil {
			return updateErr
		}
		if updateErr = requireOneRow(updated, "submit enterprise verification case"); updateErr != nil {
			return updateErr
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.enterprise-verification.submission",
			IdempotencyKey:   caseRow.CaseNo,
			OperationCode:    "compliance.enterprise-verification.submit",
			TargetType:       "enterprise_verification_case", TargetNo: caseRow.CaseNo,
			CustomerNo: caseRow.CustomerNo, ResultCode: "succeeded",
			ReasonCode: "pending_manual_review", RiskCode: "high",
			CorrelationNo:  caseRow.LegalChangeRequestNo,
			BeforeSnapshot: map[string]any{"case_status": caseStatusDraft},
			AfterSnapshot: map[string]any{
				"case_status": caseStatusPending, "reverification": reverification,
				"applicant_role": string(input.ApplicantRole), "material_types": materialTypes,
			},
		})
	})
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return result, nil
}

func (s *serviceImpl) replaySubmission(
	ctx context.Context,
	caseRow *entity.VerificationCase,
	reverification bool,
	idempotencyDigest string,
	fingerprint string,
	snapshotPath string,
	snapshotDigest string,
	envelope []byte,
) (*Submission, error) {
	if caseRow.SubmissionIdempotencyKeyDigest != idempotencyDigest ||
		caseRow.SubmissionFingerprint != fingerprint || caseRow.LegalProfileSnapshotRef == nil ||
		*caseRow.LegalProfileSnapshotRef != snapshotPath || caseRow.LegalProfileSha256 == nil ||
		*caseRow.LegalProfileSha256 != snapshotDigest ||
		caseRow.SnapshotStoredByteSize != int64(len(envelope)) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	if err := s.putOrConfirmEnvelope(ctx, snapshotPath, envelope, snapshotDigest); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &Submission{
		CaseNo: caseRow.CaseNo, StatusCode: caseStatusPending,
		Reverification: reverification, Reused: true,
	}, nil
}

func loadOwnedSubmissionCase(
	ctx context.Context,
	customerNo string,
	caseNo string,
) (*entity.VerificationCase, error) {
	var row *entity.VerificationCase
	err := dao.VerificationCase.Ctx(ctx).Where(do.VerificationCase{
		TenantId: idccap.DefaultTenantID, CaseNo: caseNo, CustomerNo: customerNo,
		SubjectTypeCode: enterpriseSubjectType,
	}).WhereIn("status_code", []string{caseStatusDraft, caseStatusPending}).Limit(1).Scan(&row)
	return row, err
}

func loadSelectedMaterials(
	ctx context.Context,
	caseRow *entity.VerificationCase,
	materialNos []string,
	role ApplicantRole,
) ([]*entity.EnterpriseMaterial, []string, error) {
	var rows []*entity.EnterpriseMaterial
	err := dao.EnterpriseMaterial.Ctx(ctx).Fields(
		"material_no", "case_no", "material_type_code", "private_object_ref", "content_sha256",
		"media_type", "byte_size", "stored_byte_size", "encryption_state_code", "status_code",
		"encryption_key_no",
	).Where(do.EnterpriseMaterial{
		TenantId: idccap.DefaultTenantID, CaseNo: caseRow.CaseNo,
		CaseSubjectTypeCode: enterpriseSubjectType, StatusCode: materialStatusAvailable,
	}).WhereIn("material_no", materialNos).Scan(&rows)
	if err != nil {
		return nil, nil, bizerr.NewCode(CodeUnavailable)
	}
	if len(rows) != len(materialNos) {
		return nil, nil, bizerr.NewCode(CodeInvalid)
	}
	typeSet := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if row == nil || row.EncryptionKeyNo != *caseRow.EncryptionKeyNo ||
			row.EncryptionStateCode != materialEncryptionState {
			return nil, nil, bizerr.NewCode(CodeInvalid)
		}
		if _, duplicate := typeSet[row.MaterialTypeCode]; duplicate {
			return nil, nil, bizerr.NewCode(CodeInvalid)
		}
		typeSet[row.MaterialTypeCode] = struct{}{}
	}
	if !containsMaterialType(typeSet, MaterialBusinessLicense) ||
		!containsMaterialType(typeSet, MaterialLegalRepresentativeID) ||
		(role == ApplicantLegalRepresentative &&
			(containsMaterialType(typeSet, MaterialAgentID) ||
				containsMaterialType(typeSet, MaterialAuthorizationLetter))) ||
		(role == ApplicantAuthorizedAgent &&
			(!containsMaterialType(typeSet, MaterialAgentID) ||
				!containsMaterialType(typeSet, MaterialAuthorizationLetter))) {
		return nil, nil, bizerr.NewCode(CodeInvalid)
	}
	types := make([]string, 0, len(typeSet))
	for materialType := range typeSet {
		types = append(types, materialType)
	}
	sort.Strings(types)
	return rows, types, nil
}

func normalizeMaterialNos(values []string) ([]string, error) {
	if len(values) == 0 || len(values) > maximumSelectedCount {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	result := append([]string(nil), values...)
	sort.Strings(result)
	for index, value := range result {
		if !validBusinessNo(value, 64) || (index > 0 && result[index-1] == value) {
			return nil, bizerr.NewCode(CodeInvalid)
		}
	}
	return result, nil
}

func containsMaterialType(values map[string]struct{}, materialType MaterialType) bool {
	_, found := values[string(materialType)]
	return found
}

func (s *serviceImpl) confirmPersistedMaterial(
	ctx context.Context,
	row *entity.EnterpriseMaterial,
) error {
	result, err := s.storage.Get(ctx, storagecap.GetInput{Path: row.PrivateObjectRef})
	if err != nil || result == nil || !result.Found || result.Object == nil || result.Body == nil {
		return gerror.New("enterprise verification material is unavailable")
	}
	if result.Object.Visibility != storagecap.VisibilityPrivate ||
		result.Object.Size != row.StoredByteSize || result.Object.ContentType != "application/octet-stream" {
		if closeErr := result.Body.Close(); closeErr != nil {
			return gerror.Wrap(closeErr, "close inconsistent enterprise verification material")
		}
		return gerror.New("enterprise verification material metadata is inconsistent")
	}
	content, readErr := io.ReadAll(io.LimitReader(result.Body, row.StoredByteSize+1))
	closeErr := result.Body.Close()
	defer clearBytes(content)
	if readErr != nil || closeErr != nil || int64(len(content)) != row.StoredByteSize ||
		sha256Hex(content) != row.ContentSha256 {
		return gerror.New("enterprise verification material integrity check failed")
	}
	return nil
}

func legalSnapshotObjectPath(caseNo string) string {
	return "enterprise/cases/" + caseNo + "/legal-profile.bin"
}
