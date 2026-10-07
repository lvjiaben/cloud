// This file validates bounded customer identifiers and self-service profile values.

package customer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"

	"lina-core/pkg/bizerr"
)

const (
	customerNumberPrefix       = "CUS_"
	customerNumberDigestLength = 32
	maximumDisplayNameRunes    = 256
	maximumIdempotencyKeyRunes = 256
	idempotencyScopeUpdateOwn  = "customer.profile.update"
)

type normalizedUpdate struct {
	customerNo      string
	displayName     string
	searchName      string
	countryCode     string
	preferredLocale string
	expectedVersion int64
	idempotencyKey  string
}

type updateFingerprint struct {
	Version         int    `json:"version"`
	CustomerNo      string `json:"customer_no"`
	DisplayName     string `json:"display_name"`
	CountryCode     string `json:"country_code"`
	PreferredLocale string `json:"preferred_locale"`
	ExpectedVersion int64  `json:"expected_version"`
}

func normalizeUpdate(input UpdateOwnBasicsInput) (normalizedUpdate, error) {
	displayName := strings.Join(strings.Fields(input.DisplayName), " ")
	countryCode := strings.ToUpper(strings.TrimSpace(input.CountryCode))
	preferredLocale := strings.TrimSpace(input.PreferredLocale)
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if !validCustomerNo(input.CustomerNo) ||
		!validDisplayName(displayName) ||
		!validCountryCode(countryCode) ||
		!validLocale(preferredLocale) ||
		input.ExpectedVersion <= 0 ||
		!validBoundedText(idempotencyKey, maximumIdempotencyKeyRunes) {
		return normalizedUpdate{}, bizerr.NewCode(CodeProfileInvalid)
	}
	return normalizedUpdate{
		customerNo:      input.CustomerNo,
		displayName:     displayName,
		searchName:      strings.ToLower(displayName),
		countryCode:     countryCode,
		preferredLocale: preferredLocale,
		expectedVersion: input.ExpectedVersion,
		idempotencyKey:  idempotencyKey,
	}, nil
}

func validCustomerNo(value string) bool {
	if len(value) != len(customerNumberPrefix)+customerNumberDigestLength ||
		!strings.HasPrefix(value, customerNumberPrefix) {
		return false
	}
	for _, character := range value[len(customerNumberPrefix):] {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func validDisplayName(value string) bool {
	return validBoundedText(value, maximumDisplayNameRunes)
}

func validBoundedText(value string, maximumRunes int) bool {
	if value == "" || value != strings.TrimSpace(value) || !utf8.ValidString(value) {
		return false
	}
	count := utf8.RuneCountInString(value)
	if count <= 0 || count > maximumRunes {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validCountryCode(value string) bool {
	return len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func validLocale(value string) bool {
	return value == "zh-CN" || value == "en-US"
}

func validType(value Type) bool {
	return value == TypeIndividual || value == TypeEnterprise
}

func validStatus(value Status) bool {
	switch value {
	case StatusPendingActivation, StatusActive, StatusRestricted, StatusDisabled, StatusAnonymized:
		return true
	default:
		return false
	}
}

func validVerificationStatus(value VerificationStatus) bool {
	switch value {
	case VerificationUnverified, VerificationPending, VerificationVerified,
		VerificationRejected, VerificationExpired, VerificationRevoked:
		return true
	default:
		return false
	}
}

func digest(domain string, value string) string {
	sum := sha256.Sum256([]byte(domain + "\x00" + value))
	return hex.EncodeToString(sum[:])
}

func validDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func requestFingerprint(input normalizedUpdate) (string, error) {
	document, err := json.Marshal(updateFingerprint{
		Version:         1,
		CustomerNo:      input.customerNo,
		DisplayName:     input.displayName,
		CountryCode:     input.countryCode,
		PreferredLocale: input.preferredLocale,
		ExpectedVersion: input.expectedVersion,
	})
	if err != nil {
		return "", err
	}
	return digest("customer-profile-request-v1", string(document)), nil
}
