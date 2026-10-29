// Package tencentcloudsms implements Tencent Cloud SMS API 2021-01-11.
package tencentcloudsms

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
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
	ProviderCode   = "tencent_cloud_sms_v20210111"
	endpoint       = "https://sms.tencentcloudapi.com/"
	host           = "sms.tencentcloudapi.com"
	apiVersion     = "2021-01-11"
	serviceName    = "sms"
	regionField    = "region"
	appIDField     = "sms_sdk_app_id"
	signNameField  = "sign_name"
	secretIDField  = "secret_id"
	secretKeyField = "secret_key"
	maximumBody    = 64 * 1024
)

var (
	declaration struct {
		sync.Once
		err error
	}
	e164Pattern   = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
	regionPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,31}$`)
	digitsPattern = regexp.MustCompile(`^[0-9]{5,20}$`)
)

// Declare registers the installation-owned Tencent Cloud SMS adapter once.
func Declare() error {
	declaration.Do(func() {
		declaration.err = channelcap.Declare(channelcap.Descriptor{
			ProviderCode: ProviderCode,
			Kind:         channelcap.KindSMS,
			Fields: []channelcap.Field{
				{Key: regionField, LabelKey: "notification.tencent_sms.region", Required: true, MaxLength: 32},
				{Key: appIDField, LabelKey: "notification.tencent_sms.app_id", Required: true, MaxLength: 20},
				{Key: signNameField, LabelKey: "notification.tencent_sms.sign_name", Required: true, MaxLength: 64},
				{Key: secretIDField, LabelKey: "notification.tencent_sms.secret_id", Secret: true, Required: true, MaxLength: 256},
				{Key: secretKeyField, LabelKey: "notification.tencent_sms.secret_key", Secret: true, Required: true, MaxLength: 256},
			},
			Factory: func(accessor secretcap.Accessor) (channelcap.Adapter, error) {
				return newAdapter(accessor, &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}}, Timeout: 20 * time.Second}, time.Now)
			},
		})
	})
	return declaration.err
}

type adapter struct {
	accessor secretcap.Accessor
	client   *http.Client
	now      func() time.Time
	purpose  secretcap.Purpose
}

type sendRequest struct {
	PhoneNumberSet []string `json:"PhoneNumberSet"`
	SmsSdkAppID    string   `json:"SmsSdkAppId"`
	SignName       string   `json:"SignName"`
	TemplateID     string   `json:"TemplateId"`
	TemplateParams []string `json:"TemplateParamSet"`
	SessionContext string   `json:"SessionContext,omitempty"`
}

type sendResponse struct {
	Response struct {
		Error *struct {
			Code string `json:"Code"`
		} `json:"Error"`
		RequestID     string `json:"RequestId"`
		SendStatusSet []struct {
			SerialNo string `json:"SerialNo"`
			Code     string `json:"Code"`
		} `json:"SendStatusSet"`
	} `json:"Response"`
}

func newAdapter(accessor secretcap.Accessor, client *http.Client, now func() time.Time) (*adapter, error) {
	if accessor == nil || client == nil || now == nil {
		return nil, gerror.New("tencent cloud sms dependencies are unavailable")
	}
	purpose, err := secretcap.NewPurpose("notify.send")
	if err != nil {
		return nil, err
	}
	return &adapter{accessor: accessor, client: client, now: now, purpose: purpose}, nil
}

func (a *adapter) ValidateConfiguration(_ context.Context, nonSecret map[string]string, secrets map[string][]byte) error {
	if !regionPattern.MatchString(nonSecret[regionField]) || !digitsPattern.MatchString(nonSecret[appIDField]) ||
		strings.TrimSpace(nonSecret[signNameField]) == "" || len(nonSecret[signNameField]) > 64 ||
		len(secrets[secretIDField]) < 8 || len(secrets[secretIDField]) > 256 ||
		len(secrets[secretKeyField]) < 16 || len(secrets[secretKeyField]) > 256 {
		return gerror.New("tencent cloud sms configuration is invalid")
	}
	return nil
}

func (a *adapter) ValidateDestination(destination []byte) error {
	if !e164Pattern.Match(destination) {
		return gerror.New("tencent cloud sms recipient is invalid")
	}
	return nil
}

func (a *adapter) Deliver(ctx context.Context, input channelcap.Delivery) (channelcap.Result, error) {
	if err := a.ValidateDestination(input.Destination); err != nil {
		return smsResult(input.TaskNo, channelcap.ResultInvalidRecipient, false, "", "invalid_recipient"), nil
	}
	if !regionPattern.MatchString(input.NonSecretConfig[regionField]) || !digitsPattern.MatchString(input.NonSecretConfig[appIDField]) ||
		input.ProviderTemplateID == "" || len(input.ProviderTemplateID) > 128 || len(input.ProviderParameters) > 64 {
		return smsResult(input.TaskNo, channelcap.ResultConfigurationError, false, "", "invalid_configuration"), nil
	}
	request, err := secretcap.NewAccessRequest(input.Credential, a.purpose)
	if err != nil {
		return smsResult(input.TaskNo, channelcap.ResultConfigurationError, false, "", "invalid_credential"), nil
	}
	result := smsResult(input.TaskNo, channelcap.ResultUnknown, false, "", "not_sent")
	accessErr := a.accessor.Access(ctx, request, func(plaintext []byte) error {
		var secrets map[string][]byte
		if err := json.Unmarshal(plaintext, &secrets); err != nil {
			return err
		}
		defer clearSecrets(secrets)
		secretID, secretKey := secrets[secretIDField], secrets[secretKeyField]
		if len(secretID) < 8 || len(secretKey) < 16 {
			return errors.New("tencent cloud sms credential is unavailable")
		}
		result = a.send(ctx, input, string(secretID), secretKey)
		return nil
	})
	if accessErr != nil {
		return smsResult(input.TaskNo, channelcap.ResultConfigurationError, false, "", "credential_unavailable"), nil
	}
	return result, nil
}

func (a *adapter) Query(_ context.Context, input channelcap.Query) (channelcap.Result, error) {
	// Tencent's pull API is an account-wide destructive queue and requires
	// separate activation. It cannot safely serve one task query here.
	return smsResult(input.TaskNo, channelcap.ResultUnknown, true, input.ProviderMessageRef, "query_not_configured"), nil
}

func (a *adapter) send(ctx context.Context, input channelcap.Delivery, secretID string, secretKey []byte) channelcap.Result {
	payload, err := json.Marshal(sendRequest{
		PhoneNumberSet: []string{string(input.Destination)}, SmsSdkAppID: input.NonSecretConfig[appIDField],
		SignName: input.NonSecretConfig[signNameField], TemplateID: input.ProviderTemplateID,
		TemplateParams: append([]string(nil), input.ProviderParameters...), SessionContext: input.TaskNo,
	})
	if err != nil || len(payload) > maximumBody {
		return smsResult(input.TaskNo, channelcap.ResultConfigurationError, false, "", "payload_invalid")
	}
	now := a.now().UTC()
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return smsResult(input.TaskNo, channelcap.ResultConfigurationError, false, "", "request_invalid")
	}
	httpRequest.Header.Set("Content-Type", "application/json; charset=utf-8")
	httpRequest.Header.Set("Host", host)
	httpRequest.Header.Set("X-Tc-Action", "SendSms")
	httpRequest.Header.Set("X-Tc-Version", apiVersion)
	httpRequest.Header.Set("X-Tc-Region", input.NonSecretConfig[regionField])
	httpRequest.Header.Set("X-Tc-Timestamp", strconv.FormatInt(now.Unix(), 10))
	httpRequest.Header.Set("Authorization", authorization(secretID, secretKey, payload, now))
	response, err := a.client.Do(httpRequest)
	if err != nil {
		return smsResult(input.TaskNo, channelcap.ResultUnknown, true, "", "transport_unknown")
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, maximumBody+1))
	if readErr != nil || len(responseBody) > maximumBody || response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return smsResult(input.TaskNo, channelcap.ResultUnknown, true, "", "response_unknown")
	}
	var decoded sendResponse
	if json.Unmarshal(responseBody, &decoded) != nil {
		return smsResult(input.TaskNo, channelcap.ResultUnknown, true, "", "response_invalid")
	}
	if decoded.Response.Error != nil {
		return classifyCode(input.TaskNo, decoded.Response.Error.Code, decoded.Response.RequestID)
	}
	if len(decoded.Response.SendStatusSet) != 1 {
		return smsResult(input.TaskNo, channelcap.ResultUnknown, true, decoded.Response.RequestID, "response_cardinality")
	}
	status := decoded.Response.SendStatusSet[0]
	if status.Code == "Ok" {
		return smsResult(input.TaskNo, channelcap.ResultAccepted, true, status.SerialNo, "accepted")
	}
	return classifyCode(input.TaskNo, status.Code, status.SerialNo)
}

func authorization(secretID string, secretKey, payload []byte, now time.Time) string {
	date := now.Format("2006-01-02")
	canonicalHeaders := "content-type:application/json; charset=utf-8\nhost:" + host + "\nx-tc-action:sendsms\n"
	signedHeaders := "content-type;host;x-tc-action"
	canonicalRequest := "POST\n/\n\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + sha256Hex(payload)
	credentialScope := date + "/" + serviceName + "/tc3_request"
	stringToSign := "TC3-HMAC-SHA256\n" + strconv.FormatInt(now.Unix(), 10) + "\n" + credentialScope + "\n" + sha256Hex([]byte(canonicalRequest))
	secretDate := hmacSHA256(append([]byte("TC3"), secretKey...), []byte(date))
	secretService := hmacSHA256(secretDate, []byte(serviceName))
	secretSigning := hmacSHA256(secretService, []byte("tc3_request"))
	signature := hex.EncodeToString(hmacSHA256(secretSigning, []byte(stringToSign)))
	clear(secretDate)
	clear(secretService)
	clear(secretSigning)
	return "TC3-HMAC-SHA256 Credential=" + secretID + "/" + credentialScope + ", SignedHeaders=" + signedHeaders + ", Signature=" + signature
}

func classifyCode(taskNo, code, reference string) channelcap.Result {
	switch {
	case code == "FailedOperation.InsufficientBalanceInSmsPackage", code == "UnauthorizedOperation.ServiceSuspendDueToArrears":
		return smsResult(taskNo, channelcap.ResultInsufficientBalance, true, reference, code)
	case code == "InvalidParameterValue.IncorrectPhoneNumber", code == "FailedOperation.PhoneNumberInBlacklist":
		return smsResult(taskNo, channelcap.ResultInvalidRecipient, true, reference, code)
	case strings.HasPrefix(code, "LimitExceeded."):
		return smsResult(taskNo, channelcap.ResultDefinitiveNotReceivedTemporary, true, reference, code)
	case strings.HasPrefix(code, "FailedOperation.Template"), strings.HasPrefix(code, "FailedOperation.Signature"),
		strings.HasPrefix(code, "InvalidParameterValue."), strings.HasPrefix(code, "UnauthorizedOperation."),
		code == "FailedOperation.ContainSensitiveWord":
		return smsResult(taskNo, channelcap.ResultConfigurationError, true, reference, code)
	case strings.HasPrefix(code, "InternalError."):
		return smsResult(taskNo, channelcap.ResultUnknown, true, reference, code)
	default:
		return smsResult(taskNo, channelcap.ResultUnknown, true, reference, code)
	}
}

func smsResult(taskNo string, class channelcap.ResultClass, crossed bool, reference, evidence string) channelcap.Result {
	digest := sha256.Sum256([]byte("tencent-sms-v20210111\x00" + taskNo + "\x00" + string(class) + "\x00" + reference + "\x00" + evidence))
	return channelcap.Result{Class: class, ProviderMessageRef: reference, EvidenceDigest: hex.EncodeToString(digest[:]), CrossedExternalBorder: crossed}
}

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func hmacSHA256(key, value []byte) []byte {
	hash := hmac.New(sha256.New, key)
	_, _ = hash.Write(value)
	return hash.Sum(nil)
}

func clearSecrets(values map[string][]byte) {
	for key, value := range values {
		clear(value)
		delete(values, key)
	}
}
