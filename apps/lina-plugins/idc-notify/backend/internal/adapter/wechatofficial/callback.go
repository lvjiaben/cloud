package wechatofficial

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
)

const maximumCallbackBody = 256 * 1024

type callbackXML struct {
	ToUserName string `xml:"ToUserName"`
	CreateTime int64  `xml:"CreateTime"`
	MsgType    string `xml:"MsgType"`
	Event      string `xml:"Event"`
	MsgID      string `xml:"MsgID"`
	Status     string `xml:"Status"`
}

func (a *adapter) VerifyChallenge(ctx context.Context, input channelcap.CallbackInput) (string, error) {
	plaintext, err := a.decryptCallback(ctx, input)
	if err != nil || len(plaintext) == 0 || len(plaintext) > 1024 {
		clear(plaintext)
		return "", errors.New("wechat official callback challenge is invalid")
	}
	defer clear(plaintext)
	return string(plaintext), nil
}

func (a *adapter) DecodeCallback(ctx context.Context, input channelcap.CallbackInput) (channelcap.CallbackEvent, error) {
	plaintext, err := a.decryptCallback(ctx, input)
	if err != nil {
		return channelcap.CallbackEvent{}, err
	}
	defer clear(plaintext)
	if len(plaintext) == 0 || len(plaintext) > maximumCallbackBody {
		return channelcap.CallbackEvent{}, errors.New("wechat official callback body is invalid")
	}
	decoder := xml.NewDecoder(io.LimitReader(bytes.NewReader(plaintext), maximumCallbackBody+1))
	decoder.Strict = true
	var event callbackXML
	if err = decoder.Decode(&event); err != nil || event.MsgType != "event" || event.Event != "TEMPLATESENDJOBFINISH" ||
		event.MsgID == "" || len(event.MsgID) > 256 || event.CreateTime <= 0 {
		return channelcap.CallbackEvent{}, errors.New("wechat official callback event is invalid")
	}
	class := callbackResultClass(event.Status)
	digest := sha256.Sum256([]byte("wechat-official-callback-v1\x00" + event.MsgID + "\x00" + strconv.FormatInt(event.CreateTime, 10) + "\x00" + string(class)))
	return channelcap.CallbackEvent{ProviderMessageRef: event.MsgID, OccurredAt: time.Unix(event.CreateTime, 0).UTC(),
		ResultClass: class, EvidenceDigest: hex.EncodeToString(digest[:])}, nil
}

func (a *adapter) decryptCallback(ctx context.Context, input channelcap.CallbackInput) ([]byte, error) {
	if a == nil || input.Signature == "" || input.Timestamp == "" || input.Nonce == "" || len(input.Encrypted) == 0 || len(input.Encrypted) > maximumCallbackBody {
		return nil, errors.New("wechat official callback envelope is invalid")
	}
	timestamp, err := strconv.ParseInt(input.Timestamp, 10, 64)
	if err != nil || timestamp <= 0 || absDuration(time.Since(time.Unix(timestamp, 0))) > 10*time.Minute {
		return nil, errors.New("wechat official callback timestamp is invalid")
	}
	request, err := secretcap.NewAccessRequest(input.Credential, a.validatePurpose)
	if err != nil {
		return nil, err
	}
	var plaintext []byte
	err = a.accessor.Access(ctx, request, func(payload []byte) error {
		var secrets map[string][]byte
		if err := json.Unmarshal(payload, &secrets); err != nil {
			return err
		}
		defer clearSecrets(secrets)
		token, aesKeyText := secrets[callbackTokenField], secrets[encodingAESKeyField]
		if !validCallbackSignature(token, input.Timestamp, input.Nonce, input.Encrypted, input.Signature) {
			return errors.New("wechat official callback signature is invalid")
		}
		key, err := base64.StdEncoding.DecodeString(string(aesKeyText) + "=")
		if err != nil || len(key) != 32 {
			clear(key)
			return errors.New("wechat official callback encryption key is invalid")
		}
		defer clear(key)
		plaintext, err = decryptMessage(key, input.Encrypted, input.Configuration[appIDField])
		return err
	})
	if err != nil {
		clear(plaintext)
		return nil, err
	}
	return plaintext, nil
}

func validCallbackSignature(token []byte, timestamp, nonce string, encrypted []byte, signature string) bool {
	if len(token) < 3 || len(signature) != sha1.Size*2 {
		return false
	}
	parts := []string{string(token), timestamp, nonce, string(encrypted)}
	sort.Strings(parts)
	digest := sha1.Sum([]byte(strings.Join(parts, "")))
	expected := hex.EncodeToString(digest[:])
	return subtle.ConstantTimeCompare([]byte(expected), []byte(strings.ToLower(signature))) == 1
}

func decryptMessage(key, encoded []byte, expectedAppID string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil || len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		clear(ciphertext)
		return nil, errors.New("wechat official callback ciphertext is invalid")
	}
	defer clear(ciphertext)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	decoded := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, key[:aes.BlockSize]).CryptBlocks(decoded, ciphertext)
	unpadded, err := unpad32(decoded)
	if err != nil || len(unpadded) < 20 {
		clear(decoded)
		return nil, errors.New("wechat official callback plaintext is invalid")
	}
	messageLength := int(binary.BigEndian.Uint32(unpadded[16:20]))
	if messageLength <= 0 || messageLength > maximumCallbackBody || 20+messageLength > len(unpadded) {
		clear(decoded)
		return nil, errors.New("wechat official callback message length is invalid")
	}
	appID := unpadded[20+messageLength:]
	if expectedAppID == "" || subtle.ConstantTimeCompare(appID, []byte(expectedAppID)) != 1 {
		clear(decoded)
		return nil, errors.New("wechat official callback application is invalid")
	}
	message := append([]byte(nil), unpadded[20:20+messageLength]...)
	clear(decoded)
	return message, nil
}

func unpad32(value []byte) ([]byte, error) {
	if len(value) == 0 {
		return nil, errors.New("empty padding")
	}
	padding := int(value[len(value)-1])
	if padding < 1 || padding > 32 || padding > len(value) {
		return nil, errors.New("invalid padding")
	}
	for _, item := range value[len(value)-padding:] {
		if int(item) != padding {
			return nil, errors.New("invalid padding")
		}
	}
	return value[:len(value)-padding], nil
}

func callbackResultClass(status string) channelcap.ResultClass {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success":
		return channelcap.ResultDelivered
	case "failed:user block":
		return channelcap.ResultInvalidRecipient
	case "failed:system failed":
		return channelcap.ResultDefinitiveNotReceivedTemporary
	case "failed:blocked":
		return channelcap.ResultDefinitiveNotReceivedPermanent
	default:
		return channelcap.ResultUnknown
	}
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}
