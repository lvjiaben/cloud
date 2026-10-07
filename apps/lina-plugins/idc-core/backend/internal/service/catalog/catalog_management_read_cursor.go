// This file protects catalog workbench keyset cursors with deployment-derived encryption.

package catalog

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
	"strconv"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	productManagementCursorVersion       = byte(1)
	productManagementCursorDigestSize    = 16
	productManagementCursorPlaintextSize = 8 + 8 + productManagementCursorDigestSize
	maximumProductManagementCursorLength = 128
)

var errInvalidProductManagementCursor = errors.New("invalid catalog product management cursor")
var productManagementCursorAAD = []byte("idc-core/catalog-management-cursor/v1")

type productManagementCursorBoundary struct {
	SortOrder int64
	ID        int64
}

type productManagementCursorCodec struct {
	aead cipher.AEAD
}

func newProductManagementCursorCodec(key []byte) (*productManagementCursorCodec, error) {
	if len(key) != 32 {
		return nil, gerror.New("catalog product management cursor key is invalid")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, gerror.Wrap(err, "create catalog product management cursor cipher")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, gerror.Wrap(err, "create catalog product management cursor AEAD")
	}
	return &productManagementCursorCodec{aead: aead}, nil
}

func (c *productManagementCursorCodec) encode(
	boundary productManagementCursorBoundary,
	input normalizedProductManagementSearch,
) (string, error) {
	if c == nil || c.aead == nil || boundary.ID <= 0 {
		return "", errInvalidProductManagementCursor
	}
	plaintext := make([]byte, productManagementCursorPlaintextSize)
	binary.BigEndian.PutUint64(plaintext[0:8], uint64(boundary.SortOrder))
	binary.BigEndian.PutUint64(plaintext[8:16], uint64(boundary.ID))
	digest := productManagementCursorFilterDigest(input)
	copy(plaintext[16:], digest[:productManagementCursorDigestSize])
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		clear(plaintext)
		return "", gerror.Wrap(err, "generate catalog product management cursor nonce")
	}
	payload := make([]byte, 1, 1+len(nonce)+len(plaintext)+c.aead.Overhead())
	payload[0] = productManagementCursorVersion
	payload = append(payload, nonce...)
	payload = c.aead.Seal(payload, nonce, plaintext, productManagementCursorAAD)
	clear(plaintext)
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func (c *productManagementCursorCodec) decode(
	encoded string,
	input normalizedProductManagementSearch,
) (*productManagementCursorBoundary, error) {
	if c == nil || c.aead == nil || len(encoded) == 0 ||
		len(encoded) > maximumProductManagementCursorLength {
		return nil, errInvalidProductManagementCursor
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	expectedPayloadSize := 1 + c.aead.NonceSize() +
		productManagementCursorPlaintextSize + c.aead.Overhead()
	if err != nil || len(payload) != expectedPayloadSize || payload[0] != productManagementCursorVersion {
		return nil, errInvalidProductManagementCursor
	}
	nonce := payload[1 : 1+c.aead.NonceSize()]
	plaintext, err := c.aead.Open(nil, nonce, payload[1+c.aead.NonceSize():], productManagementCursorAAD)
	if err != nil || len(plaintext) != productManagementCursorPlaintextSize {
		clear(plaintext)
		return nil, errInvalidProductManagementCursor
	}
	defer clear(plaintext)
	boundary := &productManagementCursorBoundary{
		SortOrder: int64(binary.BigEndian.Uint64(plaintext[0:8])),
		ID:        int64(binary.BigEndian.Uint64(plaintext[8:16])),
	}
	expected := productManagementCursorFilterDigest(input)
	if boundary.ID <= 0 ||
		subtle.ConstantTimeCompare(
			plaintext[16:],
			expected[:productManagementCursorDigestSize],
		) != 1 {
		return nil, errInvalidProductManagementCursor
	}
	return boundary, nil
}

func productManagementCursorFilterDigest(input normalizedProductManagementSearch) [sha256.Size]byte {
	return sha256.Sum256([]byte(input.namePrefix + "\x00" + input.categoryNo + "\x00" + input.status + "\x00" +
		input.source + "\x00" + strconv.Itoa(input.limit)))
}
