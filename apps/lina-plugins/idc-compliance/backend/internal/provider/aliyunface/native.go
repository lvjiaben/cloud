// This file signs and executes the fixed Aliyun POP request entirely inside
// nested secretcap callbacks, then clears request and response byte buffers.

package aliyunface

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 -- Aliyun POP 2017-03-31 mandates HMAC-SHA1.
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	aliyunAction         = "ExecuteRequest"
	aliyunVersion        = "2017-03-31"
	aliyunService        = "fin_face_verify"
	aliyunBizCode        = "FACE"
	aliyunIdentityType   = "CERT_INFO"
	maximumResponseBytes = 256 << 10
)

type httpDoer interface {
	Do(request *http.Request) (*http.Response, error)
}

type nativeClient struct {
	endpoint      string
	accessor      secretcap.Accessor
	credential    secretcap.Handle
	httpClient    httpDoer
	clock         func() time.Time
	nonce         func() (string, error)
	accessPurpose secretcap.Purpose
}

type providerEnvelope struct {
	Code      json.RawMessage `json:"code"`
	RequestID string          `json:"requestId"`
	Data      providerData    `json:"data"`
}

type providerData struct {
	CertifyID  string `json:"certifyId"`
	CertifyURL string `json:"certifyUrl"`
	Passed     string `json:"passed"`
}

func newNativeClient(
	endpoint string,
	accessor secretcap.Accessor,
	credential secretcap.Handle,
	httpClient httpDoer,
	clock func() time.Time,
	nonce func() (string, error),
) (*nativeClient, error) {
	if endpoint == "" || !validHTTPSURL(endpoint) || validateDependency(accessor) != nil ||
		credential.IsZero() || validateDependency(httpClient) != nil ||
		clock == nil || nonce == nil {
		return nil, gerror.New("Aliyun face client dependencies are invalid")
	}
	purpose, err := secretcap.NewPurpose("compliance.aliyun-face-authentication")
	if err != nil {
		return nil, gerror.Wrap(err, "construct Aliyun face credential purpose")
	}
	return &nativeClient{
		endpoint: endpoint, accessor: accessor, credential: credential,
		httpClient: httpClient, clock: clock,
		nonce: nonce, accessPurpose: purpose,
	}, nil
}

func (c *nativeClient) Initiate(ctx context.Context, request InitiateRequest) (Initiation, error) {
	if err := validateInitiateRequest(request); err != nil {
		return Initiation{}, err
	}
	parameters := map[string]string{
		"method": "init", "sceneId": request.SceneID, "outerOrderNo": request.OuterOrderNo,
		"bizCode": aliyunBizCode, "identityType": aliyunIdentityType,
		"certType": request.CertType, "certName": strings.TrimSpace(request.CertName),
		"certNo": strings.TrimSpace(request.CertNo), "returnUrl": strings.TrimSpace(request.ReturnURL),
	}
	envelope, err := c.execute(ctx, parameters, true)
	clearStringMap(parameters)
	if err != nil {
		return Initiation{}, err
	}
	if envelope.Data.CertifyID == "" || len(envelope.Data.CertifyID) > 128 ||
		!validHTTPSURL(envelope.Data.CertifyURL) {
		return Initiation{}, &CallError{Code: ErrorInvalidResponse, Unknown: true}
	}
	return Initiation{CertifyID: envelope.Data.CertifyID, CertifyURL: envelope.Data.CertifyURL}, nil
}

func (c *nativeClient) Query(ctx context.Context, request QueryRequest) (QueryResult, error) {
	if err := validateQueryRequest(request); err != nil {
		return QueryResult{}, err
	}
	parameters := map[string]string{
		"method": "query", "sceneId": strings.TrimSpace(request.SceneID),
		"certifyId": strings.TrimSpace(request.CertifyID),
	}
	envelope, err := c.execute(ctx, parameters, false)
	clearStringMap(parameters)
	if err != nil {
		var callError *CallError
		if errors.As(err, &callError) &&
			(callError.ProviderCode == "407" || callError.ProviderCode == "408") {
			return QueryResult{State: QueryStateExpired}, nil
		}
		return QueryResult{}, err
	}
	switch envelope.Data.Passed {
	case "T":
		return QueryResult{State: QueryStatePassed}, nil
	case "F":
		return QueryResult{State: QueryStateNotPassed}, nil
	default:
		return QueryResult{}, &CallError{Code: ErrorInvalidResponse, Unknown: true}
	}
}

func (c *nativeClient) Probe(ctx context.Context, sceneID string) ProbeOutcome {
	if !validSceneID(strings.TrimSpace(sceneID)) || c == nil || ctx == nil || ctx.Err() != nil {
		return ProbeOutcomeUnavailable
	}
	nonce, err := c.nonce()
	if err != nil || nonce == "" {
		return ProbeOutcomeUnavailable
	}
	digest := sha256.Sum256([]byte(nonce))
	parameters := map[string]string{
		"method": "query", "sceneId": strings.TrimSpace(sceneID),
		"certifyId": hex.EncodeToString(digest[:16]),
	}
	_, err = c.execute(ctx, parameters, false)
	clearStringMap(parameters)
	if err == nil {
		return ProbeOutcomeReachable
	}
	var callError *CallError
	if !errors.As(err, &callError) {
		return ProbeOutcomeUnavailable
	}
	switch callError.ProviderCode {
	case "406", "407", "408":
		return ProbeOutcomeReachable
	case "401", "402", "403", "404", "InvalidAccessKeyId.NotFound",
		"InvalidAccessKeyId.Inactive", "SignatureDoesNotMatch", "UnauthorizedOperation":
		return ProbeOutcomeRejected
	default:
		if strings.HasPrefix(callError.ProviderCode, "5") {
			return ProbeOutcomeUnavailable
		}
		if callError.Code == ErrorProviderRejected {
			return ProbeOutcomeRejected
		}
		return ProbeOutcomeUnavailable
	}
}

func (c *nativeClient) execute(
	ctx context.Context,
	serviceParameters map[string]string,
	initialization bool,
) (providerEnvelope, error) {
	if c == nil || ctx == nil || ctx.Err() != nil {
		return providerEnvelope{}, &CallError{Code: ErrorInvalidRequest}
	}
	credentialRequest, err := secretcap.NewAccessRequest(c.credential, c.accessPurpose)
	if err != nil {
		return providerEnvelope{}, &CallError{Code: ErrorCredentialAccess}
	}
	var (
		envelope providerEnvelope
		callErr  error
	)
	err = c.accessor.Access(ctx, credentialRequest, func(credential []byte) error {
		var parsed credentialEnvelope
		if decodeErr := json.Unmarshal(credential, &parsed); decodeErr != nil {
			callErr = &CallError{Code: ErrorCredentialAccess}
			return nil
		}
		defer parsed.clear()
		envelope, callErr = c.executeWithCredentials(
			ctx, parsed.AccessKeyID, parsed.AccessKeySecret, serviceParameters, initialization,
		)
		return nil
	})
	if err != nil {
		return providerEnvelope{}, &CallError{Code: ErrorCredentialAccess}
	}
	return envelope, callErr
}

type credentialEnvelope struct {
	AccessKeyID     []byte `json:"accessKeyId"`
	AccessKeySecret []byte `json:"accessKeySecret"`
}

func (c *credentialEnvelope) clear() {
	if c == nil {
		return
	}
	clearBytes(c.AccessKeyID)
	clearBytes(c.AccessKeySecret)
}

func (c *nativeClient) executeWithCredentials(
	ctx context.Context,
	accessKeyID []byte,
	accessKeySecret []byte,
	serviceParameters map[string]string,
	initialization bool,
) (providerEnvelope, error) {
	if !validCredentialPart(accessKeyID, minimumAccessKeyIDBytes, maximumAccessKeyIDBytes) ||
		!validCredentialPart(accessKeySecret, minimumAccessKeySecretBytes, maximumAccessKeySecretBytes) {
		return providerEnvelope{}, &CallError{Code: ErrorCredentialAccess}
	}
	nonce, err := c.nonce()
	if err != nil || nonce == "" || len(nonce) > 128 {
		return providerEnvelope{}, &CallError{Code: ErrorResultUnknown, Unknown: initialization}
	}
	serviceJSON, err := json.Marshal(serviceParameters)
	if err != nil {
		return providerEnvelope{}, &CallError{Code: ErrorInvalidRequest}
	}
	defer clearBytes(serviceJSON)
	parameters := map[string]string{
		"AccessKeyId": string(accessKeyID), "Format": "JSON", "SignatureMethod": "HMAC-SHA1",
		"Timestamp":        c.clock().UTC().Format("2006-01-02T15:04:05Z"),
		"SignatureVersion": "1.0", "SignatureNonce": nonce, "Version": aliyunVersion,
		"Action": aliyunAction, "Service": aliyunService, "ServiceParameters": string(serviceJSON),
	}
	defer clearStringMap(parameters)
	parameters["Signature"] = signPOP(parameters, accessKeySecret)
	body := encodeForm(parameters)
	defer clearBytes(body)
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return providerEnvelope{}, &CallError{Code: ErrorResultUnknown, Unknown: initialization}
	}
	httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpRequest.Header.Set("Accept", "application/json")
	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		return providerEnvelope{}, &CallError{Code: ErrorResultUnknown, Unknown: initialization}
	}
	if response == nil || response.Body == nil {
		return providerEnvelope{}, &CallError{Code: ErrorResultUnknown, Unknown: initialization}
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			return
		}
	}()
	responseBytes, readErr := io.ReadAll(io.LimitReader(response.Body, maximumResponseBytes+1))
	if readErr != nil || len(responseBytes) > maximumResponseBytes {
		clearBytes(responseBytes)
		return providerEnvelope{}, &CallError{Code: ErrorResultUnknown, Unknown: initialization}
	}
	defer clearBytes(responseBytes)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var rejected providerEnvelope
		if json.Unmarshal(responseBytes, &rejected) == nil {
			providerCode := normalizedProviderCode(rejected.Code)
			if providerCode != "" {
				return providerEnvelope{}, &CallError{
					Code: ErrorProviderRejected, ProviderCode: providerCode,
				}
			}
		}
		return providerEnvelope{}, &CallError{Code: ErrorResultUnknown, Unknown: initialization}
	}
	var envelope providerEnvelope
	if err = json.Unmarshal(responseBytes, &envelope); err != nil {
		return providerEnvelope{}, &CallError{Code: ErrorInvalidResponse, Unknown: initialization}
	}
	providerCode := normalizedProviderCode(envelope.Code)
	if providerCode != "200" {
		unknown := initialization && (providerCode == "" || strings.HasPrefix(providerCode, "5"))
		code := ErrorProviderRejected
		if unknown {
			code = ErrorResultUnknown
		}
		return providerEnvelope{}, &CallError{Code: code, ProviderCode: providerCode, Unknown: unknown}
	}
	return envelope, nil
}

func signPOP(parameters map[string]string, accessKeySecret []byte) string {
	canonical := canonicalParameters(parameters)
	stringToSign := "POST&%2F&" + percentEncode(canonical)
	key := make([]byte, len(accessKeySecret)+1)
	copy(key, accessKeySecret)
	key[len(key)-1] = '&'
	defer clearBytes(key)
	mac := hmac.New(sha1.New, key) // #nosec G401 -- provider-mandated POP signature.
	if _, err := mac.Write([]byte(stringToSign)); err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func canonicalParameters(parameters map[string]string) string {
	keys := make([]string, 0, len(parameters))
	for key := range parameters {
		if key != "Signature" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var builder strings.Builder
	for index, key := range keys {
		if index > 0 {
			builder.WriteByte('&')
		}
		builder.WriteString(percentEncode(key))
		builder.WriteByte('=')
		builder.WriteString(percentEncode(parameters[key]))
	}
	return builder.String()
}

func encodeForm(parameters map[string]string) []byte {
	keys := make([]string, 0, len(parameters))
	for key := range parameters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for index, key := range keys {
		if index > 0 {
			builder.WriteByte('&')
		}
		builder.WriteString(percentEncode(key))
		builder.WriteByte('=')
		builder.WriteString(percentEncode(parameters[key]))
	}
	return []byte(builder.String())
}

func percentEncode(value string) string {
	return strings.ReplaceAll(
		strings.ReplaceAll(strings.ReplaceAll(url.QueryEscape(value), "+", "%20"), "*", "%2A"),
		"%7E", "~",
	)
}

func normalizedProviderCode(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if raw[0] == '"' {
		if json.Unmarshal(raw, &text) != nil {
			return ""
		}
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(string(raw))
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

func clearStringMap(value map[string]string) {
	for key := range value {
		value[key] = ""
		delete(value, key)
	}
}
