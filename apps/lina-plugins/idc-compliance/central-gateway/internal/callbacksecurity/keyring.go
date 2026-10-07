// Package callbacksecurity owns the installation-local callback and
// reconciliation key families.
package callbacksecurity

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
	"sort"
)

const (
	KeyringSchemaVersion            = "v1"
	MaximumKeyringSize       int64  = 64 << 10
	SecretBytes                     = 32
	MaximumVersionsPerFamily        = 8
	CipherFormatVersion      uint16 = 1

	evidenceAADDomain   = "idc-compliance/callback-evidence/v1"
	eventHMACDomain     = "idc-compliance/callback-event/v1"
	referenceAADDomain  = "idc-compliance/external-reference/v1"
	referenceHMACDomain = "idc-compliance/external-reference-lookup/v1"
	takeoverHMACDomain  = "idc-compliance/manual-takeover-operation/v1"
)

var (
	ErrInvalidKeyring = errors.New("callback evidence keyring is invalid")
	ErrKeyUnavailable = errors.New("callback evidence key is unavailable")
)

type KeyState string

const (
	KeyStateActive   KeyState = "active"
	KeyStateRetained KeyState = "retained"
	KeyStateRevoked  KeyState = "revoked"
)

// File is the canonical private callback-security keyring representation.
type File struct {
	SchemaVersion     string     `json:"schemaVersion"`
	Evidence          FileFamily `json:"evidenceEncryption"`
	Event             FileFamily `json:"externalEventHmac"`
	Reference         FileFamily `json:"externalReferenceEncryption"`
	ReferenceLookup   FileFamily `json:"externalReferenceHmac"`
	TakeoverOperation FileFamily `json:"takeoverOperationHmac"`
}

type FileFamily struct {
	ActiveVersion uint32    `json:"activeVersion"`
	Keys          []FileKey `json:"keys"`
}

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

// Ring contains copied key bytes and must be cleared when the runtime closes.
type Ring struct {
	evidence          family
	event             family
	reference         family
	referenceLookup   family
	takeoverOperation family
}

// SealedEvidence is safe for ordinary local database persistence.
type SealedEvidence struct {
	FormatVersion uint16
	KeyVersion    uint32
	Nonce         []byte
	Ciphertext    []byte
	PlainSHA256   [sha256.Size]byte
}

// EventDigest is one retained key version and its provider-scoped HMAC.
type EventDigest struct {
	KeyVersion uint32
	Sum        [sha256.Size]byte
}

// Generate creates five cryptographically independent version-one families.
func Generate(random io.Reader) (File, error) {
	if random == nil {
		random = rand.Reader
	}
	families := make([]FileFamily, 5)
	for index := range families {
		secret := make([]byte, SecretBytes)
		if _, err := io.ReadFull(random, secret); err != nil {
			clear(secret)
			return File{}, fmt.Errorf("generate callback security key: %w", err)
		}
		families[index] = FileFamily{ActiveVersion: 1, Keys: []FileKey{{
			Version: 1, State: KeyStateActive,
			Secret: base64.RawURLEncoding.EncodeToString(secret),
		}}}
		clear(secret)
	}
	file := File{
		SchemaVersion: KeyringSchemaVersion,
		Evidence:      families[0], Event: families[1], Reference: families[2],
		ReferenceLookup: families[3], TakeoverOperation: families[4],
	}
	ring, err := New(file)
	if err != nil {
		return File{}, err
	}
	ring.Clear()
	return file, nil
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
		return nil, fmt.Errorf("encode callback evidence keyring: %w", err)
	}
	if int64(buffer.Len()) > MaximumKeyringSize {
		clear(buffer.Bytes())
		return nil, ErrInvalidKeyring
	}
	return buffer.Bytes(), nil
}

// Parse decodes one bounded canonical private keyring.
func Parse(content []byte) (File, error) {
	if len(content) == 0 || int64(len(content)) > MaximumKeyringSize {
		return File{}, ErrInvalidKeyring
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var file File
	if err := decoder.Decode(&file); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return File{}, ErrInvalidKeyring
	}
	ring, err := New(file)
	if err != nil {
		return File{}, err
	}
	ring.Clear()
	return file, nil
}

// New copies and validates all purpose-separated key families.
func New(file File) (*Ring, error) {
	if file.SchemaVersion != KeyringSchemaVersion {
		return nil, ErrInvalidKeyring
	}
	evidence, err := parseFamily(file.Evidence)
	if err != nil {
		return nil, err
	}
	event, err := parseFamily(file.Event)
	if err != nil {
		clearFamily(&evidence)
		return nil, err
	}
	reference, err := parseFamily(file.Reference)
	if err != nil {
		clearFamilies(&evidence, &event)
		return nil, err
	}
	referenceLookup, err := parseFamily(file.ReferenceLookup)
	if err != nil {
		clearFamilies(&evidence, &event, &reference)
		return nil, err
	}
	takeoverOperation, err := parseFamily(file.TakeoverOperation)
	if err != nil {
		clearFamilies(&evidence, &event, &reference, &referenceLookup)
		return nil, err
	}
	families := []*family{
		&evidence, &event, &reference, &referenceLookup, &takeoverOperation,
	}
	if !familiesIndependent(families) {
		clearFamilies(families...)
		return nil, ErrInvalidKeyring
	}
	return &Ring{
		evidence: evidence, event: event, reference: reference,
		referenceLookup: referenceLookup, takeoverOperation: takeoverOperation,
	}, nil
}

func parseFamily(file FileFamily) (family, error) {
	if file.ActiveVersion == 0 || len(file.Keys) == 0 || len(file.Keys) > MaximumVersionsPerFamily {
		return family{}, ErrInvalidKeyring
	}
	result := family{active: file.ActiveVersion, keys: make(map[uint32]keyMaterial, len(file.Keys))}
	activeCount := 0
	for _, item := range file.Keys {
		if item.Version == 0 {
			clearFamily(&result)
			return family{}, ErrInvalidKeyring
		}
		if _, duplicate := result.keys[item.Version]; duplicate {
			clearFamily(&result)
			return family{}, ErrInvalidKeyring
		}
		material := keyMaterial{state: item.State}
		switch item.State {
		case KeyStateActive, KeyStateRetained:
			decoded, err := base64.RawURLEncoding.DecodeString(item.Secret)
			if err != nil || len(decoded) != SecretBytes {
				clear(decoded)
				clearFamily(&result)
				return family{}, ErrInvalidKeyring
			}
			material.secret = append([]byte(nil), decoded...)
			clear(decoded)
		case KeyStateRevoked:
			if item.Secret != "" {
				clearFamily(&result)
				return family{}, ErrInvalidKeyring
			}
		default:
			clearFamily(&result)
			return family{}, ErrInvalidKeyring
		}
		if item.State == KeyStateActive {
			activeCount++
			if item.Version != file.ActiveVersion {
				clear(material.secret)
				clearFamily(&result)
				return family{}, ErrInvalidKeyring
			}
		}
		result.keys[item.Version] = material
	}
	if activeCount != 1 {
		clearFamily(&result)
		return family{}, ErrInvalidKeyring
	}
	return result, nil
}

// Seal encrypts one bounded evidence frame with route-bound AAD.
func (r *Ring) Seal(
	random io.Reader,
	evidenceNo string,
	routeNo string,
	provider string,
	source string,
	plaintext []byte,
) (SealedEvidence, error) {
	if r == nil {
		return SealedEvidence{}, ErrKeyUnavailable
	}
	return seal(r.evidence, random, evidenceAADDomain, plaintext,
		evidenceNo, routeNo, provider, source)
}

// SealReference encrypts one provider external reference with route-bound AAD.
func (r *Ring) SealReference(
	random io.Reader,
	routeNo string,
	provider string,
	plaintext []byte,
) (SealedEvidence, error) {
	if r == nil {
		return SealedEvidence{}, ErrKeyUnavailable
	}
	return seal(r.reference, random, referenceAADDomain, plaintext, routeNo, provider)
}

func seal(
	keys family,
	random io.Reader,
	domain string,
	plaintext []byte,
	fields ...string,
) (SealedEvidence, error) {
	if len(plaintext) == 0 {
		return SealedEvidence{}, ErrKeyUnavailable
	}
	material, ok := keys.keys[keys.active]
	if !ok || material.state != KeyStateActive || len(material.secret) != SecretBytes {
		return SealedEvidence{}, ErrKeyUnavailable
	}
	block, err := aes.NewCipher(material.secret)
	if err != nil {
		return SealedEvidence{}, ErrKeyUnavailable
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return SealedEvidence{}, ErrKeyUnavailable
	}
	if random == nil {
		random = rand.Reader
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(random, nonce); err != nil {
		clear(nonce)
		return SealedEvidence{}, ErrKeyUnavailable
	}
	aad := encodeFrame(domain, fields...)
	ciphertext := gcm.Seal(nil, nonce, plaintext, aad)
	clear(aad)
	return SealedEvidence{
		FormatVersion: CipherFormatVersion, KeyVersion: keys.active,
		Nonce: nonce, Ciphertext: ciphertext, PlainSHA256: sha256.Sum256(plaintext),
	}, nil
}

// Open decrypts evidence for synchronous, authorized verification workflows.
func (r *Ring) Open(
	evidenceNo string,
	routeNo string,
	provider string,
	source string,
	sealed SealedEvidence,
) ([]byte, error) {
	if r == nil {
		return nil, ErrKeyUnavailable
	}
	return open(
		r.evidence, evidenceAADDomain, sealed, true,
		evidenceNo, routeNo, provider, source,
	)
}

// OpenReference exposes plaintext only to the synchronous read-only query boundary.
func (r *Ring) OpenReference(
	routeNo string,
	provider string,
	sealed SealedEvidence,
) ([]byte, error) {
	if r == nil {
		return nil, ErrKeyUnavailable
	}
	return open(r.reference, referenceAADDomain, sealed, false, routeNo, provider)
}

func open(
	keys family,
	domain string,
	sealed SealedEvidence,
	verifyPlainSHA256 bool,
	fields ...string,
) ([]byte, error) {
	if sealed.FormatVersion != CipherFormatVersion {
		return nil, ErrKeyUnavailable
	}
	material, ok := keys.keys[sealed.KeyVersion]
	if !ok || material.state == KeyStateRevoked || len(material.secret) != SecretBytes {
		return nil, ErrKeyUnavailable
	}
	block, err := aes.NewCipher(material.secret)
	if err != nil {
		return nil, ErrKeyUnavailable
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(sealed.Nonce) != gcm.NonceSize() {
		return nil, ErrKeyUnavailable
	}
	aad := encodeFrame(domain, fields...)
	plaintext, err := gcm.Open(nil, sealed.Nonce, sealed.Ciphertext, aad)
	clear(aad)
	if err != nil || (verifyPlainSHA256 && !bytes.Equal(sumBytes(plaintext), sealed.PlainSHA256[:])) {
		clear(plaintext)
		return nil, ErrKeyUnavailable
	}
	return plaintext, nil
}

// EventDigests returns active and retained provider-scoped event HMACs.
func (r *Ring) EventDigests(provider string, eventID []byte) ([]EventDigest, error) {
	if r == nil {
		return nil, ErrKeyUnavailable
	}
	return hmacDigests(r.event, eventHMACDomain, 256, provider, eventID)
}

// ReferenceDigests returns active and retained provider-scoped lookup HMACs.
func (r *Ring) ReferenceDigests(provider string, externalReference []byte) ([]EventDigest, error) {
	if r == nil {
		return nil, ErrKeyUnavailable
	}
	return hmacDigests(
		r.referenceLookup, referenceHMACDomain, 512, provider, externalReference,
	)
}

// TakeoverOperationDigests returns purpose-separated operation-key HMACs.
func (r *Ring) TakeoverOperationDigests(
	installationID string,
	operationKey []byte,
) ([]EventDigest, error) {
	if r == nil || len(operationKey) != SecretBytes {
		return nil, ErrKeyUnavailable
	}
	return hmacDigests(
		r.takeoverOperation, takeoverHMACDomain, SecretBytes, installationID, operationKey,
	)
}

func hmacDigests(
	keys family,
	domain string,
	maximumValueBytes int,
	scope string,
	value []byte,
) ([]EventDigest, error) {
	if len(scope) == 0 || len(value) == 0 || len(value) > maximumValueBytes {
		return nil, ErrKeyUnavailable
	}
	versions := make([]int, 0, len(keys.keys))
	for version, material := range keys.keys {
		if material.state != KeyStateRevoked && len(material.secret) == SecretBytes {
			versions = append(versions, int(version))
		}
	}
	sort.Ints(versions)
	result := make([]EventDigest, 0, len(versions))
	frame := encodeFrame(domain, scope, string(value))
	defer clear(frame)
	for _, rawVersion := range versions {
		version := uint32(rawVersion)
		mac := hmac.New(sha256.New, keys.keys[version].secret)
		if _, err := mac.Write(frame); err != nil {
			return nil, ErrKeyUnavailable
		}
		var sum [sha256.Size]byte
		copy(sum[:], mac.Sum(nil))
		result = append(result, EventDigest{KeyVersion: version, Sum: sum})
	}
	if len(result) == 0 {
		return nil, ErrKeyUnavailable
	}
	return result, nil
}

func (r *Ring) ActiveEventVersion() uint32 {
	if r == nil {
		return 0
	}
	return r.event.active
}

func (r *Ring) ActiveEvidenceVersion() uint32 {
	if r == nil {
		return 0
	}
	return r.evidence.active
}

func (r *Ring) ActiveReferenceVersion() uint32 {
	if r == nil {
		return 0
	}
	return r.reference.active
}

func (r *Ring) ActiveReferenceLookupVersion() uint32 {
	if r == nil {
		return 0
	}
	return r.referenceLookup.active
}

func (r *Ring) ActiveTakeoverOperationVersion() uint32 {
	if r == nil {
		return 0
	}
	return r.takeoverOperation.active
}

// Clear zeroes all copied key material.
func (r *Ring) Clear() {
	if r == nil {
		return
	}
	clearFamily(&r.evidence)
	clearFamily(&r.event)
	clearFamily(&r.reference)
	clearFamily(&r.referenceLookup)
	clearFamily(&r.takeoverOperation)
}

func familiesIndependent(families []*family) bool {
	for leftIndex, left := range families {
		for _, leftKey := range left.keys {
			if len(leftKey.secret) == 0 {
				continue
			}
			for _, right := range families[leftIndex+1:] {
				for _, rightKey := range right.keys {
					if len(rightKey.secret) > 0 && bytes.Equal(leftKey.secret, rightKey.secret) {
						return false
					}
				}
			}
		}
	}
	return true
}

func clearFamilies(values ...*family) {
	for _, value := range values {
		clearFamily(value)
	}
}

func clearFamily(value *family) {
	if value == nil {
		return
	}
	for version, material := range value.keys {
		clear(material.secret)
		delete(value.keys, version)
	}
	value.active = 0
}

func encodeFrame(domain string, fields ...string) []byte {
	var buffer bytes.Buffer
	all := append([]string{domain}, fields...)
	for _, field := range all {
		var length [4]byte
		binary.BigEndian.PutUint32(length[:], uint32(len(field)))
		buffer.Write(length[:])
		buffer.WriteString(field)
	}
	return buffer.Bytes()
}

func sumBytes(value []byte) []byte {
	digest := sha256.Sum256(value)
	return digest[:]
}
