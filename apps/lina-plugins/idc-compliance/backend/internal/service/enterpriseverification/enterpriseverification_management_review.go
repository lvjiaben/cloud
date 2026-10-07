// This file owns optimistic, audited enterprise review state transitions.

package enterpriseverification

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

const (
	caseStatusVerifying      = "verifying"
	caseStatusReviewRequired = "review_required"
	caseStatusVerified       = "verified"
	caseStatusRejected       = "rejected"
	caseStatusExpired        = "expired"
	caseStatusCanceled       = "canceled"
	materialStatusLegalHold  = "legal_hold"

	decisionActionReviewStarted    = "review_started"
	decisionActionApproved         = "approved"
	decisionActionRejected         = "rejected"
	decisionActionDuplicateFlagged = "duplicate_flagged"
	decisionReasonApproved         = "manual_review_passed"
	decisionReasonDuplicate        = "duplicate_registration_risk"

	rejectionReasonMaterialsIncomplete            = "materials_incomplete"
	rejectionReasonMaterialsUnreadable            = "materials_unreadable"
	rejectionReasonLegalInformationMismatch       = "legal_information_mismatch"
	rejectionReasonRegistrationIdentifierInvalid  = "registration_identifier_invalid"
	rejectionReasonRepresentativeIdentityMismatch = "representative_identity_mismatch"
	rejectionReasonAgentAuthorizationInvalid      = "agent_authorization_invalid"
	rejectionReasonRegistrationStatusUnverifiable = "registration_status_unverifiable"
	rejectionReasonBusinessScopeUnsupported       = "business_scope_unsupported"
)

// errDuplicateRegistrationReview rolls back an attempted approval before a
// separate transaction marks the case for explicit duplicate-risk review.
var errDuplicateRegistrationReview = errors.New("enterprise registration requires duplicate review")

// rejectionReasonCodes is the closed customer-actionable rejection catalog.
var rejectionReasonCodes = map[string]struct{}{
	rejectionReasonMaterialsIncomplete: {}, rejectionReasonMaterialsUnreadable: {},
	rejectionReasonLegalInformationMismatch: {}, rejectionReasonRegistrationIdentifierInvalid: {},
	rejectionReasonRepresentativeIdentityMismatch: {}, rejectionReasonAgentAuthorizationInvalid: {},
	rejectionReasonRegistrationStatusUnverifiable: {}, rejectionReasonBusinessScopeUnsupported: {},
}

// StartReview enters verifying with optimistic concurrency and idempotent replay.
func (s *managementServiceImpl) StartReview(
	ctx context.Context,
	input ReviewInput,
) (*ReviewResult, error) {
	_, err := s.authorizeCaseAction(ctx, permissionEnterpriseCaseReview, input.CaseNo)
	if err != nil {
		return nil, err
	}
	if input.ExpectedVersion <= 0 || !validIdempotencyKey(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	idempotencyDigest := sha256Hex([]byte(input.IdempotencyKey))
	fingerprint := reviewFingerprint(decisionActionReviewStarted, input.CaseNo, input.ExpectedVersion, "")
	result := &ReviewResult{}
	err = dao.VerificationCase.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, lockErr := lockManagementCase(ctx, input.CaseNo)
		if lockErr != nil || row == nil {
			return gerror.New("enterprise verification review case is inaccessible")
		}
		if row.StatusCode == caseStatusVerifying &&
			row.ReviewIdempotencyKeyDigest == idempotencyDigest &&
			row.ReviewFingerprint == fingerprint && row.Version == input.ExpectedVersion+1 {
			*result = ReviewResult{
				CaseNo: row.CaseNo, StatusCode: row.StatusCode,
				DecisionReasonCode: row.DecisionReasonCode, Version: row.Version, Reused: true,
			}
			return nil
		}
		if row.Version != input.ExpectedVersion ||
			(row.StatusCode != caseStatusPending && row.StatusCode != caseStatusReviewRequired) {
			return gerror.New("enterprise verification review state is stale")
		}
		before := row.StatusCode
		emptyPrincipal := ""
		updated, updateErr := dao.VerificationCase.Ctx(ctx).Where(do.VerificationCase{
			Id: row.Id, TenantId: idccap.DefaultTenantID,
			StatusCode: row.StatusCode, Version: row.Version,
		}).Data(do.VerificationCase{
			StatusCode:                   caseStatusVerifying,
			ReviewIdempotencyKeyDigest:   idempotencyDigest,
			ReviewFingerprint:            fingerprint,
			DecisionReasonCode:           "",
			DecidedByPrincipalNo:         &emptyPrincipal,
			DecisionIdempotencyKeyDigest: "",
			DecisionFingerprint:          "",
			DecisionActionCode:           "",
			Version:                      row.Version + 1,
		}).Update()
		if updateErr != nil {
			return updateErr
		}
		if updateErr = requireOneRow(updated, "start enterprise verification review"); updateErr != nil {
			return updateErr
		}
		if updateErr = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.enterprise-verification.review-start",
			IdempotencyKey:   input.CaseNo + ":" + idempotencyDigest,
			OperationCode:    "compliance.enterprise-verification.review-start",
			TargetType:       "enterprise_verification_case", TargetNo: input.CaseNo,
			CustomerNo: row.CustomerNo, ResultCode: "succeeded",
			ReasonCode: "manual_review_started", RiskCode: "high",
			RequiredPermission: permissionEnterpriseCaseReview,
			CorrelationNo:      row.LegalChangeRequestNo,
			BeforeSnapshot:     map[string]any{"case_status": before, "version": row.Version},
			AfterSnapshot: map[string]any{
				"case_status": caseStatusVerifying, "version": row.Version + 1,
				"reverification": row.LegalChangeRequestNo != "",
			},
		}); updateErr != nil {
			return updateErr
		}
		*result = ReviewResult{
			CaseNo: row.CaseNo, StatusCode: caseStatusVerifying, Version: row.Version + 1,
		}
		return nil
	})
	if err != nil {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return result, nil
}

// Approve projects only the authenticated server snapshot and closes the case atomically.
func (s *managementServiceImpl) Approve(
	ctx context.Context,
	input DecisionInput,
) (*ReviewResult, error) {
	actor, err := s.authorizeCaseAction(ctx, permissionEnterpriseCaseApprove, input.CaseNo)
	if err != nil {
		return nil, err
	}
	if input.ExpectedVersion <= 0 || input.ReasonCode != "" ||
		!validIdempotencyKey(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	idempotencyDigest := sha256Hex([]byte(input.IdempotencyKey))
	fingerprint := reviewFingerprint(
		decisionActionApproved, input.CaseNo, input.ExpectedVersion, decisionReasonApproved,
	)
	row, err := loadManagementCase(ctx, input.CaseNo)
	if err != nil || row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	if replay := replayDecision(row, input.ExpectedVersion, idempotencyDigest, fingerprint); replay != nil {
		return replay, nil
	}
	if row.Version != input.ExpectedVersion ||
		(row.StatusCode != caseStatusVerifying && row.StatusCode != caseStatusReviewRequired) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	profile, err := s.loadLegalSnapshot(ctx, row)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	defer profile.clear()
	result := &ReviewResult{}
	err = dao.VerificationCase.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		locked, lockErr := lockManagementCase(ctx, input.CaseNo)
		if lockErr != nil || locked == nil || locked.Version != input.ExpectedVersion ||
			(locked.StatusCode != caseStatusVerifying && locked.StatusCode != caseStatusReviewRequired) ||
			!sameSnapshotAuthority(row, locked) {
			return gerror.New("enterprise verification approval state is stale")
		}
		_, lockedMaterials, materialErr := loadManagementMaterials(ctx, input.CaseNo, true)
		if materialErr != nil || !profile.materialSetMatches(lockedMaterials) ||
			!approvableMaterials(lockedMaterials) {
			return gerror.New("enterprise verification approval materials are unavailable")
		}
		verifiedAt := s.now().UTC().Truncate(time.Microsecond)
		projection, projectionErr := s.identities.ProjectEnterprise(
			ctx, identitycap.EnterpriseProjectionInput{
				CustomerNo: locked.CustomerNo, CaseNo: locked.CaseNo,
				Status:               identitycap.StatusVerified,
				LegalChangeRequestNo: locked.LegalChangeRequestNo,
				ReviewActorUserID:    actor, DecisionReasonCode: decisionReasonApproved,
				LegalName:                    profile.legalName,
				RegistrationCountryCode:      profile.registrationCountryCode,
				RegistrationIdentifier:       profile.registrationIdentifier,
				MaskedRegistrationIdentifier: maskRegistrationIdentifier(profile.registrationIdentifier),
				IndustryCode:                 profile.industryCode, VerifiedAt: &verifiedAt,
			},
		)
		if errors.Is(projectionErr, identitycap.ErrDuplicateEnterprise) ||
			(projectionErr == nil && projection != nil && projection.DuplicateRisk) {
			return errDuplicateRegistrationReview
		}
		if projectionErr != nil || projection == nil || projection.Status != identitycap.StatusVerified ||
			(locked.LegalChangeRequestNo != "" && projection.PreservedCurrent) {
			return gerror.New("enterprise identity approval projection is inconsistent")
		}
		principalNo := actorPrincipalNo(actor)
		updated, updateErr := dao.VerificationCase.Ctx(ctx).Where(do.VerificationCase{
			Id: locked.Id, TenantId: idccap.DefaultTenantID,
			StatusCode: locked.StatusCode, Version: locked.Version,
		}).Data(do.VerificationCase{
			StatusCode: caseStatusVerified, DecisionReasonCode: decisionReasonApproved,
			VerifiedAt: &verifiedAt, RejectedAt: nil, DecidedByPrincipalNo: &principalNo,
			IdentityLookupDigest:         &projection.RegistrationLookupDigest,
			DecisionIdempotencyKeyDigest: idempotencyDigest,
			DecisionFingerprint:          fingerprint, DecisionActionCode: decisionActionApproved,
			Version: locked.Version + 1,
		}).Update()
		if updateErr != nil {
			return updateErr
		}
		if updateErr = requireOneRow(updated, "approve enterprise verification case"); updateErr != nil {
			return updateErr
		}
		if updateErr = s.appendDecisionAudit(
			ctx, locked, permissionEnterpriseCaseApprove, idempotencyDigest,
			caseStatusVerified, decisionReasonApproved, sortedMaterialTypes(lockedMaterials),
		); updateErr != nil {
			return updateErr
		}
		*result = ReviewResult{
			CaseNo: locked.CaseNo, StatusCode: caseStatusVerified,
			DecisionReasonCode: decisionReasonApproved, Version: locked.Version + 1,
		}
		return nil
	})
	if errors.Is(err, errDuplicateRegistrationReview) {
		return s.markDuplicateRegistrationReview(
			ctx, row, actor, input.ExpectedVersion, idempotencyDigest, fingerprint,
		)
	}
	if err != nil {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return result, nil
}

// Reject records one closed reason while preserving a prior verified re-review projection.
func (s *managementServiceImpl) Reject(
	ctx context.Context,
	input DecisionInput,
) (*ReviewResult, error) {
	actor, err := s.authorizeCaseAction(ctx, permissionEnterpriseCaseReject, input.CaseNo)
	if err != nil {
		return nil, err
	}
	if input.ExpectedVersion <= 0 || !validIdempotencyKey(input.IdempotencyKey) ||
		!validRejectionReason(input.ReasonCode) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	idempotencyDigest := sha256Hex([]byte(input.IdempotencyKey))
	fingerprint := reviewFingerprint(
		decisionActionRejected, input.CaseNo, input.ExpectedVersion, input.ReasonCode,
	)
	result := &ReviewResult{}
	err = dao.VerificationCase.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, lockErr := lockManagementCase(ctx, input.CaseNo)
		if lockErr != nil || row == nil {
			return gerror.New("enterprise verification rejection case is inaccessible")
		}
		if replay := replayDecision(row, input.ExpectedVersion, idempotencyDigest, fingerprint); replay != nil {
			*result = *replay
			return nil
		}
		if row.Version != input.ExpectedVersion ||
			(row.StatusCode != caseStatusVerifying && row.StatusCode != caseStatusReviewRequired) {
			return gerror.New("enterprise verification rejection state is stale")
		}
		projection, projectionErr := s.identities.ProjectEnterprise(
			ctx, identitycap.EnterpriseProjectionInput{
				CustomerNo: row.CustomerNo, CaseNo: row.CaseNo,
				Status:               identitycap.StatusRejected,
				LegalChangeRequestNo: row.LegalChangeRequestNo,
				ReviewActorUserID:    actor, DecisionReasonCode: input.ReasonCode,
			},
		)
		if projectionErr != nil || projection == nil ||
			(row.LegalChangeRequestNo == "" && projection.Status != identitycap.StatusRejected) ||
			(row.LegalChangeRequestNo != "" &&
				(projection.Status != identitycap.StatusVerified || !projection.PreservedCurrent)) {
			return gerror.New("enterprise identity rejection projection is inconsistent")
		}
		rejectedAt := s.now().UTC().Truncate(time.Microsecond)
		principalNo := actorPrincipalNo(actor)
		updated, updateErr := dao.VerificationCase.Ctx(ctx).Where(do.VerificationCase{
			Id: row.Id, TenantId: idccap.DefaultTenantID,
			StatusCode: row.StatusCode, Version: row.Version,
		}).Data(do.VerificationCase{
			StatusCode: caseStatusRejected, DecisionReasonCode: input.ReasonCode,
			VerifiedAt: nil, RejectedAt: &rejectedAt, DecidedByPrincipalNo: &principalNo,
			DecisionIdempotencyKeyDigest: idempotencyDigest,
			DecisionFingerprint:          fingerprint, DecisionActionCode: decisionActionRejected,
			Version: row.Version + 1,
		}).Update()
		if updateErr != nil {
			return updateErr
		}
		if updateErr = requireOneRow(updated, "reject enterprise verification case"); updateErr != nil {
			return updateErr
		}
		if updateErr = s.appendDecisionAudit(
			ctx, row, permissionEnterpriseCaseReject, idempotencyDigest,
			caseStatusRejected, input.ReasonCode, nil,
		); updateErr != nil {
			return updateErr
		}
		*result = ReviewResult{
			CaseNo: row.CaseNo, StatusCode: caseStatusRejected,
			DecisionReasonCode: input.ReasonCode, Version: row.Version + 1,
		}
		return nil
	})
	if err != nil {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return result, nil
}

// authorizeCaseAction checks one exact permission and case target before existence lookup.
func (s *managementServiceImpl) authorizeCaseAction(
	ctx context.Context,
	permission string,
	caseNo string,
) (int, error) {
	if ctx == nil || ctx.Err() != nil {
		return 0, bizerr.NewCode(CodeInvalid)
	}
	actor, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: permission, TargetNo: caseNo,
	})
	if err != nil {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	return actor, nil
}

// lockManagementCase locks one submitted case in the fixed merchant scope.
func lockManagementCase(ctx context.Context, caseNo string) (*entity.VerificationCase, error) {
	var row *entity.VerificationCase
	err := dao.VerificationCase.Ctx(ctx).Where(do.VerificationCase{
		TenantId: idccap.DefaultTenantID, CaseNo: caseNo,
		SubjectTypeCode: enterpriseSubjectType,
	}).Where("submitted_at IS NOT NULL").LockUpdate().Limit(1).Scan(&row)
	return row, err
}

// replayDecision returns only an exact idempotent terminal transition replay.
func replayDecision(
	row *entity.VerificationCase,
	expectedVersion int64,
	idempotencyDigest string,
	fingerprint string,
) *ReviewResult {
	if row == nil || row.Version != expectedVersion+1 ||
		row.DecisionIdempotencyKeyDigest != idempotencyDigest ||
		row.DecisionFingerprint != fingerprint ||
		(row.StatusCode != caseStatusVerified && row.StatusCode != caseStatusRejected &&
			row.StatusCode != caseStatusReviewRequired) {
		return nil
	}
	return &ReviewResult{
		CaseNo: row.CaseNo, StatusCode: row.StatusCode,
		DecisionReasonCode: row.DecisionReasonCode, Version: row.Version, Reused: true,
	}
}

// markDuplicateRegistrationReview records duplicate risk after approval rollback.
func (s *managementServiceImpl) markDuplicateRegistrationReview(
	ctx context.Context,
	loaded *entity.VerificationCase,
	actor int,
	expectedVersion int64,
	idempotencyDigest string,
	fingerprint string,
) (*ReviewResult, error) {
	result := &ReviewResult{}
	err := dao.VerificationCase.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, lockErr := lockManagementCase(ctx, loaded.CaseNo)
		if lockErr != nil || row == nil {
			return gerror.New("enterprise duplicate review case is inaccessible")
		}
		if replay := replayDecision(row, expectedVersion, idempotencyDigest, fingerprint); replay != nil {
			*result = *replay
			return nil
		}
		if row.Version != expectedVersion ||
			(row.StatusCode != caseStatusVerifying && row.StatusCode != caseStatusReviewRequired) ||
			!sameSnapshotAuthority(loaded, row) {
			return gerror.New("enterprise duplicate review state is stale")
		}
		principalNo := actorPrincipalNo(actor)
		updated, updateErr := dao.VerificationCase.Ctx(ctx).Where(do.VerificationCase{
			Id: row.Id, TenantId: idccap.DefaultTenantID,
			StatusCode: row.StatusCode, Version: row.Version,
		}).Data(do.VerificationCase{
			StatusCode: caseStatusReviewRequired, DecisionReasonCode: decisionReasonDuplicate,
			DecidedByPrincipalNo:         &principalNo,
			DecisionIdempotencyKeyDigest: idempotencyDigest,
			DecisionFingerprint:          fingerprint, DecisionActionCode: decisionActionDuplicateFlagged,
			Version: row.Version + 1,
		}).Update()
		if updateErr != nil {
			return updateErr
		}
		if updateErr = requireOneRow(updated, "flag duplicate enterprise registration"); updateErr != nil {
			return updateErr
		}
		if updateErr = s.appendDecisionAudit(
			ctx, row, permissionEnterpriseCaseApprove, idempotencyDigest,
			caseStatusReviewRequired, decisionReasonDuplicate, nil,
		); updateErr != nil {
			return updateErr
		}
		*result = ReviewResult{
			CaseNo: row.CaseNo, StatusCode: caseStatusReviewRequired,
			DecisionReasonCode: decisionReasonDuplicate, Version: row.Version + 1,
		}
		return nil
	})
	if err != nil {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return result, nil
}

// appendDecisionAudit writes one secret-safe immutable review transition.
func (s *managementServiceImpl) appendDecisionAudit(
	ctx context.Context,
	row *entity.VerificationCase,
	permission string,
	idempotencyDigest string,
	status string,
	reason string,
	materialTypes []string,
) error {
	return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "compliance.enterprise-verification.decision",
		IdempotencyKey:   row.CaseNo + ":" + idempotencyDigest,
		OperationCode:    "compliance.enterprise-verification.decision",
		TargetType:       "enterprise_verification_case", TargetNo: row.CaseNo,
		CustomerNo: row.CustomerNo, ResultCode: "succeeded", ReasonCode: reason,
		RiskCode: "critical", RequiredPermission: permission,
		CorrelationNo:  row.LegalChangeRequestNo,
		BeforeSnapshot: map[string]any{"case_status": row.StatusCode, "version": row.Version},
		AfterSnapshot: map[string]any{
			"case_status": status, "decision_reason": reason,
			"version": row.Version + 1, "reverification": row.LegalChangeRequestNo != "",
			"material_types": materialTypes,
		},
	})
}

// reviewFingerprint binds one idempotent command to action, case, version, and reason.
func reviewFingerprint(action, caseNo string, version int64, reason string) string {
	return sha256Hex([]byte(strings.Join([]string{
		"idc/enterprise-review/v1", action, caseNo,
		strconv.FormatInt(version, 10), reason,
	}, "\x00")))
}

// validRejectionReason accepts only the closed actionable catalog.
func validRejectionReason(value string) bool {
	_, ok := rejectionReasonCodes[value]
	return ok
}

// sameSnapshotAuthority prevents approval after the encrypted snapshot binding changes.
func sameSnapshotAuthority(left, right *entity.VerificationCase) bool {
	if left == nil || right == nil || left.EncryptionKeyNo == nil || right.EncryptionKeyNo == nil ||
		left.LegalProfileSnapshotRef == nil || right.LegalProfileSnapshotRef == nil ||
		left.LegalProfileSha256 == nil || right.LegalProfileSha256 == nil {
		return false
	}
	return *left.EncryptionKeyNo == *right.EncryptionKeyNo &&
		*left.LegalProfileSnapshotRef == *right.LegalProfileSnapshotRef &&
		*left.LegalProfileSha256 == *right.LegalProfileSha256 &&
		left.SnapshotStoredByteSize == right.SnapshotStoredByteSize
}

// approvableMaterials validates the complete bounded evidence metadata set.
func approvableMaterials(rows []*entity.EnterpriseMaterial) bool {
	if len(rows) == 0 || len(rows) > maximumSelectedCount {
		return false
	}
	for _, row := range rows {
		if row == nil || (row.StatusCode != materialStatusAvailable && row.StatusCode != materialStatusLegalHold) ||
			row.EncryptionStateCode != materialEncryptionState || row.EncryptionKeyNo == "" ||
			row.PrivateObjectRef == "" || row.ContentSha256 == "" || row.StoredByteSize <= row.ByteSize {
			return false
		}
	}
	return true
}
