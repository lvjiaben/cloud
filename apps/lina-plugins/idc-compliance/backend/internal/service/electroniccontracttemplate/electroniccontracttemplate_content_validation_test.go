package electroniccontracttemplate

import (
	"bytes"
	"testing"

	"lina-core/pkg/bizerr"
)

func TestSanitizeOnlineHTMLKeepsOnlySupportedContractContent(t *testing.T) {
	t.Parallel()
	source := `<h1 style="text-align:center">Agreement</h1>` +
		`<p>{{customer_name}} and {{merchant_name}}</p>` +
		`<table><tbody><tr><td>Service</td><td>{{service_name}}</td></tr></tbody></table>` +
		`<p><a href="https://example.com/terms">Terms</a></p>` +
		`<div class="page-break"></div>`
	cleaned, err := sanitizeOnlineHTML(source)
	if err != nil {
		t.Fatalf("sanitize supported online HTML: %v", err)
	}
	for _, expected := range []string{
		`text-align:center`, `{{customer_name}}`, `{{merchant_name}}`,
		`<table>`, `https://example.com/terms`, `class="page-break"`,
	} {
		if !bytes.Contains([]byte(cleaned), []byte(expected)) {
			t.Fatalf("sanitized HTML missing %q: %s", expected, cleaned)
		}
	}
}

func TestSanitizeOnlineHTMLRejectsUnsupportedOrUnsafeContent(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"image":           `<p>Body</p><img src="https://example.com/a.png">`,
		"script":          `<p>Body</p><script>alert(1)</script>`,
		"custom variable": `<p>{{custom_value}}</p>`,
		"unsafe link":     `<p><a href="javascript:alert(1)">bad</a></p>`,
		"unsafe style":    `<p style="position:fixed">bad</p>`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := sanitizeOnlineHTML(source); !bizerr.Is(err, CodeContentInvalid) {
				t.Fatalf("sanitize error=%v, want content invalid", err)
			}
		})
	}
}

func TestNormalizeFieldsEnforcesPartiesScopeAndPageBounds(t *testing.T) {
	t.Parallel()
	valid := []FieldPlacement{
		{FieldTypeCode: FieldTypeTextVariable, PartyCode: PartyCustomer, VariableCode: VariableCustomerName, PageNo: 1, PositionXPPM: 10_000, PositionYPPM: 10_000, WidthPPM: 200_000, HeightPPM: 50_000},
		{FieldTypeCode: FieldTypeSignature, PartyCode: PartyCustomer, PageNo: 1, PositionXPPM: 10_000, PositionYPPM: 700_000, WidthPPM: 250_000, HeightPPM: 80_000},
		{FieldTypeCode: FieldTypeEnterpriseSeal, PartyCode: PartyMerchant, PageNo: 2, PositionXPPM: 700_000, PositionYPPM: 700_000, WidthPPM: 150_000, HeightPPM: 150_000},
	}
	normalized, err := normalizeFields(valid, ScopePersonal, 2)
	if err != nil || len(normalized) != len(valid) || !signingPositionsReady(normalized, ScopePersonal) {
		t.Fatalf("normalize valid fields=%+v err=%v", normalized, err)
	}
	for index, field := range normalized {
		if field.SequenceNo != index+1 || field.FieldNo != "" {
			t.Fatalf("server sequence normalization failed: %+v", normalized)
		}
	}

	invalidCases := map[string][]FieldPlacement{
		"personal customer seal": {{FieldTypeCode: FieldTypeEnterpriseSeal, PartyCode: PartyCustomer, PageNo: 1, WidthPPM: 100_000, HeightPPM: 100_000}},
		"wrong variable party":   {{FieldTypeCode: FieldTypeTextVariable, PartyCode: PartyCustomer, VariableCode: VariableAmount, PageNo: 1, WidthPPM: 100_000, HeightPPM: 50_000}},
		"outside page":           {{FieldTypeCode: FieldTypeSignature, PartyCode: PartyCustomer, PageNo: 3, WidthPPM: 100_000, HeightPPM: 50_000}},
		"outside bounds":         {{FieldTypeCode: FieldTypeSignature, PartyCode: PartyCustomer, PageNo: 1, PositionXPPM: 950_000, WidthPPM: 100_000, HeightPPM: 50_000}},
	}
	for name, fields := range invalidCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, normalizeErr := normalizeFields(fields, ScopePersonal, 2); !bizerr.Is(normalizeErr, CodeContentInvalid) {
				t.Fatalf("normalize error=%v, want content invalid", normalizeErr)
			}
		})
	}
}

func TestContractContentEnvelopeAuthenticatesBodyAndLabel(t *testing.T) {
	t.Parallel()
	key := bytes.Repeat([]byte{0x3a}, 32)
	plaintext := []byte("immutable contract template body")
	envelope, err := encryptContentEnvelope(key, plaintext, "ctpc_example/source")
	if err != nil {
		t.Fatal(err)
	}
	opened, err := decryptContentEnvelope(key, envelope, "ctpc_example/source", 1024)
	if err != nil || !bytes.Equal(opened, plaintext) {
		t.Fatalf("opened=%q err=%v", opened, err)
	}
	if _, err = decryptContentEnvelope(key, envelope, "ctpc_example/preview", 1024); err == nil {
		t.Fatal("wrong content label was accepted")
	}
	envelope[len(envelope)-1] ^= 0xff
	if _, err = decryptContentEnvelope(key, envelope, "ctpc_example/source", 1024); err == nil {
		t.Fatal("tampered envelope was accepted")
	}
}
