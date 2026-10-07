package invoiceprofile

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
	"unicode/utf8"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/identitycap"
)

type normalizedFields struct {
	TitleTypeCode     string
	InvoiceTitle      string
	TaxRegistrationNo string
	RegisteredAddress string
	RegisteredPhone   string
	BankName          string
}

func currentCustomer(ctx context.Context, reader identitycap.CurrentCustomerReader) (*identitycap.CustomerSubject, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	subject, err := reader.CurrentCustomer(ctx)
	if err != nil || subject == nil || !validBusinessNo(subject.CustomerNo) || subject.ActorUserID <= 0 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return subject, nil
}

func normalizeFields(input normalizedFields) (normalizedFields, error) {
	result := normalizedFields{
		TitleTypeCode: strings.TrimSpace(input.TitleTypeCode), InvoiceTitle: strings.TrimSpace(input.InvoiceTitle),
		TaxRegistrationNo: strings.TrimSpace(input.TaxRegistrationNo), RegisteredAddress: strings.TrimSpace(input.RegisteredAddress),
		RegisteredPhone: strings.TrimSpace(input.RegisteredPhone), BankName: strings.TrimSpace(input.BankName),
	}
	if result.TitleTypeCode != TitleTypeIndividual && result.TitleTypeCode != TitleTypeEnterprise {
		return normalizedFields{}, bizerr.NewCode(CodeInvalid)
	}
	if !validText(result.InvoiceTitle, 512, true) || !validText(result.TaxRegistrationNo, 128, false) ||
		!validText(result.RegisteredAddress, 512, false) || !validText(result.RegisteredPhone, 32, false) ||
		!validText(result.BankName, 256, false) {
		return normalizedFields{}, bizerr.NewCode(CodeInvalid)
	}
	if result.TitleTypeCode == TitleTypeIndividual {
		if result.TaxRegistrationNo != "" || result.RegisteredAddress != "" || result.RegisteredPhone != "" || result.BankName != "" {
			return normalizedFields{}, bizerr.NewCode(CodeInvalid)
		}
	} else if result.TaxRegistrationNo == "" {
		return normalizedFields{}, bizerr.NewCode(CodeInvalid)
	}
	return result, nil
}

func normalizeBankAccount(value []byte, required bool) ([]byte, error) {
	normalized := bytes.TrimSpace(value)
	if len(normalized) == 0 {
		if required {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		return nil, nil
	}
	if !utf8.Valid(normalized) || utf8.RuneCount(normalized) > 128 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	for remaining := normalized; len(remaining) > 0; {
		character, size := utf8.DecodeRune(remaining)
		if size == 0 || unicode.IsControl(character) {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		remaining = remaining[size:]
	}
	return append([]byte(nil), normalized...), nil
}

func validText(value string, maximumRunes int, required bool) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximumRunes || (required && value == "") {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validBusinessNo(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > 40 {
		return false
	}
	for _, character := range []byte(value) {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func validIdempotencyKey(value string) bool {
	return value == strings.TrimSpace(value) && len(value) >= 16 && len(value) <= 128 && validText(value, 128, true)
}

func profileNumber(key []byte, customerNo, idempotencyKey string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(customerNo))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(idempotencyKey))
	return "ivp_" + hex.EncodeToString(mac.Sum(nil)[:16])
}
