// Direct account validation accepts email, phone or both without changing invitation semantics.

package customeractivation

import (
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"net/mail"
	"strings"
)

func normalizeDirectCreate(input CreateInput) (normalizedCreate, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	phone := strings.ReplaceAll(strings.TrimSpace(input.Phone), " ", "")
	callingCode := strings.TrimSpace(input.PhoneCountryCode)
	primary := input
	primary.Email = email
	primary.Phone = phone
	primary.PhoneCountryCode = callingCode
	if email != "" {
		parsed, err := mail.ParseAddress(email)
		if err != nil || parsed.Address != email || len(email) > 320 {
			return normalizedCreate{}, bizerr.NewCode(customer.CodeProfileInvalid)
		}
		primary.Phone = ""
		primary.PhoneCountryCode = ""
	}
	if phone != "" && (!activationCallingPattern.MatchString(callingCode) || !activationPhonePattern.MatchString(phone)) {
		return normalizedCreate{}, bizerr.NewCode(customer.CodeProfileInvalid)
	}
	value, err := normalizeCreate(primary)
	if err != nil {
		return normalizedCreate{}, err
	}
	value.Email = email
	value.Phone = phone
	if phone != "" {
		value.PhoneCountryCode = callingCode
	}
	value.fingerprint = digest(createFingerprint(value) + "\x00" + email + "\x00" + callingCode + "\x00" + phone)
	return value, nil
}
