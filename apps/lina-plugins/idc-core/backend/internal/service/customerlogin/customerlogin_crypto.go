package customerlogin

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"net/mail"
	"regexp"
	"strings"
)

var phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`)

// Normalize applies the same explicit contact semantics at index, verification
// and login boundaries. Independent usernames are never rewritten.
func normalize(kind customerlogincap.Kind, value string) (string, error) {
	value = strings.TrimSpace(value)
	switch kind {
	case customerlogincap.Email:
		address, err := mail.ParseAddress(value)
		if err != nil || address.Address != value || len(value) > 320 {
			return "", bizerr.NewCode(CodeInvalid)
		}
		return strings.ToLower(value), nil
	case customerlogincap.Phone:
		if len(value) == 11 && strings.HasPrefix(value, "1") {
			value = "+86" + value
		}
		if !phonePattern.MatchString(value) {
			return "", bizerr.NewCode(CodeInvalid)
		}
		return value, nil
	case customerlogincap.Username:
		if value == "" || len(value) > 128 {
			return "", bizerr.NewCode(CodeInvalid)
		}
		return value, nil
	default:
		return "", bizerr.NewCode(CodeInvalid)
	}
}
func (s *serviceImpl) digest(kind customerlogincap.Kind, value string) string {
	value, err := normalize(kind, value)
	if err != nil {
		return ""
	}
	mac := hmac.New(sha256.New, s.indexKey)
	if _, err := mac.Write([]byte(string(kind) + "\x00" + value)); err != nil {
		return ""
	}
	return hex.EncodeToString(mac.Sum(nil))
}
func (s *serviceImpl) encrypt(identity *customerlogincap.Identity) (string, error) {
	raw, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	defer clear(raw)
	nonce := make([]byte, s.seal.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(s.seal.Seal(nonce, nonce, raw, []byte("customer-login-v1"))), nil
}
func (s *serviceImpl) decrypt(value string) (*customerlogincap.Identity, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) < s.seal.NonceSize() {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	n := s.seal.NonceSize()
	plain, err := s.seal.Open(nil, raw[:n], raw[n:], []byte("customer-login-v1"))
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	defer clear(plain)
	var identity customerlogincap.Identity
	if err = json.Unmarshal(plain, &identity); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return &identity, nil
}
