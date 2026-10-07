package contractsnapshot

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/contractcap"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// MatchCustomerSigner consumes one provider-authoritative signer and refuses
// launch unless it still matches the same current verified core identity.
func (s *serviceImpl) MatchCustomerSigner(
	ctx context.Context,
	expected contractcap.VerifiedIdentity,
	observed *contractcap.ObservedCustomerSigner,
) error {
	if observed != nil {
		defer observed.Clear()
	}
	if s == nil || ctx == nil || ctx.Err() != nil || observed == nil ||
		!validExpectedIdentity(expected) || !validObservedSigner(observed) ||
		expected.CustomerType != observed.CustomerType {
		return gerror.New("contract customer signer does not match the verified identity")
	}
	current, err := s.database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return gerror.Wrap(err, "load contract signer validation time")
	}
	now := current.Time().UTC()
	switch expected.CustomerType {
	case contractcap.CustomerTypeIndividual:
		return s.matchIndividualSigner(ctx, expected, observed, now)
	case contractcap.CustomerTypeEnterprise:
		return s.matchEnterpriseSigner(ctx, expected, observed, now)
	default:
		return gerror.New("contract customer signer type is unsupported")
	}
}

func (s *serviceImpl) matchIndividualSigner(
	ctx context.Context,
	expected contractcap.VerifiedIdentity,
	observed *contractcap.ObservedCustomerSigner,
	now time.Time,
) error {
	var profile *personalRow
	err := s.database.GetScan(ctx, &profile, `
SELECT legal_name, identity_lookup_digest, identity_case_no, identity_status_code,
       identity_verified_at, identity_expires_at, version
FROM plugin_idc_core_customer_individual_profile
WHERE tenant_id = ? AND customer_no = ?
LIMIT 1`, idccap.DefaultTenantID, expected.CustomerNo)
	if err != nil {
		return gerror.Wrap(err, "load current personal contract signer")
	}
	if profile == nil || profile.StatusCode != "verified" ||
		profile.CaseNo != expected.VerificationCaseNo ||
		profile.VerificationVersion != expected.VerificationVersion ||
		profile.VerifiedAt == nil || expired(profile.ExpiresAt, now) ||
		!secureTextEqual([]byte(profile.LegalName), expected.LegalName) ||
		!secureTextEqual(expected.LegalName, observed.LegalName) ||
		!secureDigestEqual(profile.LookupDigest, expected.SubjectLookupDigest) ||
		!secureDigestEqual(
			expected.SubjectLookupDigest,
			s.subjectDigest("idc/personal-identity/v1\x00", observed.PersonalIdentityNumber),
		) {
		return gerror.New("contract personal signer does not match the verified identity")
	}
	return nil
}

func (s *serviceImpl) matchEnterpriseSigner(
	ctx context.Context,
	expected contractcap.VerifiedIdentity,
	observed *contractcap.ObservedCustomerSigner,
	now time.Time,
) error {
	var profile *enterpriseRow
	err := s.database.GetScan(ctx, &profile, `
SELECT legal_name, registration_country_code, registration_identifier,
       registration_identifier_masked, registration_lookup_digest,
       verification_case_no, verification_status_code, verified_at, expires_at, version
FROM plugin_idc_core_customer_enterprise_profile
WHERE tenant_id = ? AND customer_no = ?
LIMIT 1`, idccap.DefaultTenantID, expected.CustomerNo)
	if err != nil {
		return gerror.Wrap(err, "load current enterprise contract signer")
	}
	if profile == nil || profile.StatusCode != "verified" ||
		profile.CaseNo != expected.VerificationCaseNo ||
		profile.VerificationVersion != expected.VerificationVersion ||
		profile.VerifiedAt == nil || expired(profile.ExpiresAt, now) ||
		profile.RegistrationCountryCode != expected.RegistrationCountryCode ||
		observed.RegistrationCountryCode != expected.RegistrationCountryCode ||
		!observed.EnterpriseHandlerAuthorization ||
		!secureTextEqual([]byte(profile.LegalName), expected.LegalName) ||
		!secureTextEqual(expected.LegalName, observed.LegalName) ||
		!secureDigestEqual(profile.LookupDigest, expected.SubjectLookupDigest) ||
		!secureDigestEqual(
			expected.SubjectLookupDigest,
			s.subjectDigest(
				"idc/enterprise-registration/v1\x00",
				observed.RegistrationIdentifier,
			),
		) {
		return gerror.New("contract enterprise signer does not match the verified identity")
	}
	return nil
}

func (s *serviceImpl) subjectDigest(purpose string, value []byte) string {
	if s == nil || len(s.lookupKey) != sha256.Size || purpose == "" || len(value) == 0 {
		return ""
	}
	mac := hmac.New(sha256.New, s.lookupKey)
	_, _ = mac.Write([]byte(purpose))
	_, _ = mac.Write(value)
	return hex.EncodeToString(mac.Sum(nil))
}

func validExpectedIdentity(value contractcap.VerifiedIdentity) bool {
	return orderNoPattern.MatchString(value.CustomerNo) && orderNoPattern.MatchString(value.VerificationCaseNo) &&
		value.VerificationVersion > 0 && !value.VerifiedAt.IsZero() &&
		validSignerText(value.LegalName, 256) && lowerHex64Pattern.MatchString(value.SubjectLookupDigest)
}

func validObservedSigner(value *contractcap.ObservedCustomerSigner) bool {
	if value == nil || !validSignerText(value.LegalName, 256) {
		return false
	}
	switch value.CustomerType {
	case contractcap.CustomerTypeIndividual:
		return validSignerText(value.PersonalIdentityNumber, 128) &&
			len(value.RegistrationIdentifier) == 0 && value.RegistrationCountryCode == "" &&
			!value.EnterpriseHandlerAuthorization
	case contractcap.CustomerTypeEnterprise:
		return len(value.PersonalIdentityNumber) == 0 &&
			validSignerText(value.RegistrationIdentifier, 128) &&
			validCountryCode(value.RegistrationCountryCode)
	default:
		return false
	}
}

func validSignerText(value []byte, maximumRunes int) bool {
	if len(value) == 0 || !utf8.Valid(value) || !bytes.Equal(value, bytes.TrimSpace(value)) ||
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

func validCountryCode(value string) bool {
	if len(value) != 2 || value != strings.ToUpper(value) {
		return false
	}
	return value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func secureTextEqual(left, right []byte) bool {
	return len(left) == len(right) && hmac.Equal(left, right)
}

func secureDigestEqual(left, right string) bool {
	return lowerHex64Pattern.MatchString(left) && lowerHex64Pattern.MatchString(right) &&
		hmac.Equal([]byte(left), []byte(right))
}
