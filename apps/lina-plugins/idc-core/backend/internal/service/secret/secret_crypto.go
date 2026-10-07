// This file implements versioned AES-256-GCM envelope encryption and canonical AAD binding.

package secret

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"sort"

	"lina-core/pkg/bizerr"
)

const (
	envelopeVersion = 1
	cipherAlgorithm = "AES-256-GCM"
	dekSize         = 32
	gcmNonceSize    = 12

	aadKindPayload     = "payload"
	aadKindKeyWrap     = "dek-wrap"
	aadKindFingerprint = "request-fingerprint"
)

var errEnvelopeInvalid = errors.New("secret envelope authentication failed")

// envelopeWrite is the complete persisted output for one payload version.
type envelopeWrite struct {
	payloadNonce       []byte
	ciphertext         []byte
	masterKeyID        string
	requestFingerprint string
	wrapNonce          []byte
	wrappedDEK         []byte
}

// keyWrapValue is the persistence-independent material needed to unwrap one DEK.
type keyWrapValue struct {
	masterKeyID string
	nonce       []byte
	ciphertext  []byte
}

// activeKey returns an owned key copy and destroys the provider-owned detached buffer.
func (s *serviceImpl) activeKey(ctx context.Context) (KeyMaterial, error) {
	material, err := s.keys.Active(ctx)
	if err != nil {
		zeroBytes(material.Key)
		return KeyMaterial{}, bizerr.NewCode(CodeUnavailable)
	}
	defer zeroBytes(material.Key)
	if !validKeyID(material.ID) || len(material.Key) != dekSize {
		return KeyMaterial{}, bizerr.NewCode(CodeUnavailable)
	}
	return KeyMaterial{ID: material.ID, Key: append([]byte(nil), material.Key...)}, nil
}

// resolveKeys performs one batch provider call and returns only validated owned copies.
func (s *serviceImpl) resolveKeys(ctx context.Context, keyIDs []string) (map[string][]byte, error) {
	unique := uniqueKeyIDs(keyIDs)
	if len(unique) == 0 || len(unique) > s.config.MaximumKeyWraps {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	for _, keyID := range unique {
		if !validKeyID(keyID) {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
	}
	provided, err := s.keys.Resolve(ctx, unique)
	if err != nil {
		zeroKeyMap(provided)
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	defer zeroKeyMap(provided)
	resolved := make(map[string][]byte, len(provided))
	allowed := make(map[string]struct{}, len(unique))
	for _, keyID := range unique {
		allowed[keyID] = struct{}{}
	}
	for keyID, key := range provided {
		if _, ok := allowed[keyID]; !ok || !validKeyID(keyID) || len(key) != dekSize {
			zeroKeyMap(resolved)
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		resolved[keyID] = append([]byte(nil), key...)
	}
	if len(resolved) == 0 {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return resolved, nil
}

func uniqueKeyIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func zeroKeyMap(values map[string][]byte) {
	for key, value := range values {
		zeroBytes(value)
		delete(values, key)
	}
}

// encryptEnvelope encrypts plaintext with a random DEK and wraps that DEK only
// with the active deployment key.
func (s *serviceImpl) encryptEnvelope(
	active KeyMaterial,
	scope Scope,
	secretRef string,
	version int,
	fence requestFence,
	plaintext []byte,
) (envelopeWrite, error) {
	dek := make([]byte, dekSize)
	if _, err := io.ReadFull(s.random, dek); err != nil {
		zeroBytes(dek)
		return envelopeWrite{}, errEnvelopeInvalid
	}
	defer zeroBytes(dek)
	payloadNonce, ciphertext, err := sealRandom(
		dek,
		plaintext,
		payloadAAD(scope, secretRef, version),
		s.random,
	)
	if err != nil {
		return envelopeWrite{}, errEnvelopeInvalid
	}
	wrapNonce, wrappedDEK, err := sealRandom(
		active.Key,
		dek,
		keyWrapAAD(scope, secretRef, version, active.ID),
		s.random,
	)
	if err != nil {
		zeroBytes(ciphertext)
		return envelopeWrite{}, errEnvelopeInvalid
	}
	requestFingerprint, err := fingerprint(dek, scope, secretRef, version, fence, plaintext)
	if err != nil {
		zeroBytes(ciphertext)
		zeroBytes(wrappedDEK)
		return envelopeWrite{}, errEnvelopeInvalid
	}
	return envelopeWrite{
		payloadNonce:       payloadNonce,
		ciphertext:         ciphertext,
		masterKeyID:        active.ID,
		requestFingerprint: requestFingerprint,
		wrapNonce:          wrapNonce,
		wrappedDEK:         wrappedDEK,
	}, nil
}

// unwrapDEK authenticates wrappers against their exact scope and key ID. The
// first valid wrapper wins; failures reveal no key or handle diagnostics.
func unwrapDEK(
	keys map[string][]byte,
	scope Scope,
	secretRef string,
	version int,
	wrappers []keyWrapValue,
) ([]byte, error) {
	for _, wrapper := range wrappers {
		key := keys[wrapper.masterKeyID]
		if len(key) != dekSize {
			continue
		}
		dek, err := openSealed(
			key,
			wrapper.nonce,
			wrapper.ciphertext,
			keyWrapAAD(scope, secretRef, version, wrapper.masterKeyID),
		)
		if err == nil && len(dek) == dekSize {
			return dek, nil
		}
		zeroBytes(dek)
	}
	return nil, errEnvelopeInvalid
}

func sealRandom(key, plaintext, aad []byte, random io.Reader) ([]byte, []byte, error) {
	nonce := make([]byte, gcmNonceSize)
	if _, err := io.ReadFull(random, nonce); err != nil {
		return nil, nil, err
	}
	ciphertext, err := sealWithNonce(key, nonce, plaintext, aad)
	if err != nil {
		return nil, nil, err
	}
	return nonce, ciphertext, nil
}

func sealWithNonce(key, nonce, plaintext, aad []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil || len(nonce) != gcmNonceSize {
		return nil, errEnvelopeInvalid
	}
	return gcm.Seal(nil, nonce, plaintext, aad), nil
}

func openSealed(key, nonce, ciphertext, aad []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil || len(nonce) != gcmNonceSize || len(ciphertext) < gcm.Overhead() {
		return nil, errEnvelopeInvalid
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, errEnvelopeInvalid
	}
	return plaintext, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != dekSize {
		return nil, errEnvelopeInvalid
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errEnvelopeInvalid
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || gcm.NonceSize() != gcmNonceSize {
		return nil, errEnvelopeInvalid
	}
	return gcm, nil
}

func fingerprint(
	key []byte,
	scope Scope,
	secretRef string,
	version int,
	fence requestFence,
	plaintext []byte,
) (string, error) {
	mac := hmac.New(sha256.New, key)
	if _, err := mac.Write(fingerprintAAD(scope, secretRef, version, fence)); err != nil {
		return "", errEnvelopeInvalid
	}
	if _, err := mac.Write(plaintext); err != nil {
		return "", errEnvelopeInvalid
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func constantTimeStringEqual(left string, right string) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func payloadAAD(scope Scope, secretRef string, version int) []byte {
	return canonicalAAD(scope, secretRef, version, aadKindPayload, "")
}

func keyWrapAAD(scope Scope, secretRef string, version int, keyID string) []byte {
	return canonicalAAD(scope, secretRef, version, aadKindKeyWrap, keyID)
}

func fingerprintAAD(scope Scope, secretRef string, version int, fence requestFence) []byte {
	result := canonicalAAD(scope, secretRef, version, aadKindFingerprint, "")
	result = appendAADField(result, fence.operation)
	result = appendAADField(result, fence.idempotencyDigest)
	result = appendAADUint32(result, fence.parentVersion)
	if fence.expectedUpdatedAt.IsZero() {
		result = appendAADUint32(result, 0)
		return appendAADUint64(result, 0)
	}
	result = appendAADUint32(result, 1)
	return appendAADUint64(result, fence.expectedUpdatedAt.UnixMicro())
}

// canonicalAAD uses length-prefixed fields so delimiters inside identifiers can
// never produce an ambiguous authenticated context.
func canonicalAAD(scope Scope, secretRef string, version int, kind string, keyID string) []byte {
	result := make([]byte, 0, 256)
	result = appendAADField(result, "idc-secret")
	result = appendAADUint32(result, envelopeVersion)
	result = appendAADUint32(result, scope.TenantID)
	result = appendAADField(result, scope.OwnerPluginID)
	result = appendAADField(result, scope.CallerScope)
	result = appendAADField(result, secretRef)
	result = appendAADUint32(result, version)
	result = appendAADField(result, kind)
	result = appendAADField(result, keyID)
	return result
}

func appendAADField(destination []byte, value string) []byte {
	destination = appendAADUint32(destination, len(value))
	return append(destination, value...)
}

func appendAADUint32(destination []byte, value int) []byte {
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], uint32(value))
	return append(destination, encoded[:]...)
}

func appendAADUint64(destination []byte, value int64) []byte {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(value))
	return append(destination, encoded[:]...)
}
