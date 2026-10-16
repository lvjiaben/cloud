package productautomationtoken

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

type testClock struct {
	mu  sync.RWMutex
	now time.Time
}

func (c *testClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now
}

func (c *testClock) Advance(value time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(value)
	c.mu.Unlock()
}

type testAudienceReader struct {
	mu       sync.RWMutex
	audience Audience
	err      error
}

func (r *testAudienceReader) ReadProviderSelectionAudience(context.Context) (Audience, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.audience, r.err
}

func (r *testAudienceReader) Set(value Audience) {
	r.mu.Lock()
	r.audience = value
	r.mu.Unlock()
}

func TestCodecRoundTripEncryptsEvidenceAndKeepsEmptyMethodsNonNil(t *testing.T) {
	codec, _, _ := newTestCodec(t)
	evidence := validEvidence([]string{})
	token, err := codec.EncodeProviderSelection(context.Background(), evidence)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(token, tokenPrefix) || len(token) > maximumTokenLength {
		t.Fatalf("unexpected token envelope: %q", token)
	}
	for _, secret := range []string{evidence.PluginID, evidence.ReleaseDigest, evidence.DeclarationDigest, validAudience().ActorDigest, validAudience().SessionDigest} {
		if strings.Contains(token, secret) {
			t.Fatalf("ciphertext leaked sensitive evidence %q", secret)
		}
	}
	methodToken := mustEncode(t, codec, validEvidence([]string{"provision"}))
	if strings.Contains(methodToken, "provision") {
		t.Fatal("ciphertext leaked an authorized method")
	}
	decoded, err := codec.DecodeProviderSelection(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.AuthorizedMethods == nil || len(decoded.AuthorizedMethods) != 0 {
		t.Fatalf("empty methods did not round-trip as non-nil []: %#v", decoded.AuthorizedMethods)
	}
	encoded, _ := json.Marshal(decoded)
	if !strings.Contains(string(encoded), `"authorizedMethods":[]`) {
		t.Fatalf("empty methods did not encode as []: %s", encoded)
	}
}

func TestSelectionTokenKeepsIdentityWithoutEmbeddingProviderSchemas(t *testing.T) {
	codec, _, _ := newTestCodec(t)
	value := validEvidence([]string{"provision", "query.status"})
	value.ProductConfiguration = &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "large", Type: capmodel.ConfigurationFieldSingleLine, Label: &capmodel.LocalizedText{EnUS: strings.Repeat("x", 10000)}}}}
	value.InterfaceConfiguration = value.ProductConfiguration
	value.CustomerConfiguration = value.ProductConfiguration
	value.InterfaceConfigurationCardinality = capmodel.InterfaceConfigurationCardinalityMultiple
	token := mustEncode(t, codec, value)
	decoded, err := codec.DecodeProviderSelection(context.Background(), token)
	if err != nil || decoded.PluginID != value.PluginID || decoded.ReleaseDigest != value.ReleaseDigest || decoded.DeclarationDigest != value.DeclarationDigest || len(token) > maximumTokenLength {
		t.Fatalf("identity token failed: %v", err)
	}
	if decoded.ProviderMetadata != nil || decoded.ProductConfiguration != nil || decoded.InterfaceConfiguration != nil || decoded.CustomerConfiguration != nil || decoded.InterfaceConfigurationCardinality != "" {
		t.Fatal("bounded selection embedded executable declarations")
	}
}

func TestCodecRejectsExternalInvalidTokens(t *testing.T) {
	codec, clock, audience := newTestCodec(t)
	evidence := validEvidence([]string{"provision", "query.status"})
	token := mustEncode(t, codec, evidence)

	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(token, tokenPrefix))
	if err != nil {
		t.Fatal(err)
	}
	tamperedRaw := append([]byte(nil), raw...)
	tamperedRaw[len(tamperedRaw)-1] ^= 1
	assertInvalid(t, codec, tokenPrefix+base64.RawURLEncoding.EncodeToString(tamperedRaw))
	assertInvalid(t, codec, nonCanonicalAlias(t, codec))
	assertInvalid(t, codec, "base64."+base64.RawURLEncoding.EncodeToString([]byte(`{"pluginId":"server-automation"}`)))
	assertInvalid(t, codec, strings.Repeat("x", maximumTokenLength+1))

	wrongKey, err := New(bytes.Repeat([]byte{0x42}, 32), audience, rand.Reader, WithClock(clock))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(wrongKey.Close)
	assertInvalid(t, wrongKey, token)

	base := audience.audience
	for name, changed := range map[string]Audience{
		"actor":         withAudience(base, func(a *Audience) { a.ActorDigest = strings.Repeat("c", 64) }),
		"session":       withAudience(base, func(a *Audience) { a.SessionDigest = strings.Repeat("d", 64) }),
		"tenant":        withAudience(base, func(a *Audience) { a.TenantID = 1 }),
		"not-direct":    withAudience(base, func(a *Audience) { a.DirectStaff = false }),
		"proxy":         withAudience(base, func(a *Audience) { a.Proxied = true }),
		"acting-tenant": withAudience(base, func(a *Audience) { a.ActingTenant = true }),
	} {
		t.Run(name, func(t *testing.T) {
			audience.Set(changed)
			assertInvalid(t, codec, token)
			audience.Set(base)
		})
	}

	clock.Advance(defaultTTL + defaultClockSkew - time.Second)
	if _, err := codec.DecodeProviderSelection(context.Background(), token); err != nil {
		t.Fatalf("token inside allowed clock skew was rejected: %v", err)
	}
	clock.Advance(2 * time.Second)
	assertInvalid(t, codec, token)
	clock.Advance(-(defaultTTL + defaultClockSkew + time.Second))

	for name, mutate := range map[string]func(*payload){
		"purpose":        func(p *payload) { p.Purpose = "other-purpose" },
		"version":        func(p *payload) { p.Version++ },
		"tenant-payload": func(p *payload) { p.TenantID = 1 },
		"future-issued": func(p *payload) {
			p.IssuedAt += (defaultClockSkew + time.Second).Milliseconds()
			p.ExpiresAt = p.IssuedAt + defaultTTL.Milliseconds()
		},
	} {
		t.Run(name, func(t *testing.T) {
			assertInvalid(t, codec, resealMutated(t, codec, token, mutate))
		})
	}
}

func TestCodecRejectsNilAndNonCanonicalEvidenceButAcceptsEmptyMethods(t *testing.T) {
	codec, _, _ := newTestCodec(t)
	invalid := validEvidence(nil)
	invalid.AuthorizedMethods = nil
	if _, err := codec.EncodeProviderSelection(context.Background(), invalid); !errors.Is(err, productautomation.ErrInvalidSelectionToken) {
		t.Fatalf("nil methods must be invalid selection token: %v", err)
	}
	for name, mutate := range map[string]func(*capmodel.BusinessExtensionProviderEvidence){
		"unsorted": func(e *capmodel.BusinessExtensionProviderEvidence) {
			e.AuthorizedMethods = []string{"query", "provision"}
		},
		"duplicate": func(e *capmodel.BusinessExtensionProviderEvidence) { e.AuthorizedMethods = []string{"query", "query"} },
		"plugin":    func(e *capmodel.BusinessExtensionProviderEvidence) { e.PluginID = "Bad Plugin" },
		"release":   func(e *capmodel.BusinessExtensionProviderEvidence) { e.ReleaseVersion = "latest" },
		"contract":  func(e *capmodel.BusinessExtensionProviderEvidence) { e.ContractVersion = "V1" },
	} {
		t.Run(name, func(t *testing.T) {
			value := validEvidence([]string{})
			mutate(value)
			if _, err := codec.EncodeProviderSelection(context.Background(), value); !errors.Is(err, productautomation.ErrInvalidSelectionToken) {
				t.Fatalf("expected invalid selection token, got %v", err)
			}
		})
	}
}

func TestCodecKeyCopyClearInternalFailuresAndConcurrency(t *testing.T) {
	key := bytes.Repeat([]byte{0x21}, 32)
	clock := &testClock{now: time.Unix(1_800_000_000, 0)}
	audience := &testAudienceReader{audience: validAudience()}
	codec, err := New(key, audience, rand.Reader, WithClock(clock))
	if err != nil {
		t.Fatal(err)
	}
	key[0] ^= 0xff
	token := mustEncode(t, codec, validEvidence([]string{}))
	if _, err = codec.DecodeProviderSelection(context.Background(), token); err != nil {
		t.Fatalf("constructor did not copy key: %v", err)
	}

	const workers = 32
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for index := 0; index < workers; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value := validEvidence([]string{"provision"})
			encoded, encodeErr := codec.EncodeProviderSelection(context.Background(), value)
			if encodeErr != nil {
				errs <- encodeErr
				return
			}
			_, decodeErr := codec.DecodeProviderSelection(context.Background(), encoded)
			errs <- decodeErr
		}()
	}
	wg.Wait()
	close(errs)
	for err = range errs {
		if err != nil {
			t.Fatalf("concurrent codec use failed: %v", err)
		}
	}
	retainedKey := codec.key
	codec.Close()
	if len(codec.key) != 0 {
		t.Fatal("Close did not clear the retained key boundary")
	}
	for _, value := range retainedKey {
		if value != 0 {
			t.Fatal("Close did not zero the copied key material")
		}
	}
	if _, err = codec.DecodeProviderSelection(context.Background(), token); err == nil || errors.Is(err, productautomation.ErrInvalidSelectionToken) {
		t.Fatalf("closed codec must return an internal lifecycle error, got %v", err)
	}
	if _, err = codec.DecodeProviderSelection(context.Background(), "malformed"); err == nil || errors.Is(err, productautomation.ErrInvalidSelectionToken) {
		t.Fatalf("closed codec must classify malformed input as internal misuse, got %v", err)
	}
	if _, err = codec.EncodeProviderSelection(context.Background(), &capmodel.BusinessExtensionProviderEvidence{}); err == nil || errors.Is(err, productautomation.ErrInvalidSelectionToken) {
		t.Fatalf("closed codec must classify invalid evidence as internal misuse, got %v", err)
	}

	failing, err := New(bytes.Repeat([]byte{0x31}, 32), audience, errorReader{}, WithClock(clock))
	if err != nil {
		t.Fatal(err)
	}
	defer failing.Close()
	if _, err = failing.EncodeProviderSelection(context.Background(), validEvidence([]string{})); err == nil || errors.Is(err, productautomation.ErrInvalidSelectionToken) {
		t.Fatalf("random failure must remain an internal error, got %v", err)
	}
	if _, err = New([]byte("short"), audience, rand.Reader); err == nil || errors.Is(err, productautomation.ErrInvalidSelectionToken) {
		t.Fatalf("invalid key must remain a construction error, got %v", err)
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

type typedNilRandom struct{}

func (*typedNilRandom) Read([]byte) (int, error) { return 0, nil }

func newTestCodec(t *testing.T) (*Codec, *testClock, *testAudienceReader) {
	t.Helper()
	clock := &testClock{now: time.Unix(1_800_000_000, 0)}
	audience := &testAudienceReader{audience: validAudience()}
	codec, err := New(bytes.Repeat([]byte{0x11}, 32), audience, rand.Reader, WithClock(clock))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(codec.Close)
	return codec, clock, audience
}

func validAudience() Audience {
	return Audience{TenantID: 0, ActorDigest: strings.Repeat("a", 64), SessionDigest: strings.Repeat("b", 64), DirectStaff: true}
}

func validEvidence(methods []string) *capmodel.BusinessExtensionProviderEvidence {
	if methods == nil {
		methods = []string{}
	}
	return &capmodel.BusinessExtensionProviderEvidence{
		PluginID: "server-automation", DisplayName: "Provider A", ReleaseVersion: "v2026.8.0",
		Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: "v1", AuthorizedMethods: methods,
		ReleaseDigest: strings.Repeat("1", 64), DeclarationDigest: strings.Repeat("a", 64),
	}
}

func mustEncode(t *testing.T, codec *Codec, value *capmodel.BusinessExtensionProviderEvidence) string {
	t.Helper()
	token, err := codec.EncodeProviderSelection(context.Background(), value)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func assertInvalid(t *testing.T, codec *Codec, token string) {
	t.Helper()
	if _, err := codec.DecodeProviderSelection(context.Background(), token); !errors.Is(err, productautomation.ErrInvalidSelectionToken) {
		t.Fatalf("expected invalid selection token, got %v", err)
	}
}

func withAudience(base Audience, mutate func(*Audience)) Audience {
	result := base
	mutate(&result)
	return result
}

func resealMutated(t *testing.T, codec *Codec, token string, mutate func(*payload)) string {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(token, tokenPrefix))
	if err != nil {
		t.Fatal(err)
	}
	aead, err := codec.aead()
	if err != nil {
		t.Fatal(err)
	}
	plain, err := aead.Open(nil, raw[:nonceSize], raw[nonceSize:], additionalData)
	if err != nil {
		t.Fatal(err)
	}
	var document payload
	if err = json.Unmarshal(plain, &document); err != nil {
		t.Fatal(err)
	}
	mutate(&document)
	plain, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	result, err := codec.seal(plain)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func nonCanonicalAlias(t *testing.T, codec *Codec) string {
	t.Helper()
	value := validEvidence([]string{})
	for attempt := 0; attempt < 3; attempt++ {
		value.DisplayName += "x"
		token := mustEncode(t, codec, value)
		suffix := strings.TrimPrefix(token, tokenPrefix)
		raw, err := base64.RawURLEncoding.DecodeString(suffix)
		if err != nil {
			t.Fatal(err)
		}
		if remainder := len(raw) % 3; remainder != 0 {
			const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
			last := strings.IndexByte(alphabet, suffix[len(suffix)-1])
			if last < 0 {
				t.Fatal("canonical Base64URL character was not found")
			}
			alias := []byte(suffix)
			alias[len(alias)-1] = alphabet[last^1]
			if decodedAlias, decodeErr := base64.RawURLEncoding.DecodeString(string(alias)); decodeErr != nil || !bytes.Equal(decodedAlias, raw) {
				t.Fatalf("failed to construct same-raw non-canonical alias: err=%v", decodeErr)
			}
			return tokenPrefix + string(alias)
		}
	}
	t.Fatal("failed to construct non-canonical Base64URL alias")
	return ""
}

func TestCodecConstructorOptionBounds(t *testing.T) {
	audience := &testAudienceReader{audience: validAudience()}
	key := bytes.Repeat([]byte{0x41}, 32)
	for name, option := range map[string]Option{
		"zero-ttl": WithTTL(0), "long-ttl": WithTTL(maximumTTL + time.Second),
		"nanosecond-ttl": WithTTL(time.Nanosecond), "fractional-millisecond-ttl": WithTTL(1500*time.Millisecond + time.Nanosecond),
		"negative-skew": WithClockSkew(-time.Second), "large-skew": WithClockSkew(maximumClockSkew + time.Second),
		"fractional-millisecond-skew": WithClockSkew(time.Millisecond + time.Nanosecond),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := New(key, audience, rand.Reader, option); err == nil {
				t.Fatal("expected unsafe option to be rejected")
			}
		})
	}
	clock := &testClock{now: time.Unix(1_800_000_000, 123_000_000)}
	subsecond, err := New(key, audience, rand.Reader, WithClock(clock), WithTTL(1500*time.Millisecond), WithClockSkew(0))
	if err != nil {
		t.Fatalf("construct sub-second-safe TTL codec: %v", err)
	}
	defer subsecond.Close()
	token := mustEncode(t, subsecond, validEvidence([]string{}))
	if _, err = subsecond.DecodeProviderSelection(context.Background(), token); err != nil {
		t.Fatalf("millisecond TTL token rejected itself: %v", err)
	}
	clock.Advance(1501 * time.Millisecond)
	assertInvalid(t, subsecond, token)
}

func TestCodecRejectsTypedNilDependencies(t *testing.T) {
	var (
		nilAudience *testAudienceReader
		nilClock    *testClock
		nilRandom   *typedNilRandom
	)
	key := bytes.Repeat([]byte{0x43}, 32)
	validAudienceReader := &testAudienceReader{audience: validAudience()}
	if _, err := New(key, nilAudience, rand.Reader); err == nil {
		t.Fatal("typed-nil audience reader was accepted")
	}
	if _, err := New(key, validAudienceReader, nilRandom); err == nil {
		t.Fatal("typed-nil random reader was accepted")
	}
	if _, err := New(key, validAudienceReader, rand.Reader, WithClock(nilClock)); err == nil {
		t.Fatal("typed-nil clock option was accepted")
	}
	if _, err := New(key, validAudienceReader, rand.Reader, WithRandom(nilRandom)); err == nil {
		t.Fatal("typed-nil random option was accepted")
	}
	if _, err := New(key, validAudienceReader, rand.Reader, func(cfg *config) error {
		cfg.clock = nilClock
		return nil
	}); err == nil {
		t.Fatal("custom option typed-nil clock was accepted")
	}

	codec, _, _ := newTestCodec(t)
	codec.audience = nilAudience
	if value, err := codec.readAudience(context.Background()); err == nil || value != (Audience{}) {
		t.Fatalf("readAudience accepted typed-nil reader: value=%+v err=%v", value, err)
	}
	codec.audience = validAudienceReader
	codec.clock = nilClock
	if value, err := codec.readAudience(context.Background()); err == nil || value != (Audience{}) {
		t.Fatalf("readAudience accepted typed-nil clock: value=%+v err=%v", value, err)
	}
}

func TestCodecConcurrentCloseIsRaceSafeAndKeepsInternalClassification(t *testing.T) {
	codec, _, _ := newTestCodec(t)
	token := mustEncode(t, codec, validEvidence([]string{}))
	start := make(chan struct{})
	errs := make(chan error, 64)
	var wg sync.WaitGroup
	for index := 0; index < 64; index++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start
			var err error
			if worker%2 == 0 {
				_, err = codec.EncodeProviderSelection(context.Background(), validEvidence([]string{}))
			} else {
				_, err = codec.DecodeProviderSelection(context.Background(), token)
			}
			errs <- err
		}(index)
	}
	close(start)
	codec.Close()
	wg.Wait()
	close(errs)
	for err := range errs {
		if errors.Is(err, productautomation.ErrInvalidSelectionToken) {
			t.Fatalf("concurrent Close was misclassified as external invalid input: %v", err)
		}
	}
}

func ExampleCodec() {
	fmt.Println(tokenPrefix)
	// Output: pase1.
}
