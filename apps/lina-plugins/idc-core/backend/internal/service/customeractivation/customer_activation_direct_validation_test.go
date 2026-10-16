// Direct validation tests cover optional contacts, normalization and retry semantics.

package customeractivation

import "testing"

func TestNormalizeDirectCreateContacts(t *testing.T) {
	base := CreateInput{CustomerType: "individual", DisplayName: "Customer", CountryCode: "CN", PreferredLocale: "zh-CN", Reason: "Administrator created customer account", IdempotencyKey: "create-contact-test-0001"}
	for _, tc := range []struct {
		name, email, phone, calling string
		invalid                     bool
	}{
		{"email", " User@Example.test ", "", "", false},
		{"phone", "", "13800138000", "+86", false},
		{"both", "user@example.test", "13800138000", "+86", false},
		{"neither", "", "", "", true},
		{"invalid email", "bad", "13800138000", "+86", true},
		{"invalid phone", "user@example.test", "bad", "+86", true},
		{"missing calling code", "", "13800138000", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			input.Email = tc.email
			input.Phone = tc.phone
			input.PhoneCountryCode = tc.calling
			result, err := normalizeDirectCreate(input)
			if (err != nil) != tc.invalid {
				t.Fatalf("invalid=%v err=%v", tc.invalid, err)
			}
			if !tc.invalid && tc.email != "" && result.Email != "user@example.test" {
				t.Fatal("email was not normalized")
			}
			if !tc.invalid && result.Phone != tc.phone {
				t.Fatal("phone was lost")
			}
		})
	}
	first := base
	first.Email = "user@example.test"
	first.Phone = "13800138000"
	first.PhoneCountryCode = "+86"
	second := first
	second.Phone = "13800138001"
	a, err := normalizeDirectCreate(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := normalizeDirectCreate(second)
	if err != nil {
		t.Fatal(err)
	}
	if a.fingerprint == b.fingerprint {
		t.Fatal("secondary contact must affect retry fingerprint")
	}
}
