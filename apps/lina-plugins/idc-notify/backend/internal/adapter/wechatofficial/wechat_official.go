// Package wechatofficial implements verified-service-account template messages.
package wechatofficial

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
)

const (
	ProviderCode        = "wechat_official_template_v1"
	appIDField          = "app_id"
	appSecretField      = "app_secret"
	callbackTokenField  = "callback_token"
	encodingAESKeyField = "encoding_aes_key"
	tokenEndpoint       = "https://api.weixin.qq.com/cgi-bin/token"
	sendEndpoint        = "https://api.weixin.qq.com/cgi-bin/message/template/send"
	maximumBody         = 64 * 1024
)

var (
	declaration struct {
		sync.Once
		err error
	}
	appIDPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)
	openIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{10,128}$`)
)

func Declare() error {
	declaration.Do(func() {
		declaration.err = channelcap.Declare(channelcap.Descriptor{
			ProviderCode: ProviderCode, Kind: channelcap.KindWeChatOfficial,
			Fields: []channelcap.Field{
				{Key: appIDField, LabelKey: "notification.wechat_official.app_id", Required: true, MaxLength: 64},
				{Key: appSecretField, LabelKey: "notification.wechat_official.app_secret", Secret: true, Required: true, MaxLength: 256},
				{Key: callbackTokenField, LabelKey: "notification.wechat_official.callback_token", Secret: true, Required: true, MaxLength: 64},
				{Key: encodingAESKeyField, LabelKey: "notification.wechat_official.encoding_aes_key", Secret: true, Required: true, MaxLength: 43},
			},
			Factory: func(accessor secretcap.Accessor) (channelcap.Adapter, error) {
				return newAdapter(accessor, &http.Client{Timeout: 20 * time.Second})
			},
		})
	})
	return declaration.err
}

type adapter struct {
	accessor        secretcap.Accessor
	client          *http.Client
	purpose         secretcap.Purpose
	validatePurpose secretcap.Purpose
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ErrCode     int    `json:"errcode"`
}

type sendResponse struct {
	ErrCode int   `json:"errcode"`
	MsgID   int64 `json:"msgid"`
}

type templateValue struct {
	Value string `json:"value"`
}

func newAdapter(accessor secretcap.Accessor, client *http.Client) (*adapter, error) {
	if accessor == nil || client == nil {
		return nil, gerror.New("wechat official dependencies are unavailable")
	}
	purpose, err := secretcap.NewPurpose("notify.send")
	if err != nil {
		return nil, err
	}
	validatePurpose, err := secretcap.NewPurpose("notify.validate")
	if err != nil {
		return nil, err
	}
	return &adapter{accessor: accessor, client: client, purpose: purpose, validatePurpose: validatePurpose}, nil
}

func (a *adapter) ValidateConfiguration(_ context.Context, nonSecret map[string]string, secrets map[string][]byte) error {
	if !appIDPattern.MatchString(nonSecret[appIDField]) || len(secrets[appSecretField]) < 16 || len(secrets[appSecretField]) > 256 ||
		len(secrets[callbackTokenField]) < 3 || len(secrets[callbackTokenField]) > 64 || len(secrets[encodingAESKeyField]) != 43 {
		return gerror.New("wechat official configuration is invalid")
	}
	return nil
}

func (a *adapter) ValidateDestination(destination []byte) error {
	if !openIDPattern.Match(destination) {
		return gerror.New("wechat official recipient is invalid")
	}
	return nil
}

func (a *adapter) Deliver(ctx context.Context, input channelcap.Delivery) (channelcap.Result, error) {
	if err := a.ValidateDestination(input.Destination); err != nil {
		return result(input.TaskNo, channelcap.ResultInvalidRecipient, false, "", "invalid_recipient"), nil
	}
	if !appIDPattern.MatchString(input.NonSecretConfig[appIDField]) || input.ProviderTemplateID == "" || len(input.ProviderData) == 0 {
		return result(input.TaskNo, channelcap.ResultConfigurationError, false, "", "mapping_invalid"), nil
	}
	request, err := secretcap.NewAccessRequest(input.Credential, a.purpose)
	if err != nil {
		return result(input.TaskNo, channelcap.ResultConfigurationError, false, "", "credential_invalid"), nil
	}
	current := result(input.TaskNo, channelcap.ResultUnknown, false, "", "not_sent")
	accessErr := a.accessor.Access(ctx, request, func(plaintext []byte) error {
		var secrets map[string][]byte
		if err := json.Unmarshal(plaintext, &secrets); err != nil {
			return err
		}
		defer clearSecrets(secrets)
		secret := secrets[appSecretField]
		if len(secret) < 16 {
			return errors.New("wechat official credential is unavailable")
		}
		current = a.send(ctx, input, secret)
		return nil
	})
	if accessErr != nil {
		return result(input.TaskNo, channelcap.ResultConfigurationError, false, "", "credential_unavailable"), nil
	}
	return current, nil
}

func (a *adapter) Query(_ context.Context, input channelcap.Query) (channelcap.Result, error) {
	return result(input.TaskNo, channelcap.ResultUnknown, true, input.ProviderMessageRef, "callback_unavailable"), nil
}

func (a *adapter) send(ctx context.Context, input channelcap.Delivery, appSecret []byte) channelcap.Result {
	data := make(map[string]templateValue, len(input.ProviderData))
	for key, value := range input.ProviderData {
		if !validDataKey(key) || len(value) > 4096 {
			return result(input.TaskNo, channelcap.ResultConfigurationError, false, "", "mapping_invalid")
		}
		data[key] = templateValue{Value: value}
	}
	payload, err := json.Marshal(struct {
		ToUser     string                   `json:"touser"`
		TemplateID string                   `json:"template_id"`
		Data       map[string]templateValue `json:"data"`
	}{ToUser: string(input.Destination), TemplateID: input.ProviderTemplateID, Data: data})
	if err != nil || len(payload) > maximumBody {
		return result(input.TaskNo, channelcap.ResultConfigurationError, false, "", "payload_invalid")
	}
	tokenURL := tokenEndpoint + "?grant_type=client_credential&appid=" + url.QueryEscape(input.NonSecretConfig[appIDField]) + "&secret=" + url.QueryEscape(string(appSecret))
	tokenRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
	if err != nil {
		return result(input.TaskNo, channelcap.ResultConfigurationError, false, "", "token_request_invalid")
	}
	tokenHTTPResponse, err := a.client.Do(tokenRequest)
	if err != nil {
		return result(input.TaskNo, channelcap.ResultDefinitiveNotReceivedTemporary, true, "", "token_transport_failed")
	}
	defer tokenHTTPResponse.Body.Close()
	tokenBody, err := io.ReadAll(io.LimitReader(tokenHTTPResponse.Body, maximumBody+1))
	if err != nil || len(tokenBody) > maximumBody || tokenHTTPResponse.StatusCode != http.StatusOK {
		return result(input.TaskNo, channelcap.ResultDefinitiveNotReceivedTemporary, true, "", "token_response_failed")
	}
	var token tokenResponse
	if json.Unmarshal(tokenBody, &token) != nil || token.ErrCode != 0 || token.AccessToken == "" {
		return result(input.TaskNo, channelcap.ResultConfigurationError, true, "", "token_rejected")
	}
	sendRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, sendEndpoint+"?access_token="+url.QueryEscape(token.AccessToken), bytes.NewReader(payload))
	token.AccessToken = ""
	if err != nil {
		return result(input.TaskNo, channelcap.ResultConfigurationError, false, "", "send_request_invalid")
	}
	sendRequest.Header.Set("Content-Type", "application/json")
	sendHTTPResponse, err := a.client.Do(sendRequest)
	if err != nil {
		return result(input.TaskNo, channelcap.ResultUnknown, true, "", "send_transport_unknown")
	}
	defer sendHTTPResponse.Body.Close()
	body, err := io.ReadAll(io.LimitReader(sendHTTPResponse.Body, maximumBody+1))
	if err != nil || len(body) > maximumBody || sendHTTPResponse.StatusCode != http.StatusOK {
		return result(input.TaskNo, channelcap.ResultUnknown, true, "", "send_response_unknown")
	}
	var response sendResponse
	if json.Unmarshal(body, &response) != nil {
		return result(input.TaskNo, channelcap.ResultUnknown, true, "", "send_response_invalid")
	}
	if response.ErrCode == 0 && response.MsgID > 0 {
		accepted := result(input.TaskNo, channelcap.ResultAccepted, true, strconv.FormatInt(response.MsgID, 10), "accepted")
		accepted.QueryDeadlineSource = "wechat_encrypted_callback"
		return accepted
	}
	return classify(input.TaskNo, response.ErrCode)
}

func classify(taskNo string, code int) channelcap.Result {
	switch code {
	case -1, 45009, 45011:
		return result(taskNo, channelcap.ResultDefinitiveNotReceivedTemporary, true, "", strconv.Itoa(code))
	case 43004, 43005, 40003:
		return result(taskNo, channelcap.ResultInvalidRecipient, true, "", strconv.Itoa(code))
	case 40001, 40013, 40014, 40037, 40125:
		return result(taskNo, channelcap.ResultConfigurationError, true, "", strconv.Itoa(code))
	default:
		return result(taskNo, channelcap.ResultUnknown, true, "", strconv.Itoa(code))
	}
}

func result(taskNo string, class channelcap.ResultClass, crossed bool, reference, evidence string) channelcap.Result {
	digest := sha256.Sum256([]byte("wechat-official-template-v1\x00" + taskNo + "\x00" + string(class) + "\x00" + reference + "\x00" + evidence))
	return channelcap.Result{Class: class, ProviderMessageRef: reference, EvidenceDigest: hex.EncodeToString(digest[:]), CrossedExternalBorder: crossed}
}

func validDataKey(value string) bool {
	if value == "" || len(value) > 64 || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if (character < 'A' || character > 'Z') && (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') && character != '_' {
			return false
		}
	}
	return true
}

func clearSecrets(values map[string][]byte) {
	for key, value := range values {
		clear(value)
		delete(values, key)
	}
}
