// Package smsbao implements the fixed SMSBao domestic HTTPS API v1.
package smsbao

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
)

const (
	ProviderCode   = "smsbao_http_v1"
	endpoint       = "https://api.smsbao.com/sms"
	signNameField  = "sign_name"
	productIDField = "product_id"
	usernameField  = "username"
	apiKeyField    = "api_key"
	maximumBody    = 4096
)

var (
	declaration struct {
		sync.Once
		err error
	}
	domesticPattern = regexp.MustCompile(`^\+861[0-9]{10}$`)
	productPattern  = regexp.MustCompile(`^[0-9]{1,32}$`)
)

func Declare() error {
	declaration.Do(func() {
		declaration.err = channelcap.Declare(channelcap.Descriptor{
			ProviderCode: ProviderCode,
			Kind:         channelcap.KindSMS,
			Fields: []channelcap.Field{
				{Key: signNameField, LabelKey: "notification.smsbao.sign_name", Required: true, MaxLength: 64},
				{Key: productIDField, LabelKey: "notification.smsbao.product_id", MaxLength: 32},
				{Key: usernameField, LabelKey: "notification.smsbao.username", Secret: true, Required: true, MaxLength: 128},
				{Key: apiKeyField, LabelKey: "notification.smsbao.api_key", Secret: true, Required: true, MaxLength: 256},
			},
			Factory: func(accessor secretcap.Accessor) (channelcap.Adapter, error) {
				return newAdapter(accessor, &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}}, Timeout: 20 * time.Second})
			},
		})
	})
	return declaration.err
}

type adapter struct {
	accessor secretcap.Accessor
	client   *http.Client
	purpose  secretcap.Purpose
}

func newAdapter(accessor secretcap.Accessor, client *http.Client) (*adapter, error) {
	if accessor == nil || client == nil {
		return nil, gerror.New("smsbao dependencies are unavailable")
	}
	purpose, err := secretcap.NewPurpose("notify.send")
	if err != nil {
		return nil, err
	}
	return &adapter{accessor: accessor, client: client, purpose: purpose}, nil
}

func (a *adapter) ValidateConfiguration(_ context.Context, nonSecret map[string]string, secrets map[string][]byte) error {
	signLength := utf8.RuneCountInString(strings.TrimSpace(nonSecret[signNameField]))
	productID := strings.TrimSpace(nonSecret[productIDField])
	if signLength < 3 || signLength > 10 || productID != "" && !productPattern.MatchString(productID) ||
		len(secrets[usernameField]) < 1 || len(secrets[usernameField]) > 128 ||
		len(secrets[apiKeyField]) < 8 || len(secrets[apiKeyField]) > 256 {
		return gerror.New("smsbao configuration is invalid")
	}
	return nil
}

func (a *adapter) ValidateDestination(destination []byte) error {
	if !domesticPattern.Match(destination) {
		return gerror.New("smsbao recipient is invalid")
	}
	return nil
}

func (a *adapter) Deliver(ctx context.Context, input channelcap.Delivery) (channelcap.Result, error) {
	if a.ValidateDestination(input.Destination) != nil {
		return smsResult(input.TaskNo, channelcap.ResultInvalidRecipient, false, "invalid_recipient"), nil
	}
	if input.ProviderTemplateID != "" || len(input.ProviderParameters) != 0 || len(input.ProviderData) != 0 ||
		strings.TrimSpace(input.Body) == "" || len(input.Body) > maximumBody {
		return smsResult(input.TaskNo, channelcap.ResultConfigurationError, false, "invalid_template_content"), nil
	}
	request, err := secretcap.NewAccessRequest(input.Credential, a.purpose)
	if err != nil {
		return smsResult(input.TaskNo, channelcap.ResultConfigurationError, false, "invalid_credential"), nil
	}
	result := smsResult(input.TaskNo, channelcap.ResultUnknown, false, "not_sent")
	accessErr := a.accessor.Access(ctx, request, func(plaintext []byte) error {
		var secrets map[string][]byte
		if json.Unmarshal(plaintext, &secrets) != nil {
			return errors.New("smsbao credential is invalid")
		}
		defer clearSecrets(secrets)
		username, apiKey := secrets[usernameField], secrets[apiKeyField]
		if len(username) == 0 || len(apiKey) < 8 {
			return errors.New("smsbao credential is unavailable")
		}
		result = a.send(ctx, input, string(username), string(apiKey))
		return nil
	})
	if accessErr != nil {
		return smsResult(input.TaskNo, channelcap.ResultConfigurationError, false, "credential_unavailable"), nil
	}
	return result, nil
}

func (a *adapter) Query(_ context.Context, input channelcap.Query) (channelcap.Result, error) {
	return smsResult(input.TaskNo, channelcap.ResultUnknown, true, "single_task_query_unavailable"), nil
}

func (a *adapter) send(ctx context.Context, input channelcap.Delivery, username, apiKey string) channelcap.Result {
	values := url.Values{}
	values.Set("u", username)
	values.Set("p", apiKey)
	values.Set("m", strings.TrimPrefix(string(input.Destination), "+86"))
	values.Set("c", "【"+input.NonSecretConfig[signNameField]+"】"+input.Body)
	if productID := strings.TrimSpace(input.NonSecretConfig[productIDField]); productID != "" {
		values.Set("g", productID)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+values.Encode(), http.NoBody)
	if err != nil {
		return smsResult(input.TaskNo, channelcap.ResultConfigurationError, false, "request_invalid")
	}
	response, err := a.client.Do(httpRequest)
	if err != nil {
		return smsResult(input.TaskNo, channelcap.ResultUnknown, true, "transport_unknown")
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 65))
	if readErr != nil || len(responseBody) > 64 || response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return smsResult(input.TaskNo, channelcap.ResultUnknown, true, "response_unknown")
	}
	code := strings.TrimSpace(string(responseBody))
	switch code {
	case "0":
		return smsResult(input.TaskNo, channelcap.ResultAccepted, true, "accepted")
	case "41":
		return smsResult(input.TaskNo, channelcap.ResultInsufficientBalance, true, code)
	case "51":
		return smsResult(input.TaskNo, channelcap.ResultInvalidRecipient, true, code)
	case "-1", "30", "40", "43", "50":
		return smsResult(input.TaskNo, channelcap.ResultConfigurationError, true, code)
	default:
		return smsResult(input.TaskNo, channelcap.ResultUnknown, true, code)
	}
}

func smsResult(taskNo string, class channelcap.ResultClass, crossed bool, evidence string) channelcap.Result {
	digest := sha256.Sum256([]byte("smsbao-v1\x00" + taskNo + "\x00" + string(class) + "\x00" + evidence))
	return channelcap.Result{Class: class, EvidenceDigest: hex.EncodeToString(digest[:]), CrossedExternalBorder: crossed}
}

func clearSecrets(values map[string][]byte) {
	for key, value := range values {
		clear(value)
		delete(values, key)
	}
}
