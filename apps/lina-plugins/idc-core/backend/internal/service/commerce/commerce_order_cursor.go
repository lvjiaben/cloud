// This file protects order keyset cursors with a deployment-derived key.
package commerce

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	orderCursorVersion       = byte(1)
	orderCursorDigestSize    = 16
	orderCursorPlaintextSize = 8 + 8 + orderCursorDigestSize
	maximumOrderCursorLength = 128
)

var errInvalidOrderCursor = errors.New("invalid order cursor")
var orderCursorAAD = []byte("idc-core/commerce-order-cursor/v1")

type orderCursorBoundary struct {
	CreatedAtMicroseconds int64
	ID                    int64
}

type orderCursorCodec struct{ aead cipher.AEAD }

func newOrderCursorCodec(key []byte) (*orderCursorCodec, error) {
	if len(key) != 32 {
		return nil, gerror.New("commerce order cursor key is invalid")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, gerror.Wrap(err, "create commerce order cursor cipher")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, gerror.Wrap(err, "create commerce order cursor AEAD")
	}
	return &orderCursorCodec{aead: aead}, nil
}

func (c *orderCursorCodec) encode(boundary orderCursorBoundary, digest [sha256.Size]byte) (string, error) {
	if c == nil || c.aead == nil || boundary.CreatedAtMicroseconds <= 0 || boundary.ID <= 0 {
		return "", errInvalidOrderCursor
	}
	plaintext := make([]byte, orderCursorPlaintextSize)
	binary.BigEndian.PutUint64(plaintext[0:8], uint64(boundary.CreatedAtMicroseconds))
	binary.BigEndian.PutUint64(plaintext[8:16], uint64(boundary.ID))
	copy(plaintext[16:], digest[:orderCursorDigestSize])
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		clear(plaintext)
		return "", gerror.Wrap(err, "generate commerce order cursor nonce")
	}
	payload := make([]byte, 1, 1+len(nonce)+len(plaintext)+c.aead.Overhead())
	payload[0] = orderCursorVersion
	payload = append(payload, nonce...)
	payload = c.aead.Seal(payload, nonce, plaintext, orderCursorAAD)
	clear(plaintext)
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func (c *orderCursorCodec) decode(encoded string, digest [sha256.Size]byte) (*orderCursorBoundary, error) {
	if c == nil || c.aead == nil || len(encoded) == 0 || len(encoded) > maximumOrderCursorLength {
		return nil, errInvalidOrderCursor
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	expected := 1 + c.aead.NonceSize() + orderCursorPlaintextSize + c.aead.Overhead()
	if err != nil || len(payload) != expected || payload[0] != orderCursorVersion {
		return nil, errInvalidOrderCursor
	}
	nonce := payload[1 : 1+c.aead.NonceSize()]
	plaintext, err := c.aead.Open(nil, nonce, payload[1+c.aead.NonceSize():], orderCursorAAD)
	if err != nil || len(plaintext) != orderCursorPlaintextSize {
		clear(plaintext)
		return nil, errInvalidOrderCursor
	}
	defer clear(plaintext)
	boundary := &orderCursorBoundary{
		CreatedAtMicroseconds: int64(binary.BigEndian.Uint64(plaintext[0:8])),
		ID:                    int64(binary.BigEndian.Uint64(plaintext[8:16])),
	}
	if boundary.CreatedAtMicroseconds <= 0 || boundary.ID <= 0 ||
		subtle.ConstantTimeCompare(plaintext[16:], digest[:orderCursorDigestSize]) != 1 {
		return nil, errInvalidOrderCursor
	}
	return boundary, nil
}
