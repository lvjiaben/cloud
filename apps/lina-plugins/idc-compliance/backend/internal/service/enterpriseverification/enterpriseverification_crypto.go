// This file implements deterministic, authenticated, purpose-separated envelopes.

package enterpriseverification

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"io"

	"github.com/gogf/gf/v2/errors/gerror"
)

// enterpriseEnvelopeMagic identifies the bounded AES-GCM envelope format.
var enterpriseEnvelopeMagic = [...]byte{'L', 'E', 'E', '1'}

func encryptEnvelope(key, plaintext []byte, label string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, gerror.New("enterprise material encryption key is invalid")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, gerror.New("enterprise material encryption is unavailable")
	}
	nonceMAC := hmac.New(sha256.New, key)
	for _, value := range [][]byte{
		[]byte("idc/enterprise-envelope-nonce/v1\x00"), []byte(label), {0}, plaintext,
	} {
		if err = writeHash(nonceMAC, value); err != nil {
			return nil, err
		}
	}
	nonce := nonceMAC.Sum(nil)[:aead.NonceSize()]
	aad := []byte("idc/enterprise-envelope/v1\x00" + label)
	ciphertext := aead.Seal(nil, nonce, plaintext, aad)
	envelope := make([]byte, 0, len(enterpriseEnvelopeMagic)+len(nonce)+len(ciphertext))
	envelope = append(envelope, enterpriseEnvelopeMagic[:]...)
	envelope = append(envelope, nonce...)
	envelope = append(envelope, ciphertext...)
	return envelope, nil
}

func decryptEnvelope(key, envelope []byte, label string, maximumPlaintext int) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, gerror.New("enterprise material encryption key is invalid")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, gerror.New("enterprise material encryption is unavailable")
	}
	headerSize := len(enterpriseEnvelopeMagic) + aead.NonceSize()
	if maximumPlaintext <= 0 || len(envelope) < headerSize+aead.Overhead() ||
		len(envelope) > headerSize+aead.Overhead()+maximumPlaintext ||
		!bytes.Equal(envelope[:len(enterpriseEnvelopeMagic)], enterpriseEnvelopeMagic[:]) {
		return nil, gerror.New("enterprise material envelope is invalid")
	}
	nonce := envelope[len(enterpriseEnvelopeMagic):headerSize]
	ciphertext := envelope[headerSize:]
	aad := []byte("idc/enterprise-envelope/v1\x00" + label)
	plaintext, err := aead.Open(nil, nonce, ciphertext, aad)
	if err != nil || len(plaintext) > maximumPlaintext {
		clearBytes(plaintext)
		return nil, gerror.New("enterprise material envelope authentication failed")
	}
	return plaintext, nil
}

func decodeGeneratedKey(plaintext []byte) ([]byte, error) {
	decoded := make([]byte, base64.RawURLEncoding.DecodedLen(len(plaintext)))
	count, err := base64.RawURLEncoding.Decode(decoded, plaintext)
	if err != nil || count != 32 {
		clearBytes(decoded)
		return nil, gerror.New("enterprise material encryption key is invalid")
	}
	return decoded[:count], nil
}

func keyedFingerprint(key []byte, purpose string, values ...[]byte) (string, error) {
	mac := hmac.New(sha256.New, key)
	if err := writeHash(mac, []byte("idc/enterprise-fingerprint/"+purpose+"/v1\x00")); err != nil {
		return "", err
	}
	var size [4]byte
	for _, value := range values {
		binary.BigEndian.PutUint32(size[:], uint32(len(value)))
		if err := writeHash(mac, size[:]); err != nil {
			return "", err
		}
		if err := writeHash(mac, value); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func writeHash(target hash.Hash, value []byte) error {
	if _, err := target.Write(value); err != nil {
		return gerror.Wrap(err, "write enterprise cryptographic digest")
	}
	return nil
}

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func appendField(target []byte, value []byte) []byte {
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(value)))
	target = append(target, size[:]...)
	return append(target, value...)
}

func buildLegalSnapshot(input SubmitInput, legal validatedLegalProfile, materials []string) []byte {
	buffer := make([]byte, 0, 256)
	buffer = append(buffer, 'L', 'E', 'P', '1')
	buffer = appendField(buffer, legal.legalName)
	buffer = appendField(buffer, []byte(input.RegistrationCountryCode))
	buffer = appendField(buffer, legal.registrationIdentifier)
	buffer = appendField(buffer, legal.legalRepresentativeName)
	buffer = appendField(buffer, legal.legalRepresentativeIdentityNumber)
	buffer = appendField(buffer, legal.agentName)
	buffer = appendField(buffer, legal.agentIdentityNumber)
	buffer = appendField(buffer, []byte(input.IndustryCode))
	buffer = appendField(buffer, []byte(input.ApplicantRole))
	for _, materialNo := range materials {
		buffer = appendField(buffer, []byte(materialNo))
	}
	return buffer
}

func randomHex(source io.Reader, count int) (string, error) {
	buffer := make([]byte, count)
	if _, err := io.ReadFull(source, buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
