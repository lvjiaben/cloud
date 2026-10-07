package invoiceprofile

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"io"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const (
	cursorVersion       = byte(1)
	cursorDigestSize    = 16
	cursorPlaintextSize = 8 + 8 + cursorDigestSize
	maximumCursorLength = 128
)

var cursorAAD = []byte("idc-core/invoice-profile-cursor/v1")

type cursorBoundary struct {
	UpdatedAtMicroseconds int64
	ID                    int64
}

type cursorCodec struct{ aead cipher.AEAD }

func deriveKey(key []byte, purpose string) []byte {
	digest := sha256.New()
	_, _ = digest.Write([]byte(purpose))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write(key)
	return digest.Sum(nil)
}

func newCursorCodec(key []byte) (*cursorCodec, error) {
	derived := deriveKey(key, "idc-core/invoice-profile-cursor-key/v1")
	defer clear(derived)
	block, err := aes.NewCipher(derived)
	if err != nil {
		return nil, gerror.Wrap(err, "create invoice profile cursor cipher")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, gerror.Wrap(err, "create invoice profile cursor AEAD")
	}
	return &cursorCodec{aead: aead}, nil
}

func customerDigest(customerNo string) [sha256.Size]byte {
	return sha256.Sum256([]byte("idc-core/invoice-profile-customer/v1\x00" + customerNo))
}

func (c *cursorCodec) encode(boundary cursorBoundary, digest [sha256.Size]byte) (string, error) {
	if c == nil || c.aead == nil || boundary.UpdatedAtMicroseconds <= 0 || boundary.ID <= 0 {
		return "", bizerrInvalidCursor()
	}
	plaintext := make([]byte, cursorPlaintextSize)
	binary.BigEndian.PutUint64(plaintext[0:8], uint64(boundary.UpdatedAtMicroseconds))
	binary.BigEndian.PutUint64(plaintext[8:16], uint64(boundary.ID))
	copy(plaintext[16:], digest[:cursorDigestSize])
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		clear(plaintext)
		return "", gerror.Wrap(err, "generate invoice profile cursor nonce")
	}
	payload := make([]byte, 1, 1+len(nonce)+len(plaintext)+c.aead.Overhead())
	payload[0] = cursorVersion
	payload = append(payload, nonce...)
	payload = c.aead.Seal(payload, nonce, plaintext, cursorAAD)
	clear(plaintext)
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func (c *cursorCodec) decode(encoded string, digest [sha256.Size]byte) (*cursorBoundary, error) {
	if c == nil || c.aead == nil || len(encoded) == 0 || len(encoded) > maximumCursorLength {
		return nil, bizerrInvalidCursor()
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	expected := 1 + c.aead.NonceSize() + cursorPlaintextSize + c.aead.Overhead()
	if err != nil || len(payload) != expected || payload[0] != cursorVersion {
		return nil, bizerrInvalidCursor()
	}
	nonce := payload[1 : 1+c.aead.NonceSize()]
	plaintext, err := c.aead.Open(nil, nonce, payload[1+c.aead.NonceSize():], cursorAAD)
	if err != nil || len(plaintext) != cursorPlaintextSize {
		clear(plaintext)
		return nil, bizerrInvalidCursor()
	}
	defer clear(plaintext)
	boundary := &cursorBoundary{
		UpdatedAtMicroseconds: int64(binary.BigEndian.Uint64(plaintext[0:8])),
		ID:                    int64(binary.BigEndian.Uint64(plaintext[8:16])),
	}
	if boundary.UpdatedAtMicroseconds <= 0 || boundary.ID <= 0 ||
		subtle.ConstantTimeCompare(plaintext[16:], digest[:cursorDigestSize]) != 1 {
		return nil, bizerrInvalidCursor()
	}
	return boundary, nil
}

func bizerrInvalidCursor() error { return bizerr.NewCode(CodeInvalid) }
