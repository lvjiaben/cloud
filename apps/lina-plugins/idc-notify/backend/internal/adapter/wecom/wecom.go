// Package wecom implements installation-owned custom-application messages and
// the official current-employee OAuth identity exchange.
package wecom

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
	ProviderCode   = "wecom_custom_app_v1"
	corpIDField    = "corp_id"
	agentIDField   = "agent_id"
	appSecretField = "app_secret"
	tokenEndpoint  = "https://qyapi.weixin.qq.com/cgi-bin/gettoken"
	identityURL    = "https://qyapi.weixin.qq.com/cgi-bin/auth/getuserinfo"
	sendEndpoint   = "https://qyapi.weixin.qq.com/cgi-bin/message/send"
	maximumBody    = 64 * 1024
)

var (
	declaration struct {
		sync.Once
		err error
	}
	corpIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)
	userIDPattern = regexp.MustCompile(`^[A-Za-z0-9_.@-]{1,64}$`)
)

// OAuthResolver exchanges one provider-issued, single-use authorization code
// for the current enterprise member identity. It never accepts a caller-entered
// UserID and returns no employee profile fields.
type OAuthResolver interface {
	ResolveOAuthUser(context.Context, secretcap.Handle, map[string]string, string) ([]byte, error)
}

func Declare() error {
	declaration.Do(func() {
		declaration.err = channelcap.Declare(channelcap.Descriptor{
			ProviderCode: ProviderCode, Kind: channelcap.KindWeChatWork,
			Fields: []channelcap.Field{
				{Key: corpIDField, LabelKey: "notification.wecom.corp_id", Required: true, MaxLength: 64},
				{Key: agentIDField, LabelKey: "notification.wecom.agent_id", Required: true, MaxLength: 20},
				{Key: appSecretField, LabelKey: "notification.wecom.app_secret", Secret: true, Required: true, MaxLength: 256},
			},
			Factory: func(accessor secretcap.Accessor) (channelcap.Adapter, error) {
				return New(accessor, &http.Client{Timeout: 20 * time.Second})
			},
		})
	})
	return declaration.err
}

type adapter struct {
	accessor        secretcap.Accessor
	client          *http.Client
	sendPurpose     secretcap.Purpose
	validatePurpose secretcap.Purpose
}

type tokenResponse struct {
	ErrCode     int    `json:"errcode"`
	AccessToken string `json:"access_token"`
}

type identityResponse struct {
	ErrCode int    `json:"errcode"`
	UserID  string `json:"userid"`
	OpenID  string `json:"openid"`
}

type sendResponse struct {
	ErrCode        int    `json:"errcode"`
	MessageID      string `json:"msgid"`
	InvalidUser    string `json:"invaliduser"`
	UnlicensedUser string `json:"unlicenseduser"`
}

// New creates the shared provider adapter used by delivery and OAuth binding.
func New(accessor secretcap.Accessor, client *http.Client) (*adapter, error) {
	if accessor == nil || client == nil {
		return nil, gerror.New("wecom dependencies are unavailable")
	}
	sendPurpose, err := secretcap.NewPurpose("notify.send")
	if err != nil {
		return nil, err
	}
	validatePurpose, err := secretcap.NewPurpose("notify.validate")
	if err != nil {
		return nil, err
	}
	return &adapter{accessor: accessor, client: client, sendPurpose: sendPurpose, validatePurpose: validatePurpose}, nil
}

func (a *adapter) ValidateConfiguration(_ context.Context, nonSecret map[string]string, secrets map[string][]byte) error {
	agentID, err := strconv.ParseInt(nonSecret[agentIDField], 10, 64)
	if !corpIDPattern.MatchString(nonSecret[corpIDField]) || err != nil || agentID <= 0 ||
		len(secrets[appSecretField]) < 16 || len(secrets[appSecretField]) > 256 {
		return gerror.New("wecom configuration is invalid")
	}
	return nil
}

func (a *adapter) ValidateDestination(destination []byte) error {
	if !userIDPattern.Match(destination) {
		return gerror.New("wecom recipient is invalid")
	}
	return nil
}

func (a *adapter) Deliver(ctx context.Context, input channelcap.Delivery) (channelcap.Result, error) {
	if a == nil || a.client == nil || a.accessor == nil || a.ValidateDestination(input.Destination) != nil ||
		!validPublicConfiguration(input.NonSecretConfig) || strings.TrimSpace(input.Body) == "" || len(input.Body) > maximumBody {
		return result(input.TaskNo, channelcap.ResultConfigurationError, false, "", "request_invalid"), nil
	}
	request, err := secretcap.NewAccessRequest(input.Credential, a.sendPurpose)
	if err != nil {
		return result(input.TaskNo, channelcap.ResultConfigurationError, false, "", "credential_invalid"), nil
	}
	current := result(input.TaskNo, channelcap.ResultConfigurationError, false, "", "credential_unavailable")
	err = a.accessor.Access(ctx, request, func(payload []byte) error {
		secrets, decodeErr := decodeSecrets(payload)
		if decodeErr != nil {
			return decodeErr
		}
		defer clearSecrets(secrets)
		current = a.send(ctx, input, secrets[appSecretField])
		return nil
	})
	if err != nil {
		return result(input.TaskNo, channelcap.ResultConfigurationError, false, "", "credential_unavailable"), nil
	}
	return current, nil
}

// ResolveOAuthUser consumes one provider code once. Transport or malformed
// responses fail closed; callers must issue a fresh state and code instead of
// retrying an outcome that may already have consumed the provider code.
func (a *adapter) ResolveOAuthUser(
	ctx context.Context,
	credential secretcap.Handle,
	configuration map[string]string,
	code string,
) ([]byte, error) {
	if a == nil || a.client == nil || a.accessor == nil || !validPublicConfiguration(configuration) ||
		code == "" || code != strings.TrimSpace(code) || len(code) > 512 {
		return nil, gerror.New("wecom OAuth request is invalid")
	}
	request, err := secretcap.NewAccessRequest(credential, a.validatePurpose)
	if err != nil {
		return nil, err
	}
	var userID []byte
	err = a.accessor.Access(ctx, request, func(payload []byte) error {
		secrets, decodeErr := decodeSecrets(payload)
		if decodeErr != nil {
			return decodeErr
		}
		defer clearSecrets(secrets)
		token, tokenErr := a.accessToken(ctx, configuration[corpIDField], secrets[appSecretField])
		if tokenErr != nil {
			return tokenErr
		}
		defer clear(token)
		resolved, resolveErr := a.resolveIdentity(ctx, token, code)
		if resolveErr != nil {
			clear(resolved)
			return resolveErr
		}
		userID = resolved
		return nil
	})
	if err != nil {
		clear(userID)
		return nil, gerror.Wrap(err, "resolve WeCom current member")
	}
	return userID, nil
}

func (a *adapter) Query(_ context.Context, input channelcap.Query) (channelcap.Result, error) {
	return result(input.TaskNo, channelcap.ResultUnknown, true, input.ProviderMessageRef, "query_unavailable"), nil
}

func (a *adapter) send(ctx context.Context, input channelcap.Delivery, appSecret []byte) channelcap.Result {
	token, err := a.accessToken(ctx, input.NonSecretConfig[corpIDField], appSecret)
	if err != nil {
		return result(input.TaskNo, channelcap.ResultDefinitiveNotReceivedTemporary, false, "", "token_unavailable")
	}
	defer clear(token)
	agentID, _ := strconv.ParseInt(input.NonSecretConfig[agentIDField], 10, 64)
	payload, err := json.Marshal(struct {
		ToUser  string `json:"touser"`
		MsgType string `json:"msgtype"`
		AgentID int64  `json:"agentid"`
		Text    struct {
			Content string `json:"content"`
		} `json:"text"`
	}{ToUser: string(input.Destination), MsgType: "text", AgentID: agentID,
		Text: struct {
			Content string `json:"content"`
		}{Content: input.Body}})
	if err != nil || len(payload) > maximumBody {
		return result(input.TaskNo, channelcap.ResultConfigurationError, false, "", "payload_invalid")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		sendEndpoint+"?access_token="+url.QueryEscape(string(token)), bytes.NewReader(payload))
	if err != nil {
		return result(input.TaskNo, channelcap.ResultConfigurationError, false, "", "send_request_invalid")
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := a.client.Do(request)
	if err != nil {
		return result(input.TaskNo, channelcap.ResultUnknown, true, "", "send_transport_unknown")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maximumBody+1))
	if err != nil || len(body) > maximumBody || response.StatusCode != http.StatusOK {
		return result(input.TaskNo, channelcap.ResultUnknown, true, "", "send_response_unknown")
	}
	var decoded sendResponse
	if json.Unmarshal(body, &decoded) != nil {
		return result(input.TaskNo, channelcap.ResultUnknown, true, "", "send_response_invalid")
	}
	if decoded.ErrCode == 0 && decoded.InvalidUser == "" && decoded.UnlicensedUser == "" {
		return result(input.TaskNo, channelcap.ResultAccepted, true, decoded.MessageID, "accepted")
	}
	if decoded.InvalidUser != "" || decoded.UnlicensedUser != "" || decoded.ErrCode == 81013 || decoded.ErrCode == 301002 {
		return result(input.TaskNo, channelcap.ResultInvalidRecipient, true, decoded.MessageID, strconv.Itoa(decoded.ErrCode))
	}
	switch decoded.ErrCode {
	case -1, 45009:
		return result(input.TaskNo, channelcap.ResultDefinitiveNotReceivedTemporary, true, decoded.MessageID, strconv.Itoa(decoded.ErrCode))
	case 40014, 42001, 48002, 60020:
		return result(input.TaskNo, channelcap.ResultConfigurationError, true, decoded.MessageID, strconv.Itoa(decoded.ErrCode))
	default:
		return result(input.TaskNo, channelcap.ResultUnknown, true, decoded.MessageID, strconv.Itoa(decoded.ErrCode))
	}
}

func (a *adapter) accessToken(ctx context.Context, corpID string, appSecret []byte) ([]byte, error) {
	if !corpIDPattern.MatchString(corpID) || len(appSecret) < 16 || len(appSecret) > 256 {
		return nil, errors.New("wecom credential is invalid")
	}
	endpoint := tokenEndpoint + "?corpid=" + url.QueryEscape(corpID) + "&corpsecret=" + url.QueryEscape(string(appSecret))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	response, err := a.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maximumBody+1))
	if err != nil || len(body) > maximumBody || response.StatusCode != http.StatusOK {
		return nil, errors.New("wecom token response is unavailable")
	}
	var decoded tokenResponse
	if json.Unmarshal(body, &decoded) != nil || decoded.ErrCode != 0 || decoded.AccessToken == "" || len(decoded.AccessToken) > 2048 {
		return nil, errors.New("wecom token was rejected")
	}
	return []byte(decoded.AccessToken), nil
}

func (a *adapter) resolveIdentity(ctx context.Context, token []byte, code string) ([]byte, error) {
	endpoint := identityURL + "?access_token=" + url.QueryEscape(string(token)) + "&code=" + url.QueryEscape(code)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	response, err := a.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maximumBody+1))
	if err != nil || len(body) > maximumBody || response.StatusCode != http.StatusOK {
		return nil, errors.New("wecom identity response is unavailable")
	}
	var decoded identityResponse
	if json.Unmarshal(body, &decoded) != nil || decoded.ErrCode != 0 || decoded.OpenID != "" || !userIDPattern.MatchString(decoded.UserID) {
		return nil, errors.New("wecom identity is not an enterprise member")
	}
	return []byte(decoded.UserID), nil
}

func validPublicConfiguration(configuration map[string]string) bool {
	agentID, err := strconv.ParseInt(configuration[agentIDField], 10, 64)
	return corpIDPattern.MatchString(configuration[corpIDField]) && err == nil && agentID > 0
}

func decodeSecrets(payload []byte) (map[string][]byte, error) {
	var secrets map[string][]byte
	if json.Unmarshal(payload, &secrets) != nil || len(secrets[appSecretField]) < 16 || len(secrets[appSecretField]) > 256 {
		clearSecrets(secrets)
		return nil, errors.New("wecom credential payload is invalid")
	}
	return secrets, nil
}

func result(taskNo string, class channelcap.ResultClass, crossed bool, reference, evidence string) channelcap.Result {
	digest := sha256.Sum256([]byte("wecom-custom-app-v1\x00" + taskNo + "\x00" + string(class) + "\x00" + reference + "\x00" + evidence))
	return channelcap.Result{Class: class, ProviderMessageRef: reference, EvidenceDigest: hex.EncodeToString(digest[:]), CrossedExternalBorder: crossed}
}

func clearSecrets(values map[string][]byte) {
	for key, value := range values {
		clear(value)
		delete(values, key)
	}
}
