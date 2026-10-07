package publicjson

import (
	"encoding/base64"
	"encoding/hex"
	"runtime"
	"strings"
	"testing"
)

func TestCredentialScannerRejectsStructuredAndObfuscatedSecrets(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("token=must-not-pass"))
	hexEncoded := hex.EncodeToString([]byte("api_key=must-not-pass"))
	tests := map[string]string{
		"assignment":                "token=must-not-pass",
		"authorization":             "Authorization: Bearer must-not-pass",
		"basic authorization":       "Authorization: Basic dXNlcjpwYXNz",
		"uri password":              "postgres://alice:hunter2@example.invalid/db",
		"uri username":              "redis://alice@example.invalid/0",
		"html quoted href":          `<a href="https://alice:hunter2@example.invalid/">open</a>`,
		"html unquoted href":        `<a href=https://alice:hunter2@example.invalid/>open</a>`,
		"non-breaking space":        "Bearer\u00a0must-not-pass",
		"format rune in scheme":     "Be\u200barer must-not-pass",
		"format rune separator":     "token\u2060=must-not-pass",
		"HTML named entity":         "api&nbsp;key=must-not-pass",
		"HTML numeric entity":       "api&#43;key=must-not-pass",
		"HTML metadata":             `<meta name="api-key" content="must-not-pass">`,
		"HTML property metadata":    `<meta property="session-token" content="must-not-pass">`,
		"HTML comment":              "<!-- token=must-not-pass -->",
		"HTML tag body":             "<script>Bearer must-not-pass</script>",
		"percent wrapper":           "%2574oken%253Dmust-not-pass",
		"mixed malformed percent":   "%74oken%ZZ=must-not-pass",
		"base64 wrapper":            encoded,
		"hex wrapper":               hexEncoded,
		"credential path":           "https://example.invalid/token/must-not-pass",
		"query credential":          "https://example.invalid/?api_key=must-not-pass",
		"private key":               "-----BEGIN PRIVATE KEY-----",
		"structured collection key": "token[0].value=must-not-pass",
	}
	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			if !ContainsCredentialText(value) {
				t.Fatalf("credential-shaped text was accepted: %q", value)
			}
		})
	}
}

func TestCredentialScannerAllowsPublicDescriptions(t *testing.T) {
	tests := map[string]string{
		"basic plan":          "Basic plan",
		"password reset path": "POST /api/password/reset completed",
		"reset capability":    "passwordResetSupported",
		"cookie policy":       "cookiePolicy",
		"percentage":          "Save 20%25 on renewal",
		"public URL":          "https://example.invalid/catalog/basic",
		"basic adjective":     "The basic enterprise package",
		"plus signs":          "C++ hosting plan",
	}
	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			if ContainsCredentialText(value) {
				t.Fatalf("public description was rejected: %q", value)
			}
		})
	}
}

func TestForbiddenKeyRejectsEncodedFormsAndAllowsCapabilities(t *testing.T) {
	for _, key := range []string{
		"token", "clientSecret", "authorization_header", "token[0]", "privateKeyFingerprint",
		"%74oken", "%2574oken", "%74oken%ZZ", "api&#95;key",
	} {
		if !ForbiddenKey(key) {
			t.Errorf("credential-shaped key was accepted: %q", key)
		}
	}
	for _, key := range []string{
		"passwordResetSupported", "sessionTimeoutSeconds", "cookiePolicy", "discountPercent",
	} {
		if ForbiddenKey(key) {
			t.Errorf("public capability key was rejected: %q", key)
		}
	}
}

func TestCanonicalizeRejectsSecretsDuplicatesAndInvalidRoots(t *testing.T) {
	limits := Limits{MaximumBytes: 4096, MaximumDepth: 16, MaximumNodes: 128}
	for name, document := range map[string]string{
		"secret key":       `{"clientSecret":"hidden"}`,
		"secret value":     `{"description":"Bearer must-not-pass"}`,
		"duplicate root":   `{"name":"a","name":"b"}`,
		"duplicate nested": `{"nested":{"x":1,"x":2}}`,
		"scalar root":      `"public"`,
		"trailing value":   `{"name":"public"}{}`,
		"invalid JSON":     `{"name":`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Canonicalize([]byte(document), limits); err == nil {
				t.Fatalf("document was accepted: %s", document)
			}
		})
	}
}

func TestCanonicalizeReturnsDeterministicDetachedJSON(t *testing.T) {
	input := []byte(`{ "z": 1, "a": {"enabled": true}, "items": [3, 2, 1] }`)
	canonical, err := Canonicalize(input, Limits{
		MaximumBytes: 4096, MaximumDepth: 16, MaximumNodes: 128,
	})
	if err != nil {
		t.Fatalf("canonicalize: %v", err)
	}
	if got, want := string(canonical), `{"a":{"enabled":true},"items":[3,2,1],"z":1}`; got != want {
		t.Fatalf("canonical JSON=%s, want %s", got, want)
	}
	input[0] = '['
	if string(canonical) == string(input) {
		t.Fatal("canonical result aliases caller input")
	}
}

func TestCanonicalizeEnforcesMaximumPayloadDepthAndNodeBounds(t *testing.T) {
	const maximumBytes = 64
	exact := []byte(`{"name":"` + strings.Repeat("x", maximumBytes-len(`{"name":""}`)) + `"}`)
	limits := Limits{MaximumBytes: maximumBytes, MaximumDepth: 8, MaximumNodes: 8}
	if len(exact) != maximumBytes {
		t.Fatalf("test payload size=%d, want %d", len(exact), maximumBytes)
	}
	if _, err := Canonicalize(exact, limits); err != nil {
		t.Fatalf("exact maximum payload was rejected: %v", err)
	}
	if _, err := Canonicalize(append(exact, ' '), limits); err == nil {
		t.Fatal("maximum+1 payload was accepted")
	}
	if _, err := Canonicalize([]byte(`[[[[]]]]`), Limits{
		MaximumBytes: 64, MaximumDepth: 2, MaximumNodes: 16,
	}); err == nil {
		t.Fatal("over-depth payload was accepted")
	}
	if _, err := Canonicalize([]byte(`[0,1,2,3]`), Limits{
		MaximumBytes: 64, MaximumDepth: 4, MaximumNodes: 4,
	}); err == nil {
		t.Fatal("over-node payload was accepted")
	}
}

func TestCanonicalizeAcceptsExactlyOneHundredThousandNodes(t *testing.T) {
	const scalarNodes = 99_999
	document := []byte("[" + strings.Repeat("\"x\",", scalarNodes-1) + "\"x\"]")
	limits := Limits{MaximumBytes: 768 * 1024, MaximumDepth: 4, MaximumNodes: 100_000}
	if _, err := Canonicalize(document, limits); err != nil {
		t.Fatalf("100k-node boundary was rejected: %v", err)
	}
	document = append(document[:len(document)-1], ',', '"', 'x', '"', ']')
	if _, err := Canonicalize(document, limits); err == nil {
		t.Fatal("100001-node payload was accepted")
	}
}

func TestCredentialScanningHasBoundedAllocationPressure(t *testing.T) {
	const stringNodes = 20_000
	document := []byte("[\"abcdefghijklmnop\"" +
		strings.Repeat(",\"abcdefghijklmnop\"", stringNodes-1) + "]")
	limits := Limits{MaximumBytes: 768 * 1024, MaximumDepth: 4, MaximumNodes: 100_000}
	runtime.GC()
	var before runtime.MemStats
	var after runtime.MemStats
	runtime.ReadMemStats(&before)
	if _, err := Canonicalize(document, limits); err != nil {
		t.Fatalf("canonicalize public strings: %v", err)
	}
	runtime.ReadMemStats(&after)
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 24*1024*1024 {
		t.Fatalf("scanner allocated %d bytes for %d tiny strings", allocated, stringNodes)
	}
}

func TestCredentialScannerFailsClosedWhenWrapperCandidatesOverflow(t *testing.T) {
	tokens := make([]string, maximumQueueCandidates+1)
	for index := range tokens {
		tokens[index] = base64.StdEncoding.EncodeToString([]byte("public-value-" + strings.Repeat("x", index+1)))
	}
	if !ContainsCredentialText(strings.Join(tokens, " ")) {
		t.Fatal("wrapper candidate overflow was accepted")
	}
}
