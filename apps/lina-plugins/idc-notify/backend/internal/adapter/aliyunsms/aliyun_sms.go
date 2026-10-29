// Package aliyunsms implements Aliyun SMS SendSms 2017-05-25 with ACS3 signing.
package aliyunsms

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
)

const (
	ProviderCode         = "aliyun_sms_v20170525"
	endpoint             = "https://dysmsapi.aliyuncs.com/"
	host                 = "dysmsapi.aliyuncs.com"
	apiVersion           = "2017-05-25"
	action               = "SendSms"
	signNameField        = "sign_name"
	accessKeyIDField     = "access_key_id"
	accessKeySecretField = "access_key_secret"
	maximumResponse      = 64 * 1024
)

var (
	declaration struct {
		sync.Once
		err error
	}
	e164Pattern     = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
	templatePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

func Declare() error {
	declaration.Do(func() {
		declaration.err = channelcap.Declare(channelcap.Descriptor{
			ProviderCode: ProviderCode,
			Kind:         channelcap.KindSMS,
			Fields: []channelcap.Field{
				{Key: signNameField, LabelKey: "notification.aliyun_sms.sign_name", Required: true, MaxLength: 64},
				{Key: accessKeyIDField, LabelKey: "notification.aliyun_sms.access_key_id", Secret: true, Required: true, MaxLength: 256},
				{Key: accessKeySecretField, LabelKey: "notification.aliyun_sms.access_key_secret", Secret: true, Required: true, MaxLength: 256},
			},
			Factory: func(accessor secretcap.Accessor) (channelcap.Adapter, error) {
				return newAdapter(accessor, &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}}, Timeout: 20 * time.Second}, time.Now, randomNonce)
			},
		})
	})
	return declaration.err
}

type adapter struct {
	accessor secretcap.Accessor
	client   *http.Client
	now      func() time.Time
	nonce    func() (string, error)
	purpose  secretcap.Purpose
}

type sendResponse struct {
	Code      string `json:"Code"`
	BizID     string `json:"BizId"`
	RequestID string `json:"RequestId"`
}

func newAdapter(accessor secretcap.Accessor, client *http.Client, now func() time.Time, nonce func() (string, error)) (*adapter, error) {
	if accessor == nil || client == nil || now == nil || nonce == nil {
		return nil, gerror.New("aliyun sms dependencies are unavailable")
	}
	purpose, err := secretcap.NewPurpose("notify.send")
	if err != nil {
		return nil, err
	}
	return &adapter{accessor: accessor, client: client, now: now, nonce: nonce, purpose: purpose}, nil
}

func (a *adapter) ValidateConfiguration(_ context.Context, nonSecret map[string]string, secrets map[string][]byte) error {
	if strings.TrimSpace(nonSecret[signNameField]) == "" || len(nonSecret[signNameField]) > 64 ||
		len(secrets[accessKeyIDField]) < 8 || len(secrets[accessKeyIDField]) > 256 ||
		len(secrets[accessKeySecretField]) < 16 || len(secrets[accessKeySecretField]) > 256 {
		return gerror.New("aliyun sms configuration is invalid")
	}
	return nil
}

func (a *adapter) ValidateDestination(destination []byte) error {
	if !e164Pattern.Match(destination) {
		return gerror.New("aliyun sms recipient is invalid")
	}
	return nil
}

func (a *adapter) Deliver(ctx context.Context, input channelcap.Delivery) (channelcap.Result, error) {
	if a.ValidateDestination(input.Destination) != nil {
		return smsResult(input.TaskNo, channelcap.ResultInvalidRecipient, false, "", "invalid_recipient"), nil
	}
	if !templatePattern.MatchString(input.ProviderTemplateID) || len(input.ProviderData) > 64 ||
		strings.TrimSpace(input.NonSecretConfig[signNameField]) == "" {
		return smsResult(input.TaskNo, channelcap.ResultConfigurationError, false, "", "invalid_configuration"), nil
	}
	request, err := secretcap.NewAccessRequest(input.Credential, a.purpose)
	if err != nil {
		return smsResult(input.TaskNo, channelcap.ResultConfigurationError, false, "", "invalid_credential"), nil
	}
	result := smsResult(input.TaskNo, channelcap.ResultUnknown, false, "", "not_sent")
	accessErr := a.accessor.Access(ctx, request, func(plaintext []byte) error {
		var secrets map[string][]byte
		if json.Unmarshal(plaintext, &secrets) != nil {
			return errors.New("aliyun sms credential is invalid")
		}
		defer clearSecrets(secrets)
		accessKeyID, accessKeySecret := secrets[accessKeyIDField], secrets[accessKeySecretField]
		if len(accessKeyID) < 8 || len(accessKeySecret) < 16 {
			return errors.New("aliyun sms credential is unavailable")
		}
		result = a.send(ctx, input, string(accessKeyID), accessKeySecret)
		return nil
	})
	if accessErr != nil {
		return smsResult(input.TaskNo, channelcap.ResultConfigurationError, false, "", "credential_unavailable"), nil
	}
	return result, nil
}

func (a *adapter) Query(_ context.Context, input channelcap.Query) (channelcap.Result, error) {
	return smsResult(input.TaskNo, channelcap.ResultUnknown, true, input.ProviderMessageRef, "single_task_query_unavailable"), nil
}

func (a *adapter) send(ctx context.Context, input channelcap.Delivery, accessKeyID string, accessKeySecret []byte) channelcap.Result {
	templateParameters, err := json.Marshal(input.ProviderData)
	if err != nil {
		return smsResult(input.TaskNo, channelcap.ResultConfigurationError, false, "", "template_parameters_invalid")
	}
	query := map[string]string{
		"PhoneNumbers":  string(input.Destination),
		"SignName":      input.NonSecretConfig[signNameField],
		"TemplateCode":  input.ProviderTemplateID,
		"TemplateParam": string(templateParameters),
		"OutId":         input.TaskNo,
	}
	nonce, err := a.nonce()
	if err != nil {
		return smsResult(input.TaskNo, channelcap.ResultConfigurationError, false, "", "nonce_unavailable")
	}
	date := a.now().UTC().Format("2006-01-02T15:04:05Z")
	canonicalQuery := canonicalQueryString(query)
	emptyPayloadHash := sha256Hex(nil)
	canonicalHeaders := "host:" + host + "\n" +
		"x-acs-action:" + action + "\n" +
		"x-acs-content-sha256:" + emptyPayloadHash + "\n" +
		"x-acs-date:" + date + "\n" +
		"x-acs-signature-nonce:" + nonce + "\n" +
		"x-acs-version:" + apiVersion + "\n"
	signedHeaders := "host;x-acs-action;x-acs-content-sha256;x-acs-date;x-acs-signature-nonce;x-acs-version"
	canonicalRequest := "POST\n/\n" + canonicalQuery + "\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + emptyPayloadHash
	stringToSign := "ACS3-HMAC-SHA256\n" + sha256Hex([]byte(canonicalRequest))
	signature := hmacSHA256Hex(accessKeySecret, []byte(stringToSign))

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"?"+canonicalQuery, http.NoBody)
	if err != nil {
		return smsResult(input.TaskNo, channelcap.ResultConfigurationError, false, "", "request_invalid")
	}
	httpRequest.Host = host
	httpRequest.Header.Set("X-Acs-Action", action)
	httpRequest.Header.Set("X-Acs-Content-Sha256", emptyPayloadHash)
	httpRequest.Header.Set("X-Acs-Date", date)
	httpRequest.Header.Set("X-Acs-Signature-Nonce", nonce)
	httpRequest.Header.Set("X-Acs-Version", apiVersion)
	httpRequest.Header.Set("Authorization", "ACS3-HMAC-SHA256 Credential="+accessKeyID+",SignedHeaders="+signedHeaders+",Signature="+signature)
	response, err := a.client.Do(httpRequest)
	if err != nil {
		return smsResult(input.TaskNo, channelcap.ResultUnknown, true, "", "transport_unknown")
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, maximumResponse+1))
	if readErr != nil || len(responseBody) > maximumResponse {
		return smsResult(input.TaskNo, channelcap.ResultUnknown, true, "", "response_unknown")
	}
	var decoded sendResponse
	if json.Unmarshal(responseBody, &decoded) != nil {
		return smsResult(input.TaskNo, channelcap.ResultUnknown, true, "", "response_invalid")
	}
	reference := decoded.BizID
	if reference == "" {
		reference = decoded.RequestID
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return classifyCode(input.TaskNo, decoded.Code, reference)
	}
	if decoded.Code == "OK" {
		return smsResult(input.TaskNo, channelcap.ResultAccepted, true, reference, "accepted")
	}
	return classifyCode(input.TaskNo, decoded.Code, reference)
}

func classifyCode(taskNo, code, reference string) channelcap.Result {
	upper := strings.ToUpper(code)
	switch {
	case strings.Contains(upper, "MOBILE_NUMBER_ILLEGAL"), strings.Contains(upper, "BLACK_KEY_CONTROL_LIMIT"):
		return smsResult(taskNo, channelcap.ResultInvalidRecipient, true, reference, code)
	case strings.Contains(upper, "AMOUNT_NOT_ENOUGH"), strings.Contains(upper, "BALANCE"):
		return smsResult(taskNo, channelcap.ResultInsufficientBalance, true, reference, code)
	case strings.Contains(upper, "THROTTLING"), strings.Contains(upper, "BUSINESS_LIMIT_CONTROL"):
		return smsResult(taskNo, channelcap.ResultDefinitiveNotReceivedTemporary, true, reference, code)
	case strings.Contains(upper, "TEMPLATE"), strings.Contains(upper, "SIGN_NAME"), strings.Contains(upper, "SIGNATURE"),
		strings.Contains(upper, "ACCESSKEY"), strings.Contains(upper, "FORBIDDEN"), strings.Contains(upper, "INVALIDPARAMETER"):
		return smsResult(taskNo, channelcap.ResultConfigurationError, true, reference, code)
	default:
		return smsResult(taskNo, channelcap.ResultUnknown, true, reference, code)
	}
}

func canonicalQueryString(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, rfc3986(key)+"="+rfc3986(values[key]))
	}
	return strings.Join(parts, "&")
}

func rfc3986(value string) string {
	encoded := url.QueryEscape(value)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}

func randomNonce() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func smsResult(taskNo string, class channelcap.ResultClass, crossed bool, reference, evidence string) channelcap.Result {
	digest := sha256.Sum256([]byte("aliyun-sms-v20170525\x00" + taskNo + "\x00" + string(class) + "\x00" + reference + "\x00" + evidence))
	return channelcap.Result{Class: class, ProviderMessageRef: reference, EvidenceDigest: hex.EncodeToString(digest[:]), CrossedExternalBorder: crossed}
}

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func hmacSHA256Hex(key, value []byte) string {
	hash := hmac.New(sha256.New, key)
	_, _ = hash.Write(value)
	return hex.EncodeToString(hash.Sum(nil))
}

func clearSecrets(values map[string][]byte) {
	for key, value := range values {
		clear(value)
		delete(values, key)
	}
}
