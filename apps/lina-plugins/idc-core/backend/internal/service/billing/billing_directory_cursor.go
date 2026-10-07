package billing

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
	directoryCursorVersion       = byte(1)
	directoryCursorDigestSize    = 16
	directoryCursorPlaintextSize = 8 + 8 + directoryCursorDigestSize
	maximumDirectoryCursorLength = 128
)

var errInvalidDirectoryCursor = errors.New("invalid billing directory cursor")
var directoryCursorAAD = []byte("idc-core/billing-directory-cursor/v1")

type directoryCursorBoundary struct {
	SortTimeMicroseconds int64
	ID                   int64
}

type directoryCursorCodec struct{ aead cipher.AEAD }

func newDirectoryCursorCodec(key []byte) (*directoryCursorCodec, error) {
	if len(key) != 32 {
		return nil, gerror.New("billing directory cursor key is invalid")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, gerror.Wrap(err, "create billing directory cursor cipher")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, gerror.Wrap(err, "create billing directory cursor AEAD")
	}
	return &directoryCursorCodec{aead: aead}, nil
}

func (c *directoryCursorCodec) encode(
	boundary directoryCursorBoundary,
	digest [sha256.Size]byte,
) (string, error) {
	if c == nil || c.aead == nil || boundary.SortTimeMicroseconds <= 0 || boundary.ID <= 0 {
		return "", errInvalidDirectoryCursor
	}
	plaintext := make([]byte, directoryCursorPlaintextSize)
	binary.BigEndian.PutUint64(plaintext[0:8], uint64(boundary.SortTimeMicroseconds))
	binary.BigEndian.PutUint64(plaintext[8:16], uint64(boundary.ID))
	copy(plaintext[16:], digest[:directoryCursorDigestSize])
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		clear(plaintext)
		return "", gerror.Wrap(err, "generate billing directory cursor nonce")
	}
	payload := make([]byte, 1, 1+len(nonce)+len(plaintext)+c.aead.Overhead())
	payload[0] = directoryCursorVersion
	payload = append(payload, nonce...)
	payload = c.aead.Seal(payload, nonce, plaintext, directoryCursorAAD)
	clear(plaintext)
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func (c *directoryCursorCodec) decode(
	encoded string,
	digest [sha256.Size]byte,
) (*directoryCursorBoundary, error) {
	if c == nil || c.aead == nil || len(encoded) == 0 || len(encoded) > maximumDirectoryCursorLength {
		return nil, errInvalidDirectoryCursor
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	expected := 1 + c.aead.NonceSize() + directoryCursorPlaintextSize + c.aead.Overhead()
	if err != nil || len(payload) != expected || payload[0] != directoryCursorVersion {
		return nil, errInvalidDirectoryCursor
	}
	nonce := payload[1 : 1+c.aead.NonceSize()]
	plaintext, err := c.aead.Open(nil, nonce, payload[1+c.aead.NonceSize():], directoryCursorAAD)
	if err != nil || len(plaintext) != directoryCursorPlaintextSize {
		clear(plaintext)
		return nil, errInvalidDirectoryCursor
	}
	defer clear(plaintext)
	boundary := &directoryCursorBoundary{
		SortTimeMicroseconds: int64(binary.BigEndian.Uint64(plaintext[0:8])),
		ID:                   int64(binary.BigEndian.Uint64(plaintext[8:16])),
	}
	if boundary.SortTimeMicroseconds <= 0 || boundary.ID <= 0 ||
		subtle.ConstantTimeCompare(plaintext[16:], digest[:directoryCursorDigestSize]) != 1 {
		return nil, errInvalidDirectoryCursor
	}
	return boundary, nil
}
