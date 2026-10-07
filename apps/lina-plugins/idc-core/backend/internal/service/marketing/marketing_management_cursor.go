// This file protects marketing workbench keyset cursors with deployment-derived encryption.

package marketing

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
	marketingManagementCursorVersion       = byte(1)
	marketingManagementCursorDigestSize    = 16
	marketingManagementCursorPlaintextSize = 8 + 8 + marketingManagementCursorDigestSize
	maximumMarketingManagementCursorLength = 128
)

var errInvalidMarketingManagementCursor = errors.New("invalid marketing management cursor")
var marketingManagementCursorAAD = []byte("idc-core/marketing-management-cursor/v1")

type marketingManagementCursorBoundary struct {
	UpdatedAtMicroseconds int64
	ID                    int64
}

type marketingManagementCursorCodec struct{ aead cipher.AEAD }

func newMarketingManagementCursorCodec(key []byte) (*marketingManagementCursorCodec, error) {
	if len(key) != 32 {
		return nil, gerror.New("marketing management cursor key is invalid")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, gerror.Wrap(err, "create marketing management cursor cipher")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, gerror.Wrap(err, "create marketing management cursor AEAD")
	}
	return &marketingManagementCursorCodec{aead: aead}, nil
}

func (c *marketingManagementCursorCodec) encode(
	boundary marketingManagementCursorBoundary,
	filterDigest [sha256.Size]byte,
) (string, error) {
	if c == nil || c.aead == nil || boundary.UpdatedAtMicroseconds <= 0 || boundary.ID <= 0 {
		return "", errInvalidMarketingManagementCursor
	}
	plaintext := make([]byte, marketingManagementCursorPlaintextSize)
	binary.BigEndian.PutUint64(plaintext[0:8], uint64(boundary.UpdatedAtMicroseconds))
	binary.BigEndian.PutUint64(plaintext[8:16], uint64(boundary.ID))
	copy(plaintext[16:], filterDigest[:marketingManagementCursorDigestSize])
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		clear(plaintext)
		return "", gerror.Wrap(err, "generate marketing management cursor nonce")
	}
	payload := make([]byte, 1, 1+len(nonce)+len(plaintext)+c.aead.Overhead())
	payload[0] = marketingManagementCursorVersion
	payload = append(payload, nonce...)
	payload = c.aead.Seal(payload, nonce, plaintext, marketingManagementCursorAAD)
	clear(plaintext)
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func (c *marketingManagementCursorCodec) decode(
	encoded string,
	filterDigest [sha256.Size]byte,
) (*marketingManagementCursorBoundary, error) {
	if c == nil || c.aead == nil || len(encoded) == 0 || len(encoded) > maximumMarketingManagementCursorLength {
		return nil, errInvalidMarketingManagementCursor
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	expectedSize := 1 + c.aead.NonceSize() + marketingManagementCursorPlaintextSize + c.aead.Overhead()
	if err != nil || len(payload) != expectedSize || payload[0] != marketingManagementCursorVersion {
		return nil, errInvalidMarketingManagementCursor
	}
	nonce := payload[1 : 1+c.aead.NonceSize()]
	plaintext, err := c.aead.Open(nil, nonce, payload[1+c.aead.NonceSize():], marketingManagementCursorAAD)
	if err != nil || len(plaintext) != marketingManagementCursorPlaintextSize {
		clear(plaintext)
		return nil, errInvalidMarketingManagementCursor
	}
	defer clear(plaintext)
	boundary := &marketingManagementCursorBoundary{
		UpdatedAtMicroseconds: int64(binary.BigEndian.Uint64(plaintext[0:8])),
		ID:                    int64(binary.BigEndian.Uint64(plaintext[8:16])),
	}
	if boundary.UpdatedAtMicroseconds <= 0 || boundary.ID <= 0 ||
		subtle.ConstantTimeCompare(plaintext[16:], filterDigest[:marketingManagementCursorDigestSize]) != 1 {
		return nil, errInvalidMarketingManagementCursor
	}
	return boundary, nil
}
