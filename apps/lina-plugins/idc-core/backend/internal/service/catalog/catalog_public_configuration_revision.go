// This file binds anonymous configuration choices to one current published version.

package catalog

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/gogf/gf/v2/errors/gerror"
)

const publicConfigurationRevisionLength = 43

var publicConfigurationRevisionPurpose = []byte("idc-core/public-configuration-revision/v1")

var publicQuoteTermPurpose = []byte("idc-core/public-quote-term/v1")

var publicStructuredQuoteTermPurpose = []byte("idc-core/public-structured-quote-term/v1")

var publicStructuredConfigurationPurpose = []byte("idc-core/public-structured-configuration/v1")

var publicStructuredFieldPurpose = []byte("idc-core/public-structured-field/v1")

var publicStructuredOptionPurpose = []byte("idc-core/public-structured-option/v1")

type publicConfigurationRevisionCodec struct {
	key            [sha256.Size]byte
	optionKey      [sha256.Size]byte
	optionNonceKey [sha256.Size]byte
}

type publicStructuredOptionPayload struct {
	Revision string `json:"revision"`
	FieldKey string `json:"fieldKey"`
	Value    string `json:"value"`
}

func newPublicConfigurationRevisionCodec(masterKey []byte) (*publicConfigurationRevisionCodec, error) {
	if len(masterKey) != 32 {
		return nil, gerror.New("public configuration revision key is invalid")
	}
	deriver := hmac.New(sha256.New, masterKey)
	_, _ = deriver.Write(publicConfigurationRevisionPurpose)
	codec := &publicConfigurationRevisionCodec{}
	copy(codec.key[:], deriver.Sum(nil))
	optionDeriver := hmac.New(sha256.New, codec.key[:])
	_, _ = optionDeriver.Write(publicStructuredOptionPurpose)
	copy(codec.optionKey[:], optionDeriver.Sum(nil))
	nonceDeriver := hmac.New(sha256.New, codec.key[:])
	_, _ = nonceDeriver.Write([]byte("idc-core/public-structured-option-nonce/v1"))
	copy(codec.optionNonceKey[:], nonceDeriver.Sum(nil))
	return codec, nil
}

func (c *publicConfigurationRevisionCodec) encode(publicSlug, versionNo string) (string, error) {
	if c == nil || publicSlug == "" || versionNo == "" {
		return "", gerror.New("public configuration revision input is invalid")
	}
	mac := hmac.New(sha256.New, c.key[:])
	_, _ = mac.Write([]byte(publicSlug))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(versionNo))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (c *publicConfigurationRevisionCodec) verify(revision, publicSlug, versionNo string) bool {
	if c == nil || len(revision) != publicConfigurationRevisionLength {
		return false
	}
	provided, err := base64.RawURLEncoding.DecodeString(revision)
	if err != nil || len(provided) != sha256.Size {
		return false
	}
	expected, err := c.encode(publicSlug, versionNo)
	if err != nil {
		return false
	}
	expectedBytes, err := base64.RawURLEncoding.DecodeString(expected)
	return err == nil && subtle.ConstantTimeCompare(provided, expectedBytes) == 1
}

func (c *publicConfigurationRevisionCodec) encodeQuoteTerm(
	publicSlug string,
	versionNo string,
	billingRuleNo string,
) (string, error) {
	if c == nil || publicSlug == "" || versionNo == "" || billingRuleNo == "" {
		return "", gerror.New("public quote term input is invalid")
	}
	mac := hmac.New(sha256.New, c.key[:])
	_, _ = mac.Write(publicQuoteTermPurpose)
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(publicSlug))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(versionNo))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(billingRuleNo))
	return "term_" + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (c *publicConfigurationRevisionCodec) encodeStructuredQuoteTerm(publicSlug, versionNo, billingRuleNo, selectionDigest string) (string, error) {
	if len(selectionDigest) != 64 {
		return "", gerror.New("structured quote selection digest is invalid")
	}
	token, err := c.encodeOpaque(publicStructuredQuoteTermPurpose, publicSlug, versionNo, billingRuleNo, selectionDigest)
	if err != nil {
		return "", err
	}
	return "term_" + token, nil
}

func (c *publicConfigurationRevisionCodec) encodeStructuredRevision(
	publicSlug string,
	versionNo string,
	configurationVersion int64,
	declarationDigest string,
) (string, error) {
	return c.encodeOpaque(publicStructuredConfigurationPurpose, publicSlug, versionNo,
		fmt.Sprintf("%d", configurationVersion), declarationDigest)
}

func (c *publicConfigurationRevisionCodec) encodeStructuredField(revision, fieldKey string) (string, error) {
	token, err := c.encodeOpaque(publicStructuredFieldPurpose, revision, fieldKey)
	if err != nil {
		return "", err
	}
	return "field_" + token, nil
}

func (c *publicConfigurationRevisionCodec) encodeStructuredOption(
	revision string,
	fieldKey string,
	value string,
) (string, error) {
	if c == nil || revision == "" || fieldKey == "" || value == "" || len(value) > 256 {
		return "", gerror.New("public structured option input is invalid")
	}
	plaintext, err := json.Marshal(publicStructuredOptionPayload{Revision: revision, FieldKey: fieldKey, Value: value})
	if err != nil {
		return "", gerror.Wrap(err, "encode public structured option")
	}
	block, err := aes.NewCipher(c.optionKey[:])
	if err != nil {
		return "", gerror.Wrap(err, "construct public structured option cipher")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", gerror.Wrap(err, "construct public structured option cipher")
	}
	// The token must be stable within one immutable public revision so the
	// authoritative selected value matches the corresponding option token.
	// A purpose-isolated keyed digest derives a distinct nonce for each exact
	// plaintext; changing revision, field, or value changes the nonce.
	nonce := make([]byte, aead.NonceSize())
	nonceMAC := hmac.New(sha256.New, c.optionNonceKey[:])
	_, _ = nonceMAC.Write(plaintext)
	copy(nonce, nonceMAC.Sum(nil))
	sealed := aead.Seal(nonce, nonce, plaintext, publicStructuredOptionPurpose)
	return "option_" + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (c *publicConfigurationRevisionCodec) decodeStructuredOption(token, revision, fieldKey string) (string, error) {
	if c == nil || revision == "" || fieldKey == "" || len(token) < len("option_")+1 || len(token) > 2048 || token[:len("option_")] != "option_" {
		return "", gerror.New("public structured option token is invalid")
	}
	sealed, err := base64.RawURLEncoding.DecodeString(token[len("option_"):])
	if err != nil || base64.RawURLEncoding.EncodeToString(sealed) != token[len("option_"):] {
		return "", gerror.New("public structured option token is invalid")
	}
	block, err := aes.NewCipher(c.optionKey[:])
	if err != nil {
		return "", gerror.Wrap(err, "construct public structured option cipher")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil || len(sealed) <= aead.NonceSize() {
		return "", gerror.New("public structured option token is invalid")
	}
	plaintext, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], publicStructuredOptionPurpose)
	if err != nil {
		return "", gerror.New("public structured option token is invalid")
	}
	var payload publicStructuredOptionPayload
	if err = json.Unmarshal(plaintext, &payload); err != nil || payload.Revision != revision || payload.FieldKey != fieldKey || payload.Value == "" || len(payload.Value) > 256 {
		return "", gerror.New("public structured option token is invalid")
	}
	return payload.Value, nil
}

func (c *publicConfigurationRevisionCodec) encodeOpaque(purpose []byte, parts ...string) (string, error) {
	if c == nil || len(purpose) == 0 {
		return "", gerror.New("public opaque token codec is unavailable")
	}
	mac := hmac.New(sha256.New, c.key[:])
	_, _ = mac.Write(purpose)
	for _, part := range parts {
		if part == "" {
			return "", gerror.New("public opaque token input is invalid")
		}
		_, _ = mac.Write([]byte{0})
		_, _ = mac.Write([]byte(part))
	}
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
