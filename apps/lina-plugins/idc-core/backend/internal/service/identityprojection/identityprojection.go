// Package identityprojection owns the core customer identity payment-gate projection.
package identityprojection

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

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
	individualType       = "individual"
	sensitiveConsentType = "sensitive_personal_information"
	consentGranted       = "granted"
	maximumLegalRunes    = 256
	identityUniqueFence  = "uk_plugin_idc_core_ind_profile_identity_digest"
)

type serviceImpl struct {
	database  gdb.DB
	principal principal.Reader
	lookupKey []byte
	random    io.Reader
}

var _ identitycap.Service = (*serviceImpl)(nil)

// New creates the source-only identity projection from explicit owner dependencies.
func New(database gdb.DB, principalReader principal.Reader, lookupKey []byte) (identitycap.Service, error) {
	if nilDependency(database) || nilDependency(principalReader) || len(lookupKey) != sha256.Size {
		clear(lookupKey)
		return nil, gerror.New("identity projection dependencies are invalid")
	}
	return &serviceImpl{
		database: database, principal: principalReader,
		lookupKey: append([]byte(nil), lookupKey...),
		random:    rand.Reader,
	}, nil
}

// PreparePersonal proves current ownership and the latest separate consent.
func (s *serviceImpl) PreparePersonal(
	ctx context.Context,
	consentNo string,
) (*identitycap.PersonalSubject, error) {
	if ctx == nil || ctx.Err() != nil || !validBusinessNo(consentNo, 40) {
		return nil, gerror.New("personal identity preparation is not accessible")
	}
	subject, err := s.CurrentPersonal(ctx)
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
			ConsentTypeCode: sensitiveConsentType,
		}).OrderDesc("occurred_at").OrderDesc("id").Limit(1).Scan(&latest)
	if err != nil {
		return nil, gerror.Wrap(err, "load latest sensitive information consent")
	}
	if latest == nil || latest.ConsentNo != consentNo || latest.ActionCode != consentGranted ||
		latest.PolicyReference != identitycap.PersonalConsentPolicyReference ||
		latest.PolicyVersion != identitycap.PersonalConsentPolicyVersion || latest.OccurredAt == nil {
		return nil, gerror.New("personal identity consent is not effective")
	}
	return subject, nil
}

// CurrentPersonal proves current ownership without accepting a customer number from the caller.
func (s *serviceImpl) CurrentPersonal(ctx context.Context) (*identitycap.PersonalSubject, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("personal identity subject is not accessible")
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer ||
		audience.Status != principal.AudienceStatusActive || audience.UserID <= 0 ||
		!validBusinessNo(audience.CustomerNo, 40) {
		return nil, gerror.New("personal identity preparation is not accessible")
	}
	var subject *entity.CustomerSubject
	err = s.database.Model(dao.CustomerSubject.Table()).Ctx(ctx).
		Fields("tenant_id", "customer_no", "customer_type_code", "status_code").
		Where(do.CustomerSubject{TenantId: idccap.DefaultTenantID, CustomerNo: audience.CustomerNo}).
		Limit(1).Scan(&subject)
	if err != nil {
		return nil, gerror.Wrap(err, "load personal identity subject")
	}
	if subject == nil || (subject.CustomerTypeCode != individualType &&
		subject.CustomerTypeCode != enterpriseType) ||
		!eligibleCustomerStatus(subject.StatusCode) {
		return nil, gerror.New("personal identity preparation is not accessible")
	}
	var profile *entity.CustomerIndividualProfile
	if err := s.database.Model(dao.CustomerIndividualProfile.Table()).Ctx(ctx).Fields("manual_verification", "identity_status_code", "updated_at").Where(do.CustomerIndividualProfile{TenantId: idccap.DefaultTenantID, CustomerNo: audience.CustomerNo}).Limit(1).Scan(&profile); err != nil || profile == nil {
		return nil, gerror.New("personal identity projection is unavailable")
	}
	return &identitycap.PersonalSubject{
		ManualVerification: profile.ManualVerification, VerificationStatus: identitycap.Status(profile.IdentityStatusCode), VerificationUpdatedAt: profile.UpdatedAt,
		CustomerNo:         audience.CustomerNo,
		PrincipalReference: "usr:" + strconv.Itoa(audience.UserID),
		ActorUserID:        audience.UserID,
	}, nil
}

// CurrentCustomer proves the shared current customer for customer-owned resources.
func (s *serviceImpl) CurrentCustomer(ctx context.Context) (*identitycap.CustomerSubject, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("customer identity subject is not accessible")
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer ||
		audience.Status != principal.AudienceStatusActive || audience.UserID <= 0 ||
		!validBusinessNo(audience.CustomerNo, 40) {
		return nil, gerror.New("customer identity subject is not accessible")
	}
	var subject *entity.CustomerSubject
	err = s.database.Model(dao.CustomerSubject.Table()).Ctx(ctx).
		Fields("tenant_id", "customer_no", "customer_type_code", "status_code").
		Where(do.CustomerSubject{TenantId: idccap.DefaultTenantID, CustomerNo: audience.CustomerNo}).
		Limit(1).Scan(&subject)
	if err != nil {
		return nil, gerror.Wrap(err, "load customer identity subject")
	}
	if subject == nil || (subject.CustomerTypeCode != individualType &&
		subject.CustomerTypeCode != enterpriseType) || !eligibleCustomerStatus(subject.StatusCode) {
		return nil, gerror.New("customer identity subject is not accessible")
	}
	return &identitycap.CustomerSubject{
		CustomerNo: audience.CustomerNo, CustomerTypeCode: subject.CustomerTypeCode,
		PrincipalReference: "usr:" + strconv.Itoa(audience.UserID), ActorUserID: audience.UserID,
	}, nil
}

// ProjectPersonal applies a monotonic case projection and clears caller-owned raw buffers.
func (s *serviceImpl) ProjectPersonal(
	ctx context.Context,
	input identitycap.PersonalProjectionInput,
) (*identitycap.PersonalProjection, error) {
	defer clear(input.LegalName)
	defer clear(input.IdentityNumber)
	if err := validateProjectionInput(input); err != nil {
		return nil, err
	}
	var projection *identitycap.PersonalProjection
	var duplicateFallback *identitycap.PersonalProjection
	hasAmbientTransaction := gdb.TXFromCtx(ctx, dao.CustomerIndividualProfile.Group()) != nil
	err := dao.CustomerIndividualProfile.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		result, fallback, applyErr := s.applyProjection(ctx, input)
		duplicateFallback = fallback
		if applyErr == nil {
			projection = result
		}
		return applyErr
	})
	if err != nil {
		if duplicateFallback != nil && strings.Contains(err.Error(), identityUniqueFence) {
			if hasAmbientTransaction {
				return nil, gerror.Wrap(
					identitycap.ErrDuplicateIdentity,
					"personal identity uniqueness fence rejected projection",
				)
			}
			return duplicateFallback, nil
		}
		return nil, err
	}
	return projection, nil
}

func (s *serviceImpl) applyProjection(
	ctx context.Context,
	input identitycap.PersonalProjectionInput,
) (*identitycap.PersonalProjection, *identitycap.PersonalProjection, error) {
	var subject *entity.CustomerSubject
	if err := dao.CustomerSubject.Ctx(ctx).
		Fields("tenant_id", "customer_no", "customer_type_code", "status_code").
		Where(do.CustomerSubject{TenantId: idccap.DefaultTenantID, CustomerNo: input.CustomerNo}).
		LockUpdate().Limit(1).Scan(&subject); err != nil {
		return nil, nil, gerror.Wrap(err, "lock personal identity subject")
	}
	if subject == nil || (subject.CustomerTypeCode != individualType &&
		subject.CustomerTypeCode != enterpriseType) ||
		!eligibleCustomerStatus(subject.StatusCode) {
		return nil, nil, gerror.New("personal identity projection is not accessible")
	}
	var profile *entity.CustomerIndividualProfile
	if err := dao.CustomerIndividualProfile.Ctx(ctx).
		Where(do.CustomerIndividualProfile{
			TenantId: idccap.DefaultTenantID, CustomerNo: input.CustomerNo,
		}).LockUpdate().Limit(1).Scan(&profile); err != nil {
		return nil, nil, gerror.Wrap(err, "lock personal identity projection")
	}
	if profile == nil || profile.CustomerTypeCode != individualType || profile.Version <= 0 {
		return nil, nil, gerror.New("personal identity projection is unavailable")
	}
	if profile.ManualVerification && profile.IdentityStatusCode == string(identitycap.StatusVerified) {
		return nil, nil, gerror.New("personal identity was manually approved; clear verification before starting another case")
	}
	currentStatus := identitycap.Status(profile.IdentityStatusCode)
	if profile.IdentityCaseNo != "" && profile.IdentityCaseNo != input.CaseNo &&
		(currentStatus == identitycap.StatusPending || currentStatus == identitycap.StatusVerified) {
		return nil, nil, gerror.New("personal identity projection belongs to another active case")
	}
	if profile.IdentityCaseNo == input.CaseNo && currentStatus == input.Status {
		replayed, err := s.replayProjection(profile, input)
		return replayed, nil, err
	}
	if input.Status != identitycap.StatusPending &&
		(profile.IdentityCaseNo != input.CaseNo || currentStatus != identitycap.StatusPending) {
		return nil, nil, gerror.New("personal identity projection transition is stale")
	}

	nextVersion := profile.Version + 1
	update := do.CustomerIndividualProfile{
		IdentityCaseNo: input.CaseNo, IdentityStatusCode: string(input.Status),
		Version: nextVersion, ManualVerification: false,
	}
	projection := &identitycap.PersonalProjection{Status: input.Status, Version: nextVersion}
	var duplicateFallback *identitycap.PersonalProjection
	if input.Status == identitycap.StatusVerified {
		digest, err := s.identityDigest(input.IdentityNumber)
		if err != nil {
			return nil, nil, err
		}
		duplicate, err := duplicateIdentity(ctx, input.CustomerNo, digest)
		if err != nil {
			return nil, nil, err
		}
		if duplicate {
			return &identitycap.PersonalProjection{
				Status: currentStatus, Version: profile.Version, DuplicateRisk: true,
			}, nil, nil
		}
		duplicateFallback = &identitycap.PersonalProjection{
			Status: currentStatus, Version: profile.Version, DuplicateRisk: true,
		}
		verifiedAt := input.VerifiedAt.UTC().Truncate(time.Microsecond)
		update.LegalName = string(input.LegalName)
		update.LegalNameMasked = input.MaskedName
		update.IdentityIdentifierMasked = input.MaskedIdentityNo
		update.IdentityLookupDigest = digest
		update.IdentityVerifiedAt = &verifiedAt
		update.IdentityExpiresAt = normalizedTime(input.ExpiresAt)
		projection.IdentityLookupDigest = digest
	} else {
		update.LegalName = ""
		update.LegalNameMasked = ""
		update.IdentityIdentifierMasked = ""
		update.IdentityLookupDigest = ""
		update.IdentityExpiresAt = normalizedTime(input.ExpiresAt)
	}
	result, err := dao.CustomerIndividualProfile.Ctx(ctx).
		Where(do.CustomerIndividualProfile{
			Id: profile.Id, TenantId: idccap.DefaultTenantID, Version: profile.Version,
		}).Data(update).Update()
	if err != nil {
		return nil, duplicateFallback, gerror.Wrap(err, "update personal identity projection")
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return nil, nil, gerror.New("personal identity projection lost its version fence")
	}
	return projection, nil, nil
}

func (s *serviceImpl) replayProjection(
	profile *entity.CustomerIndividualProfile,
	input identitycap.PersonalProjectionInput,
) (*identitycap.PersonalProjection, error) {
	result := &identitycap.PersonalProjection{
		Status: input.Status, Version: profile.Version, Reused: true,
	}
	if input.Status != identitycap.StatusVerified {
		return result, nil
	}
	digest, err := s.identityDigest(input.IdentityNumber)
	if err != nil {
		return nil, err
	}
	if profile.LegalName != string(input.LegalName) || profile.LegalNameMasked != input.MaskedName ||
		profile.IdentityIdentifierMasked != input.MaskedIdentityNo ||
		!hmac.Equal([]byte(profile.IdentityLookupDigest), []byte(digest)) {
		return nil, gerror.New("personal identity verified replay changed its identity")
	}
	result.IdentityLookupDigest = digest
	return result, nil
}

func duplicateIdentity(ctx context.Context, customerNo, digest string) (bool, error) {
	count, err := dao.CustomerIndividualProfile.Ctx(ctx).
		Where(do.CustomerIndividualProfile{
			TenantId: idccap.DefaultTenantID, IdentityLookupDigest: digest,
		}).WhereNot("customer_no", customerNo).Count()
	if err != nil {
		return false, gerror.Wrap(err, "check duplicate personal identity")
	}
	return count > 0, nil
}

func (s *serviceImpl) identityDigest(identityNumber []byte) (string, error) {
	mac := hmac.New(sha256.New, s.lookupKey)
	if _, err := mac.Write([]byte("idc/personal-identity/v1\x00")); err != nil {
		return "", gerror.Wrap(err, "write personal identity digest purpose")
	}
	if _, err := mac.Write(identityNumber); err != nil {
		return "", gerror.Wrap(err, "write personal identity digest value")
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func validateProjectionInput(input identitycap.PersonalProjectionInput) error {
	if !validBusinessNo(input.CustomerNo, 40) || !validBusinessNo(input.CaseNo, 40) {
		return gerror.New("personal identity projection input is invalid")
	}
	switch input.Status {
	case identitycap.StatusPending, identitycap.StatusRejected, identitycap.StatusExpired:
		if len(input.LegalName) != 0 || len(input.IdentityNumber) != 0 ||
			input.MaskedName != "" || input.MaskedIdentityNo != "" || input.VerifiedAt != nil {
			return gerror.New("personal identity non-verified projection contains identity data")
		}
	case identitycap.StatusVerified:
		if input.VerifiedAt == nil || input.VerifiedAt.IsZero() ||
			!validIdentityText(input.LegalName, maximumLegalRunes) ||
			!validIdentityText(input.IdentityNumber, 128) ||
			!validMaskedValue(input.MaskedName, 256) ||
			!validMaskedValue(input.MaskedIdentityNo, 128) {
			return gerror.New("personal identity verified projection is invalid")
		}
	default:
		return gerror.New("personal identity projection status is invalid")
	}
	return nil
}

func validBusinessNo(value string, maximum int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maximum {
		return false
	}
	for _, character := range []byte(value) {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func validIdentityText(value []byte, maximumRunes int) bool {
	if !utf8.Valid(value) || len(value) == 0 || !bytes.Equal(value, bytes.TrimSpace(value)) ||
		utf8.RuneCount(value) > maximumRunes {
		return false
	}
	for remaining := value; len(remaining) > 0; {
		character, size := utf8.DecodeRune(remaining)
		if size == 0 || unicode.IsControl(character) {
			return false
		}
		remaining = remaining[size:]
	}
	return true
}

func validMaskedValue(value string, maximumRunes int) bool {
	return utf8.ValidString(value) && value == strings.TrimSpace(value) && value != "" &&
		utf8.RuneCountInString(value) <= maximumRunes
}

func eligibleCustomerStatus(value string) bool {
	switch value {
	case "pending_activation", "active", "restricted":
		return true
	default:
		return false
	}
}

func normalizedTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	normalized := value.UTC().Truncate(time.Microsecond)
	return &normalized
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
