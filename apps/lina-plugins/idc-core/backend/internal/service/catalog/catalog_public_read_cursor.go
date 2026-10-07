// This file encrypts anonymous catalog keyset cursors and binds every filter.

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
	publicCatalogCursorVersion       = byte(1)
	publicCatalogCursorDigestSize    = 16
	publicCatalogCursorPlaintextSize = 8 + 8 + publicCatalogCursorDigestSize
	maximumPublicCatalogCursorLength = 128
)

var errInvalidPublicCatalogCursor = errors.New("invalid public catalog cursor")
var publicCatalogCursorAAD = []byte("idc-core/public-catalog-cursor/v1")

type publicCatalogCursorBoundary struct {
	SortOrder int64
	ID        int64
}

type publicCatalogCursorCodec struct {
	aead cipher.AEAD
}

func newPublicCatalogCursorCodec(key []byte) (*publicCatalogCursorCodec, error) {
	if len(key) != 32 {
		return nil, gerror.New("public catalog cursor key is invalid")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, gerror.Wrap(err, "create public catalog cursor cipher")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, gerror.Wrap(err, "create public catalog cursor AEAD")
	}
	return &publicCatalogCursorCodec{aead: aead}, nil
}

func (c *publicCatalogCursorCodec) encode(
	boundary publicCatalogCursorBoundary,
	input normalizedPublicCatalogSearch,
) (string, error) {
	if c == nil || c.aead == nil || boundary.ID <= 0 {
		return "", errInvalidPublicCatalogCursor
	}
	plaintext := make([]byte, publicCatalogCursorPlaintextSize)
	binary.BigEndian.PutUint64(plaintext[0:8], uint64(boundary.SortOrder))
	binary.BigEndian.PutUint64(plaintext[8:16], uint64(boundary.ID))
	digest := publicCatalogCursorFilterDigest(input)
	copy(plaintext[16:], digest[:publicCatalogCursorDigestSize])
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		clear(plaintext)
		return "", gerror.Wrap(err, "generate public catalog cursor nonce")
	}
	payload := make([]byte, 1, 1+len(nonce)+len(plaintext)+c.aead.Overhead())
	payload[0] = publicCatalogCursorVersion
	payload = append(payload, nonce...)
	payload = c.aead.Seal(payload, nonce, plaintext, publicCatalogCursorAAD)
	clear(plaintext)
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func (c *publicCatalogCursorCodec) decode(
	encoded string,
	input normalizedPublicCatalogSearch,
) (*publicCatalogCursorBoundary, error) {
	if c == nil || c.aead == nil || len(encoded) == 0 || len(encoded) > maximumPublicCatalogCursorLength {
		return nil, errInvalidPublicCatalogCursor
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	expectedSize := 1 + c.aead.NonceSize() + publicCatalogCursorPlaintextSize + c.aead.Overhead()
	if err != nil || len(payload) != expectedSize || payload[0] != publicCatalogCursorVersion {
		return nil, errInvalidPublicCatalogCursor
	}
	nonce := payload[1 : 1+c.aead.NonceSize()]
	plaintext, err := c.aead.Open(nil, nonce, payload[1+c.aead.NonceSize():], publicCatalogCursorAAD)
	if err != nil || len(plaintext) != publicCatalogCursorPlaintextSize {
		clear(plaintext)
		return nil, errInvalidPublicCatalogCursor
	}
	defer clear(plaintext)
	boundary := &publicCatalogCursorBoundary{
		SortOrder: int64(binary.BigEndian.Uint64(plaintext[0:8])),
		ID:        int64(binary.BigEndian.Uint64(plaintext[8:16])),
	}
	expected := publicCatalogCursorFilterDigest(input)
	if boundary.ID <= 0 || subtle.ConstantTimeCompare(
		plaintext[16:], expected[:publicCatalogCursorDigestSize],
	) != 1 {
		return nil, errInvalidPublicCatalogCursor
	}
	return boundary, nil
}

func publicCatalogCursorFilterDigest(input normalizedPublicCatalogSearch) [sha256.Size]byte {
	return sha256.Sum256([]byte(input.namePrefix + "\x00" + input.categoryNo + "\x00" +
		input.currency + "\x00" + strconv.Itoa(input.limit)))
}
