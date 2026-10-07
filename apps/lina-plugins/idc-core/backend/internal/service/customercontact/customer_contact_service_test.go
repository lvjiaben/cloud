package customercontact

import (
	"bytes"
	"testing"
)

func TestNormalizeSaveContactDestinations(t *testing.T) {
	input := SaveInput{
		DisplayName: "  Billing contact  ", Email: " BILLING@EXAMPLE.COM ",
		PhoneCountryCode: "+86", Phone: "138 0013 8000", PreferredLocale: "zh-CN",
		IdempotencyKey: "contact-create-0123456789",
	}
	value, err := normalizeSave(input)
	if err != nil {
		t.Fatalf("normalize valid contact failed: %v", err)
	}
	if value.displayName != "Billing contact" || value.email != "billing@example.com" ||
		value.phone != "13800138000" || value.phoneCountryCode != "+86" || value.expectedVersion != 0 {
		t.Fatalf("unexpected normalized contact: %+v", value)
	}

	invalid := []SaveInput{
		{DisplayName: "Missing destination", PreferredLocale: "zh-CN", IdempotencyKey: "contact-create-0123456789"},
		{DisplayName: "Bad email", Email: "not-an-email", PreferredLocale: "zh-CN", IdempotencyKey: "contact-create-0123456789"},
		{DisplayName: "Bad phone", PhoneCountryCode: "86", Phone: "13800138000", PreferredLocale: "zh-CN", IdempotencyKey: "contact-create-0123456789"},
		{DisplayName: "Bad locale", Email: "a@example.com", PreferredLocale: "fr-FR", IdempotencyKey: "contact-create-0123456789"},
		{ContactNo: "CTC_0123456789abcdef0123456789abcdef", ExpectedVersion: 0, DisplayName: "No version", Email: "a@example.com", PreferredLocale: "en-US", IdempotencyKey: "contact-update-0123456789"},
	}
	for index, candidate := range invalid {
		if _, err = normalizeSave(candidate); err == nil {
			t.Fatalf("invalid contact %d was accepted", index)
		}
	}
}

func TestDestinationDigestIsKeyedAndDomainSeparated(t *testing.T) {
	left := &serviceImpl{lookupKey: bytes.Repeat([]byte{0x11}, 32)}
	right := &serviceImpl{lookupKey: bytes.Repeat([]byte{0x22}, 32)}
	email := left.destinationDigest("email", "person@example.com")
	phone := left.destinationDigest("phone", "person@example.com")
	otherKey := right.destinationDigest("email", "person@example.com")
	if len(email) != 64 || email == phone || email == otherKey {
		t.Fatalf("contact lookup digests are not independently keyed: email=%q phone=%q other=%q", email, phone, otherKey)
	}
	if left.destinationDigest("email", "") != "" {
		t.Fatal("empty destination produced a lookup digest")
	}
}
