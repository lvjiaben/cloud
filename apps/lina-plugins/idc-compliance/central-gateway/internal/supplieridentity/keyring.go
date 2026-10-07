// Package supplieridentity owns installation-local supplier-subject encryption,
// duplicate lookup, idempotency keys, and lifecycle persistence.
package supplieridentity

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
)

const (
	// KeyringSchemaVersion identifies the one accepted private-file schema.
	KeyringSchemaVersion = "v1"
	// MaximumKeyringSize bounds private startup and operator inputs.
	MaximumKeyringSize int64 = 64 << 10
	// SecretBytes is the exact size of every independent key.
	SecretBytes = 32
	// MaximumVersionsPerFamily prevents silent removal during rotation.
	MaximumVersionsPerFamily = 8
	// CipherFormatVersion identifies the AES-GCM persistence frame.
	CipherFormatVersion uint16 = 1

	lookupDomain      = "idc-compliance/supplier-subject-lookup/v1"
	idempotencyDomain = "idc-compliance/supplier-identity-idempotency/v1"
	cipherAADDomain   = "idc-compliance/supplier-subject-cipher/v1"
)

// Purpose identifies one cryptographically independent key family.
type Purpose string

const (
	PurposeSubjectEncryption Purpose = "subject_encryption"
	PurposeSubjectLookup     Purpose = "subject_lookup"
	PurposeIdentityOperation Purpose = "identity_operation"
)

// KeyState separates current writers from retained readers.
type KeyState string

const (
	KeyStateActive   KeyState = "active"
	KeyStateRetained KeyState = "retained"
	KeyStateRevoked  KeyState = "revoked"
)

var (
	ErrInvalidKeyring = errors.New("supplier identity keyring is invalid")
	ErrKeyUnavailable = errors.New("supplier identity key is unavailable")
)

// File is the canonical private supplier-identity keyring representation.
type File struct {
	SchemaVersion string     `json:"schemaVersion"`
	Encryption    FileFamily `json:"subjectEncryption"`
	Lookup        FileFamily `json:"subjectLookup"`
	Idempotency   FileFamily `json:"identityOperation"`
}

// FileFamily stores one independently versioned key family.
type FileFamily struct {
	ActiveVersion uint32    `json:"activeVersion"`
	Keys          []FileKey `json:"keys"`
}

// FileKey stores one canonical base64url-encoded 256-bit secret. Revoked
// entries retain only their version marker and never retain secret material.
type FileKey struct {
	Version uint32   `json:"version"`
	State   KeyState `json:"state"`
	Secret  string   `json:"secret,omitempty"`
}

type keyMaterial struct {
	state  KeyState
	secret []byte
}

type family struct {
	active uint32
	keys   map[uint32]keyMaterial
}

// Ring contains copied in-memory key material and must be cleared at shutdown.
type Ring struct {
	encryption family
	lookup     family
	idempotent family
}

// Ciphertext is safe for ordinary local database persistence.
type Ciphertext struct {
	FormatVersion uint16
	KeyVersion    uint32
	Nonce         []byte
	Payload       []byte
}

// Digest pairs one retained key version with its HMAC result.
type Digest struct {
	KeyVersion uint32
	Sum        [sha256.Size]byte
}

// Generate creates three independent version-one key families.
func Generate(random io.Reader) (File, error) {
	if random == nil {
		random = rand.Reader
	}
	families := make([]FileFamily, 3)
	for index := range families {
		secret, err := randomBytes(random, SecretBytes)
		if err != nil {
			return File{}, fmt.Errorf("generate supplier identity key: %w", err)
		}
		families[index] = FileFamily{ActiveVersion: 1, Keys: []FileKey{{
			Version: 1, State: KeyStateActive,
			Secret: base64.RawURLEncoding.EncodeToString(secret),
		}}}
		clearBytes(secret)
	}
	file := File{
		SchemaVersion: KeyringSchemaVersion,
		Encryption:    families[0], Lookup: families[1], Idempotency: families[2],
	}
	ring, err := New(file)
	if err != nil {
		return File{}, err
	}
	ring.Clear()
	return file, nil
}

// Rotate appends a new active key to exactly one family and retains the old
// active key. It refuses to discard a version without external verification.
func Rotate(file File, purpose Purpose, random io.Reader) (File, error) {
	ring, err := New(file)
	if err != nil {
		return File{}, err
	}
	ring.Clear()
	file = cloneFile(file)
	if random == nil {
		random = rand.Reader
	}
	target, err := fileFamily(&file, purpose)
	if err != nil || len(target.Keys) >= MaximumVersionsPerFamily {
		return File{}, ErrInvalidKeyring
	}
	for index := range target.Keys {
		if target.Keys[index].Version == target.ActiveVersion {
			target.Keys[index].State = KeyStateRetained
		}
	}
	if target.Keys[len(target.Keys)-1].Version == math.MaxUint32 {
		return File{}, ErrInvalidKeyring
	}
	next := target.Keys[len(target.Keys)-1].Version + 1
	secret, err := randomBytes(random, SecretBytes)
	if err != nil {
		return File{}, fmt.Errorf("rotate supplier identity key: %w", err)
	}
	defer clearBytes(secret)
	target.ActiveVersion = next
	target.Keys = append(target.Keys, FileKey{
		Version: next, State: KeyStateActive,
		Secret: base64.RawURLEncoding.EncodeToString(secret),
	})
	rotated, validateErr := New(file)
	if validateErr != nil {
		return File{}, validateErr
	}
	rotated.Clear()
	return file, nil
}

func cloneFile(source File) File {
	result := source
	result.Encryption.Keys = append([]FileKey(nil), source.Encryption.Keys...)
	result.Lookup.Keys = append([]FileKey(nil), source.Lookup.Keys...)
	result.Idempotency.Keys = append([]FileKey(nil), source.Idempotency.Keys...)
	return result
}

// Marshal encodes one validated keyring without HTML escaping.
func Marshal(file File) ([]byte, error) {
	ring, err := New(file)
	if err != nil {
		return nil, err
	}
	ring.Clear()
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err = encoder.Encode(file); err != nil {
		return nil, fmt.Errorf("encode supplier identity keyring: %w", err)
	}
	content := buffer.Bytes()
	if int64(len(content)) > MaximumKeyringSize {
		clearBytes(content)
		return nil, ErrInvalidKeyring
	}
	return content, nil
}

// Parse decodes a bounded canonical keyring and rejects unknown fields.
func Parse(content []byte) (File, error) {
	if len(content) == 0 || int64(len(content)) > MaximumKeyringSize {
		return File{}, ErrInvalidKeyring
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var file File
	if err := decoder.Decode(&file); err != nil {
		return File{}, ErrInvalidKeyring
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return File{}, ErrInvalidKeyring
	}
	ring, err := New(file)
	if err != nil {
		return File{}, err
	}
	ring.Clear()
	return file, nil
}

// New copies and validates every retained secret into a runtime ring.
func New(file File) (*Ring, error) {
	if file.SchemaVersion != KeyringSchemaVersion {
		return nil, ErrInvalidKeyring
	}
	encryption, err := parseFamily(file.Encryption)
	if err != nil {
		return nil, err
	}
	lookup, err := parseFamily(file.Lookup)
	if err != nil {
		clearFamily(&encryption)
		return nil, err
	}
	idempotent, err := parseFamily(file.Idempotency)
	if err != nil {
		clearFamily(&encryption)
		clearFamily(&lookup)
		return nil, err
	}
	if duplicateSecret(encryption, lookup, idempotent) {
		clearFamily(&encryption)
		clearFamily(&lookup)
		clearFamily(&idempotent)
		return nil, ErrInvalidKeyring
	}
	return &Ring{encryption: encryption, lookup: lookup, idempotent: idempotent}, nil
}

// ActiveVersion returns the current write version for one family.
func (r *Ring) ActiveVersion(purpose Purpose) uint32 {
	family, err := r.runtimeFamily(purpose)
	if err != nil {
		return 0
	}
	return family.active
}

// Seal encrypts one canonical subject with random AES-256-GCM nonce material.
func (r *Ring) Seal(
	random io.Reader,
	installationID string,
	bindingNo string,
	provider string,
	plaintext []byte,
) (Ciphertext, error) {
	if random == nil {
		random = rand.Reader
	}
	material, ok := r.encryption.keys[r.encryption.active]
	if !ok || material.state != KeyStateActive || len(material.secret) != SecretBytes ||
		installationID == "" || bindingNo == "" || provider == "" || len(plaintext) == 0 {
		return Ciphertext{}, ErrKeyUnavailable
	}
	block, err := aes.NewCipher(material.secret)
	if err != nil {
		return Ciphertext{}, ErrKeyUnavailable
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return Ciphertext{}, ErrKeyUnavailable
	}
	nonce, err := randomBytes(random, aead.NonceSize())
	if err != nil {
		return Ciphertext{}, fmt.Errorf("generate supplier subject nonce: %w", err)
	}
	aad := cipherAAD(installationID, bindingNo, provider, CipherFormatVersion, r.encryption.active)
	payload := aead.Seal(nil, nonce, plaintext, aad)
	clearBytes(aad)
	return Ciphertext{
		FormatVersion: CipherFormatVersion, KeyVersion: r.encryption.active,
		Nonce: nonce, Payload: payload,
	}, nil
}

// Open authenticates and decrypts one persisted supplier subject.
func (r *Ring) Open(
	installationID string,
	bindingNo string,
	provider string,
	sealed Ciphertext,
) ([]byte, error) {
	if r == nil || sealed.FormatVersion != CipherFormatVersion {
		return nil, ErrKeyUnavailable
	}
	material, ok := r.encryption.keys[sealed.KeyVersion]
	if !ok || material.state == KeyStateRevoked || len(material.secret) != SecretBytes {
		return nil, ErrKeyUnavailable
	}
	block, err := aes.NewCipher(material.secret)
	if err != nil {
		return nil, ErrKeyUnavailable
	}
	aead, err := cipher.NewGCM(block)
	if err != nil || len(sealed.Nonce) != aead.NonceSize() || len(sealed.Payload) < aead.Overhead() {
		return nil, ErrKeyUnavailable
	}
	aad := cipherAAD(installationID, bindingNo, provider, sealed.FormatVersion, sealed.KeyVersion)
	plaintext, err := aead.Open(nil, sealed.Nonce, sealed.Payload, aad)
	clearBytes(aad)
	if err != nil {
		clearBytes(plaintext)
		return nil, ErrKeyUnavailable
	}
	return plaintext, nil
}

// LookupDigests calculates candidates across all active and retained lookup keys.
func (r *Ring) LookupDigests(provider string, canonicalSubject []byte) ([]Digest, error) {
	if r == nil || provider == "" || len(canonicalSubject) == 0 {
		return nil, ErrKeyUnavailable
	}
	return familyDigests(r.lookup, lookupDomain, provider, canonicalSubject)
}

// IdempotencyDigests calculates candidates across all active and retained
// identity-operation keys without persisting the raw operation key.
func (r *Ring) IdempotencyDigests(operationKind string, operationKey []byte) ([]Digest, error) {
	if r == nil || operationKind == "" || len(operationKey) != SecretBytes {
		return nil, ErrKeyUnavailable
	}
	return familyDigests(r.idempotent, idempotencyDomain, operationKind, operationKey)
}

// Clear zeroes every copied secret and makes the ring unusable.
func (r *Ring) Clear() {
	if r == nil {
		return
	}
	clearFamily(&r.encryption)
	clearFamily(&r.lookup)
	clearFamily(&r.idempotent)
}

func parseFamily(source FileFamily) (family, error) {
	if source.ActiveVersion == 0 || len(source.Keys) == 0 || len(source.Keys) > MaximumVersionsPerFamily {
		return family{}, ErrInvalidKeyring
	}
	result := family{active: source.ActiveVersion, keys: make(map[uint32]keyMaterial, len(source.Keys))}
	var previous uint32
	activeCount := 0
	for _, item := range source.Keys {
		if item.Version == 0 || item.Version <= previous {
			clearFamily(&result)
			return family{}, ErrInvalidKeyring
		}
		previous = item.Version
		if _, exists := result.keys[item.Version]; exists {
			clearFamily(&result)
			return family{}, ErrInvalidKeyring
		}
		switch item.State {
		case KeyStateActive:
			activeCount++
			if item.Version != source.ActiveVersion {
				clearFamily(&result)
				return family{}, ErrInvalidKeyring
			}
		case KeyStateRetained:
			if item.Version == source.ActiveVersion {
				clearFamily(&result)
				return family{}, ErrInvalidKeyring
			}
		case KeyStateRevoked:
			if item.Secret != "" || item.Version == source.ActiveVersion {
				clearFamily(&result)
				return family{}, ErrInvalidKeyring
			}
			result.keys[item.Version] = keyMaterial{state: item.State}
			continue
		default:
			clearFamily(&result)
			return family{}, ErrInvalidKeyring
		}
		secret, err := decodeSecret(item.Secret)
		if err != nil {
			clearFamily(&result)
			return family{}, ErrInvalidKeyring
		}
		result.keys[item.Version] = keyMaterial{state: item.State, secret: secret}
	}
	if activeCount != 1 {
		clearFamily(&result)
		return family{}, ErrInvalidKeyring
	}
	if source.Keys[len(source.Keys)-1].Version != source.ActiveVersion {
		clearFamily(&result)
		return family{}, ErrInvalidKeyring
	}
	return result, nil
}

func fileFamily(file *File, purpose Purpose) (*FileFamily, error) {
	if file == nil {
		return nil, ErrInvalidKeyring
	}
	switch purpose {
	case PurposeSubjectEncryption:
		return &file.Encryption, nil
	case PurposeSubjectLookup:
		return &file.Lookup, nil
	case PurposeIdentityOperation:
		return &file.Idempotency, nil
	default:
		return nil, ErrInvalidKeyring
	}
}

func (r *Ring) runtimeFamily(purpose Purpose) (*family, error) {
	if r == nil {
		return nil, ErrKeyUnavailable
	}
	switch purpose {
	case PurposeSubjectEncryption:
		return &r.encryption, nil
	case PurposeSubjectLookup:
		return &r.lookup, nil
	case PurposeIdentityOperation:
		return &r.idempotent, nil
	default:
		return nil, ErrKeyUnavailable
	}
}

func familyDigests(source family, domain, scope string, value []byte) ([]Digest, error) {
	result := make([]Digest, 0, len(source.keys))
	versions := make([]uint32, 0, len(source.keys))
	for version := range source.keys {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(left, right int) bool {
		return versions[left] < versions[right]
	})
	for _, version := range versions {
		material, ok := source.keys[version]
		if !ok || material.state == KeyStateRevoked {
			continue
		}
		hash := hmac.New(sha256.New, material.secret)
		frame := appendFrame(nil, []byte(domain))
		frame = appendFrame(frame, []byte(scope))
		frame = appendFrame(frame, value)
		_, _ = hash.Write(frame)
		clearBytes(frame)
		var sum [sha256.Size]byte
		copy(sum[:], hash.Sum(nil))
		result = append(result, Digest{KeyVersion: version, Sum: sum})
	}
	if len(result) == 0 {
		return nil, ErrKeyUnavailable
	}
	return result, nil
}

func duplicateSecret(families ...family) bool {
	seen := make(map[[sha256.Size]byte]struct{})
	for _, source := range families {
		for _, material := range source.keys {
			if material.state == KeyStateRevoked {
				continue
			}
			digest := sha256.Sum256(material.secret)
			if _, exists := seen[digest]; exists {
				return true
			}
			seen[digest] = struct{}{}
		}
	}
	return false
}

func cipherAAD(
	installationID, bindingNo, provider string,
	formatVersion uint16,
	keyVersion uint32,
) []byte {
	format := make([]byte, 2)
	key := make([]byte, 4)
	binary.BigEndian.PutUint16(format, formatVersion)
	binary.BigEndian.PutUint32(key, keyVersion)
	frame := appendFrame(nil, []byte(cipherAADDomain))
	for _, value := range [][]byte{
		[]byte(installationID), []byte(bindingNo), []byte(provider), format, key,
	} {
		frame = appendFrame(frame, value)
	}
	clearBytes(format)
	clearBytes(key)
	return frame
}

func appendFrame(destination, value []byte) []byte {
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	destination = append(destination, length[:]...)
	return append(destination, value...)
}

func decodeSecret(encoded string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(decoded) != SecretBytes ||
		base64.RawURLEncoding.EncodeToString(decoded) != encoded {
		clearBytes(decoded)
		return nil, ErrInvalidKeyring
	}
	return decoded, nil
}

func randomBytes(random io.Reader, size int) ([]byte, error) {
	value := make([]byte, size)
	if _, err := io.ReadFull(random, value); err != nil {
		clearBytes(value)
		return nil, err
	}
	return value, nil
}

func clearFamily(value *family) {
	if value == nil {
		return
	}
	for version, material := range value.keys {
		clearBytes(material.secret)
		delete(value.keys, version)
	}
	value.active = 0
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
