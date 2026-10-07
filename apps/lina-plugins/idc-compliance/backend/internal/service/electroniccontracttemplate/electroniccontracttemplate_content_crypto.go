// This file owns deterministic authenticated envelopes for immutable template revisions.

package electroniccontracttemplate

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"io"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

var contractContentEnvelopeMagic = [...]byte{'L', 'C', 'T', '1'}

func (s *serviceImpl) withContentKey(
	ctx context.Context,
	reference string,
	consume func([]byte) error,
) error {
	if s == nil || nilDependency(s.keyAccessor) || consume == nil {
		return gerror.New("contract template content key boundary is unavailable")
	}
	handle, err := secretcap.NewHandle(reference)
	if err != nil {
		return err
	}
	purpose, err := secretcap.NewPurpose("compliance.contract-template-content")
	if err != nil {
		return err
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return err
	}
	return s.keyAccessor.Access(ctx, request, func(encoded []byte) error {
		key, decodeErr := decodeContentKey(encoded)
		if decodeErr != nil {
			return decodeErr
		}
		defer clearContentBytes(key)
		return consume(key)
	})
}

func encryptContentEnvelope(key, plaintext []byte, label string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, gerror.New("contract template encryption key is invalid")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, gerror.New("contract template encryption is unavailable")
	}
	nonceMAC := hmac.New(sha256.New, key)
	for _, value := range [][]byte{
		[]byte("idc/contract-template-envelope-nonce/v1\x00"),
		[]byte(label),
		{0},
		plaintext,
	} {
		if _, err = nonceMAC.Write(value); err != nil {
			return nil, gerror.Wrap(err, "derive contract template envelope nonce")
		}
	}
	nonce := nonceMAC.Sum(nil)[:aead.NonceSize()]
	aad := []byte("idc/contract-template-envelope/v1\x00" + label)
	ciphertext := aead.Seal(nil, nonce, plaintext, aad)
	envelope := make([]byte, 0, len(contractContentEnvelopeMagic)+len(nonce)+len(ciphertext))
	envelope = append(envelope, contractContentEnvelopeMagic[:]...)
	envelope = append(envelope, nonce...)
	envelope = append(envelope, ciphertext...)
	return envelope, nil
}

func decryptContentEnvelope(key, envelope []byte, label string, maximumPlaintext int) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, gerror.New("contract template encryption key is invalid")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, gerror.New("contract template encryption is unavailable")
	}
	headerSize := len(contractContentEnvelopeMagic) + aead.NonceSize()
	if maximumPlaintext <= 0 || len(envelope) < headerSize+aead.Overhead() ||
		len(envelope) > headerSize+aead.Overhead()+maximumPlaintext ||
		!bytes.Equal(envelope[:len(contractContentEnvelopeMagic)], contractContentEnvelopeMagic[:]) {
		return nil, gerror.New("contract template envelope is invalid")
	}
	nonce := envelope[len(contractContentEnvelopeMagic):headerSize]
	plaintext, err := aead.Open(
		nil,
		nonce,
		envelope[headerSize:],
		[]byte("idc/contract-template-envelope/v1\x00"+label),
	)
	if err != nil || len(plaintext) > maximumPlaintext {
		clearContentBytes(plaintext)
		return nil, gerror.New("contract template envelope authentication failed")
	}
	return plaintext, nil
}

func decodeContentKey(encoded []byte) ([]byte, error) {
	decoded := make([]byte, base64.RawURLEncoding.DecodedLen(len(encoded)))
	count, err := base64.RawURLEncoding.Decode(decoded, encoded)
	if err != nil || count != 32 {
		clearContentBytes(decoded)
		return nil, gerror.New("contract template encryption key is invalid")
	}
	return decoded[:count], nil
}

func clearContentBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

func readBounded(reader io.Reader, maximum int64) ([]byte, error) {
	if reader == nil || maximum <= 0 {
		return nil, gerror.New("bounded contract template reader is invalid")
	}
	value, err := io.ReadAll(io.LimitReader(reader, maximum+1))
	if err != nil || int64(len(value)) == 0 || int64(len(value)) > maximum {
		clearContentBytes(value)
		return nil, gerror.New("contract template content is empty or too large")
	}
	return value, nil
}
