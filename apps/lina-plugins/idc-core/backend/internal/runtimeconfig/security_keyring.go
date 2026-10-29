// Package runtimeconfig validates deployment-owned security configuration.
package runtimeconfig

import (
	"context"
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/reauth"
	secretservice "lina-plugin-idc-core/backend/internal/service/secret"
)

const (
	// SecurityKeyringEnvironment is the only deployment input accepted for
	// audit signing and secret envelope roots. It stays outside persisted data.
	SecurityKeyringEnvironment = "LINAPRO_IDC_CORE_SECURITY_KEYRING"
	maximumSecurityKeyCount    = 8
	securityRootKeyBytes       = 32
	derivedSecurityKeyBytes    = 32
	securityDerivationSalt     = "idc-core/security-keyring/v1"
)

type encodedSecurityKeyring struct {
	ActiveKeyID string            `json:"active_key_id"`
	Reservation string            `json:"reservation_key"`
	RootKeys    map[string]string `json:"keys"`
}

type decodedSecurityKeyring struct {
	activeKeyID string
	reservation []byte
	rootKeys    map[string][]byte
}

type deploymentSecretKeyProvider struct {
	activeKeyID string
	keys        map[string][]byte
}

type deploymentRecentAuthKeyProvider struct {
	activeKeyID string
	keys        map[string][]byte
}

var _ secretservice.KeyProvider = (*deploymentSecretKeyProvider)(nil)
var _ reauth.SigningKeyProvider = (*deploymentRecentAuthKeyProvider)(nil)

// LoadAuditConfig reads and validates a versioned deployment keyring. Missing
// or malformed input fails startup; no random, database, or hard-coded fallback
// is permitted because old evidence must remain verifiable after a restart.
func LoadAuditConfig() (audit.Config, error) {
	raw, exists := os.LookupEnv(SecurityKeyringEnvironment)
	if !exists {
		return audit.Config{}, gerror.New("IDC core security keyring is not configured")
	}
	return ParseAuditConfig(raw)
}

// ParseAuditConfig converts one canonical JSON keyring into purpose-separated
// audit keys. It is exported for deterministic deployment validation tests.
func ParseAuditConfig(raw string) (audit.Config, error) {
	decoded, err := parseSecurityKeyring(raw)
	if err != nil {
		return audit.Config{}, err
	}
	defer clearDecodedSecurityKeyring(&decoded)
	ids := sortedSecurityKeyIDs(decoded.rootKeys)
	replayKeys := make([]audit.IntegrityKey, 0, len(ids))
	integrityKeys := make([]audit.IntegrityKey, 0, len(ids))
	for _, id := range ids {
		replay, replayErr := derive(decoded.rootKeys[id], "audit/replay/"+id)
		integrity, integrityErr := derive(decoded.rootKeys[id], "audit/integrity/"+id)
		if replayErr != nil || integrityErr != nil {
			clear(replay)
			clear(integrity)
			clearAuditKeys(replayKeys)
			clearAuditKeys(integrityKeys)
			return audit.Config{}, gerror.New("IDC core security key derivation failed")
		}
		replayKeys = append(replayKeys, audit.IntegrityKey{ID: "replay-" + id, Secret: replay})
		integrityKeys = append(integrityKeys, audit.IntegrityKey{ID: "integrity-" + id, Secret: integrity})
	}
	config := audit.Config{
		ReservationLookupKey: audit.IntegrityKey{
			ID: "reservation-v1", Secret: append([]byte(nil), decoded.reservation...),
		},
	}
	for index, id := range ids {
		if id == decoded.activeKeyID {
			config.ActiveReplayDigestKey = replayKeys[index]
			config.ActiveIntegrityKey = integrityKeys[index]
			continue
		}
		config.RetainedReplayDigestKeys = append(config.RetainedReplayDigestKeys, replayKeys[index])
		config.RetainedIntegrityKeys = append(config.RetainedIntegrityKeys, integrityKeys[index])
	}
	return config, nil
}

// LoadSecretKeyProvider derives a versioned envelope-wrapping key provider
// from the same deployment roots using a distinct HKDF purpose namespace.
func LoadSecretKeyProvider() (secretservice.KeyProvider, error) {
	raw, exists := os.LookupEnv(SecurityKeyringEnvironment)
	if !exists {
		return nil, gerror.New("IDC core security keyring is not configured")
	}
	return ParseSecretKeyProvider(raw)
}

// LoadCatalogCursorKey derives the active, deployment-owned key used only for
// encrypting short-lived catalog management cursors. A deployment key rotation
// intentionally invalidates cursors issued before restart; callers restart at
// the first page instead of retaining another long-lived secret purpose.
func LoadCatalogCursorKey() ([]byte, error) {
	raw, exists := os.LookupEnv(SecurityKeyringEnvironment)
	if !exists {
		return nil, gerror.New("IDC core security keyring is not configured")
	}
	decoded, err := parseSecurityKeyring(raw)
	if err != nil {
		return nil, err
	}
	defer clearDecodedSecurityKeyring(&decoded)
	key, err := derive(
		decoded.rootKeys[decoded.activeKeyID],
		"catalog/management-cursor/v1/"+decoded.activeKeyID,
	)
	if err != nil {
		clear(key)
		return nil, gerror.New("IDC core security key derivation failed")
	}
	return key, nil
}

// LoadProductFulfillmentEvidenceKey derives the active deployment-owned key
// used only to seal owner-private provider evidence. The returned key ID is
// persisted with the ciphertext so retained deployment roots can be resolved
// during a later audited evidence-reader implementation.
func LoadProductFulfillmentEvidenceKey() (string, []byte, error) {
	raw, exists := os.LookupEnv(SecurityKeyringEnvironment)
	if !exists {
		return "", nil, gerror.New("IDC core security keyring is not configured")
	}
	decoded, err := parseSecurityKeyring(raw)
	if err != nil {
		return "", nil, err
	}
	defer clearDecodedSecurityKeyring(&decoded)
	key, err := derive(
		decoded.rootKeys[decoded.activeKeyID],
		"product-fulfillment/provider-evidence/v1/"+decoded.activeKeyID,
	)
	if err != nil {
		clear(key)
		return "", nil, gerror.New("IDC core security key derivation failed")
	}
	return decoded.activeKeyID, key, nil
}

// LoadMarketingCursorKey derives the active deployment key used only for
// short-lived marketing segment and campaign keyset cursors.
func LoadMarketingCursorKey() ([]byte, error) {
	raw, exists := os.LookupEnv(SecurityKeyringEnvironment)
	if !exists {
		return nil, gerror.New("IDC core security keyring is not configured")
	}
	decoded, err := parseSecurityKeyring(raw)
	if err != nil {
		return nil, err
	}
	defer clearDecodedSecurityKeyring(&decoded)
	key, err := derive(
		decoded.rootKeys[decoded.activeKeyID],
		"marketing/management-cursor/v1/"+decoded.activeKeyID,
	)
	if err != nil {
		clear(key)
		return nil, gerror.New("IDC core security key derivation failed")
	}
	return key, nil
}

// LoadCommerceOrderCursorKey derives the active deployment key used only for
// customer and staff order-directory cursors.
func LoadCommerceOrderCursorKey() ([]byte, error) {
	raw, exists := os.LookupEnv(SecurityKeyringEnvironment)
	if !exists {
		return nil, gerror.New("IDC core security keyring is not configured")
	}
	decoded, err := parseSecurityKeyring(raw)
	if err != nil {
		return nil, err
	}
	defer clearDecodedSecurityKeyring(&decoded)
	key, err := derive(
		decoded.rootKeys[decoded.activeKeyID],
		"commerce/order-directory-cursor/v1/"+decoded.activeKeyID,
	)
	if err != nil {
		clear(key)
		return nil, gerror.New("IDC core security key derivation failed")
	}
	return key, nil
}

// LoadBillingDirectoryCursorKey derives the active deployment key used only
// for authenticated customer and staff financial-directory cursors.
func LoadBillingDirectoryCursorKey() ([]byte, error) {
	raw, exists := os.LookupEnv(SecurityKeyringEnvironment)
	if !exists {
		return nil, gerror.New("IDC core security keyring is not configured")
	}
	decoded, err := parseSecurityKeyring(raw)
	if err != nil {
		return nil, err
	}
	defer clearDecodedSecurityKeyring(&decoded)
	key, err := derive(
		decoded.rootKeys[decoded.activeKeyID],
		"billing/directory-cursor/v1/"+decoded.activeKeyID,
	)
	if err != nil {
		clear(key)
		return nil, gerror.New("IDC core security key derivation failed")
	}
	return key, nil
}

// LoadCustomerAssistanceTokenKey derives an installation-owned key used only
// to create deterministic, replay-safe, short-lived assistance access tokens.
func LoadCustomerAssistanceTokenKey() ([]byte, error) {
	raw, exists := os.LookupEnv(SecurityKeyringEnvironment)
	if !exists {
		return nil, gerror.New("IDC core security keyring is not configured")
	}
	decoded, err := parseSecurityKeyring(raw)
	if err != nil {
		return nil, err
	}
	defer clearDecodedSecurityKeyring(&decoded)
	key, err := derive(decoded.reservation, "customer/assistance-token/v1")
	if err != nil {
		clear(key)
		return nil, gerror.New("IDC core security key derivation failed")
	}
	return key, nil
}

// LoadProductAutomationSelectionKey derives the installation-owned key used
// only to encrypt product-automation provider selections. It deliberately does
// not reuse audience-binding, cursor, identity, onboarding,
// recent-authentication, audit, or secret-envelope key labels.
func LoadProductAutomationSelectionKey() ([]byte, error) {
	return loadProductAutomationSelectionSubkey("token-encryption/v1")
}

// LoadProductAutomationSelectionAudienceKey derives the installation-owned
// key used only for provider-selection actor and session HMAC bindings. It is
// cryptographically isolated from the token encryption key.
func LoadProductAutomationSelectionAudienceKey() ([]byte, error) {
	return loadProductAutomationSelectionSubkey("audience-binding/v1")
}

func loadProductAutomationSelectionSubkey(subpurpose string) ([]byte, error) {
	raw, exists := os.LookupEnv(SecurityKeyringEnvironment)
	if !exists {
		return nil, gerror.New("IDC core security keyring is not configured")
	}
	decoded, err := parseSecurityKeyring(raw)
	if err != nil {
		return nil, err
	}
	defer clearDecodedSecurityKeyring(&decoded)
	key, err := derive(decoded.reservation, "product-automation/provider-selection/"+subpurpose)
	if err != nil {
		clear(key)
		return nil, gerror.New("IDC core security key derivation failed")
	}
	return key, nil
}

// LoadIdentityLookupKey derives the installation-stable key used only for
// exact personal-identity duplicate-risk lookup. It derives from the dedicated
// reservation root so rotating envelope keys cannot silently change persisted
// identity digests.
func LoadIdentityLookupKey() ([]byte, error) {
	raw, exists := os.LookupEnv(SecurityKeyringEnvironment)
	if !exists {
		return nil, gerror.New("IDC core security keyring is not configured")
	}
	decoded, err := parseSecurityKeyring(raw)
	if err != nil {
		return nil, err
	}
	defer clearDecodedSecurityKeyring(&decoded)
	key, err := derive(decoded.reservation, "customer/identity-lookup/v1")
	if err != nil {
		clear(key)
		return nil, gerror.New("IDC core security key derivation failed")
	}
	return key, nil
}

// LoadCustomerOnboardingTokenKey derives the installation-stable key used to
// recover the same opaque onboarding token for an idempotent verified request.
func LoadCustomerOnboardingTokenKey() ([]byte, error) {
	raw, exists := os.LookupEnv(SecurityKeyringEnvironment)
	if !exists {
		return nil, gerror.New("IDC core security keyring is not configured")
	}
	decoded, err := parseSecurityKeyring(raw)
	if err != nil {
		return nil, err
	}
	defer clearDecodedSecurityKeyring(&decoded)
	key, err := derive(decoded.reservation, "customer/onboarding-token/v1")
	if err != nil {
		clear(key)
		return nil, gerror.New("IDC core security key derivation failed")
	}
	return key, nil
}

// LoadRecentAuthSigningKeyProvider derives rotating proof-signing keys from a
// purpose namespace that is distinct from audit, envelope, and cursor keys.
func LoadRecentAuthSigningKeyProvider() (reauth.SigningKeyProvider, error) {
	raw, exists := os.LookupEnv(SecurityKeyringEnvironment)
	if !exists {
		return nil, gerror.New("IDC core security keyring is not configured")
	}
	return ParseRecentAuthSigningKeyProvider(raw)
}

// ParseRecentAuthSigningKeyProvider is the deterministic validation entry for
// recent-authentication proof signing and retained-key verification.
func ParseRecentAuthSigningKeyProvider(raw string) (reauth.SigningKeyProvider, error) {
	decoded, err := parseSecurityKeyring(raw)
	if err != nil {
		return nil, err
	}
	defer clearDecodedSecurityKeyring(&decoded)
	keys := make(map[string][]byte, len(decoded.rootKeys))
	for id, root := range decoded.rootKeys {
		key, deriveErr := derive(root, "reauth/proof-signing/v1/"+id)
		if deriveErr != nil {
			clearSecurityKeys(keys)
			return nil, gerror.New("IDC core security key derivation failed")
		}
		keys[id] = key
	}
	return &deploymentRecentAuthKeyProvider{
		activeKeyID: decoded.activeKeyID,
		keys:        keys,
	}, nil
}

// ParseSecretKeyProvider is the deterministic deployment-validation entry for
// secret envelope keys. Returned key material is detached on every access.
func ParseSecretKeyProvider(raw string) (secretservice.KeyProvider, error) {
	decoded, err := parseSecurityKeyring(raw)
	if err != nil {
		return nil, err
	}
	defer clearDecodedSecurityKeyring(&decoded)
	keys := make(map[string][]byte, len(decoded.rootKeys))
	for id, root := range decoded.rootKeys {
		key, deriveErr := derive(root, "secret/envelope/"+id)
		if deriveErr != nil {
			clearSecurityKeys(keys)
			return nil, gerror.New("IDC core security key derivation failed")
		}
		keys[id] = key
	}
	return &deploymentSecretKeyProvider{
		activeKeyID: decoded.activeKeyID,
		keys:        keys,
	}, nil
}

func parseSecurityKeyring(raw string) (decodedSecurityKeyring, error) {
	if raw == "" || raw != strings.TrimSpace(raw) || len(raw) > 16*1024 {
		return decodedSecurityKeyring{}, gerror.New("IDC core security keyring is invalid")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var encoded encodedSecurityKeyring
	if err := decoder.Decode(&encoded); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return decodedSecurityKeyring{}, gerror.New("IDC core security keyring is invalid")
	}
	if !validKeyID(encoded.ActiveKeyID) || len(encoded.RootKeys) == 0 ||
		len(encoded.RootKeys) > maximumSecurityKeyCount {
		return decodedSecurityKeyring{}, gerror.New("IDC core security keyring is invalid")
	}
	reservation, err := decodeCanonicalKey(encoded.Reservation)
	if err != nil {
		return decodedSecurityKeyring{}, err
	}
	rootKeys := make(map[string][]byte, len(encoded.RootKeys))
	for id, encodedRoot := range encoded.RootKeys {
		if !validKeyID(id) {
			clear(reservation)
			clearSecurityKeys(rootKeys)
			return decodedSecurityKeyring{}, gerror.New("IDC core security keyring is invalid")
		}
		root, decodeErr := decodeCanonicalKey(encodedRoot)
		if decodeErr != nil {
			clear(reservation)
			clearSecurityKeys(rootKeys)
			return decodedSecurityKeyring{}, decodeErr
		}
		if constantTimeEqual(root, reservation) {
			clear(root)
			clear(reservation)
			clearSecurityKeys(rootKeys)
			return decodedSecurityKeyring{}, gerror.New("IDC core security key purposes must use distinct roots")
		}
		rootKeys[id] = root
	}
	if _, activeFound := rootKeys[encoded.ActiveKeyID]; !activeFound {
		clear(reservation)
		clearSecurityKeys(rootKeys)
		return decodedSecurityKeyring{}, gerror.New("IDC core active security key is missing")
	}
	return decodedSecurityKeyring{
		activeKeyID: encoded.ActiveKeyID,
		reservation: reservation,
		rootKeys:    rootKeys,
	}, nil
}

func (p *deploymentSecretKeyProvider) Active(ctx context.Context) (secretservice.KeyMaterial, error) {
	if p == nil || ctx.Err() != nil {
		return secretservice.KeyMaterial{}, gerror.New("IDC core secret key provider is unavailable")
	}
	key := p.keys[p.activeKeyID]
	if !validKeyID(p.activeKeyID) || len(key) != securityRootKeyBytes {
		return secretservice.KeyMaterial{}, gerror.New("IDC core secret key provider is unavailable")
	}
	return secretservice.KeyMaterial{ID: p.activeKeyID, Key: append([]byte(nil), key...)}, nil
}

func (p *deploymentSecretKeyProvider) Resolve(
	ctx context.Context,
	keyIDs []string,
) (map[string][]byte, error) {
	if p == nil || ctx.Err() != nil || len(keyIDs) == 0 || len(keyIDs) > maximumSecurityKeyCount {
		return nil, gerror.New("IDC core secret key provider is unavailable")
	}
	resolved := make(map[string][]byte, len(keyIDs))
	for _, id := range keyIDs {
		if !validKeyID(id) {
			clearSecurityKeys(resolved)
			return nil, gerror.New("IDC core secret key provider is unavailable")
		}
		if key := p.keys[id]; len(key) == securityRootKeyBytes {
			resolved[id] = append([]byte(nil), key...)
		}
	}
	return resolved, nil
}

func (p *deploymentRecentAuthKeyProvider) Active(ctx context.Context) (reauth.ActiveSigningKey, error) {
	if p == nil || ctx == nil || ctx.Err() != nil {
		return reauth.ActiveSigningKey{}, gerror.New("IDC core recent authentication key provider is unavailable")
	}
	key := p.keys[p.activeKeyID]
	if !validKeyID(p.activeKeyID) || len(key) != derivedSecurityKeyBytes {
		return reauth.ActiveSigningKey{}, gerror.New("IDC core recent authentication key provider is unavailable")
	}
	return reauth.ActiveSigningKey{KeyID: p.activeKeyID, Key: append([]byte(nil), key...)}, nil
}

func (p *deploymentRecentAuthKeyProvider) Resolve(ctx context.Context, keyID string) ([]byte, error) {
	if p == nil || ctx == nil || ctx.Err() != nil || !validKeyID(keyID) {
		return nil, gerror.New("IDC core recent authentication key provider is unavailable")
	}
	key := p.keys[keyID]
	if len(key) != derivedSecurityKeyBytes {
		return nil, nil
	}
	return append([]byte(nil), key...), nil
}

func sortedSecurityKeyIDs(keys map[string][]byte) []string {
	ids := make([]string, 0, len(keys))
	for id := range keys {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func clearDecodedSecurityKeyring(keyring *decodedSecurityKeyring) {
	if keyring == nil {
		return
	}
	clear(keyring.reservation)
	clearSecurityKeys(keyring.rootKeys)
	keyring.activeKeyID = ""
}

func clearSecurityKeys(keys map[string][]byte) {
	for id, key := range keys {
		clear(key)
		delete(keys, id)
	}
}

func decodeCanonicalKey(value string) ([]byte, error) {
	decoded, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil || len(decoded) != securityRootKeyBytes ||
		base64.StdEncoding.EncodeToString(decoded) != value {
		clear(decoded)
		return nil, gerror.New("IDC core security keyring is invalid")
	}
	return decoded, nil
}

func derive(root []byte, purpose string) ([]byte, error) {
	return hkdf.Key(sha256.New, root, []byte(securityDerivationSalt), purpose, derivedSecurityKeyBytes)
}

func validKeyID(value string) bool {
	if len(value) == 0 || len(value) > 40 || value != strings.TrimSpace(value) {
		return false
	}
	for index, character := range []byte(value) {
		if (character >= 'a' && character <= 'z') ||
			(index > 0 && character >= '0' && character <= '9') ||
			(index > 0 && (character == '-' || character == '_')) {
			continue
		}
		return false
	}
	return true
}

func constantTimeEqual(left []byte, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var difference byte
	for index := range left {
		difference |= left[index] ^ right[index]
	}
	return difference == 0
}

func clearAuditKeys(keys []audit.IntegrityKey) {
	for index := range keys {
		clear(keys[index].Secret)
		keys[index].ID = ""
	}
}

// LoadCustomerLoginKeys derives independent, installation-stable index and
// encryption keys. Envelope-key rotation does not invalidate login contacts.
func LoadCustomerLoginKeys() ([]byte, []byte, error) {
	raw, exists := os.LookupEnv(SecurityKeyringEnvironment)
	if !exists {
		return nil, nil, gerror.New("IDC core security keyring is not configured")
	}
	decoded, err := parseSecurityKeyring(raw)
	if err != nil {
		return nil, nil, err
	}
	defer clearDecodedSecurityKeyring(&decoded)
	index, err := derive(decoded.reservation, "customer/login-contact-index/v1")
	if err != nil {
		return nil, nil, err
	}
	seal, err := derive(decoded.reservation, "customer/login-contact-encryption/v1")
	if err != nil {
		clear(index)
		return nil, nil, err
	}
	return index, seal, nil
}
