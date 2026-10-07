// This file executes one host-owned supplier login and bounded request sequence.
// Endpoint, credentials and authentication tokens never cross the plugin bridge.

package productautomation

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
)

const maximumSupplierSessionFormBytes = 64 * 1024

type supplierSessionSecretResolver func(string, func([]byte) error) error

func (s *serviceImpl) runtimeOutboundSession(ctx context.Context, callerPluginID, contextID string, request supplierinterfacecap.OutboundSessionRequest) (*supplierinterfacecap.OutboundSessionResponse, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || s.outboundClientFactory == nil || supplierinterfacecap.ValidateOutboundSessionRequest(request) != nil {
		return nil, gerror.New("supplier runtime outbound session request is unavailable")
	}
	runtimeContext, ok := s.runtimeContexts.get(contextID, callerPluginID)
	if !ok || !containsString(runtimeContext.authorizedMethods, runtimeContext.method) {
		return nil, gerror.New("supplier runtime outbound session context is unavailable")
	}
	endpoint, ok := runtimeEndpoint(runtimeContext, request.EndpointField)
	if !ok {
		return nil, gerror.New("supplier runtime outbound session endpoint is unavailable")
	}
	client, cleanup, err := s.outboundClientFactory(endpoint)
	if err != nil || client == nil || cleanup == nil {
		return nil, gerror.New("supplier runtime outbound session client is unavailable")
	}
	defer cleanup()
	base, err := url.Parse(endpoint)
	if err != nil {
		return nil, gerror.New("supplier runtime outbound session endpoint is unavailable")
	}
	return executeSupplierOutboundSession(ctx, client, base, request, func(fieldKey string, consume func([]byte) error) error {
		return s.runtimeSecretAccess(ctx, callerPluginID, contextID, fieldKey, consume)
	})
}

func executeSupplierOutboundSession(ctx context.Context, client supplierOutboundDoer, base *url.URL, request supplierinterfacecap.OutboundSessionRequest, resolve supplierSessionSecretResolver) (*supplierinterfacecap.OutboundSessionResponse, error) {
	if ctx == nil || client == nil || base == nil || resolve == nil || supplierinterfacecap.ValidateOutboundSessionRequest(request) != nil {
		return nil, gerror.New("supplier outbound session is unavailable")
	}
	var sensitiveValues [][]byte
	defer func() {
		for _, value := range sensitiveValues {
			clear(value)
		}
	}()
	authBody, authSecrets, err := encodeSupplierSessionForm(request.Authentication.Form, request.Authentication.SecretFormFields, resolve)
	if err != nil {
		return nil, gerror.New("supplier outbound session credentials are unavailable")
	}
	sensitiveValues = append(sensitiveValues, authSecrets...)
	authContent, authStatus, err := executeSupplierSessionRequest(ctx, client, base, request.Authentication.Method, request.Authentication.Path, nil, authBody, "", nil, supplierinterfacecap.MaximumOutboundPayload)
	clear(authBody)
	if err != nil || authStatus < http.StatusOK || authStatus >= http.StatusMultipleChoices {
		clear(authContent)
		return nil, gerror.New("supplier outbound session authentication failed")
	}
	for _, secret := range sensitiveValues {
		if supplierSecretAppears(authContent, secret) {
			clear(authContent)
			return nil, gerror.New("supplier outbound session authentication response contains credential material")
		}
	}
	token, err := decodeSupplierSessionToken(authContent, request.Authentication.TokenSelector, request.Authentication.TokenField)
	clear(authContent)
	if err != nil {
		return nil, gerror.New("supplier outbound session authentication response is invalid")
	}
	sensitiveValues = append(sensitiveValues, token)
	result := &supplierinterfacecap.OutboundSessionResponse{Steps: make([]supplierinterfacecap.OutboundSessionStepResponse, 0, len(request.Steps))}
	remaining := supplierinterfacecap.MaximumOutboundPayload
	for _, step := range request.Steps {
		body, stepSecrets, encodeErr := encodeSupplierSessionForm(step.Form, step.SecretFormFields, resolve)
		if encodeErr != nil {
			clear(body)
			return nil, gerror.New("supplier outbound session step credentials are unavailable")
		}
		sensitiveValues = append(sensitiveValues, stepSecrets...)
		content, status, callErr := executeSupplierSessionRequest(ctx, client, base, step.Method, step.Path, step.Query, body, request.Authentication.TokenHeader, token, remaining)
		clear(body)
		if callErr != nil {
			clear(content)
			return nil, callErr
		}
		remaining -= len(content)
		for _, secret := range sensitiveValues {
			if supplierSecretAppears(content, secret) {
				clear(content)
				return nil, gerror.New("supplier outbound session response contains credential material")
			}
		}
		projected, projectErr := projectSupplierResponse(content, step.ResponseFields)
		clear(content)
		if projectErr != nil {
			return nil, projectErr
		}
		result.Steps = append(result.Steps, supplierinterfacecap.OutboundSessionStepResponse{StatusCode: status, Data: projected})
	}
	if supplierinterfacecap.ValidateOutboundSessionResponse(result) != nil {
		return nil, gerror.New("supplier outbound session response is invalid")
	}
	return result, nil
}

func executeSupplierSessionRequest(ctx context.Context, client supplierOutboundDoer, base *url.URL, method, path string, queryValues map[string]string, formBody []byte, tokenHeader string, token []byte, maximumBytes int) ([]byte, int, error) {
	target := *base
	target.Path = strings.TrimRight(base.Path, "/") + path
	query := target.Query()
	for key, value := range queryValues {
		query.Set(key, value)
	}
	target.RawQuery = query.Encode()
	body := io.Reader(http.NoBody)
	if len(formBody) > 0 {
		body = bytes.NewReader(formBody)
	}
	request, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, 0, gerror.New("supplier outbound session request is unavailable")
	}
	request.GetBody = nil
	request.Header.Set("Accept", "application/json")
	if len(formBody) > 0 {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if len(token) > 0 {
		request.Header.Set(tokenHeader, string(token))
		defer request.Header.Del(tokenHeader)
	}
	response, err := client.Do(request)
	if err != nil || response == nil || response.Body == nil {
		return nil, 0, gerror.New("supplier outbound session request failed")
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusMultipleChoices && response.StatusCode < http.StatusBadRequest {
		return nil, 0, gerror.New("supplier outbound session redirect is forbidden")
	}
	if maximumBytes < 1 {
		return nil, 0, gerror.New("supplier outbound session response budget is exhausted")
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, int64(maximumBytes)+1))
	if err != nil || len(content) == 0 || len(content) > maximumBytes || !json.Valid(content) {
		clear(content)
		return nil, 0, gerror.New("supplier outbound session response is invalid")
	}
	return content, response.StatusCode, nil
}

func encodeSupplierSessionForm(public, secret map[string]string, resolve supplierSessionSecretResolver) ([]byte, [][]byte, error) {
	keys := make([]string, 0, len(public)+len(secret))
	for key := range public {
		keys = append(keys, key)
	}
	for key := range secret {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var (
		buffer    bytes.Buffer
		sensitive = make([][]byte, 0, len(secret))
	)
	for index, key := range keys {
		if index > 0 {
			buffer.WriteByte('&')
		}
		appendSupplierFormValue(&buffer, []byte(key))
		buffer.WriteByte('=')
		if value, ok := public[key]; ok {
			appendSupplierFormValue(&buffer, []byte(value))
			continue
		}
		err := resolve(secret[key], func(value []byte) error {
			copied := append([]byte(nil), value...)
			sensitive = append(sensitive, copied)
			appendSupplierFormValue(&buffer, value)
			return nil
		})
		if err != nil {
			clear(buffer.Bytes())
			for _, value := range sensitive {
				clear(value)
			}
			return nil, nil, err
		}
	}
	if buffer.Len() > maximumSupplierSessionFormBytes {
		clear(buffer.Bytes())
		for _, value := range sensitive {
			clear(value)
		}
		return nil, nil, gerror.New("supplier outbound session form is too large")
	}
	result := append([]byte(nil), buffer.Bytes()...)
	clear(buffer.Bytes())
	return result, sensitive, nil
}

func appendSupplierFormValue(buffer *bytes.Buffer, value []byte) {
	const hexadecimal = "0123456789ABCDEF"
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || strings.ContainsRune("-_.~", rune(character)) {
			buffer.WriteByte(character)
		} else if character == ' ' {
			buffer.WriteByte('+')
		} else {
			buffer.WriteByte('%')
			buffer.WriteByte(hexadecimal[character>>4])
			buffer.WriteByte(hexadecimal[character&15])
		}
	}
}

func decodeSupplierSessionToken(content []byte, selector, field string) ([]byte, error) {
	raw := bytes.TrimSpace(content)
	if selector == supplierinterfacecap.OutboundTokenField {
		var object map[string]json.RawMessage
		if json.Unmarshal(raw, &object) != nil || len(object) > 128 {
			return nil, gerror.New("supplier outbound session token object is invalid")
		}
		raw = bytes.TrimSpace(object[field])
	}
	if len(raw) < 3 || raw[0] != '"' || raw[len(raw)-1] != '"' || len(raw) > 8194 {
		return nil, gerror.New("supplier outbound session token is invalid")
	}
	token := append([]byte(nil), raw[1:len(raw)-1]...)
	if len(token) == 0 || bytes.ContainsAny(token, "\\\r\n\x00") {
		clear(token)
		return nil, gerror.New("supplier outbound session token is invalid")
	}
	return token, nil
}
