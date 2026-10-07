// This file verifies bounded credential normalization and wrapper decoding.

package credentialscan

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

// TestContainsRejectsEncodedCredentials verifies all supported wrapper forms,
// including the deepest accepted wrapper, still reach the direct scanner.
func TestContainsRejectsEncodedCredentials(t *testing.T) {
	plain := []byte("apiKey=must-not-pass")
	plusPlain := []byte("\u083eapiKey=must-not-pass")
	slashPlain := []byte("\u083fapiKey=must-not-pass")
	for _, value := range []string{
		base64.StdEncoding.EncodeToString(plain),
		base64.RawStdEncoding.EncodeToString(plain),
		base64.URLEncoding.EncodeToString(plain),
		base64.RawURLEncoding.EncodeToString(plain),
		hex.EncodeToString(plain),
		strings.ToUpper(hex.EncodeToString(plain)),
		base64.StdEncoding.EncodeToString(plusPlain),
		base64.RawStdEncoding.EncodeToString(slashPlain),
		base64.URLEncoding.EncodeToString(plusPlain),
		base64.RawURLEncoding.EncodeToString(slashPlain),
	} {
		if !Contains("opaque payload " + value) {
			t.Fatalf("encoded credential was not detected: %q", value)
		}
	}
	nested := string(plain)
	for range maximumEncodedDecodeRounds {
		nested = base64.RawURLEncoding.EncodeToString([]byte(nested))
	}
	if !Contains(nested) {
		t.Fatal("credential at the maximum wrapper depth was not detected")
	}
	nested = base64.RawURLEncoding.EncodeToString([]byte(nested))
	if !Contains(nested) {
		t.Fatal("over-budget encoded wrapper did not fail closed")
	}
}

// TestContainsRejectsEncodedKeysInvalidUTF8AndFormatSplits verifies wrapper
// decoding cannot be bypassed through key encoding, invalid byte prefixes, or
// invisible format runes inserted inside an encoded token.
func TestContainsRejectsEncodedKeysInvalidUTF8AndFormatSplits(t *testing.T) {
	encodedKey := base64.RawStdEncoding.EncodeToString([]byte("apiToken"))
	if !sensitiveJSONKey(encodedKey) || !SensitiveDiagnosticKey(encodedKey) {
		t.Fatal("Base64-encoded sensitive structured key was not detected")
	}
	hexKey := hex.EncodeToString([]byte("apiToken"))
	if !sensitiveJSONKey(hexKey) {
		t.Fatal("hex-encoded sensitive JSON key was not detected")
	}
	safeKey := base64.RawStdEncoding.EncodeToString([]byte("regionName"))
	if sensitiveJSONKey(safeKey) || SensitiveDiagnosticKey(safeKey) {
		t.Fatal("encoded non-sensitive structured key was rejected")
	}

	invalidPrefixed := base64.RawStdEncoding.EncodeToString(append([]byte{0xff}, []byte("apiKey=must-not-pass")...))
	if !Contains(invalidPrefixed) {
		t.Fatal("invalid UTF-8 prefix hid a decoded credential")
	}
	invalidSafe := base64.RawStdEncoding.EncodeToString(append([]byte{0xff}, []byte("region=cn-test")...))
	if Contains(invalidSafe) {
		t.Fatal("invalid UTF-8 prefix alone was treated as a credential")
	}

	plain := []byte("apiKey=must-not-pass")
	for _, encoded := range []string{
		base64.RawStdEncoding.EncodeToString(plain),
		hex.EncodeToString(plain),
	} {
		if !Contains(interleaveFormatRune(encoded, 4)) {
			t.Fatal("format-rune split encoded credential was not detected")
		}
	}
}

// TestContainsRejectsMixedNestedWrappers verifies the four-layer budget is
// shared across Base64 and hexadecimal representations and fails closed when
// a fifth wrapper remains.
func TestContainsRejectsMixedNestedWrappers(t *testing.T) {
	credential := "apiKey=must-not-pass"
	credential = hex.EncodeToString([]byte(credential))
	credential = base64.RawURLEncoding.EncodeToString([]byte(credential))
	credential = hex.EncodeToString([]byte(credential))
	credential = base64.RawStdEncoding.EncodeToString([]byte(credential))
	if !Contains(credential) {
		t.Fatal("mixed four-layer credential was not detected")
	}

	safe := "region=cn-test"
	for layer := 0; layer < maximumEncodedDecodeRounds; layer++ {
		if layer%2 == 0 {
			safe = hex.EncodeToString([]byte(safe))
		} else {
			safe = base64.RawURLEncoding.EncodeToString([]byte(safe))
		}
	}
	if Contains(safe) {
		t.Fatal("safe value at the mixed wrapper limit was rejected")
	}
	if maximumEncodedDecodeRounds%2 == 0 {
		safe = hex.EncodeToString([]byte(safe))
	} else {
		safe = base64.RawURLEncoding.EncodeToString([]byte(safe))
	}
	if !Contains(safe) {
		t.Fatal("fifth mixed wrapper did not fail closed")
	}
}

// TestContainsRejectsMarkupAndGenericURLUserinfo verifies metadata on closing
// tags and arbitrary legal URL schemes share the same direct detection path.
func TestContainsRejectsMarkupAndGenericURLUserinfo(t *testing.T) {
	for _, value := range []string{
		"</span token=must-not-pass>",
		"mysql://user:must-not-pass@db.example.test/catalog",
		"mongodb+srv://user:must-not-pass@db.example.test/catalog",
		"amqp://user:must-not-pass@mq.example.test/vhost",
		"//user:must-not-pass@provider.example.test/api",
	} {
		if !Contains(value) {
			t.Fatalf("credential-bearing value was not detected: %q", value)
		}
	}
}

// TestContainsAllowsCredentialFreeEncodedAndURLValues protects common safe
// values from false positives introduced by generalized decoding and parsing.
func TestContainsAllowsCredentialFreeEncodedAndURLValues(t *testing.T) {
	plain := []byte("region=cn-test")
	for _, value := range []string{
		base64.StdEncoding.EncodeToString(plain),
		base64.RawURLEncoding.EncodeToString(plain),
		hex.EncodeToString(plain),
		"mysql://db.example.test/catalog",
		"mongodb+srv://cluster.example.test/catalog",
		"amqp://mq.example.test/vhost",
		"//cdn.example.test/assets/app.js",
		"status: all systems nominal",
		"</span status=complete>",
	} {
		if Contains(value) {
			t.Fatalf("safe value was rejected: %q", value)
		}
	}
}

// TestContainsEnforcesCandidateAndLengthBudgets verifies decoding remains
// bounded and hostile '=' runs are consumed without retrying every suffix.
func TestContainsEnforcesCandidateAndLengthBudgets(t *testing.T) {
	if Contains(strings.Repeat("=", maximumCredentialMaterialBytes)) {
		t.Fatal("credential-free padding run at the input limit was rejected")
	}
	if !Contains(strings.Repeat("x", maximumCredentialMaterialBytes+1)) {
		t.Fatal("oversized input did not fail closed")
	}
	atCandidateLimit := strings.Repeat("cmVnaW9u ", maximumEncodedCandidateCount)
	if Contains(atCandidateLimit) {
		t.Fatal("credential-free input at the candidate limit was rejected")
	}
	manyCandidates := strings.Repeat("cmVnaW9u ", maximumEncodedCandidateCount+1)
	if !Contains(manyCandidates) {
		t.Fatal("candidate budget exhaustion did not fail closed")
	}
}

// BenchmarkContainsPaddingRun guards the linear scanner shape for repeated
// padding without making wall-clock timing part of correctness tests.
func BenchmarkContainsPaddingRun(b *testing.B) {
	value := strings.Repeat("=", 64<<10)
	b.ResetTimer()
	for range b.N {
		Contains(value)
	}
}

// TestContainsRepeatedAuthorityMarkersCompletes guards the monotonic scan of a
// one-mebibyte hostile token containing only protocol-relative markers.
func TestContainsRepeatedAuthorityMarkersCompletes(t *testing.T) {
	_ = Contains(strings.Repeat("//", maximumCredentialMaterialBytes/2))
}

// BenchmarkContainsRepeatedAuthorityMarkers guards the authority scanner's
// linear shape without making elapsed time a correctness assertion.
func BenchmarkContainsRepeatedAuthorityMarkers(b *testing.B) {
	value := strings.Repeat("//", 64<<10)
	b.ResetTimer()
	for range b.N {
		Contains(value)
	}
}

// interleaveFormatRune inserts an invisible Cf rune often enough to split all
// encoded tokens below the standalone decoding threshold.
func interleaveFormatRune(value string, width int) string {
	var builder strings.Builder
	for index := 0; index < len(value); index += width {
		end := index + width
		if end > len(value) {
			end = len(value)
		}
		if index > 0 {
			builder.WriteRune('\u200b')
		}
		builder.WriteString(value[index:end])
	}
	return builder.String()
}
