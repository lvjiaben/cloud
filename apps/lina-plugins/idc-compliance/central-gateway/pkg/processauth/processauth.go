// Package processauth defines installation-local authentication shared by the
// compliance plugin and its local gateway process.
package processauth

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Protocol bounds, domains, and header names are shared by both local processes.
const (
	Version                       = "v1"
	SecretBytes                   = 32
	NonceBytes                    = 32
	BusinessIdempotencyBytes      = 32
	MaximumKeyCount               = 4
	MaximumKeyringSize            = 8192
	MaximumInstallationIDFileSize = 128
	MaximumPayloadBytes           = 1 << 20
	MaximumRequestTargetBytes     = 4096
	MaximumQueryKeys              = 32
	MaximumQueryBytes             = 1024
	MaximumAuthenticationWindow   = 60 * time.Second
	MaximumFutureClockSkew        = 30 * time.Second
	SignatureDomain               = "idc-compliance-local-gateway/process-auth/v1"
	BusinessContentDomain         = "idc-compliance-local-gateway/business-content/v1"
	BusinessIdempotencyDomain     = "idc-compliance-local-gateway/business-idempotency/v1"
	NonceDigestDomain             = "idc-compliance-local-gateway/process-nonce/v1"
	HeaderProtocol                = "X-Lina-Gateway-Protocol"
	HeaderInstallationID          = "X-Lina-Gateway-Installation-Id"
	HeaderOperationScope          = "X-Lina-Gateway-Operation-Scope"
	HeaderKeyID                   = "X-Lina-Gateway-Key-Id"
	HeaderTimestampMS             = "X-Lina-Gateway-Timestamp-Millis"
	HeaderExpiresMS               = "X-Lina-Gateway-Expires-Millis"
	HeaderNonce                   = "X-Lina-Gateway-Nonce"
	HeaderBusinessIdempotency     = "X-Lina-Gateway-Business-Idempotency"
	HeaderContentSHA              = "X-Lina-Gateway-Content-Sha256"
	HeaderBusinessContentSHA      = "X-Lina-Gateway-Business-Content-Sha256"
	HeaderSignature               = "X-Lina-Gateway-Signature"
)

// PayloadMode identifies the server-declared canonicalization rule for a route.
type PayloadMode string

const (
	// PayloadModeEmpty permits only an empty request payload.
	PayloadModeEmpty PayloadMode = "empty"
	// PayloadModeJSON applies RFC 8785 before hashing an application/json payload.
	PayloadModeJSON PayloadMode = "json-rfc8785"
	// PayloadModeRaw hashes bounded wire bytes for an explicitly declared route.
	PayloadModeRaw PayloadMode = "raw"
)

// KeyState controls whether one signing key may sign, verify, or do neither.
type KeyState string

const (
	// KeyStateActive signs new requests and verifies its short-lived frames.
	KeyStateActive KeyState = "active"
	// KeyStateVerifyOnly verifies frames issued before a key rotation.
	KeyStateVerifyOnly KeyState = "verify_only"
	// KeyStateRevoked retains only the key identifier without secret material.
	KeyStateRevoked KeyState = "revoked"
)

var (
	// ErrInvalid reports malformed local protocol or configuration input.
	ErrInvalid = errors.New("local process authentication input is invalid")
	// ErrRejected reports a well-formed frame that fails authentication.
	ErrRejected = errors.New("local process authentication was rejected")
	// Protocol patterns enforce a single textual representation for signed identifiers.
	keyIDPattern          = regexp.MustCompile(`^lgk_[0-9a-f]{16}$`)
	installationIDPattern = regexp.MustCompile(`^linst_[0-9a-f]{64}$`)
	operationScopePattern = regexp.MustCompile(`^[a-z][a-z0-9.:-]{1,63}$`)
	businessKeyPattern    = regexp.MustCompile(`^lbi1_[A-Za-z0-9_-]{43}$`)
)

// FileKey is one bounded keyring entry written to private installation storage.
type FileKey struct {
	ID     string   `json:"id"`
	State  KeyState `json:"state"`
	Secret string   `json:"secret,omitempty"`
}

// File is the bounded deployment representation. It is a secret file and must
// never be logged, returned by an API, or committed to source control.
type File struct {
	Version                   string    `json:"version"`
	ActiveKeyID               string    `json:"activeKeyId"`
	BusinessIdempotencySecret string    `json:"businessIdempotencySecret"`
	Keys                      []FileKey `json:"keys"`
}

// Credentials are short-lived request headers. The raw nonce and business
// idempotency key must never be persisted by the gateway.
type Credentials struct {
	Protocol               string
	InstallationID         string
	OperationScope         string
	KeyID                  string
	TimestampMS            int64
	ExpiresMS              int64
	Nonce                  string
	BusinessIdempotencyKey string
	ContentSHA256          string
	BusinessContentSHA256  string
	Signature              string
}

// Verified is the persistence-safe projection of an authenticated frame. It
// contains only keyed or one-way digests for the raw nonce and idempotency key.
type Verified struct {
	InstallationID        string
	OperationScope        string
	KeyID                 string
	Method                string
	RequestTarget         string
	PayloadMode           PayloadMode
	NonceSHA256           [sha256.Size]byte
	FrameSHA256           [sha256.Size]byte
	IdempotencyHMAC       [sha256.Size]byte
	BusinessContentSHA256 [sha256.Size]byte
	BodySHA256            [sha256.Size]byte
	IssuedAt              time.Time
	ExpiresAt             time.Time
}

// Ring is the in-memory copy of one installation's validated live secrets.
type Ring struct {
	activeID                  string
	keys                      map[string]keyMaterial
	businessIdempotencySecret []byte
}

// keyMaterial stores one copied signing secret and its lifecycle state.
type keyMaterial struct {
	state  KeyState
	secret []byte
}

// Generate creates the first installation-local active signing key and an
// independent stable business-idempotency HMAC key.
func Generate(random io.Reader) (File, error) {
	if random == nil {
		random = rand.Reader
	}
	key, err := generateKey(random)
	if err != nil {
		return File{}, err
	}
	businessSecret := make([]byte, SecretBytes)
	if _, err = io.ReadFull(random, businessSecret); err != nil {
		clearBytes(businessSecret)
		return File{}, fmt.Errorf("generate local business-idempotency key: %w", err)
	}
	defer clearBytes(businessSecret)
	return File{
		Version: Version, ActiveKeyID: key.ID,
		BusinessIdempotencySecret: base64.RawURLEncoding.EncodeToString(businessSecret),
		Keys:                      []FileKey{key},
	}, nil
}

// Rotate adds a new active signing key and keeps the independent business
// idempotency HMAC key stable so a request can recover across key rotation.
func Rotate(file File, random io.Reader) (File, error) {
	validated, err := New(file)
	if err != nil {
		return File{}, err
	}
	validated.Clear()
	key, err := generateKey(random)
	if err != nil {
		return File{}, err
	}
	for index := range file.Keys {
		if file.Keys[index].ID == file.ActiveKeyID {
			file.Keys[index].State = KeyStateVerifyOnly
		}
	}
	file.ActiveKeyID = key.ID
	file.Keys = compactKeys(append(file.Keys, key))
	validated, err = New(file)
	if err != nil {
		return File{}, err
	}
	validated.Clear()
	return file, nil
}

// Revoke removes one non-active signing key's secret material. The active key
// must be rotated before it can be revoked.
func Revoke(file File, keyID string) (File, error) {
	validated, err := New(file)
	if err != nil || !keyIDPattern.MatchString(keyID) || keyID == file.ActiveKeyID {
		if validated != nil {
			validated.Clear()
		}
		return File{}, ErrInvalid
	}
	validated.Clear()
	found := false
	for index := range file.Keys {
		if file.Keys[index].ID == keyID {
			file.Keys[index].State = KeyStateRevoked
			file.Keys[index].Secret = ""
			found = true
		}
	}
	if !found {
		return File{}, ErrInvalid
	}
	validated, err = New(file)
	if err != nil {
		return File{}, err
	}
	validated.Clear()
	return file, nil
}

// Parse decodes a canonical keyring file and rejects unknown fields.
func Parse(content []byte) (File, error) {
	if len(content) == 0 || len(content) > MaximumKeyringSize {
		return File{}, ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var file File
	if err := decoder.Decode(&file); err != nil {
		return File{}, ErrInvalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return File{}, ErrInvalid
	}
	validated, err := New(file)
	if err != nil {
		return File{}, err
	}
	validated.Clear()
	return file, nil
}

// Marshal returns the deployment file using deterministic indentation.
func Marshal(file File) ([]byte, error) {
	validated, err := New(file)
	if err != nil {
		return nil, err
	}
	validated.Clear()
	sort.Slice(file.Keys, func(left, right int) bool { return file.Keys[left].ID < file.Keys[right].ID })
	content, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return nil, ErrInvalid
	}
	return append(content, '\n'), nil
}

// New constructs a ring and copies all live secret bytes.
func New(file File) (*Ring, error) {
	if file.Version != Version || !keyIDPattern.MatchString(file.ActiveKeyID) ||
		len(file.Keys) == 0 || len(file.Keys) > MaximumKeyCount {
		return nil, ErrInvalid
	}
	businessSecret, err := decodeSecret(file.BusinessIdempotencySecret)
	if err != nil {
		return nil, err
	}
	ring := &Ring{
		activeID: file.ActiveKeyID, keys: make(map[string]keyMaterial, len(file.Keys)),
		businessIdempotencySecret: businessSecret,
	}
	activeCount := 0
	for _, key := range file.Keys {
		if !keyIDPattern.MatchString(key.ID) {
			ring.Clear()
			return nil, ErrInvalid
		}
		if _, exists := ring.keys[key.ID]; exists {
			ring.Clear()
			return nil, ErrInvalid
		}
		material := keyMaterial{state: key.State}
		switch key.State {
		case KeyStateActive, KeyStateVerifyOnly:
			material.secret, err = decodeSecret(key.Secret)
			if err != nil {
				ring.Clear()
				return nil, err
			}
		case KeyStateRevoked:
			if key.Secret != "" {
				ring.Clear()
				return nil, ErrInvalid
			}
		default:
			ring.Clear()
			return nil, ErrInvalid
		}
		if key.ID == file.ActiveKeyID {
			if key.State != KeyStateActive {
				ring.Clear()
				return nil, ErrInvalid
			}
			activeCount++
		} else if key.State == KeyStateActive {
			ring.Clear()
			return nil, ErrInvalid
		}
		ring.keys[key.ID] = material
	}
	if activeCount != 1 {
		ring.Clear()
		return nil, ErrInvalid
	}
	return ring, nil
}

// ActiveKeyID returns the active local signing key identifier.
func (r *Ring) ActiveKeyID() string {
	if r == nil {
		return ""
	}
	return r.activeID
}

// GenerateInstallationID creates a 256-bit opaque installation identifier.
func GenerateInstallationID(random io.Reader) (string, error) {
	value, err := randomBytes(random, 32)
	if err != nil {
		return "", fmt.Errorf("generate local installation id: %w", err)
	}
	defer clearBytes(value)
	return "linst_" + hex.EncodeToString(value), nil
}

// ParseInstallationID reads the one-line identifier shared by the two local
// processes and rejects aliases or trailing non-whitespace data.
func ParseInstallationID(content []byte) (string, error) {
	if len(content) == 0 || len(content) > MaximumInstallationIDFileSize {
		return "", ErrInvalid
	}
	value := strings.TrimSpace(string(content))
	if !installationIDPattern.MatchString(value) {
		return "", ErrInvalid
	}
	return value, nil
}

// GenerateBusinessIdempotencyKey creates a 256-bit raw business key for one
// stable business intent. The caller retains it only for safe local recovery.
func GenerateBusinessIdempotencyKey(random io.Reader) (string, error) {
	value, err := randomBytes(random, BusinessIdempotencyBytes)
	if err != nil {
		return "", fmt.Errorf("generate local business-idempotency key: %w", err)
	}
	defer clearBytes(value)
	return "lbi1_" + base64.RawURLEncoding.EncodeToString(value), nil
}

// Sign creates credentials for one exact installation, server-selected scope,
// method, canonical target, payload, and business idempotency key.
func (r *Ring) Sign(
	installationID string,
	operationScope string,
	method string,
	requestTarget string,
	payloadMode PayloadMode,
	body []byte,
	businessIdempotencyKey string,
	now time.Time,
	validity time.Duration,
	random io.Reader,
) (Credentials, error) {
	canonicalTarget, canonicalPayload, err := validateRequest(
		installationID, operationScope, method, requestTarget, payloadMode, body,
		businessIdempotencyKey,
	)
	if r == nil || random == nil || now.IsZero() || validity <= 0 ||
		validity > MaximumAuthenticationWindow || err != nil {
		return Credentials{}, ErrInvalid
	}
	defer clearBytes(canonicalPayload)
	key, ok := r.keys[r.activeID]
	if !ok || key.state != KeyStateActive || len(key.secret) != SecretBytes {
		return Credentials{}, ErrRejected
	}
	nonce, err := randomBytes(random, NonceBytes)
	if err != nil {
		return Credentials{}, fmt.Errorf("generate local process nonce: %w", err)
	}
	defer clearBytes(nonce)
	bodyDigest := sha256.Sum256(canonicalPayload)
	businessDigest := businessContentDigest(
		installationID, operationScope, method, canonicalTarget, payloadMode, bodyDigest,
	)
	credentials := Credentials{
		Protocol: Version, InstallationID: installationID, OperationScope: operationScope,
		KeyID: r.activeID, TimestampMS: now.UTC().UnixMilli(),
		ExpiresMS:              now.UTC().Add(validity).UnixMilli(),
		Nonce:                  base64.RawURLEncoding.EncodeToString(nonce),
		BusinessIdempotencyKey: businessIdempotencyKey,
		ContentSHA256:          hex.EncodeToString(bodyDigest[:]),
		BusinessContentSHA256:  hex.EncodeToString(businessDigest[:]),
	}
	credentials.Signature, err = signature(
		key.secret, method, canonicalTarget, payloadMode, credentials,
	)
	if err != nil {
		return Credentials{}, ErrRejected
	}
	return credentials, nil
}

// Verify authenticates one request using database-authoritative time supplied
// by the caller and returns only persistence-safe digests.
func (r *Ring) Verify(
	expectedInstallationID string,
	expectedOperationScope string,
	method string,
	requestTarget string,
	payloadMode PayloadMode,
	body []byte,
	credentials Credentials,
	databaseNow time.Time,
	maximumWindow time.Duration,
) (Verified, error) {
	canonicalTarget, canonicalPayload, err := validateRequest(
		expectedInstallationID, expectedOperationScope, method, requestTarget, payloadMode, body,
		credentials.BusinessIdempotencyKey,
	)
	if r == nil || databaseNow.IsZero() || maximumWindow <= 0 || err != nil ||
		maximumWindow > MaximumAuthenticationWindow ||
		credentials.Protocol != Version || credentials.InstallationID != expectedInstallationID ||
		credentials.OperationScope != expectedOperationScope ||
		!keyIDPattern.MatchString(credentials.KeyID) {
		return Verified{}, ErrInvalid
	}
	defer clearBytes(canonicalPayload)
	key, ok := r.keys[credentials.KeyID]
	if !ok || key.state == KeyStateRevoked || len(key.secret) != SecretBytes {
		return Verified{}, ErrRejected
	}
	issuedAt := time.UnixMilli(credentials.TimestampMS).UTC()
	expiresAt := time.UnixMilli(credentials.ExpiresMS).UTC()
	databaseNow = databaseNow.UTC()
	if !expiresAt.After(issuedAt) || expiresAt.Sub(issuedAt) > maximumWindow ||
		!expiresAt.After(databaseNow) || issuedAt.After(databaseNow.Add(MaximumFutureClockSkew)) {
		return Verified{}, ErrRejected
	}
	nonce, err := decodeCanonical(credentials.Nonce, NonceBytes)
	if err != nil {
		return Verified{}, ErrRejected
	}
	defer clearBytes(nonce)
	businessKey, err := decodeBusinessKey(credentials.BusinessIdempotencyKey)
	if err != nil {
		return Verified{}, ErrRejected
	}
	defer clearBytes(businessKey)
	bodyDigest := sha256.Sum256(canonicalPayload)
	if credentials.ContentSHA256 != hex.EncodeToString(bodyDigest[:]) {
		return Verified{}, ErrRejected
	}
	businessDigest := businessContentDigest(
		expectedInstallationID, expectedOperationScope, method, canonicalTarget, payloadMode,
		bodyDigest,
	)
	if credentials.BusinessContentSHA256 != hex.EncodeToString(businessDigest[:]) {
		return Verified{}, ErrRejected
	}
	expectedSignature, err := signature(
		key.secret, method, canonicalTarget, payloadMode, credentials,
	)
	if err != nil {
		return Verified{}, ErrRejected
	}
	provided, decodeErr := base64.RawURLEncoding.DecodeString(credentials.Signature)
	expected, expectedErr := base64.RawURLEncoding.DecodeString(expectedSignature)
	valid := decodeErr == nil && expectedErr == nil && len(provided) == sha256.Size &&
		len(expected) == sha256.Size &&
		base64.RawURLEncoding.EncodeToString(provided) == credentials.Signature &&
		subtle.ConstantTimeCompare(provided, expected) == 1
	clearBytes(provided)
	clearBytes(expected)
	if !valid {
		return Verified{}, ErrRejected
	}
	frame := signatureFrame(method, canonicalTarget, payloadMode, credentials)
	frame = appendFrame(frame, []byte(credentials.Signature))
	frameDigest := sha256.Sum256(frame)
	clearBytes(frame)
	idempotencyHMAC, err := r.businessIdempotencyDigest(expectedInstallationID, businessKey)
	if err != nil {
		return Verified{}, ErrRejected
	}
	return Verified{
		InstallationID: expectedInstallationID, OperationScope: expectedOperationScope,
		KeyID: credentials.KeyID, Method: method, RequestTarget: canonicalTarget,
		PayloadMode: payloadMode,
		NonceSHA256: nonceDigest(expectedInstallationID, nonce), FrameSHA256: frameDigest,
		IdempotencyHMAC:       idempotencyHMAC,
		BusinessContentSHA256: businessDigest, BodySHA256: bodyDigest,
		IssuedAt: issuedAt, ExpiresAt: expiresAt,
	}, nil
}

// Clear zeroes every copied secret and makes the ring unusable.
func (r *Ring) Clear() {
	if r == nil {
		return
	}
	for id, material := range r.keys {
		clearBytes(material.secret)
		delete(r.keys, id)
	}
	clearBytes(r.businessIdempotencySecret)
	r.businessIdempotencySecret = nil
	r.activeID = ""
}

// generateKey creates one random active HMAC signing key.
func generateKey(random io.Reader) (FileKey, error) {
	if random == nil {
		random = rand.Reader
	}
	idBytes, err := randomBytes(random, 8)
	if err != nil {
		return FileKey{}, fmt.Errorf("generate local key id: %w", err)
	}
	defer clearBytes(idBytes)
	secret, err := randomBytes(random, SecretBytes)
	if err != nil {
		return FileKey{}, fmt.Errorf("generate local process key: %w", err)
	}
	defer clearBytes(secret)
	return FileKey{
		ID: "lgk_" + hex.EncodeToString(idBytes), State: KeyStateActive,
		Secret: base64.RawURLEncoding.EncodeToString(secret),
	}, nil
}

// randomBytes fills an exact-size slice from the supplied cryptographic reader.
func randomBytes(random io.Reader, size int) ([]byte, error) {
	if random == nil {
		random = rand.Reader
	}
	value := make([]byte, size)
	if _, err := io.ReadFull(random, value); err != nil {
		clearBytes(value)
		return nil, err
	}
	return value, nil
}

// decodeSecret validates one canonical 256-bit key encoding.
func decodeSecret(encoded string) ([]byte, error) {
	decoded, err := decodeCanonical(encoded, SecretBytes)
	if err != nil {
		return nil, ErrInvalid
	}
	return decoded, nil
}

// decodeCanonical rejects non-canonical base64url aliases and wrong sizes.
func decodeCanonical(encoded string, size int) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(decoded) != size || base64.RawURLEncoding.EncodeToString(decoded) != encoded {
		clearBytes(decoded)
		return nil, ErrInvalid
	}
	return decoded, nil
}

// decodeBusinessKey validates and decodes one versioned 256-bit business key.
func decodeBusinessKey(value string) ([]byte, error) {
	if !businessKeyPattern.MatchString(value) {
		return nil, ErrInvalid
	}
	return decodeCanonical(strings.TrimPrefix(value, "lbi1_"), BusinessIdempotencyBytes)
}

// compactKeys retains the newest bounded key history during rotation.
func compactKeys(keys []FileKey) []FileKey {
	if len(keys) <= MaximumKeyCount {
		return keys
	}
	result := make([]FileKey, 0, MaximumKeyCount)
	for index := len(keys) - 1; index >= 0 && len(result) < MaximumKeyCount; index-- {
		result = append(result, keys[index])
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result
}

// validateRequest produces the exact target and payload representations to sign.
func validateRequest(
	installationID string,
	operationScope string,
	method string,
	requestTarget string,
	payloadMode PayloadMode,
	body []byte,
	businessIdempotencyKey string,
) (string, []byte, error) {
	if !installationIDPattern.MatchString(installationID) ||
		!operationScopePattern.MatchString(operationScope) ||
		!validMethod(method) || len(body) > MaximumPayloadBytes {
		return "", nil, ErrInvalid
	}
	businessKey, err := decodeBusinessKey(businessIdempotencyKey)
	if err != nil {
		return "", nil, ErrInvalid
	}
	clearBytes(businessKey)
	canonicalTarget, err := canonicalRequestTarget(requestTarget)
	if err != nil {
		return "", nil, ErrInvalid
	}
	canonicalPayload, err := canonicalizePayload(payloadMode, body)
	if err != nil {
		return "", nil, ErrInvalid
	}
	return canonicalTarget, canonicalPayload, nil
}

// validMethod limits the protocol to reviewed HTTP method tokens.
func validMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

// businessContentDigest identifies stable canonical intent across retries and rotation.
func businessContentDigest(
	installationID string,
	operationScope string,
	method string,
	requestTarget string,
	payloadMode PayloadMode,
	bodyDigest [sha256.Size]byte,
) [sha256.Size]byte {
	frame := make([]byte, 0, 256)
	for _, value := range [][]byte{
		[]byte(BusinessContentDomain), []byte(installationID), []byte(operationScope),
		[]byte(method), []byte(requestTarget), []byte(payloadMode), bodyDigest[:],
	} {
		frame = appendFrame(frame, value)
	}
	digest := sha256.Sum256(frame)
	clearBytes(frame)
	return digest
}

// nonceDigest separates identical raw nonce bytes used by different installations.
func nonceDigest(installationID string, nonce []byte) [sha256.Size]byte {
	frame := appendFrame(nil, []byte(NonceDigestDomain))
	frame = appendFrame(frame, []byte(installationID))
	frame = appendFrame(frame, nonce)
	digest := sha256.Sum256(frame)
	clearBytes(frame)
	return digest
}

// businessIdempotencyDigest hides the raw business key behind a stable keyed digest.
func (r *Ring) businessIdempotencyDigest(
	installationID string,
	businessKey []byte,
) ([sha256.Size]byte, error) {
	hash := hmac.New(sha256.New, r.businessIdempotencySecret)
	frame := appendFrame(nil, []byte(BusinessIdempotencyDomain))
	frame = appendFrame(frame, []byte(installationID))
	frame = appendFrame(frame, businessKey)
	if _, err := hash.Write(frame); err != nil {
		clearBytes(frame)
		return [sha256.Size]byte{}, err
	}
	clearBytes(frame)
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}

// signature computes the HMAC for one length-delimited canonical frame.
func signature(
	secret []byte,
	method string,
	requestTarget string,
	payloadMode PayloadMode,
	credentials Credentials,
) (string, error) {
	hash := hmac.New(sha256.New, secret)
	frame := signatureFrame(method, requestTarget, payloadMode, credentials)
	if _, err := hash.Write(frame); err != nil {
		clearBytes(frame)
		return "", err
	}
	clearBytes(frame)
	return base64.RawURLEncoding.EncodeToString(hash.Sum(nil)), nil
}

// signatureFrame serializes all authenticated frame fields without ambiguity.
func signatureFrame(
	method string,
	requestTarget string,
	payloadMode PayloadMode,
	credentials Credentials,
) []byte {
	issuedAt := make([]byte, 8)
	expiresAt := make([]byte, 8)
	binary.BigEndian.PutUint64(issuedAt, uint64(credentials.TimestampMS))
	binary.BigEndian.PutUint64(expiresAt, uint64(credentials.ExpiresMS))
	frame := make([]byte, 0, 512)
	for _, value := range [][]byte{
		[]byte(SignatureDomain), []byte(credentials.Protocol), []byte(credentials.InstallationID),
		[]byte(credentials.OperationScope), []byte(method), []byte(requestTarget),
		[]byte(payloadMode), issuedAt, expiresAt, []byte(credentials.KeyID), []byte(credentials.Nonce),
		[]byte(credentials.BusinessIdempotencyKey), []byte(credentials.ContentSHA256),
		[]byte(credentials.BusinessContentSHA256),
	} {
		frame = appendFrame(frame, value)
	}
	return frame
}

// appendFrame appends one big-endian length-prefixed byte sequence.
func appendFrame(destination []byte, value []byte) []byte {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	destination = append(destination, length[:]...)
	return append(destination, value...)
}

// clearBytes removes copied secret material from the supplied slice.
func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
