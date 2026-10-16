// This file owns enterprise-subject proof and purpose-specific consent.

package identityprojection

import (
	"context"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	enterpriseType               = "enterprise"
	enterpriseConsentType        = "enterprise_verification"
	enterpriseConsentNumberType  = "customer_consent_enterprise_verification"
	legalChangeStatusSubmitted   = "submitted"
	legalChangeStatusUnderReview = "under_review"
	legalChangeStatusApproved    = "approved"
	legalChangeStatusRejected    = "rejected"
)

// CurrentEnterprise proves current ownership without accepting a customer number.
func (s *serviceImpl) CurrentEnterprise(
	ctx context.Context,
) (*identitycap.EnterpriseSubject, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("enterprise identity subject is not accessible")
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer ||
		audience.Status != principal.AudienceStatusActive || audience.UserID <= 0 ||
		!validBusinessNo(audience.CustomerNo, 40) {
		return nil, gerror.New("enterprise identity preparation is not accessible")
	}
	var eligibility *struct {
		CustomerNo             string     `json:"customer_no"`
		CustomerTypeCode       string     `json:"customer_type_code"`
		StatusCode             string     `json:"status_code"`
		ManualVerification     bool       `json:"manual_verification"`
		VerificationStatusCode string     `json:"verification_status_code"`
		VerificationUpdatedAt  *time.Time `json:"verification_updated_at"`
		IdentityStatusCode     string     `json:"identity_status_code"`
		IdentityVerifiedAt     *time.Time `json:"identity_verified_at"`
		IdentityExpiresAt      *time.Time `json:"identity_expires_at"`
	}
	err = s.database.GetScan(ctx, &eligibility, `
SELECT subject.customer_no, subject.customer_type_code, subject.status_code,
       personal.identity_status_code, personal.identity_verified_at, personal.identity_expires_at,
 enterprise.manual_verification, enterprise.verification_status_code, enterprise.updated_at AS verification_updated_at
FROM plugin_idc_core_customer_subject subject
JOIN plugin_idc_core_customer_individual_profile personal
  ON personal.tenant_id = subject.tenant_id AND personal.customer_no = subject.customer_no
LEFT JOIN plugin_idc_core_customer_enterprise_profile enterprise ON enterprise.tenant_id = subject.tenant_id AND enterprise.customer_no = subject.customer_no
WHERE subject.tenant_id = ? AND subject.customer_no = ?
LIMIT 1`, idccap.DefaultTenantID, audience.CustomerNo)
	if err != nil {
		return nil, gerror.Wrap(err, "load enterprise identity subject")
	}
	if eligibility == nil || (eligibility.CustomerTypeCode != individualType &&
		eligibility.CustomerTypeCode != enterpriseType) ||
		!eligibleCustomerStatus(eligibility.StatusCode) ||
		(!eligibility.ManualVerification && (eligibility.IdentityStatusCode != string(identitycap.StatusVerified) ||
			eligibility.IdentityVerifiedAt == nil || (eligibility.IdentityExpiresAt != nil && !eligibility.IdentityExpiresAt.After(time.Now().UTC())))) {
		return nil, gerror.New("enterprise identity preparation is not accessible")
	}
	return &identitycap.EnterpriseSubject{
		ManualVerification: eligibility.ManualVerification, VerificationStatus: identitycap.Status(eligibility.VerificationStatusCode), VerificationUpdatedAt: eligibility.VerificationUpdatedAt,
		CustomerNo: audience.CustomerNo, PrincipalReference: "usr:" + strconv.Itoa(audience.UserID),
		ActorUserID: audience.UserID,
	}, nil
}

// RecordEnterpriseConsent appends or safely replays one notice grant.
func (s *serviceImpl) RecordEnterpriseConsent(
	ctx context.Context,
	input identitycap.EnterpriseConsentInput,
) (*identitycap.EnterpriseConsent, error) {
	if s == nil || ctx == nil || ctx.Err() != nil ||
		input.PolicyVersion != identitycap.EnterpriseConsentPolicyVersion ||
		!validConsentLocale(input.Locale) || !validConsentIdempotencyKey(input.IdempotencyKey) {
		return nil, gerror.New("enterprise identity consent input is invalid")
	}
	subject, err := s.prepareEnterpriseSubject(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	var result *identitycap.EnterpriseConsent
	err = dao.CustomerConsent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		consentNo, reserveErr := s.reserveConsentNumber(
			ctx, enterpriseConsentNumberType, "enterprise-consent", subject.CustomerNo,
			input.IdempotencyKey,
		)
		if reserveErr != nil {
			return reserveErr
		}
		var existing *entity.CustomerConsent
		if loadErr := dao.CustomerConsent.Ctx(ctx).Where(do.CustomerConsent{
			TenantId: idccap.DefaultTenantID, ConsentNo: consentNo, CustomerNo: subject.CustomerNo,
		}).Limit(1).Scan(&existing); loadErr != nil {
			return gerror.Wrap(loadErr, "load enterprise identity consent")
		}
		if existing != nil {
			if !matchesEnterpriseConsent(existing, input, subject.ActorUserID) {
				return gerror.New("enterprise identity consent idempotency conflict")
			}
			result = &identitycap.EnterpriseConsent{
				ConsentNo: existing.ConsentNo, PolicyVersion: existing.PolicyVersion,
				OccurredAt: existing.OccurredAt.UTC(), Reused: true,
			}
			return nil
		}
		evidence := s.personalConsentDigest(
			"enterprise-consent-evidence-v1",
			subject.CustomerNo+"\x00"+consentNo+"\x00"+
				identitycap.EnterpriseConsentPolicyReference+"\x00"+input.PolicyVersion+"\x00"+
				input.Locale+"\x00"+strconv.Itoa(subject.ActorUserID),
		)
		rowID, insertErr := dao.CustomerConsent.Ctx(ctx).Data(do.CustomerConsent{
			TenantId: idccap.DefaultTenantID, ConsentNo: consentNo, CustomerNo: subject.CustomerNo,
			ConsentTypeCode: enterpriseConsentType,
			PolicyReference: identitycap.EnterpriseConsentPolicyReference,
			PolicyVersion:   input.PolicyVersion, ActionCode: consentGranted,
			SourceCode: personalConsentSource, Locale: input.Locale, EvidenceDigest: evidence,
			OccurredAt: &now, RecordedBy: subject.ActorUserID,
		}).InsertAndGetId()
		if insertErr != nil {
			return gerror.Wrap(insertErr, "insert enterprise identity consent")
		}
		if assignErr := assignPersonalConsentNumber(ctx, consentNo, rowID, now); assignErr != nil {
			return assignErr
		}
		result = &identitycap.EnterpriseConsent{
			ConsentNo: consentNo, PolicyVersion: input.PolicyVersion, OccurredAt: now,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// PrepareEnterprise proves the latest notice and optional legal-change request.
func (s *serviceImpl) PrepareEnterprise(
	ctx context.Context,
	consentNo string,
	legalChangeRequestNo string,
) (*identitycap.EnterprisePreparation, error) {
	if !validEnterprisePreparationInput(ctx, consentNo, legalChangeRequestNo) {
		return nil, gerror.New("enterprise identity preparation is not accessible")
	}
	subject, err := s.prepareEnterpriseSubject(ctx)
	if err != nil {
		return nil, err
	}
	var latest *entity.CustomerConsent
	err = s.database.Model(dao.CustomerConsent.Table()).Ctx(ctx).
		Fields(
			"id", "consent_no", "customer_no", "consent_type_code", "policy_reference",
			"policy_version", "action_code", "occurred_at",
		).
		Where(do.CustomerConsent{
			TenantId: idccap.DefaultTenantID, CustomerNo: subject.CustomerNo,
			ConsentTypeCode: enterpriseConsentType,
		}).OrderDesc("occurred_at").OrderDesc("id").Limit(1).Scan(&latest)
	if err != nil {
		return nil, gerror.Wrap(err, "load latest enterprise verification consent")
	}
	if latest == nil || latest.ConsentNo != consentNo || latest.ActionCode != consentGranted ||
		latest.PolicyReference != identitycap.EnterpriseConsentPolicyReference ||
		latest.PolicyVersion != identitycap.EnterpriseConsentPolicyVersion || latest.OccurredAt == nil {
		return nil, gerror.New("enterprise identity consent is not effective")
	}
	var profile *entity.CustomerEnterpriseProfile
	if err = dao.CustomerEnterpriseProfile.Ctx(ctx).Fields(
		"customer_no", "customer_type_code", "verification_case_no", "verification_status_code", "version",
	).Where(do.CustomerEnterpriseProfile{
		TenantId: idccap.DefaultTenantID, CustomerNo: subject.CustomerNo,
	}).Limit(1).Scan(&profile); err != nil {
		return nil, gerror.Wrap(err, "load enterprise identity projection")
	}
	if profile == nil || profile.CustomerTypeCode != enterpriseType || profile.Version <= 0 {
		return nil, gerror.New("enterprise identity projection is unavailable")
	}
	preparation := &identitycap.EnterprisePreparation{
		Subject: *subject, CurrentCaseNo: profile.VerificationCaseNo,
		CurrentProfileVersion: profile.Version,
	}
	if legalChangeRequestNo == "" {
		if profile.VerificationStatusCode == string(identitycap.StatusVerified) {
			return nil, gerror.New("verified enterprise requires a legal change request")
		}
		return preparation, nil
	}
	var change *entity.CustomerLegalChangeRequest
	if err = dao.CustomerLegalChangeRequest.Ctx(ctx).Fields(
		"request_no", "customer_no", "customer_type_code", "status_code",
		"current_verification_status_code", "current_verification_case_no", "current_profile_version",
		"review_verification_case_no", "submitted_at",
	).Where(do.CustomerLegalChangeRequest{
		TenantId: idccap.DefaultTenantID, RequestNo: legalChangeRequestNo,
		CustomerNo: subject.CustomerNo, CustomerTypeCode: enterpriseType,
	}).Limit(1).Scan(&change); err != nil {
		return nil, gerror.Wrap(err, "load enterprise legal change request")
	}
	if change == nil ||
		(change.StatusCode != legalChangeStatusSubmitted && change.StatusCode != legalChangeStatusUnderReview) ||
		(change.StatusCode == legalChangeStatusSubmitted && change.ReviewVerificationCaseNo != "") ||
		(change.StatusCode == legalChangeStatusUnderReview &&
			!validBusinessNo(change.ReviewVerificationCaseNo, 40)) ||
		change.SubmittedAt == nil || latest.OccurredAt.Before(*change.SubmittedAt) ||
		profile.VerificationStatusCode != string(identitycap.StatusVerified) ||
		change.CurrentVerificationStatusCode != string(identitycap.StatusVerified) ||
		change.CurrentVerificationCaseNo != profile.VerificationCaseNo ||
		change.CurrentProfileVersion != profile.Version {
		return nil, gerror.New("enterprise legal change request is not accessible")
	}
	preparation.LegalChangeRequestNo = change.RequestNo
	preparation.ReviewCaseNo = change.ReviewVerificationCaseNo
	preparation.Reverification = true
	return preparation, nil
}

func (s *serviceImpl) reserveConsentNumber(
	ctx context.Context,
	numberType string,
	purpose string,
	customerNo string,
	idempotencyKey string,
) (string, error) {
	request := do.BusinessNumber{
		TenantId: idccap.DefaultTenantID, NumberType: numberType,
		ReservationScopeDigest: s.personalConsentDigest(purpose+"-scope-v1", customerNo),
		IdempotencyKeyDigest: s.personalConsentDigest(
			purpose+"-idempotency-v1", idempotencyKey,
		),
	}
	if existing, err := loadPersonalConsentNumber(ctx, request); err != nil || existing != "" {
		return existing, err
	}
	for range personalConsentNumberAttempts {
		candidate, err := newPersonalConsentNumber(s.random)
		if err != nil {
			return "", err
		}
		inserted, err := dao.BusinessNumber.Ctx(ctx).Data(do.BusinessNumber{
			TenantId: idccap.DefaultTenantID, NumberType: numberType, BusinessNo: candidate,
			ReservationScopeDigest: request.ReservationScopeDigest,
			IdempotencyKeyDigest:   request.IdempotencyKeyDigest,
		}).InsertIgnore()
		if err != nil {
			return "", gerror.Wrap(err, "reserve enterprise identity consent number")
		}
		affected, err := inserted.RowsAffected()
		if err != nil {
			return "", gerror.Wrap(err, "read enterprise identity consent number reservation")
		}
		if affected == 1 {
			return candidate, nil
		}
		if existing, loadErr := loadPersonalConsentNumber(ctx, request); loadErr != nil || existing != "" {
			return existing, loadErr
		}
	}
	return "", gerror.New("enterprise identity consent number collision retry limit reached")
}

func matchesEnterpriseConsent(
	row *entity.CustomerConsent,
	input identitycap.EnterpriseConsentInput,
	actorUserID int,
) bool {
	return row != nil && row.OccurredAt != nil && row.ConsentTypeCode == enterpriseConsentType &&
		row.PolicyReference == identitycap.EnterpriseConsentPolicyReference &&
		row.PolicyVersion == input.PolicyVersion && row.ActionCode == consentGranted &&
		row.SourceCode == personalConsentSource && row.Locale == input.Locale &&
		row.RecordedBy == actorUserID
}

// requirePersonalVerification retains the personal-first prerequisite for new enterprise submissions.
func (s *serviceImpl) requirePersonalVerification(ctx context.Context, customerNo string) error {
	var profile *entity.CustomerIndividualProfile
	if err := s.database.Model(dao.CustomerIndividualProfile.Table()).Ctx(ctx).Where(do.CustomerIndividualProfile{TenantId: idccap.DefaultTenantID, CustomerNo: customerNo}).Scan(&profile); err != nil {
		return err
	}
	if profile == nil || profile.IdentityStatusCode != string(identitycap.StatusVerified) || profile.IdentityVerifiedAt == nil || (profile.IdentityExpiresAt != nil && !profile.IdentityExpiresAt.After(time.Now().UTC())) {
		return gerror.New("verified personal identity is required before enterprise verification")
	}
	return nil
}

// prepareEnterpriseSubject applies the personal-first prerequisite to writes, including a staff-cleared enterprise.
func (s *serviceImpl) prepareEnterpriseSubject(ctx context.Context) (*identitycap.EnterpriseSubject, error) {
	subject, err := s.CurrentEnterprise(ctx)
	if err != nil {
		return nil, err
	}
	if subject.ManualVerification {
		if err = s.requirePersonalVerification(ctx, subject.CustomerNo); err != nil {
			return nil, err
		}
	}
	return subject, nil
}

// validEnterprisePreparationInput preserves the preparation entry validation
// independently of database reads and consent/ownership state checks.
func validEnterprisePreparationInput(ctx context.Context, consentNo, legalChangeRequestNo string) bool {
	return ctx != nil && ctx.Err() == nil && validBusinessNo(consentNo, 40) &&
		(legalChangeRequestNo == "" || validBusinessNo(legalChangeRequestNo, 40))
}
