// Package productautomationtoken provides short-lived encrypted provider
// selections for the product-automation administration flow.
package productautomationtoken

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

const (
	formatVersion       = 1
	tokenPrefix         = "pase1."
	tokenPurpose        = "product-automation.provider-selection"
	defaultTTL          = 5 * time.Minute
	defaultClockSkew    = 30 * time.Second
	maximumTTL          = 15 * time.Minute
	maximumClockSkew    = time.Minute
	maximumTokenLength  = 4096
	maximumDisplayRunes = 256
	nonceSize           = 12
)

var (
	additionalData         = []byte(tokenPurpose + "|v1")
	opaqueDigestPattern    = regexp.MustCompile(`^[A-Za-z0-9_-]{16,256}$`)
	pluginIDPattern        = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	releaseVersionPattern  = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$`)
	contractVersionPattern = regexp.MustCompile(`^v[1-9][0-9]*$`)
	methodPattern          = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)
	digestPattern          = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Audience is a server-derived, non-user-controlled binding for one direct
// staff session. ActorDigest and SessionDigest must be stable opaque digests,
// never raw identifiers or credentials.
type Audience struct {
	TenantID      int
	ActorDigest   string
	SessionDigest string
	DirectStaff   bool
	Proxied       bool
	ActingTenant  bool
}

// AudienceReader derives the current trusted staff-session audience from ctx.
type AudienceReader interface {
	ReadProviderSelectionAudience(ctx context.Context) (Audience, error)
}

// Clock supplies wall-clock time and is injectable for deterministic tests.
type Clock interface {
	Now() time.Time
}

type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }

type config struct {
	ttl    time.Duration
	skew   time.Duration
	clock  Clock
	random io.Reader
}

// Option configures safe codec dependencies.
type Option func(*config) error

// WithClock replaces the wall clock.
func WithClock(clock Clock) Option {
	return func(cfg *config) error {
		if isNilDependency(clock) {
			return errors.New("provider selection token clock is required")
		}
		cfg.clock = clock
		return nil
	}
}

// WithRandom replaces the cryptographic nonce source.
func WithRandom(random io.Reader) Option {
	return func(cfg *config) error {
		if isNilDependency(random) {
			return errors.New("provider selection token random source is required")
		}
		cfg.random = random
		return nil
	}
}

// WithTTL replaces the default five-minute lifetime.
func WithTTL(ttl time.Duration) Option {
	return func(cfg *config) error {
		cfg.ttl = ttl
		return nil
	}
}

// WithClockSkew replaces the allowed clock skew.
func WithClockSkew(skew time.Duration) Option {
	return func(cfg *config) error {
		cfg.skew = skew
		return nil
	}
}

// Codec encrypts and authenticates provider selections with AES-256-GCM.
type Codec struct {
	mu       sync.RWMutex
	key      []byte
	audience AudienceReader
	clock    Clock
	ttl      time.Duration
	skew     time.Duration
	randomMu sync.Mutex
	random   io.Reader
}

type payload struct {
	Version   int      `json:"version"`
	Purpose   string   `json:"purpose"`
	IssuedAt  int64    `json:"issuedAt"`
	ExpiresAt int64    `json:"expiresAt"`
	TenantID  int      `json:"tenant"`
	Actor     string   `json:"actor"`
	Session   string   `json:"session"`
	Evidence  evidence `json:"evidence"`
}

type evidence struct {
	PluginID          string   `json:"pluginId"`
	DisplayName       string   `json:"displayName"`
	ReleaseVersion    string   `json:"releaseVersion"`
	Point             string   `json:"point"`
	ContractVersion   string   `json:"contractVersion"`
	AuthorizedMethods []string `json:"authorizedMethods"`
	ReleaseDigest     string   `json:"releaseDigest"`
	DeclarationDigest string   `json:"declarationDigest"`
}

// New constructs a production codec and copies the supplied key.
func New(key []byte, audience AudienceReader, random io.Reader, options ...Option) (*Codec, error) {
	if len(key) != 32 {
		return nil, errors.New("provider selection token requires a 32-byte key")
	}
	if isNilDependency(audience) {
		return nil, errors.New("provider selection token audience reader is required")
	}
	if isNilDependency(random) {
		return nil, errors.New("provider selection token random source is required")
	}
	cfg := config{ttl: defaultTTL, skew: defaultClockSkew, clock: wallClock{}, random: random}
	for _, option := range options {
		if option == nil {
			return nil, errors.New("provider selection token option is required")
		}
		if err := option(&cfg); err != nil {
			return nil, err
		}
	}
	if isNilDependency(cfg.clock) || isNilDependency(cfg.random) {
		return nil, errors.New("provider selection token runtime dependencies are required")
	}
	if cfg.ttl < time.Millisecond || cfg.ttl > maximumTTL || cfg.ttl%time.Millisecond != 0 {
		return nil, errors.New("provider selection token TTL is outside the safe range")
	}
	if cfg.skew < 0 || cfg.skew > maximumClockSkew || cfg.skew%time.Millisecond != 0 {
		return nil, errors.New("provider selection token clock skew is outside the safe range")
	}
	return &Codec{
		key: append([]byte(nil), key...), audience: audience, clock: cfg.clock,
		ttl: cfg.ttl, skew: cfg.skew, random: cfg.random,
	}, nil
}

// Close clears the in-memory key. Calls after Close fail as internal misuse.
func (c *Codec) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for index := range c.key {
		c.key[index] = 0
	}
	c.key = nil
}

// EncodeProviderSelection returns an encrypted, session-bound selection.
func (c *Codec) EncodeProviderSelection(ctx context.Context, value *capmodel.BusinessExtensionProviderEvidence) (string, error) {
	if err := c.ensureOpen(); err != nil {
		return "", err
	}
	if err := validateEvidence(value); err != nil {
		return "", invalidToken(err.Error())
	}
	audience, err := c.readAudience(ctx)
	if err != nil {
		return "", err
	}
	now := c.clock.Now().UTC()
	document := payload{
		Version: formatVersion, Purpose: tokenPurpose, IssuedAt: now.UnixMilli(), ExpiresAt: now.Add(c.ttl).UnixMilli(),
		TenantID: audience.TenantID, Actor: audience.ActorDigest, Session: audience.SessionDigest,
		Evidence: evidenceFromModel(value),
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return "", fmt.Errorf("marshal provider selection token: %w", err)
	}
	return c.seal(encoded)
}

// DecodeProviderSelection authenticates, decrypts, and rebinds one selection
// to the current direct staff session. It returns identity only; executable
// provider declarations must be reloaded by exact release and digest before use.
func (c *Codec) DecodeProviderSelection(ctx context.Context, token string) (*capmodel.BusinessExtensionProviderEvidence, error) {
	if err := c.ensureOpen(); err != nil {
		return nil, err
	}
	if token == "" || len(token) > maximumTokenLength || !strings.HasPrefix(token, tokenPrefix) {
		return nil, invalidToken("invalid format")
	}
	suffix := strings.TrimPrefix(token, tokenPrefix)
	raw, err := base64.RawURLEncoding.DecodeString(suffix)
	if err != nil || len(raw) < nonceSize+16 {
		return nil, invalidToken("invalid encoding")
	}
	if base64.RawURLEncoding.EncodeToString(raw) != suffix {
		return nil, invalidToken("non-canonical encoding")
	}
	aead, err := c.aead()
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(nil, raw[:nonceSize], raw[nonceSize:], additionalData)
	if err != nil {
		return nil, invalidToken("authentication failed")
	}
	var document payload
	decoder := json.NewDecoder(strings.NewReader(string(plaintext)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&document); err != nil {
		return nil, invalidToken("invalid payload")
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return nil, invalidToken("invalid trailing payload")
	}
	if document.Version != formatVersion || document.Purpose != tokenPurpose {
		return nil, invalidToken("invalid version or purpose")
	}
	now := c.clock.Now().UTC()
	issuedAt := time.UnixMilli(document.IssuedAt)
	expiresAt := time.UnixMilli(document.ExpiresAt)
	if document.ExpiresAt <= document.IssuedAt || issuedAt.After(now.Add(c.skew)) || now.After(expiresAt.Add(c.skew)) || expiresAt.Sub(issuedAt) != c.ttl {
		return nil, invalidToken("invalid lifetime")
	}
	audience, err := c.readAudience(ctx)
	if err != nil {
		return nil, err
	}
	if document.TenantID != audience.TenantID || !constantEqual(document.Actor, audience.ActorDigest) || !constantEqual(document.Session, audience.SessionDigest) {
		return nil, invalidToken("audience mismatch")
	}
	result := document.Evidence.toModel()
	if err = validateEvidence(result); err != nil {
		return nil, invalidToken("invalid evidence")
	}
	return result, nil
}

func (c *Codec) readAudience(ctx context.Context) (Audience, error) {
	if c == nil || ctx == nil || ctx.Err() != nil || isNilDependency(c.audience) || isNilDependency(c.clock) {
		return Audience{}, errors.New("provider selection token codec is unavailable")
	}
	value, err := c.audience.ReadProviderSelectionAudience(ctx)
	if err != nil {
		return Audience{}, fmt.Errorf("read provider selection token audience: %w", err)
	}
	if value.TenantID != 0 || !value.DirectStaff || value.Proxied || value.ActingTenant ||
		!opaqueDigestPattern.MatchString(value.ActorDigest) || !opaqueDigestPattern.MatchString(value.SessionDigest) {
		return Audience{}, invalidToken("invalid staff audience")
	}
	return value, nil
}

func isNilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func (c *Codec) seal(plaintext []byte) (string, error) {
	aead, err := c.aead()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, nonceSize)
	c.randomMu.Lock()
	_, randomErr := io.ReadFull(c.random, nonce)
	c.randomMu.Unlock()
	if randomErr != nil {
		return "", fmt.Errorf("read provider selection token nonce: %w", randomErr)
	}
	sealed := aead.Seal(nil, nonce, plaintext, additionalData)
	raw := make([]byte, 0, len(nonce)+len(sealed))
	raw = append(raw, nonce...)
	raw = append(raw, sealed...)
	token := tokenPrefix + base64.RawURLEncoding.EncodeToString(raw)
	if len(token) > maximumTokenLength {
		return "", errors.New("encrypted provider selection token exceeds maximum length")
	}
	return token, nil
}

func (c *Codec) aead() (cipher.AEAD, error) {
	if c == nil {
		return nil, errors.New("provider selection token codec is unavailable")
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.key) != 32 {
		return nil, errors.New("provider selection token codec is closed")
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("create provider selection token cipher: %w", err)
	}
	return cipher.NewGCM(block)
}

func (c *Codec) ensureOpen() error {
	if c == nil {
		return errors.New("provider selection token codec is unavailable")
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.key) != 32 {
		return errors.New("provider selection token codec is closed")
	}
	return nil
}

func validateEvidence(value *capmodel.BusinessExtensionProviderEvidence) error {
	if value == nil || !pluginIDPattern.MatchString(value.PluginID) || len(value.PluginID) > 64 ||
		value.DisplayName == "" || value.DisplayName != strings.TrimSpace(value.DisplayName) || !utf8.ValidString(value.DisplayName) || utf8.RuneCountInString(value.DisplayName) > maximumDisplayRunes ||
		!releaseVersionPattern.MatchString(value.ReleaseVersion) || value.Point != capmodel.BusinessExtensionProductFulfillment ||
		!contractVersionPattern.MatchString(value.ContractVersion) || value.AuthorizedMethods == nil ||
		!digestPattern.MatchString(value.ReleaseDigest) || !digestPattern.MatchString(value.DeclarationDigest) {
		return errors.New("provider evidence is not canonical")
	}
	previous := ""
	for _, method := range value.AuthorizedMethods {
		if !methodPattern.MatchString(method) || method <= previous {
			return errors.New("provider methods are not canonical")
		}
		previous = method
	}
	return nil
}

func evidenceFromModel(value *capmodel.BusinessExtensionProviderEvidence) evidence {
	return evidence{
		PluginID: value.PluginID, DisplayName: value.DisplayName, ReleaseVersion: value.ReleaseVersion,
		Point: value.Point, ContractVersion: value.ContractVersion,
		AuthorizedMethods: append([]string{}, value.AuthorizedMethods...),
		ReleaseDigest:     value.ReleaseDigest, DeclarationDigest: value.DeclarationDigest,
	}
}

func (value evidence) toModel() *capmodel.BusinessExtensionProviderEvidence {
	return &capmodel.BusinessExtensionProviderEvidence{
		PluginID: value.PluginID, DisplayName: value.DisplayName, ReleaseVersion: value.ReleaseVersion,
		Point: value.Point, ContractVersion: value.ContractVersion,
		AuthorizedMethods: append([]string{}, value.AuthorizedMethods...),
		ReleaseDigest:     value.ReleaseDigest, DeclarationDigest: value.DeclarationDigest,
	}
}

func constantEqual(left, right string) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func invalidToken(reason string) error {
	return fmt.Errorf("%w: %s", productautomation.ErrInvalidSelectionToken, reason)
}

var _ productautomation.ProviderSelectionCodec = (*Codec)(nil)
