// This file verifies fail-closed Provider validation, bounds, state invariants, and secret defenses.

package resourcecap

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

func TestValidateChannelRefAcceptsOpaqueHostIdentifier(t *testing.T) {
	for _, value := range []string{
		"CHN_b3c8eb85a062feafb09e",
		"CHN_1850f133d8c669e87de8",
	} {
		if err := ValidateChannelRef(value); err != nil {
			t.Fatalf("ValidateChannelRef(%q) error = %v", value, err)
		}
	}
}

// credentialPublicValidationPaths returns every public contract entry that
// can persist or emit caller-controlled diagnostics.
func credentialPublicValidationPaths() []struct {
	name     string
	validate func(string) error
} {
	return []struct {
		name     string
		validate func(string) error
	}{
		{name: "ProviderConfig JSON", validate: func(value string) error {
			return (ProviderConfig{
				ChannelRef: "channel-test",
				Endpoint:   "https://provider.example.test/api",
				Settings:   []byte(`{"note":` + strconv.Quote(value) + `}`),
			}).Validate()
		}},
		{name: "Evidence", validate: func(value string) error {
			return (Evidence{
				Kind: "response", Summary: value, ObservedAt: validationReferenceTime(),
			}).Validate()
		}},
		{name: "ProviderFailure", validate: func(value string) error {
			return (ProviderFailure{Code: "UPSTREAM_FAILED", Message: value}).Validate()
		}},
		{name: "owner metadata", validate: func(value string) error {
			return ValidateOwnerMetadata([]byte(`{"note":` + strconv.Quote(value) + `}`))
		}},
		{name: "log", validate: func(value string) error {
			return ValidateLogEntry(LogLevelInfo, value)
		}},
	}
}

// TestProviderConfigValidationRejectsSecretsAndInvalidBindings verifies channel
// configuration carries only non-secret JSON and core-owned opaque handles.
func TestProviderConfigValidationRejectsSecretsAndInvalidBindings(t *testing.T) {
	handle, err := secretcap.NewHandle("sec_provider_test")
	if err != nil {
		t.Fatalf("create handle: %v", err)
	}
	valid := ProviderConfig{
		ChannelRef: "channel-test",
		Endpoint:   "https://provider.example.test/api",
		Settings:   []byte(`{"region":"cn-test","nested":[{"timeout":30}]}`),
		SecretBindings: []SecretBinding{
			{Name: "apiCredential", Handle: handle},
		},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid ProviderConfig rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*ProviderConfig)
	}{
		{name: "userinfo endpoint", mutate: func(value *ProviderConfig) {
			value.Endpoint = "https://user:password@provider.example.test"
		}},
		{name: "credential query endpoint", mutate: func(value *ProviderConfig) {
			value.Endpoint = "https://provider.example.test/api?api_key=must-not-pass"
		}},
		{name: "benign query endpoint", mutate: func(value *ProviderConfig) {
			value.Endpoint = "https://provider.example.test/api?region=cn-test"
		}},
		{name: "nested secret key", mutate: func(value *ProviderConfig) {
			value.Settings = []byte(`{"nested":[{"api_key":"must-not-pass"}]}`)
		}},
		{name: "nested credential string", mutate: func(value *ProviderConfig) {
			value.Settings = []byte(`{"note":{"detail":"token=must-not-pass"}}`)
		}},
		{name: "duplicate root key", mutate: func(value *ProviderConfig) {
			value.Settings = []byte(`{"region":"first","region":"second"}`)
		}},
		{name: "duplicate nested key", mutate: func(value *ProviderConfig) {
			value.Settings = []byte(`{"nested":{"region":"first","region":"second"}}`)
		}},
		{name: "escaped duplicate key", mutate: func(value *ProviderConfig) {
			value.Settings = []byte(`{"region":1,"\u0072egion":2}`)
		}},
		{name: "JSON array root", mutate: func(value *ProviderConfig) { value.Settings = []byte(`[]`) }},
		{name: "JSON null root", mutate: func(value *ProviderConfig) { value.Settings = []byte(`null`) }},
		{name: "JSON depth budget", mutate: func(value *ProviderConfig) {
			value.Settings = []byte(strings.Repeat(`{"nested":`, 65) + `0` + strings.Repeat(`}`, 65))
		}},
		{name: "JSON node budget", mutate: func(value *ProviderConfig) {
			value.Settings = []byte(`{"items":[` + strings.Repeat(`0,`, 10000) + `0]}`)
		}},
		{name: "duplicate binding", mutate: func(value *ProviderConfig) {
			value.SecretBindings = append(value.SecretBindings, value.SecretBindings[0])
		}},
		{name: "zero handle", mutate: func(value *ProviderConfig) {
			value.SecretBindings[0].Handle = secretcap.Handle{}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			candidate.Settings = append([]byte(nil), valid.Settings...)
			candidate.SecretBindings = append([]SecretBinding(nil), valid.SecretBindings...)
			test.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("expected invalid ProviderConfig to fail")
			}
		})
	}
}

// TestProviderConfigValidationRejectsCredentialReferencesAndEndpointPaths
// verifies reference suffixes and URL encoding cannot bypass secret checks.
func TestProviderConfigValidationRejectsCredentialReferencesAndEndpointPaths(t *testing.T) {
	base := ProviderConfig{ChannelRef: "channel-test", Endpoint: "https://provider.example.test/api"}
	fiveLevelEncodedKey := "access%4Bey"
	for level := 1; level < 5; level++ {
		fiveLevelEncodedKey = strings.ReplaceAll(fiveLevelEncodedKey, "%", "%25")
	}
	credentialKeys := []string{
		"tokenRef", "passwordReference", "secretRef", "cookieRef", "authenticationMode",
		"sessionID", "credentialRef", "privateKeyRef", "accessKeyRef", "%74oken", fiveLevelEncodedKey,
	}
	for _, key := range credentialKeys {
		t.Run(key, func(t *testing.T) {
			candidate := base
			candidate.Settings = []byte(`{"` + key + `":"opaque"}`)
			err := candidate.Validate()
			if err == nil {
				t.Fatal("expected credential-named JSON reference to fail")
			}
			if strings.Contains(err.Error(), key) {
				t.Fatal("validation error echoed the hostile JSON key")
			}
		})
	}
	for _, key := range []string{"unreviewedRef", "unreviewedref"} {
		unknownReference := base
		unknownReference.Settings = []byte(`{"` + key + `":"opaque"}`)
		if err := unknownReference.Validate(); err == nil {
			t.Fatal("expected an unreviewed JSON reference name to fail closed")
		}
	}
	fiveLevelEncoded := "token%3Dmust-not-pass"
	for level := 1; level < 5; level++ {
		fiveLevelEncoded = strings.ReplaceAll(fiveLevelEncoded, "%", "%25")
	}
	overBudgetEncoding := "mode%3Dsafe"
	for level := 0; level <= 64; level++ {
		overBudgetEncoding = strings.ReplaceAll(overBudgetEncoding, "%", "%25")
	}
	for _, endpoint := range []string{
		"https://provider.example.test/api/token=must-not-pass",
		"https://provider.example.test/api/%74oken=must-not-pass",
		"https://provider.example.test/api/%2574oken=must-not-pass",
		"https://provider.example.test/api/" + fiveLevelEncoded,
		"https://provider.example.test/api/" + overBudgetEncoding,
		"https://provider.example.test/api/%zz",
	} {
		candidate := base
		candidate.Endpoint = endpoint
		err := candidate.Validate()
		if err == nil {
			t.Fatal("expected credential-bearing endpoint path to fail")
		}
		if strings.Contains(err.Error(), "must-not-pass") {
			t.Fatal("validation error echoed the hostile endpoint")
		}
	}
	safe := base
	safe.Settings = []byte(`{"regionRef":"region-test","lineref":"line-test","externalResourceRef":"resource-test","transportMode":"signed"}`)
	if err := safe.Validate(); err != nil {
		t.Fatalf("explicit safe JSON references rejected: %v", err)
	}
}

// TestCredentialMaterialCoverageAndStablePercentDecoding verifies assignments,
// request headers, path segments, and nested encodings cannot bypass scanning.
func TestCredentialMaterialCoverageAndStablePercentDecoding(t *testing.T) {
	hostile := []string{
		"token: must-not-pass",
		"password=must-not-pass",
		"secret : must-not-pass",
		"cookie=must-not-pass",
		"auth: must-not-pass",
		"session=must-not-pass",
		"credential: must-not-pass",
		"private_key=must-not-pass",
		"access-key: must-not-pass",
		"Authorization: Basic must-not-pass",
		"Proxy-Authorization: Basic must-not-pass",
		"Cookie: session=must-not-pass",
		"Set-Cookie: session=must-not-pass",
		"X-Auth-Token: must-not-pass",
		"X-Access-Key: must-not-pass",
		"API Key: must-not-pass",
		"API+Key: must-not-pass",
		"API + Key: must-not-pass",
		"private+key=must-not-pass",
		"private%20key=must-not-pass",
		"private%2Bkey=must-not-pass",
		"private%252Bkey=must-not-pass",
		"private%2520key=must-not-pass",
		"API%E3%80%80Key: must-not-pass",
		"token[0]=must-not-pass",
		"token [0]=must-not-pass",
		"token%20[0].value=must-not-pass",
		"<token>[0]=must-not-pass",
		"<token> [0].value=must-not-pass",
		"token[0].value=must-not-pass",
		"api[key]=must-not-pass",
		"api [key].value=must-not-pass",
		"private[key]=must-not-pass",
		"api[0][key]=must-not-pass",
		"private[0].key=must-not-pass",
		"access+[0]+key=must-not-pass",
		"access%252B%255B0%255D%252Bkey=must-not-pass",
		"token%5B0%5D=must-not-pass",
		"<token>: must-not-pass",
		"%3Ctoken%3E%3A must-not-pass",
		"Bearer\u2003must-not-pass",
		"-----BEGIN PRIVATE\u00a0KEY-----must-not-pass",
		"https://user:must-not-pass@provider.example.test/api",
		"https://provider.example.test/api/password/reset?token=must-not-pass",
		"https://provider.example.test/api/password/reset/must-not-pass",
		"postgres://user:must-not-pass@db.example.test/catalog",
		"redis://:must-not-pass@cache.example.test/0",
		"to<b>ken=must-not-pass",
		"api<!--split-->key: must-not-pass",
		"api&#32;key: must-not-pass",
		"to&#x6b;en=must-not-pass",
		"/api/token/must-not-pass",
		"/api/password/must-not-pass",
		"/api/secret/must-not-pass",
		"/api/cookie/must-not-pass",
		"/api/auth/must-not-pass",
		"/api/session/must-not-pass",
		"/api/credential/must-not-pass",
		"/api/private-key/must-not-pass",
		"/api/access/key/must-not-pass",
	}
	for _, value := range hostile {
		if !containsCredentialMaterial(value) {
			t.Fatalf("expected credential material to be detected for %q", value)
		}
	}

	fiveLevelEncoded := "access%2Dkey%3Dmust-not-pass"
	for level := 1; level < 5; level++ {
		fiveLevelEncoded = strings.ReplaceAll(fiveLevelEncoded, "%", "%25")
	}
	if !containsCredentialMaterial(fiveLevelEncoded) {
		t.Fatal("expected five-level percent encoding to be detected")
	}
	if containsCredentialMaterial("region%3Dcn-test") {
		t.Fatal("stable non-credential percent encoding was rejected")
	}
	if containsCredentialMaterial("progress: 50% complete") {
		t.Fatal("ordinary percentage text was rejected")
	}
	if containsCredentialMaterial("literal %zz annotation") {
		t.Fatal("literal non-escape percent text was rejected")
	}

	overBudgetEncoding := "mode%3Dsafe"
	for level := 0; level <= 64; level++ {
		overBudgetEncoding = strings.ReplaceAll(overBudgetEncoding, "%", "%25")
	}
	if !containsCredentialMaterial(overBudgetEncoding) {
		t.Fatal("expected non-stable over-budget percent encoding to fail closed")
	}
}

// TestCredentialNameNormalizationHandlesUnicodeAndWrappers verifies field
// names cannot hide behind punctuation, bracket syntax, or Unicode spacing.
func TestCredentialNameNormalizationHandlesUnicodeAndWrappers(t *testing.T) {
	hostile := []string{
		`"token": "must-not-pass"`,
		`("Authorization") : Basic must-not-pass`,
		"[token]\u2003: must-not-pass",
		"token\u00a0: must-not-pass",
		"token[]=must-not-pass",
		"token [0].value=must-not-pass",
		"settings[token]=must-not-pass",
		"api [key].value=must-not-pass",
		"private[0].key=must-not-pass",
		"access+[0]+key=must-not-pass",
		"<token> [0].value=must-not-pass",
		"to\u200dken: must-not-pass",
		"API Key: must-not-pass",
		"private + key=must-not-pass",
		"\u3010\u2018api_token\u2019\u3011\u202f=must-not-pass",
	}
	for _, value := range hostile {
		if !containsCredentialMaterial(value) {
			t.Fatalf("wrapped credential field was not detected: %q", value)
		}
	}
	for _, whitespace := range []rune{
		'\t', '\n', '\v', '\f', '\r', ' ', '\u0085', '\u00a0', '\u1680',
		'\u2000', '\u2001', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006',
		'\u2007', '\u2008', '\u2009', '\u200a', '\u2028', '\u2029', '\u202f',
		'\u205f', '\u3000',
	} {
		if !containsCredentialMaterial("token" + string(whitespace) + ": must-not-pass") {
			t.Fatalf("credential field crossed Unicode whitespace %U", whitespace)
		}
	}
	for _, value := range []string{
		"authorization check completed: successfully",
		"authorization+check+completed: successfully",
		"public key: rotated",
		"items[0]=ready",
		"public[key]=rotated",
		"API response time: 50% budget used",
	} {
		if containsCredentialMaterial(value) {
			t.Fatalf("ordinary field was rejected: %q", value)
		}
	}
}

// TestWrappedCredentialNamesRejectAcrossPublicValidationPaths verifies all
// persistence and logging entry points share the hardened field-name scan.
func TestWrappedCredentialNamesRejectAcrossPublicValidationPaths(t *testing.T) {
	checks := []struct {
		name     string
		validate func() error
	}{
		{name: "JSON string", validate: func() error {
			return (ProviderConfig{
				ChannelRef: "channel-test",
				Endpoint:   "https://provider.example.test/api",
				Settings:   []byte(`{"note":"\"token\": \"must-not-pass\""}`),
			}).Validate()
		}},
		{name: "evidence", validate: func() error {
			return (Evidence{Kind: "response", Summary: "token\u00a0: must-not-pass", ObservedAt: time.Now()}).Validate()
		}},
		{name: "failure", validate: func() error {
			return (ProviderFailure{Code: "UPSTREAM_FAILED", Message: "token[]=must-not-pass"}).Validate()
		}},
		{name: "log", validate: func() error {
			return ValidateLogEntry(LogLevelInfo, `"Authorization": "Basic must-not-pass"`)
		}},
		{name: "JSON multi-word key", validate: func() error {
			return (ProviderConfig{
				ChannelRef: "channel-test",
				Endpoint:   "https://provider.example.test/api",
				Settings:   []byte(`{"note":"API Key: must-not-pass"}`),
			}).Validate()
		}},
		{name: "JSON truncated key", validate: func() error {
			return (ProviderConfig{
				ChannelRef: "channel-test",
				Endpoint:   "https://provider.example.test/api",
				Settings: []byte(
					`{"note":"token` + strings.Repeat("x", 257) + `=must-not-pass"}`,
				),
			}).Validate()
		}},
		{name: "evidence encoded words", validate: func() error {
			return (Evidence{Kind: "response", Summary: "private%20key=must-not-pass", ObservedAt: time.Now()}).Validate()
		}},
		{name: "failure indexed key", validate: func() error {
			return (ProviderFailure{Code: "UPSTREAM_FAILED", Message: "token[0]=must-not-pass"}).Validate()
		}},
		{name: "log angle wrapper", validate: func() error {
			return ValidateLogEntry(LogLevelInfo, "<token>: must-not-pass")
		}},
		{name: "JSON bracket property", validate: func() error {
			return (ProviderConfig{
				ChannelRef: "channel-test",
				Endpoint:   "https://provider.example.test/api",
				Settings:   []byte(`{"note":"api[key]=must-not-pass"}`),
			}).Validate()
		}},
		{name: "JSON property chain", validate: func() error {
			return (ProviderConfig{
				ChannelRef: "channel-test",
				Endpoint:   "https://provider.example.test/api",
				Settings:   []byte(`{"note":"token[0].value=must-not-pass"}`),
			}).Validate()
		}},
		{name: "evidence bracket property", validate: func() error {
			return (Evidence{Kind: "response", Summary: "private[key]=must-not-pass", ObservedAt: time.Now()}).Validate()
		}},
		{name: "evidence encoded plus", validate: func() error {
			return (Evidence{Kind: "response", Summary: "private%2Bkey=must-not-pass", ObservedAt: time.Now()}).Validate()
		}},
		{name: "failure spaced index", validate: func() error {
			return (ProviderFailure{Code: "UPSTREAM_FAILED", Message: "token [0]=must-not-pass"}).Validate()
		}},
		{name: "log wrapped index", validate: func() error {
			return ValidateLogEntry(LogLevelInfo, "<token>[0]=must-not-pass")
		}},
		{name: "log form plus", validate: func() error {
			return ValidateLogEntry(LogLevelInfo, "API+Key: must-not-pass")
		}},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			err := check.validate()
			if err == nil {
				t.Fatal("expected wrapped credential material to fail validation")
			}
			if strings.Contains(err.Error(), "must-not-pass") {
				t.Fatal("validation error echoed credential material")
			}
		})
	}
}

// TestObfuscatedCredentialsRejectAcrossEveryPublicValidationPath verifies the
// same normalization and URL defenses protect persistence and logging callers.
func TestObfuscatedCredentialsRejectAcrossEveryPublicValidationPath(t *testing.T) {
	hostile := []string{
		"api[0][key]=must-not-pass",
		"private[0].key=must-not-pass",
		"access+[0]+key=must-not-pass",
		"access%252B%255B0%255D%252Bkey=must-not-pass",
		"Bearer\u2003must-not-pass",
		"Bearer\u200Bmust-not-pass",
		"Bea\u200Brer must-not-pass",
		"PRIVATE\u2003KEY: must-not-pass",
		"-----BEGIN PRIVATE\u00a0KEY-----must-not-pass",
		"-----BEGIN\u200BPRIVATE KEY-----must-not-pass",
		"https://user:must-not-pass@provider.example.test/api",
		"postgres://user:must-not-pass@db.example.test/catalog",
		"redis://:must-not-pass@cache.example.test/0",
		"to<b>ken=must-not-pass",
		"api<!--split-->key: must-not-pass",
		"api&#32;key: must-not-pass",
		"to&#x6b;en=must-not-pass",
		"to%26%23x6b%3Ben=must-not-pass",
		"safe<!-- token=must-not-pass -->",
		`<span title="token=must-not-pass">safe</span>`,
		"<token>must-not-pass</token>",
	}
	paths := []struct {
		name     string
		validate func(string) error
	}{
		{name: "ProviderConfig JSON", validate: func(value string) error {
			return (ProviderConfig{
				ChannelRef: "channel-test",
				Endpoint:   "https://provider.example.test/api",
				Settings:   []byte(`{"note":` + strconv.Quote(value) + `}`),
			}).Validate()
		}},
		{name: "Evidence", validate: func(value string) error {
			return (Evidence{
				Kind: "response", Summary: value, ObservedAt: validationReferenceTime(),
			}).Validate()
		}},
		{name: "ProviderFailure", validate: func(value string) error {
			return (ProviderFailure{Code: "UPSTREAM_FAILED", Message: value}).Validate()
		}},
		{name: "log", validate: func(value string) error {
			return ValidateLogEntry(LogLevelInfo, value)
		}},
	}
	for _, value := range hostile {
		for _, path := range paths {
			t.Run(path.name+"/"+strconv.Itoa(len(value)), func(t *testing.T) {
				err := path.validate(value)
				if err == nil {
					t.Fatalf("expected obfuscated credential to fail through %s", path.name)
				}
				if strings.Contains(err.Error(), "must-not-pass") {
					t.Fatal("validation error echoed obfuscated credential material")
				}
			})
		}
	}
}

// TestEncodedCredentialsRejectAcrossEveryPublicValidationPath verifies raw
// Base64 and hexadecimal wrappers cannot conceal credential assignments.
func TestEncodedCredentialsRejectAcrossEveryPublicValidationPath(t *testing.T) {
	plain := []byte("apiKey=must-not-pass")
	plusPlain := []byte("\u083eapiKey=must-not-pass")
	slashPlain := []byte("\u083fapiKey=must-not-pass")
	hostile := []string{
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
	}
	for _, encoded := range hostile {
		value := "opaque payload " + encoded
		for _, path := range credentialPublicValidationPaths() {
			t.Run(path.name+"/"+strconv.Itoa(len(encoded)), func(t *testing.T) {
				err := path.validate(value)
				if err == nil {
					t.Fatalf("expected encoded credential to fail through %s", path.name)
				}
				if strings.Contains(err.Error(), "must-not-pass") {
					t.Fatal("validation error echoed decoded credential material")
				}
			})
		}
	}
}

// TestClosingTagAttributesAndGenericURLUserinfoRejectAcrossPublicPaths covers
// closing-tag metadata, arbitrary legal schemes, and protocol-relative URLs.
func TestClosingTagAttributesAndGenericURLUserinfoRejectAcrossPublicPaths(t *testing.T) {
	hostile := []string{
		"</span token=must-not-pass>",
		"mysql://user:must-not-pass@db.example.test/catalog",
		"mongodb+srv://user:must-not-pass@db.example.test/catalog",
		"amqp://user:must-not-pass@mq.example.test/vhost",
		"//user:must-not-pass@provider.example.test/api",
	}
	for _, value := range hostile {
		for _, path := range credentialPublicValidationPaths() {
			t.Run(path.name+"/"+strconv.Itoa(len(value)), func(t *testing.T) {
				err := path.validate(value)
				if err == nil {
					t.Fatalf("expected credential-bearing value to fail through %s", path.name)
				}
				if strings.Contains(err.Error(), "must-not-pass") {
					t.Fatal("validation error echoed URL or markup credential material")
				}
			})
		}
	}
}

// TestEncodedAndGenericURLSafeValuesPassEveryPublicValidationPath prevents the
// additional decoding and URL scans from rejecting credential-free values.
func TestEncodedAndGenericURLSafeValuesPassEveryPublicValidationPath(t *testing.T) {
	safePlain := []byte("region=cn-test")
	safe := []string{
		"opaque payload " + base64.StdEncoding.EncodeToString(safePlain),
		"opaque payload " + base64.RawURLEncoding.EncodeToString(safePlain),
		"opaque payload " + hex.EncodeToString(safePlain),
		"mysql://db.example.test/catalog",
		"mongodb+srv://cluster.example.test/catalog",
		"amqp://mq.example.test/vhost",
		"//cdn.example.test/assets/app.js",
		"status: all systems nominal",
		"</span status=complete>",
	}
	for _, value := range safe {
		for _, path := range credentialPublicValidationPaths() {
			t.Run(path.name+"/"+strconv.Itoa(len(value)), func(t *testing.T) {
				if err := path.validate(value); err != nil {
					t.Fatalf("safe value rejected through %s: %v", path.name, err)
				}
			})
		}
	}
}

// TestCredentialNormalizationBudgetsFailClosed verifies direct callers cannot
// bypass scanner limits with oversized input or more nested transformations
// than the fixed normalization pass ceiling permits.
func TestCredentialNormalizationBudgetsFailClosed(t *testing.T) {
	oversized := strings.Repeat("x", (1<<20)+1)
	if !containsCredentialMaterial(oversized) {
		t.Fatal("expected oversized credential material scan to fail closed")
	}

	nestedEntity := "&#x6b;"
	for range 65 {
		nestedEntity = strings.ReplaceAll(nestedEntity, "&", "&amp;")
	}
	if !containsCredentialMaterial("to" + nestedEntity + "en=must-not-pass") {
		t.Fatal("expected over-budget nested HTML entities to fail closed")
	}

	oversizedWhitespaceJSON := ProviderConfig{
		ChannelRef: "channel-test",
		Endpoint:   "https://provider.example.test/api",
		Settings:   []byte(strings.Repeat(" ", (1<<20)+1)),
	}
	if err := oversizedWhitespaceJSON.Validate(); err == nil {
		t.Fatal("expected raw JSON payload limit to apply before whitespace trimming")
	}
}

// TestSafeRoutesAndMarkupPassEveryPublicValidationPath prevents route names,
// markup, and credential-free DSNs from becoming scanner false positives.
func TestSafeRoutesAndMarkupPassEveryPublicValidationPath(t *testing.T) {
	safe := []string{
		"POST /api/password/reset completed",
		"https://provider.example.test/api/password/reset",
		"postgres://db.example.test/catalog",
		"redis://cache.example.test/0",
		"rendered <b>status</b>: completed",
		"safe<!-- status=complete -->",
		`<span title="status ready">completed</span>`,
		"status\u200Bready",
		"public&#32;key: rotated",
	}
	paths := []struct {
		name     string
		validate func(string) error
	}{
		{name: "ProviderConfig JSON", validate: func(value string) error {
			return (ProviderConfig{
				ChannelRef: "channel-test",
				Endpoint:   "https://provider.example.test/api",
				Settings:   []byte(`{"note":` + strconv.Quote(value) + `}`),
			}).Validate()
		}},
		{name: "Evidence", validate: func(value string) error {
			return (Evidence{
				Kind: "response", Summary: value, ObservedAt: validationReferenceTime(),
			}).Validate()
		}},
		{name: "ProviderFailure", validate: func(value string) error {
			return (ProviderFailure{Code: "UPSTREAM_FAILED", Message: value}).Validate()
		}},
		{name: "log", validate: func(value string) error {
			return ValidateLogEntry(LogLevelInfo, value)
		}},
	}
	for _, value := range safe {
		for _, path := range paths {
			t.Run(path.name+"/"+strconv.Itoa(len(value)), func(t *testing.T) {
				if err := path.validate(value); err != nil {
					t.Fatalf("safe diagnostic rejected through %s: %v", path.name, err)
				}
			})
		}
	}
}

// TestOrdinaryPercentTextPassesPublicValidationPaths prevents the credential
// defense from rejecting percentages or unrelated prose.
func TestOrdinaryPercentTextPassesPublicValidationPaths(t *testing.T) {
	const (
		safeText      = "capacity: 80% available"
		safeMultiword = "public key rotation status: 80% complete"
	)
	for _, value := range []string{
		safeText,
		safeMultiword,
		"progress: 50% complete",
		"literal %zz annotation",
		"the authorization check completed successfully",
		"authorization check completed: successfully",
		"authorization+check+completed: successfully",
		"authorization%2Bcheck%2Bcompleted: successfully",
		"API response time: 50% budget used",
		"private network status: healthy",
		"private+network+status: healthy",
		"public key: rotated",
		"items[0]=ready",
		"items [0]=ready",
		"items [0].value=ready",
		"public[key]=rotated",
		"public [key].status=rotated",
		"public key[0]=rotated",
		"public+key: rotated",
		"math+value=42",
		"progress[0].value=80%",
		"progress[0]=80%",
		"POST /api/password/reset completed",
		"https://provider.example.test/api/password/reset",
		"postgres://db.example.test/catalog",
		"redis://cache.example.test/0",
		"rendered <b>status</b>: completed",
		"public&#32;key: rotated",
		"percentage%20complete: yes",
		"less < than expected: 75% complete",
		": status remains unknown",
	} {
		if containsCredentialMaterial(value) {
			t.Fatalf("ordinary diagnostic text was rejected: %q", value)
		}
	}

	config := ProviderConfig{
		ChannelRef: "channel-test",
		Endpoint:   "https://provider.example.test/api",
		Settings:   []byte(`{"note":"public key rotation status: 80% complete"}`),
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("ordinary percentage JSON string rejected: %v", err)
	}
	if err := (Evidence{Kind: "response", Summary: safeMultiword, ObservedAt: time.Now()}).Validate(); err != nil {
		t.Fatalf("ordinary percentage evidence rejected: %v", err)
	}
	if err := (ProviderFailure{Code: "UPSTREAM_FAILED", Message: safeMultiword}).Validate(); err != nil {
		t.Fatalf("ordinary percentage failure rejected: %v", err)
	}
	if err := ValidateLogEntry(LogLevelInfo, safeMultiword); err != nil {
		t.Fatalf("ordinary percentage log rejected: %v", err)
	}
	plusSafeConfig := config
	plusSafeConfig.Settings = []byte(`{"note":"authorization+check+completed: successfully"}`)
	if err := plusSafeConfig.Validate(); err != nil {
		t.Fatalf("ordinary form-style JSON text rejected: %v", err)
	}
	if err := (Evidence{
		Kind: "response", Summary: "public+key: rotated", ObservedAt: time.Now(),
	}).Validate(); err != nil {
		t.Fatalf("ordinary form-style evidence rejected: %v", err)
	}
	if err := (ProviderFailure{
		Code: "UPSTREAM_FAILED", Message: "private+network+status: healthy",
	}).Validate(); err != nil {
		t.Fatalf("ordinary form-style failure rejected: %v", err)
	}
	if err := ValidateLogEntry(LogLevelInfo, "authorization%2Bcheck%2Bcompleted: successfully"); err != nil {
		t.Fatalf("ordinary encoded form-style log rejected: %v", err)
	}
}

// TestProviderEnvValidationRejectsMissingAndTypedNilDependencies verifies DI
// validation fails before a factory can retain an incomplete environment.
func TestProviderEnvValidationRejectsMissingAndTypedNilDependencies(t *testing.T) {
	valid := ProviderEnv{
		PluginID:               "source-plugin",
		ProviderID:             "provider",
		Secrets:                validationSecretAccessor{},
		SecretSink:             validationSecretSink{},
		AuthenticationSessions: validationAuthenticationSessions{},
		HTTPClient:             validationHTTPClient{},
		Clock:                  validationClock{},
		Logger:                 validationLogger{},
		Metrics:                validationMetrics{},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid ProviderEnv rejected: %v", err)
	}
	var typedNil *validationHTTPClient
	valid.HTTPClient = typedNil
	if err := valid.Validate(); err == nil {
		t.Fatal("expected typed-nil HTTP client to fail")
	}
	valid.HTTPClient = validationHTTPClient{}
	var typedNilSink *validationSecretSink
	valid.SecretSink = typedNilSink
	if err := valid.Validate(); err == nil {
		t.Fatal("expected typed-nil secret sink to fail")
	}
}

// TestCapabilitySnapshotValidationIsCompleteAndConsistent verifies product and
// action availability cannot be derived from partial or contradictory data.
func TestCapabilitySnapshotValidationIsCompleteAndConsistent(t *testing.T) {
	now := validationReferenceTime()
	valid := validCapabilitySnapshot()
	if err := valid.ValidateAt(now); err != nil {
		t.Fatalf("valid capability snapshot rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*CapabilitySnapshot)
	}{
		{name: "missing capability", mutate: func(value *CapabilitySnapshot) {
			value.Capabilities = value.Capabilities[:len(value.Capabilities)-1]
		}},
		{name: "duplicate capability", mutate: func(value *CapabilitySnapshot) {
			value.Capabilities[1].Code = value.Capabilities[0].Code
		}},
		{name: "unsupported without reason", mutate: func(value *CapabilitySnapshot) {
			value.Capabilities[0].Support = SupportLevelUnsupported
		}},
		{name: "conditional without constraint", mutate: func(value *CapabilitySnapshot) {
			value.Capabilities[0].Support = SupportLevelConditional
		}},
		{name: "invalid constraint range", mutate: func(value *CapabilitySnapshot) {
			minimum, maximum := int64(10), int64(1)
			value.Capabilities[0].Constraints = []ParameterConstraint{{
				Name: "cpu", Minimum: &minimum, Maximum: &maximum,
			}}
		}},
		{name: "credential pattern", mutate: func(value *CapabilitySnapshot) {
			value.Capabilities[0].Support = SupportLevelConditional
			value.Capabilities[0].Constraints = []ParameterConstraint{{Name: "mode", Pattern: `^token=must-not-pass$`}}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			candidate.Capabilities = append([]CapabilityDescriptor(nil), valid.Capabilities...)
			test.mutate(&candidate)
			if err := candidate.ValidateAt(now); err == nil {
				t.Fatal("expected invalid capability snapshot to fail")
			}
		})
	}
}

// TestCapabilitySnapshotValidationRequiresExplicitOwnerClock verifies callers
// cannot bypass freshness checks or depend on process-global wall-clock time.
func TestCapabilitySnapshotValidationRequiresExplicitOwnerClock(t *testing.T) {
	now := validationReferenceTime()
	valid := validCapabilitySnapshot()
	if err := valid.Validate(); err == nil {
		t.Fatal("expected validation without an explicit owner clock to fail closed")
	}
	tests := []struct {
		name       string
		now        time.Time
		observedAt time.Time
		wantError  bool
	}{
		{name: "current", now: now, observedAt: now},
		{name: "future boundary", now: now, observedAt: now.Add(maximumObservationFutureSkew)},
		{name: "future beyond boundary", now: now, observedAt: now.Add(maximumObservationFutureSkew + time.Nanosecond), wantError: true},
		{name: "stale boundary", now: now, observedAt: now.Add(-maximumCapabilityObservationAge)},
		{name: "stale beyond boundary", now: now, observedAt: now.Add(-maximumCapabilityObservationAge - time.Nanosecond), wantError: true},
		{name: "year 9999 observation", now: now, observedAt: time.Date(9999, time.January, 1, 0, 0, 0, 0, time.UTC), wantError: true},
		{name: "missing owner clock", observedAt: now, wantError: true},
		{name: "year 9999 owner clock", now: time.Date(9999, time.January, 1, 0, 0, 0, 0, time.UTC), observedAt: now, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			candidate.ObservedAt = test.observedAt
			err := candidate.ValidateAt(test.now)
			if test.wantError && err == nil {
				t.Fatal("expected observation time validation to fail")
			}
			if !test.wantError && err != nil {
				t.Fatalf("trusted observation time rejected: %v", err)
			}
		})
	}

	evidence := Evidence{
		Kind:       "response",
		Summary:    "redacted response",
		ObservedAt: time.Date(9999, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := evidence.Validate(); err == nil {
		t.Fatal("expected a year-9999 evidence timestamp to fail intrinsic validation")
	}
}

// TestCapabilitySnapshotValidationEnforcesAggregateBudgets verifies many
// individually valid constraints cannot create an unbounded aggregate graph.
func TestCapabilitySnapshotValidationEnforcesAggregateBudgets(t *testing.T) {
	now := validationReferenceTime()
	nodeHeavy := validCapabilitySnapshot()
	for descriptorIndex := range nodeHeavy.Capabilities {
		descriptor := &nodeHeavy.Capabilities[descriptorIndex]
		descriptor.Support = SupportLevelConditional
		for constraintIndex := 0; constraintIndex < maximumConstraintCount; constraintIndex++ {
			descriptor.Constraints = append(descriptor.Constraints, ParameterConstraint{
				Name: "field" + strconv.Itoa(constraintIndex),
				AllowedValues: []string{
					"value-a",
					"value-b",
					"value-c",
				},
			})
		}
	}
	if err := nodeHeavy.ValidateAt(now); err == nil || !strings.Contains(err.Error(), "nodes") {
		t.Fatalf("expected aggregate node budget failure, got %v", err)
	}

	byteHeavy := validCapabilitySnapshot()
	byteHeavy.Capabilities[0].Support = SupportLevelConditional
	for constraintIndex := 0; constraintIndex < 8; constraintIndex++ {
		constraint := ParameterConstraint{Name: "field" + strconv.Itoa(constraintIndex)}
		for valueIndex := 0; valueIndex < 128; valueIndex++ {
			constraint.AllowedValues = append(
				constraint.AllowedValues,
				strings.Repeat("x", 1024)+strconv.Itoa(valueIndex),
			)
		}
		byteHeavy.Capabilities[0].Constraints = append(byteHeavy.Capabilities[0].Constraints, constraint)
	}
	if err := byteHeavy.ValidateAt(now); err == nil || !strings.Contains(err.Error(), "bytes") {
		t.Fatalf("expected aggregate byte budget failure, got %v", err)
	}
}

// TestParameterConstraintPatternErrorsDoNotEchoSource verifies invalid or
// credential-bearing expressions fail without reflecting attacker text.
func TestParameterConstraintPatternErrorsDoNotEchoSource(t *testing.T) {
	for _, pattern := range []string{"[must-not-pass", "^token=must-not-pass$"} {
		constraint := ParameterConstraint{Name: "mode", Pattern: pattern}
		err := constraint.Validate()
		if err == nil {
			t.Fatalf("expected pattern %q to fail", pattern)
		}
		if strings.Contains(err.Error(), "must-not-pass") {
			t.Fatal("constraint validation error echoed the hostile pattern")
		}
	}
}

// TestProviderCapacityValidationEnforcesPageAndSecretBounds verifies pages cannot
// cross kinds, overrun the request, leak secret keys, or publish ambiguous ends.
func TestProviderCapacityValidationEnforcesPageAndSecretBounds(t *testing.T) {
	request := ProviderCapacityRequest{
		Kind: ProviderCapacityKindPlan, Cursor: "cursor-current", Limit: 2, Filters: []byte(`{"regionRef":"region-test"}`),
	}
	page := ProviderCapacityPage{
		Items: []ProviderCapacityItem{{
			Kind: ProviderCapacityKindPlan, ExternalRef: "plan-test", Data: []byte(`{"cpu":2}`), ObservedAt: time.Now(),
		}},
		Complete: true,
		Evidence: []Evidence{{
			Kind: "request", Reference: "request-test", ObservedAt: time.Now(),
		}},
	}
	if err := page.Validate(request); err != nil {
		t.Fatalf("valid providerCapacity page rejected: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*ProviderCapacityPage)
	}{
		{name: "kind mismatch", mutate: func(value *ProviderCapacityPage) { value.Items[0].Kind = ProviderCapacityKindImage }},
		{name: "secret data", mutate: func(value *ProviderCapacityPage) {
			value.Items[0].Data = []byte(`{"credentials":{"accessToken":"must-not-pass"}}`)
		}},
		{name: "complete with cursor", mutate: func(value *ProviderCapacityPage) { value.NextCursor = "next" }},
		{name: "incomplete without cursor", mutate: func(value *ProviderCapacityPage) { value.Complete = false }},
		{name: "duplicate item", mutate: func(value *ProviderCapacityPage) {
			value.Items = append(value.Items, value.Items[0])
		}},
		{name: "cursor does not advance", mutate: func(value *ProviderCapacityPage) {
			value.Complete = false
			value.NextCursor = request.Cursor
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := page
			candidate.Items = append([]ProviderCapacityItem(nil), page.Items...)
			test.mutate(&candidate)
			if err := candidate.Validate(request); err == nil {
				t.Fatal("expected invalid providerCapacity page to fail")
			}
		})
	}
}

// TestMachineEnumsAreBoundedSecretAwareAndNotReflected verifies typed enum
// boundaries reject hostile values before membership checks.
func TestMachineEnumsAreBoundedSecretAwareAndNotReflected(t *testing.T) {
	const hostile = "token=must-not-pass"
	tests := []struct {
		name     string
		validate func() error
	}{
		{name: "providerCapacity kind", validate: func() error {
			return (ProviderCapacityRequest{Kind: ProviderCapacityKind(hostile), Limit: 1}).Validate()
		}},
		{name: "capability code", validate: func() error {
			return (CapabilityDescriptor{Code: CapabilityCode(hostile), Support: SupportLevelSupported}).Validate()
		}},
		{name: "support level", validate: func() error {
			return (CapabilityDescriptor{Code: CapabilityProvision, Support: SupportLevel(hostile)}).Validate()
		}},
		{name: "task status", validate: func() error {
			return (OperationResult{Status: TaskStatus(hostile)}).Validate()
		}},
		{name: "resource state", validate: func() error {
			return (ResourceSnapshot{ExternalResourceRef: "resource-test", State: ResourceState(hostile), ObservedAt: time.Now()}).Validate()
		}},
		{name: "log level", validate: func() error {
			return ValidateLogEntry(LogLevel(hostile), "safe message")
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.validate()
			if err == nil {
				t.Fatal("expected hostile machine enum to fail")
			}
			if strings.Contains(err.Error(), "must-not-pass") {
				t.Fatal("validation error echoed the hostile machine enum")
			}
		})
	}
	oversized := ProviderCapacityRequest{Kind: ProviderCapacityKind(strings.Repeat("x", maximumMachineValueBytes+1)), Limit: 1}
	if err := oversized.Validate(); err == nil {
		t.Fatal("expected oversized machine enum to fail")
	}
}

// TestOperationValidationEnforcesIdempotencyAndResultState verifies every
// state-machine observation has the references and failure semantics it needs.
func TestOperationValidationEnforcesIdempotencyAndResultState(t *testing.T) {
	request := OperationRequest{
		RequestID: "operation-test", Capability: CapabilityStart, ResourceRef: "resource-test",
		Parameters: []byte(`{"mode":"normal"}`),
	}
	if err := request.Validate(); err != nil {
		t.Fatalf("valid operation request rejected: %v", err)
	}
	request.RequestID = ""
	if err := request.Validate(); err == nil {
		t.Fatal("expected empty idempotency key to fail")
	}
	request = OperationRequest{RequestID: "operation-test", Capability: CapabilityProviderCapacity}
	if err := request.Validate(); err == nil {
		t.Fatal("expected read capability to be rejected by Execute")
	}

	validResults := []OperationResult{
		{Status: TaskStatusAccepted, ExternalTaskRef: "task-test"},
		{Status: TaskStatusRunning, ExternalTaskRef: "task-test"},
		{Status: TaskStatusSucceeded, ExternalResourceRef: "resource-test"},
		{Status: TaskStatusFailed, Failure: &ProviderFailure{Code: "UPSTREAM_FAILED", Message: "upstream request failed"}},
		{Status: TaskStatusUnknown, Failure: &ProviderFailure{
			Code: "RESULT_UNKNOWN", Message: "upstream result is unknown", ResultUnknown: true,
		}},
	}
	for _, result := range validResults {
		if err := result.Validate(); err != nil {
			t.Fatalf("valid result %q rejected: %v", result.Status, err)
		}
	}
	invalidResults := []OperationResult{
		{Status: TaskStatusAccepted},
		{Status: TaskStatusRunning, ExternalTaskRef: "task-test", Failure: &ProviderFailure{Code: "FAILED", Message: "failed"}},
		{Status: TaskStatusSucceeded},
		{Status: TaskStatusFailed, Failure: &ProviderFailure{Code: "FAILED", Message: "failed", ResultUnknown: true}},
		{Status: TaskStatusUnknown, Failure: &ProviderFailure{Code: "UNKNOWN", Message: "unknown"}},
		{Status: TaskStatusUnknown, Failure: &ProviderFailure{
			Code: "RESULT_UNKNOWN", Message: "upstream result is unknown", ResultUnknown: true, Retryable: true,
		}},
		{Status: "other"},
	}
	for _, result := range invalidResults {
		if err := result.Validate(); err == nil {
			t.Fatalf("expected invalid result %#v to fail", result)
		}
	}
}

// TestResourceStateNormalizationFailsClosed verifies unrecognized upstream
// states never become an optimistic local state.
func TestResourceStateNormalizationFailsClosed(t *testing.T) {
	for _, value := range []string{"upstream-new-state", "", " ACTIVE-NOW ", "success"} {
		if got := NormalizeResourceState(value); got != ResourceStateUnknown {
			t.Fatalf("NormalizeResourceState(%q)=%q, want unknown", value, got)
		}
	}
	if got := NormalizeResourceState(" ACTIVE "); got != ResourceStateActive {
		t.Fatalf("normalized active=%q, want active", got)
	}
	snapshot := ResourceSnapshot{
		ExternalResourceRef: "resource-test", State: "upstream-new-state", ObservedAt: time.Now(),
	}
	if err := snapshot.Validate(); err == nil {
		t.Fatal("expected direct unrecognized ResourceState to fail validation")
	}
}

// TestLogMetricAndEvidenceValidationRejectsCredentialMaterial verifies public
// validation helpers support defense-in-depth owner wrappers.
func TestLogMetricAndEvidenceValidationRejectsCredentialMaterial(t *testing.T) {
	if err := ValidateLogEntry(LogLevelInfo, "provider request completed", LogField{Key: "requestRef", Value: "request-test"}); err != nil {
		t.Fatalf("valid log entry rejected: %v", err)
	}
	if err := ValidateLogEntry(LogLevelInfo, "Authorization: Bearer secret"); err == nil {
		t.Fatal("expected credential-bearing log message to fail")
	}
	if err := ValidateLogEntry(LogLevelInfo, "provider request completed", LogField{Key: "secretRef", Value: "sec-test"}); err == nil {
		t.Fatal("expected secret-reference log field to fail")
	}
	if err := ValidateMetricObservation("provider.latency", 1.5, MetricLabel{Name: "provider", Value: "test"}); err != nil {
		t.Fatalf("valid metric rejected: %v", err)
	}
	if err := ValidateMetric("provider.requests", MetricLabel{Name: "apiToken", Value: "secret"}); err == nil {
		t.Fatal("expected sensitive metric label to fail")
	}
	evidence := Evidence{Kind: "response", Summary: "token=must-not-pass", ObservedAt: time.Now()}
	if err := evidence.Validate(); err == nil {
		t.Fatal("expected credential-bearing evidence to fail")
	}
	evidence = Evidence{Kind: "response", Reference: "token=must-not-pass", ObservedAt: time.Now()}
	if err := evidence.Validate(); err == nil {
		t.Fatal("expected credential-bearing evidence reference to fail")
	}
	query := TaskQuery{ExternalTaskRef: "token=must-not-pass"}
	if err := query.Validate(); err == nil {
		t.Fatal("expected credential-bearing task reference to fail")
	}
}

// FuzzNormalizeResourceStateAlwaysReturnsContractValue ensures arbitrary
// upstream state text can never escape the normalized enum boundary.
func FuzzNormalizeResourceStateAlwaysReturnsContractValue(f *testing.F) {
	for _, seed := range []string{"active", "UNKNOWN", "new-upstream-state", "", "\x00secret"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if got := NormalizeResourceState(value); !isResourceState(got) {
			t.Fatalf("normalization returned out-of-contract state %q", got)
		}
	})
}

// validCapabilitySnapshot constructs a complete supported capability matrix.
func validCapabilitySnapshot() CapabilitySnapshot {
	codes := SupportedCapabilityCodes()
	descriptors := make([]CapabilityDescriptor, 0, len(codes))
	for _, code := range codes {
		descriptors = append(descriptors, CapabilityDescriptor{Code: code, Support: SupportLevelSupported})
	}
	return CapabilitySnapshot{
		AdapterVersion: "v1.0.0",
		Capabilities:   descriptors,
		ObservedAt:     validationReferenceTime(),
	}
}

// validationReferenceTime returns a deterministic owner-clock value shared by
// observation-boundary tests.
func validationReferenceTime() time.Time {
	return time.Date(2026, time.August, 7, 12, 0, 0, 0, time.UTC)
}

// validationSecretAccessor is the minimal non-nil secret dependency test fake.
type validationSecretAccessor struct{}

// Access satisfies secretcap.Accessor without exposing test secret material.
func (validationSecretAccessor) Access(context.Context, secretcap.AccessRequest, secretcap.Consumer) error {
	return nil
}

// validationSecretSink is the minimal non-nil confidential-output dependency test fake.
type validationSecretSink struct{}

// Store satisfies SecretSink without retaining or producing test secret material.
func (validationSecretSink) Store(
	context.Context,
	SecretStoreRequest,
	SecretProducer,
) (secretcap.Handle, error) {
	return secretcap.Handle{}, nil
}

type validationAuthenticationSessions struct{}

func (validationAuthenticationSessions) Access(
	context.Context,
	AuthenticationSessionRequest,
	AuthenticationTokenProducer,
	AuthenticationTokenConsumer,
) error {
	return nil
}

func (validationAuthenticationSessions) Invalidate(
	context.Context,
	AuthenticationSessionInvalidation,
) error {
	return nil
}

// validationHTTPClient is the minimal non-nil HTTP dependency test fake.
type validationHTTPClient struct{}

// Do satisfies HTTPClient without performing network I/O.
func (validationHTTPClient) Do(*http.Request) (*http.Response, error) { return nil, nil }

// validationClock is the minimal non-nil clock dependency test fake.
type validationClock struct{}

// Now supplies one deterministic non-zero test timestamp.
func (validationClock) Now() time.Time { return validationReferenceTime() }

// validationLogger is the minimal non-nil logger dependency test fake.
type validationLogger struct{}

// Log satisfies Logger without retaining diagnostics.
func (validationLogger) Log(context.Context, LogLevel, string, ...LogField) {}

// validationMetrics is the minimal non-nil metrics dependency test fake.
type validationMetrics struct{}

// AddCounter satisfies Metrics without retaining observations.
func (validationMetrics) AddCounter(context.Context, string, int64, ...MetricLabel) {}

// Observe satisfies Metrics without retaining observations.
func (validationMetrics) Observe(context.Context, string, float64, ...MetricLabel) {}
