package productfulfillment

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"

	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

type sealedProviderEvidence struct {
	KeyID, Digest     string
	Nonce, Ciphertext []byte
}

type providerEvidenceSealer interface {
	Seal(taskNo string, attemptNo int, evidence []byte) (*sealedProviderEvidence, error)
}

type aesProviderEvidenceSealer struct {
	keyID  string
	aead   cipher.AEAD
	random io.Reader
}

func NewProviderEvidenceSealer(keyID string, key []byte) (providerEvidenceSealer, error) {
	if keyID == "" || len(keyID) > 64 || len(key) != 32 {
		return nil, errors.New("product fulfillment evidence key is invalid")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &aesProviderEvidenceSealer{keyID: keyID, aead: aead, random: rand.Reader}, nil
}

func (s *aesProviderEvidenceSealer) Seal(taskNo string, attemptNo int, evidence []byte) (*sealedProviderEvidence, error) {
	if s == nil || s.aead == nil || taskNo == "" || attemptNo < 1 || len(evidence) == 0 || len(evidence) > productfulfillmentcap.MaximumEvidenceBytes {
		return nil, errors.New("product fulfillment provider evidence is invalid")
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := io.ReadFull(s.random, nonce); err != nil {
		clear(nonce)
		return nil, err
	}
	aad := providerEvidenceAAD(taskNo, attemptNo)
	ciphertext := s.aead.Seal(nil, nonce, evidence, aad)
	clear(aad)
	return &sealedProviderEvidence{KeyID: s.keyID, Digest: sha256Hex(evidence), Nonce: nonce, Ciphertext: ciphertext}, nil
}

func providerEvidenceAAD(taskNo string, attemptNo int) []byte {
	value := make([]byte, 0, len(taskNo)+16)
	value = append(value, "pfl-evidence-v1\x00"...)
	value = append(value, taskNo...)
	value = append(value, 0)
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, uint64(attemptNo))
	value = append(value, encoded...)
	clear(encoded)
	return value
}
