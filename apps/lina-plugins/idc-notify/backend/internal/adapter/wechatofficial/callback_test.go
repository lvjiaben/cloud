package wechatofficial

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
)

type callbackAccessor struct{ payload []byte }

func (a callbackAccessor) Access(_ context.Context, _ secretcap.AccessRequest, consume secretcap.Consumer) error {
	return consume(append([]byte(nil), a.payload...))
}

func TestEncryptedCallbackChallengeAndEvent(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	aesKey := strings.TrimSuffix(base64.StdEncoding.EncodeToString(key), "=")
	secrets, err := json.Marshal(map[string][]byte{callbackTokenField: []byte("callback-token"), encodingAESKeyField: []byte(aesKey)})
	if err != nil {
		t.Fatal(err)
	}
	adapter := &adapter{accessor: callbackAccessor{payload: secrets}}
	adapter.validatePurpose, err = secretcap.NewPurpose("notify.validate")
	if err != nil {
		t.Fatal(err)
	}
	handle, _ := secretcap.NewHandle("callback-test-handle")
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	challenge := encryptedFixture(t, key, []byte("challenge-value"), "wx_test_app")
	plain, err := adapter.VerifyChallenge(context.Background(), channelcap.CallbackInput{Credential: handle,
		Configuration: map[string]string{appIDField: "wx_test_app"}, Signature: signatureFixture("callback-token", timestamp, "nonce", challenge),
		Timestamp: timestamp, Nonce: "nonce", Encrypted: challenge})
	if err != nil || plain != "challenge-value" {
		t.Fatalf("challenge failed: %q %v", plain, err)
	}

	eventXML := []byte(`<xml><ToUserName><![CDATA[gh_test]]></ToUserName><CreateTime>` + timestamp + `</CreateTime><MsgType><![CDATA[event]]></MsgType><Event><![CDATA[TEMPLATESENDJOBFINISH]]></Event><MsgID>123456789</MsgID><Status><![CDATA[success]]></Status></xml>`)
	encrypted := encryptedFixture(t, key, eventXML, "wx_test_app")
	event, err := adapter.DecodeCallback(context.Background(), channelcap.CallbackInput{Credential: handle,
		Configuration: map[string]string{appIDField: "wx_test_app"}, Signature: signatureFixture("callback-token", timestamp, "nonce-2", encrypted),
		Timestamp: timestamp, Nonce: "nonce-2", Encrypted: encrypted})
	if err != nil {
		t.Fatal(err)
	}
	if event.ProviderMessageRef != "123456789" || event.ResultClass != channelcap.ResultDelivered || len(event.EvidenceDigest) != 64 {
		t.Fatalf("unexpected event: %+v", event)
	}

	if _, err = adapter.DecodeCallback(context.Background(), channelcap.CallbackInput{Credential: handle,
		Configuration: map[string]string{appIDField: "another_app"}, Signature: signatureFixture("callback-token", timestamp, "nonce-2", encrypted),
		Timestamp: timestamp, Nonce: "nonce-2", Encrypted: encrypted}); err == nil {
		t.Fatal("cross-app callback was accepted")
	}
}

func encryptedFixture(t *testing.T, key, message []byte, appID string) []byte {
	t.Helper()
	plain := make([]byte, 20+len(message)+len(appID))
	copy(plain[:16], []byte("0123456789abcdef"))
	binary.BigEndian.PutUint32(plain[16:20], uint32(len(message)))
	copy(plain[20:], message)
	copy(plain[20+len(message):], appID)
	padding := 32 - len(plain)%32
	for index := 0; index < padding; index++ {
		plain = append(plain, byte(padding))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, key[:aes.BlockSize]).CryptBlocks(ciphertext, plain)
	return []byte(base64.StdEncoding.EncodeToString(ciphertext))
}

func signatureFixture(token, timestamp, nonce string, encrypted []byte) string {
	parts := []string{token, timestamp, nonce, string(encrypted)}
	sort.Strings(parts)
	digest := sha1.Sum([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(digest[:])
}
