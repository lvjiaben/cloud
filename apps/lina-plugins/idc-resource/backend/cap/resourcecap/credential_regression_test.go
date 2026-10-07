// This file verifies credential scanner regressions through every public value boundary.

package resourcecap

import (
	"encoding/base64"
	"encoding/hex"
	"testing"
)

// TestPublicValidationRejectsEncodedCredentialRegressions applies every
// hostile representation to JSON, evidence, failure, and log values.
func TestPublicValidationRejectsEncodedCredentialRegressions(t *testing.T) {
	plain := []byte("apiKey=must-not-pass")
	invalidPrefixed := base64.RawStdEncoding.EncodeToString(append([]byte{0xff}, plain...))
	base64Split := interleavePublicFormatRune(base64.RawStdEncoding.EncodeToString(plain), 4)
	hexSplit := interleavePublicFormatRune(hex.EncodeToString(plain), 4)
	mixed := hex.EncodeToString(plain)
	mixed = base64.RawURLEncoding.EncodeToString([]byte(mixed))
	mixed = hex.EncodeToString([]byte(mixed))
	mixed = base64.RawStdEncoding.EncodeToString([]byte(mixed))
	overBudget := "region=cn-test"
	for layer := 0; layer < 5; layer++ {
		if layer%2 == 0 {
			overBudget = hex.EncodeToString([]byte(overBudget))
		} else {
			overBudget = base64.RawURLEncoding.EncodeToString([]byte(overBudget))
		}
	}

	for _, hostile := range []string{invalidPrefixed, base64Split, hexSplit, mixed, overBudget} {
		for _, path := range credentialPublicValidationPaths() {
			t.Run(path.name, func(t *testing.T) {
				if err := path.validate(hostile); err == nil {
					t.Fatalf("public validation accepted encoded credential %q", hostile)
				}
			})
		}
	}
}

// TestStructuredValidationRejectsEncodedSensitiveKeys verifies JSON, log, and
// metric key names cannot hide credential semantics behind an encoding.
func TestStructuredValidationRejectsEncodedSensitiveKeys(t *testing.T) {
	base64Key := base64.RawStdEncoding.EncodeToString([]byte("apiToken"))
	hexKey := hex.EncodeToString([]byte("apiToken"))
	for _, settings := range [][]byte{
		[]byte(`{"` + base64Key + `":"opaque"}`),
		[]byte(`{"` + hexKey + `":"opaque"}`),
	} {
		config := ProviderConfig{
			ChannelRef: "channel-test", Endpoint: "https://provider.example.test", Settings: settings,
		}
		if err := config.Validate(); err == nil {
			t.Fatal("encoded sensitive JSON key was accepted")
		}
	}
	if err := ValidateLogEntry(LogLevelInfo, "provider request completed", LogField{
		Key: base64Key, Value: "opaque",
	}); err == nil {
		t.Fatal("encoded sensitive log key was accepted")
	}
	if err := ValidateMetric("provider.requests", MetricLabel{
		Name: base64Key, Value: "opaque",
	}); err == nil {
		t.Fatal("encoded sensitive metric key was accepted")
	}
}

// interleavePublicFormatRune mirrors an attacker splitting an encoded token
// across invisible Unicode format runes at a public validation boundary.
func interleavePublicFormatRune(value string, width int) string {
	result := ""
	for index := 0; index < len(value); index += width {
		end := index + width
		if end > len(value) {
			end = len(value)
		}
		if result != "" {
			result += "\u200b"
		}
		result += value[index:end]
	}
	return result
}
