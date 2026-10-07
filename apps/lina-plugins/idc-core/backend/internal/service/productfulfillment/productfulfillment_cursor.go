package productfulfillment

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
	"strconv"
)

const fulfillmentCursorPlaintextSize = 57

var fulfillmentCursorAAD = []byte("idc-core/product-fulfillment-management-cursor/v1")
var errInvalidFulfillmentCursor = errors.New("invalid product fulfillment cursor")

type fulfillmentCursorCodec struct{ aead cipher.AEAD }

func newFulfillmentCursorCodec(key []byte) (*fulfillmentCursorCodec, error) {
	if len(key) != 32 {
		return nil, errInvalidFulfillmentCursor
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &fulfillmentCursorCodec{aead: aead}, nil
}
func (c *fulfillmentCursorCodec) encode(last string, digest [32]byte) (string, error) {
	if c == nil || c.aead == nil || len(last) < 1 || len(last) > 40 {
		return "", errInvalidFulfillmentCursor
	}
	plain := make([]byte, fulfillmentCursorPlaintextSize)
	plain[0] = byte(len(last))
	copy(plain[1:41], last)
	copy(plain[41:], digest[:16])
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	payload := append([]byte{1}, nonce...)
	payload = c.aead.Seal(payload, nonce, plain, fulfillmentCursorAAD)
	clear(plain)
	return base64.RawURLEncoding.EncodeToString(payload), nil
}
func (c *fulfillmentCursorCodec) decode(encoded string, digest [32]byte) (string, error) {
	if c == nil || c.aead == nil || len(encoded) < 1 || len(encoded) > 160 {
		return "", errInvalidFulfillmentCursor
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	size := 1 + c.aead.NonceSize() + fulfillmentCursorPlaintextSize + c.aead.Overhead()
	if err != nil || base64.RawURLEncoding.EncodeToString(payload) != encoded || len(payload) != size || payload[0] != 1 {
		return "", errInvalidFulfillmentCursor
	}
	nonce := payload[1 : 1+c.aead.NonceSize()]
	plain, err := c.aead.Open(nil, nonce, payload[1+c.aead.NonceSize():], fulfillmentCursorAAD)
	if err != nil || len(plain) != fulfillmentCursorPlaintextSize {
		return "", errInvalidFulfillmentCursor
	}
	defer clear(plain)
	length := int(plain[0])
	if length < 1 || length > 40 || subtle.ConstantTimeCompare(plain[41:], digest[:16]) != 1 {
		return "", errInvalidFulfillmentCursor
	}
	last := string(plain[1 : 1+length])
	if !managementReferencePattern.MatchString(last) {
		return "", errInvalidFulfillmentCursor
	}
	return last, nil
}
func fulfillmentCursorDigest(filter ListInput, scopeDigest string) [32]byte {
	return sha256.Sum256([]byte(filter.OrderNo + "\x00" + filter.CustomerNo + "\x00" + filter.Mode + "\x00" + filter.SafeStatus + "\x00" + scopeDigest + "\x00" + strconv.Itoa(filter.Limit)))
}
