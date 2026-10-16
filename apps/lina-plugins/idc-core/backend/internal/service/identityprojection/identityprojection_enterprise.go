// This file owns the enterprise legal payment-gate projection and coordinates
// category-only legal-change workflow state without storing review materials.

package identityprojection

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const enterpriseRegistrationUniqueFence = "uk_plugin_idc_core_ent_profile_registration_digest"

// ProjectEnterprise applies one initial-verification or legal-change decision.
func (s *serviceImpl) ProjectEnterprise(
	ctx context.Context,
	input identitycap.EnterpriseProjectionInput,
) (*identitycap.EnterpriseProjection, error) {
	defer clear(input.LegalName)
	defer clear(input.RegistrationIdentifier)
	if err := validateEnterpriseProjectionInput(input); err != nil {
		return nil, err
	}
	var projection *identitycap.EnterpriseProjection
	var duplicateFallback *identitycap.EnterpriseProjection
	hasAmbientTransaction := gdb.TXFromCtx(ctx, dao.CustomerEnterpriseProfile.Group()) != nil
	err := dao.CustomerEnterpriseProfile.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		result, fallback, applyErr := s.applyEnterpriseProjection(ctx, input)
		duplicateFallback = fallback
		if applyErr == nil {
			projection = result
		}
		return applyErr
	})
	if err != nil {
		if duplicateFallback != nil && strings.Contains(err.Error(), enterpriseRegistrationUniqueFence) {
			if hasAmbientTransaction {
				return nil, gerror.Wrap(
					identitycap.ErrDuplicateEnterprise,
					"enterprise registration uniqueness fence rejected projection",
				)
			}
			return duplicateFallback, nil
		}
		return nil, err
	}
	return projection, nil
}

func (s *serviceImpl) applyEnterpriseProjection(
	ctx context.Context,
	input identitycap.EnterpriseProjectionInput,
) (*identitycap.EnterpriseProjection, *identitycap.EnterpriseProjection, error) {
	var subject *entity.CustomerSubject
	if err := dao.CustomerSubject.Ctx(ctx).
		Fields("id", "tenant_id", "customer_no", "customer_type_code", "status_code", "version").
		Where(do.CustomerSubject{
			TenantId: idccap.DefaultTenantID, CustomerNo: input.CustomerNo,
		}).LockUpdate().Limit(1).Scan(&subject); err != nil {
		return nil, nil, gerror.Wrap(err, "lock enterprise identity subject")
	}
	if subject == nil || (subject.CustomerTypeCode != individualType &&
		subject.CustomerTypeCode != enterpriseType) ||
		!eligibleCustomerStatus(subject.StatusCode) {
		return nil, nil, gerror.New("enterprise identity projection is not accessible")
	}
	var personal *entity.CustomerIndividualProfile
	if err := dao.CustomerIndividualProfile.Ctx(ctx).Fields(
		"tenant_id", "customer_no", "customer_type_code", "identity_status_code",
		"identity_verified_at", "identity_expires_at", "version",
	).Where(do.CustomerIndividualProfile{
		TenantId: idccap.DefaultTenantID, CustomerNo: input.CustomerNo,
	}).LockUpdate().Limit(1).Scan(&personal); err != nil {
		return nil, nil, gerror.Wrap(err, "lock personal identity prerequisite")
	}
	if personal == nil || personal.CustomerTypeCode != individualType ||
		personal.IdentityStatusCode != string(identitycap.StatusVerified) ||
		personal.IdentityVerifiedAt == nil || personal.Version <= 0 ||
		(personal.IdentityExpiresAt != nil && !personal.IdentityExpiresAt.After(time.Now().UTC())) {
		return nil, nil, gerror.New("verified personal identity is required before enterprise verification")
	}
	var profile *entity.CustomerEnterpriseProfile
	if err := dao.CustomerEnterpriseProfile.Ctx(ctx).Where(do.CustomerEnterpriseProfile{
		TenantId: idccap.DefaultTenantID, CustomerNo: input.CustomerNo,
	}).LockUpdate().Limit(1).Scan(&profile); err != nil {
		return nil, nil, gerror.Wrap(err, "lock enterprise identity projection")
	}
	if profile == nil || profile.CustomerTypeCode != enterpriseType || profile.Version <= 0 {
		return nil, nil, gerror.New("enterprise identity projection is unavailable")
	}
	if profile.ManualVerification && profile.VerificationStatusCode == string(identitycap.StatusVerified) {
		return nil, nil, gerror.New("enterprise identity was manually approved; clear verification before starting another case")
	}
	if input.LegalChangeRequestNo != "" {
		if subject.CustomerTypeCode != enterpriseType {
			return nil, nil, gerror.New("enterprise legal change is not accessible before enterprise approval")
		}
		return s.applyEnterpriseReverification(ctx, profile, input)
	}
	return s.applyInitialEnterpriseProjection(ctx, subject, profile, input)
}

func (s *serviceImpl) applyInitialEnterpriseProjection(
	ctx context.Context,
	subject *entity.CustomerSubject,
	profile *entity.CustomerEnterpriseProfile,
	input identitycap.EnterpriseProjectionInput,
) (*identitycap.EnterpriseProjection, *identitycap.EnterpriseProjection, error) {
	currentStatus := identitycap.Status(profile.VerificationStatusCode)
	if profile.VerificationCaseNo == input.CaseNo && currentStatus == input.Status {
		replayed, err := s.replayEnterpriseProjection(profile, input, false)
		if err == nil && replayed != nil && input.Status == identitycap.StatusVerified {
			err = upgradeCustomerSubjectToEnterprise(ctx, subject)
		}
		return replayed, nil, err
	}
	if input.Status == identitycap.StatusPending {
		if currentStatus == identitycap.StatusVerified || currentStatus == identitycap.StatusPending {
			return nil, nil, gerror.New("enterprise identity projection belongs to another active case")
		}
	} else if profile.VerificationCaseNo != input.CaseNo || currentStatus != identitycap.StatusPending {
		return nil, nil, gerror.New("enterprise identity projection transition is stale")
	}
	projection, fallback, err := s.updateEnterpriseProfile(ctx, profile, input, false)
	if err != nil || projection == nil || projection.DuplicateRisk ||
		input.Status != identitycap.StatusVerified {
		return projection, fallback, err
	}
	if err = upgradeCustomerSubjectToEnterprise(ctx, subject); err != nil {
		return nil, nil, err
	}
	return projection, fallback, nil
}

func upgradeCustomerSubjectToEnterprise(
	ctx context.Context,
	subject *entity.CustomerSubject,
) error {
	if subject == nil || subject.CustomerTypeCode == enterpriseType {
		return nil
	}
	if subject.CustomerTypeCode != individualType || subject.Id <= 0 || subject.Version <= 0 {
		return gerror.New("customer subject cannot be upgraded to enterprise")
	}
	updated, err := dao.CustomerSubject.Ctx(ctx).Where(do.CustomerSubject{
		Id: subject.Id, TenantId: idccap.DefaultTenantID,
		CustomerTypeCode: individualType, Version: subject.Version,
	}).Data(do.CustomerSubject{
		CustomerTypeCode: enterpriseType, Version: subject.Version + 1,
	}).Update()
	if err != nil {
		return gerror.Wrap(err, "upgrade customer subject to enterprise")
	}
	return requireSingleAffected(updated, "customer enterprise upgrade lost its version fence")
}

func (s *serviceImpl) applyEnterpriseReverification(
	ctx context.Context,
	profile *entity.CustomerEnterpriseProfile,
	input identitycap.EnterpriseProjectionInput,
) (*identitycap.EnterpriseProjection, *identitycap.EnterpriseProjection, error) {
	var change *entity.CustomerLegalChangeRequest
	if err := dao.CustomerLegalChangeRequest.Ctx(ctx).Where(do.CustomerLegalChangeRequest{
		TenantId: idccap.DefaultTenantID, RequestNo: input.LegalChangeRequestNo,
		CustomerNo: input.CustomerNo, CustomerTypeCode: enterpriseType,
	}).LockUpdate().Limit(1).Scan(&change); err != nil {
		return nil, nil, gerror.Wrap(err, "lock enterprise legal change request")
	}
	if change == nil || change.CurrentVerificationStatusCode != string(identitycap.StatusVerified) ||
		change.CurrentVerificationCaseNo == "" || change.CurrentProfileVersion <= 0 ||
		input.CaseNo == change.CurrentVerificationCaseNo ||
		(change.StatusCode == legalChangeStatusSubmitted && change.ReviewVerificationCaseNo != "") ||
		(change.StatusCode != legalChangeStatusSubmitted &&
			change.ReviewVerificationCaseNo != input.CaseNo) {
		return nil, nil, gerror.New("enterprise legal change request is unavailable")
	}

	if change.StatusCode == legalChangeStatusApproved && input.Status == identitycap.StatusVerified {
		if change.ReviewedByUserId != input.ReviewActorUserID ||
			change.ResolutionCode != input.DecisionReasonCode || change.ReviewedAt == nil {
			return nil, nil, gerror.New("enterprise legal change approval replay changed its decision")
		}
		replayed, err := s.replayEnterpriseProjection(profile, input, true)
		return replayed, nil, err
	}
	if change.StatusCode == legalChangeStatusRejected && input.Status == identitycap.StatusRejected {
		if change.ReviewedByUserId != input.ReviewActorUserID ||
			change.ResolutionCode != input.DecisionReasonCode || change.ReviewedAt == nil {
			return nil, nil, gerror.New("enterprise legal change rejection replay changed its decision")
		}
		return &identitycap.EnterpriseProjection{
			Status: identitycap.StatusVerified, Version: profile.Version,
			RegistrationLookupDigest: profile.RegistrationLookupDigest,
			PreservedCurrent:         true, Reused: true,
		}, nil, nil
	}
	if profile.VerificationStatusCode != string(identitycap.StatusVerified) ||
		profile.VerificationCaseNo != change.CurrentVerificationCaseNo ||
		profile.Version != change.CurrentProfileVersion {
		return nil, nil, gerror.New("enterprise legal projection changed after review submission")
	}

	switch input.Status {
	case identitycap.StatusPending:
		return startEnterpriseReverification(ctx, profile, change, input)
	case identitycap.StatusRejected:
		return rejectEnterpriseReverification(ctx, profile, change, input)
	case identitycap.StatusVerified:
		return s.approveEnterpriseReverification(ctx, profile, change, input)
	default:
		return nil, nil, gerror.New("enterprise re-verification status is invalid")
	}
}

func startEnterpriseReverification(
	ctx context.Context,
	profile *entity.CustomerEnterpriseProfile,
	change *entity.CustomerLegalChangeRequest,
	input identitycap.EnterpriseProjectionInput,
) (*identitycap.EnterpriseProjection, *identitycap.EnterpriseProjection, error) {
	if change.StatusCode == legalChangeStatusUnderReview {
		if change.ReviewVerificationCaseNo != input.CaseNo {
			return nil, nil, gerror.New("enterprise legal change review belongs to another case")
		}
		return preservedEnterpriseProjection(profile, true), nil, nil
	}
	if change.StatusCode != legalChangeStatusSubmitted {
		return nil, nil, gerror.New("enterprise legal change request cannot enter review")
	}
	updated, err := dao.CustomerLegalChangeRequest.Ctx(ctx).Where(do.CustomerLegalChangeRequest{
		Id: change.Id, TenantId: idccap.DefaultTenantID, StatusCode: legalChangeStatusSubmitted,
	}).Data(do.CustomerLegalChangeRequest{
		StatusCode: legalChangeStatusUnderReview, ReviewVerificationCaseNo: input.CaseNo,
	}).Update()
	if err != nil {
		return nil, nil, gerror.Wrap(err, "start enterprise legal change review")
	}
	if err = requireSingleAffected(updated, "enterprise legal change review lost its state fence"); err != nil {
		return nil, nil, err
	}
	return preservedEnterpriseProjection(profile, false), nil, nil
}

func rejectEnterpriseReverification(
	ctx context.Context,
	profile *entity.CustomerEnterpriseProfile,
	change *entity.CustomerLegalChangeRequest,
	input identitycap.EnterpriseProjectionInput,
) (*identitycap.EnterpriseProjection, *identitycap.EnterpriseProjection, error) {
	if change.StatusCode != legalChangeStatusUnderReview {
		return nil, nil, gerror.New("enterprise legal change rejection is stale")
	}
	if err := resolveEnterpriseLegalChange(ctx, change, input, legalChangeStatusRejected); err != nil {
		return nil, nil, err
	}
	return preservedEnterpriseProjection(profile, false), nil, nil
}

func (s *serviceImpl) approveEnterpriseReverification(
	ctx context.Context,
	profile *entity.CustomerEnterpriseProfile,
	change *entity.CustomerLegalChangeRequest,
	input identitycap.EnterpriseProjectionInput,
) (*identitycap.EnterpriseProjection, *identitycap.EnterpriseProjection, error) {
	if change.StatusCode != legalChangeStatusUnderReview {
		return nil, nil, gerror.New("enterprise legal change approval is stale")
	}
	if err := validateEnterpriseLegalChangeValues(profile, change, input); err != nil {
		return nil, nil, err
	}
	projection, fallback, err := s.updateEnterpriseProfile(ctx, profile, input, true)
	if err != nil || projection == nil || projection.DuplicateRisk {
		return projection, fallback, err
	}
	if err = resolveEnterpriseLegalChange(ctx, change, input, legalChangeStatusApproved); err != nil {
		return nil, nil, err
	}
	return projection, fallback, nil
}

func preservedEnterpriseProjection(
	profile *entity.CustomerEnterpriseProfile,
	reused bool,
) *identitycap.EnterpriseProjection {
	return &identitycap.EnterpriseProjection{
		Status: identitycap.StatusVerified, Version: profile.Version,
		RegistrationLookupDigest: profile.RegistrationLookupDigest,
		PreservedCurrent:         true, Reused: reused,
	}
}

func (s *serviceImpl) updateEnterpriseProfile(
	ctx context.Context,
	profile *entity.CustomerEnterpriseProfile,
	input identitycap.EnterpriseProjectionInput,
	reverification bool,
) (*identitycap.EnterpriseProjection, *identitycap.EnterpriseProjection, error) {
	nextVersion := profile.Version + 1
	update := do.CustomerEnterpriseProfile{ManualVerification: false,
		VerificationCaseNo: input.CaseNo, VerificationStatusCode: string(input.Status),
		Version: nextVersion,
	}
	projection := &identitycap.EnterpriseProjection{Status: input.Status, Version: nextVersion}
	var duplicateFallback *identitycap.EnterpriseProjection
	if input.Status == identitycap.StatusVerified {
		digest, err := s.enterpriseRegistrationDigest(input.RegistrationIdentifier)
		if err != nil {
			return nil, nil, err
		}
		duplicate, err := duplicateEnterpriseRegistration(ctx, input.CustomerNo, digest)
		if err != nil {
			return nil, nil, err
		}
		if duplicate {
			return &identitycap.EnterpriseProjection{
				Status: identitycap.Status(profile.VerificationStatusCode), Version: profile.Version,
				RegistrationLookupDigest: profile.RegistrationLookupDigest,
				DuplicateRisk:            true, PreservedCurrent: reverification,
			}, nil, nil
		}
		duplicateFallback = &identitycap.EnterpriseProjection{
			Status: identitycap.Status(profile.VerificationStatusCode), Version: profile.Version,
			RegistrationLookupDigest: profile.RegistrationLookupDigest,
			DuplicateRisk:            true, PreservedCurrent: reverification,
		}
		verifiedAt := input.VerifiedAt.UTC().Truncate(time.Microsecond)
		update.LegalName = string(input.LegalName)
		update.RegistrationCountryCode = input.RegistrationCountryCode
		update.RegistrationIdentifier = string(input.RegistrationIdentifier)
		update.RegistrationIdentifierMasked = input.MaskedRegistrationIdentifier
		update.RegistrationLookupDigest = digest
		update.IndustryCode = input.IndustryCode
		update.VerifiedAt = &verifiedAt
		update.ExpiresAt = normalizedTime(input.ExpiresAt)
		projection.RegistrationLookupDigest = digest
	} else {
		update.LegalName = ""
		update.RegistrationIdentifier = ""
		update.RegistrationIdentifierMasked = ""
		update.RegistrationLookupDigest = ""
		update.IndustryCode = ""
		update.VerifiedAt = nil
		update.ExpiresAt = normalizedTime(input.ExpiresAt)
	}
	updated, err := dao.CustomerEnterpriseProfile.Ctx(ctx).Where(do.CustomerEnterpriseProfile{
		Id: profile.Id, TenantId: idccap.DefaultTenantID, Version: profile.Version,
	}).Data(update).Update()
	if err != nil {
		return nil, duplicateFallback, gerror.Wrap(err, "update enterprise identity projection")
	}
	if err = requireSingleAffected(updated, "enterprise identity projection lost its version fence"); err != nil {
		return nil, nil, err
	}
	return projection, nil, nil
}

func (s *serviceImpl) replayEnterpriseProjection(
	profile *entity.CustomerEnterpriseProfile,
	input identitycap.EnterpriseProjectionInput,
	reverification bool,
) (*identitycap.EnterpriseProjection, error) {
	result := &identitycap.EnterpriseProjection{
		Status: input.Status, Version: profile.Version, Reused: true,
		PreservedCurrent: reverification && input.Status != identitycap.StatusVerified,
	}
	if input.Status != identitycap.StatusVerified {
		return result, nil
	}
	digest, err := s.enterpriseRegistrationDigest(input.RegistrationIdentifier)
	if err != nil {
		return nil, err
	}
	if profile.VerificationCaseNo != input.CaseNo || profile.LegalName != string(input.LegalName) ||
		profile.RegistrationCountryCode != input.RegistrationCountryCode ||
		profile.RegistrationIdentifier != string(input.RegistrationIdentifier) ||
		profile.RegistrationIdentifierMasked != input.MaskedRegistrationIdentifier ||
		profile.IndustryCode != input.IndustryCode ||
		!hmac.Equal([]byte(profile.RegistrationLookupDigest), []byte(digest)) {
		return nil, gerror.New("enterprise identity verified replay changed its legal profile")
	}
	result.RegistrationLookupDigest = digest
	return result, nil
}

func resolveEnterpriseLegalChange(
	ctx context.Context,
	change *entity.CustomerLegalChangeRequest,
	input identitycap.EnterpriseProjectionInput,
	status string,
) error {
	now := time.Now().UTC().Truncate(time.Microsecond)
	updated, err := dao.CustomerLegalChangeRequest.Ctx(ctx).Where(do.CustomerLegalChangeRequest{
		Id: change.Id, TenantId: idccap.DefaultTenantID, StatusCode: legalChangeStatusUnderReview,
	}).Data(do.CustomerLegalChangeRequest{
		StatusCode: status, ReviewedByUserId: input.ReviewActorUserID,
		ReviewedAt: &now, ResolutionCode: input.DecisionReasonCode,
	}).Update()
	if err != nil {
		return gerror.Wrap(err, "resolve enterprise legal change request")
	}
	return requireSingleAffected(updated, "enterprise legal change resolution lost its state fence")
}

func validateEnterpriseLegalChangeValues(
	profile *entity.CustomerEnterpriseProfile,
	change *entity.CustomerLegalChangeRequest,
	input identitycap.EnterpriseProjectionInput,
) error {
	if profile == nil || change == nil {
		return gerror.New("enterprise legal change comparison is unavailable")
	}
	if !change.ChangeLegalName && profile.LegalName != string(input.LegalName) {
		return gerror.New("enterprise legal change attempted an undeclared name replacement")
	}
	if !change.ChangeLegalIdentifier &&
		(profile.RegistrationCountryCode != input.RegistrationCountryCode ||
			profile.RegistrationIdentifier != string(input.RegistrationIdentifier)) {
		return gerror.New("enterprise legal change attempted an undeclared identifier replacement")
	}
	return nil
}

func requireSingleAffected(result sql.Result, message string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read guarded update result")
	}
	if affected != 1 {
		return gerror.New(message)
	}
	return nil
}

func duplicateEnterpriseRegistration(ctx context.Context, customerNo, digest string) (bool, error) {
	count, err := dao.CustomerEnterpriseProfile.Ctx(ctx).Where(do.CustomerEnterpriseProfile{
		TenantId: idccap.DefaultTenantID, RegistrationLookupDigest: digest,
	}).WhereNot("customer_no", customerNo).Count()
	if err != nil {
		return false, gerror.Wrap(err, "check duplicate enterprise registration")
	}
	return count > 0, nil
}

func (s *serviceImpl) enterpriseRegistrationDigest(identifier []byte) (string, error) {
	mac := hmac.New(sha256.New, s.lookupKey)
	if _, err := mac.Write([]byte("idc/enterprise-registration/v1\x00")); err != nil {
		return "", gerror.Wrap(err, "write enterprise registration digest purpose")
	}
	if _, err := mac.Write(identifier); err != nil {
		return "", gerror.Wrap(err, "write enterprise registration digest value")
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func validateEnterpriseProjectionInput(input identitycap.EnterpriseProjectionInput) error {
	if !validBusinessNo(input.CustomerNo, 40) || !validBusinessNo(input.CaseNo, 40) ||
		(input.LegalChangeRequestNo != "" && !validBusinessNo(input.LegalChangeRequestNo, 40)) {
		return gerror.New("enterprise identity projection input is invalid")
	}
	terminal := input.Status == identitycap.StatusVerified || input.Status == identitycap.StatusRejected
	if (terminal && (input.ReviewActorUserID <= 0 ||
		!validBusinessNo(input.DecisionReasonCode, 64))) ||
		(!terminal && (input.ReviewActorUserID != 0 || input.DecisionReasonCode != "")) {
		return gerror.New("enterprise identity decision provenance is invalid")
	}
	switch input.Status {
	case identitycap.StatusPending, identitycap.StatusRejected, identitycap.StatusExpired:
		if len(input.LegalName) != 0 || len(input.RegistrationIdentifier) != 0 ||
			input.RegistrationCountryCode != "" || input.MaskedRegistrationIdentifier != "" ||
			input.IndustryCode != "" || input.VerifiedAt != nil {
			return gerror.New("enterprise non-verified projection contains legal data")
		}
	case identitycap.StatusVerified:
		if input.VerifiedAt == nil || input.VerifiedAt.IsZero() ||
			!validIdentityText(input.LegalName, 512) ||
			!validIdentityText(input.RegistrationIdentifier, 128) ||
			!validCountryCode(input.RegistrationCountryCode) ||
			!validMaskedValue(input.MaskedRegistrationIdentifier, 128) ||
			!validOptionalCode(input.IndustryCode, 64) ||
			(input.ExpiresAt != nil && !input.ExpiresAt.After(*input.VerifiedAt)) {
			return gerror.New("enterprise verified projection is invalid")
		}
	default:
		return gerror.New("enterprise identity projection status is invalid")
	}
	return nil
}

func validCountryCode(value string) bool {
	return len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' &&
		value[1] >= 'A' && value[1] <= 'Z'
}

func validOptionalCode(value string, maximumRunes int) bool {
	if value == "" {
		return true
	}
	return utf8.ValidString(value) && value == strings.TrimSpace(value) &&
		utf8.RuneCountInString(value) <= maximumRunes
}
