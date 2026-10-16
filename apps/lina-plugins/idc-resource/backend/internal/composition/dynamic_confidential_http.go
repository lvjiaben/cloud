// This file owns the host-confidential HTTP boundary for managed dynamic Providers.

package composition

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
	"lina-plugin-idc-resource/backend/internal/service/channelinvocation"
)

const maximumConfidentialHTTPResponse = 1 << 20

var errAuthenticationRefreshRequired = errors.New("resource Provider authentication refresh required")

var confidentialHTTPFieldPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)

var forbiddenConfidentialHTTPFields = map[string]struct{}{
	"address": {}, "apikey": {}, "accesskey": {}, "accesstoken": {}, "auth": {},
	"authentication": {}, "authorization": {}, "cookie": {}, "credential": {}, "credentials": {},
	"endpoint": {}, "host": {}, "hostname": {}, "key": {}, "managementaddress": {},
	"managementurl": {}, "osuser": {}, "passwd": {}, "password": {}, "passwordhash": {},
	"path": {}, "privatekey": {}, "publickey": {}, "refreshtoken": {}, "rootpassword": {},
	"secret": {}, "secretkey": {}, "sessiontoken": {}, "token": {}, "uri": {}, "url": {},
	"username": {},
}

type confidentialHTTPHandler struct {
	configuration            channelconfig.Configuration
	environment              resourcecap.ProviderEnv
	operationSecrets         []resourcecap.SecretBinding
	login                    *resourcecap.ConfidentialHTTPRequest
	forceFreshAuthentication bool
}

func NewProductionDynamicOutboundFactory(secretSource secretcap.Source, leases ...*channelinvocation.Service) DynamicOutboundFactory {
	return func(ctx context.Context, configuration channelconfig.Configuration, method string, operationSecrets []resourcecap.SecretBinding) (pluginhost.ConfidentialOutboundHandler, func(), error) {
		var invocationLease *channelinvocation.Lease
		settings, settingsErr := decodeChannelAuthenticationSettings(configuration.Config.Settings)
		if settingsErr == nil && settings.SerializeInvocations && method == resourcecap.DynamicProviderMethodExecute {
			if len(leases) != 1 || leases[0] == nil {
				return nil, nil, gerror.New("resource channel invocation serialization is unavailable")
			}
			var leaseErr error
			invocationLease, leaseErr = leases[0].Acquire(ctx, configuration.Config.ChannelRef)
			if leaseErr != nil {
				return nil, nil, leaseErr
			}
		}
		environment, cleanup, err := productionProviderEnvironment(ctx, configuration, secretSource)
		if err != nil {
			if invocationLease != nil {
				_ = invocationLease.Release(context.WithoutCancel(ctx))
			}
			return nil, nil, err
		}
		handler := &confidentialHTTPHandler{configuration: configuration, environment: environment, operationSecrets: append([]resourcecap.SecretBinding(nil), operationSecrets...), forceFreshAuthentication: method == resourcecap.DynamicProviderMethodVerifyConnection}
		return handler.Execute, func() {
			cleanup()
			if invocationLease != nil {
				releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				defer cancel()
				_ = invocationLease.Release(releaseCtx)
			}
		}, nil
	}
}

func (h *confidentialHTTPHandler) Execute(ctx context.Context, input *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
	if h == nil || input == nil || input.Sequence == 0 || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("confidential Provider HTTP step is invalid")
	}
	var request resourcecap.ConfidentialHTTPRequest
	decoder := json.NewDecoder(bytes.NewReader(input.Payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || decoder.Decode(&struct{}{}) != io.EOF || normalizeConfidentialSecretFormBindings(&request) != nil || validateConfidentialHTTPRequest(request) != nil {
		return nil, gerror.New("confidential Provider HTTP step is invalid")
	}
	if err := h.validateConfidentialOperationSlots(request); err != nil {
		return nil, err
	}
	if slices.Contains(request.ResponseFields, "key") && h.configuration.PluginID != "idc-peer" {
		return nil, gerror.New("confidential schema projection is not authorized")
	}
	switch request.Kind {
	case resourcecap.ConfidentialHTTPKindAuthenticate:
		if err := h.authenticate(ctx, request); err != nil {
			h.login = nil
			return nil, err
		}
		return marshalConfidentialHTTPResponse(resourcecap.ConfidentialHTTPResponse{StatusCode: http.StatusOK, Authenticated: true})
	case resourcecap.ConfidentialHTTPKindRequest:
		var response *resourcecap.ConfidentialHTTPResponse
		consume := func(token []byte, generation int64) error {
			var callErr error
			response, callErr = h.performBusiness(ctx, request, token)
			if callErr == nil && confidentialPlaintextAppears(response.Data, token) {
				return gerror.New("confidential Provider HTTP response contains authentication material")
			}
			if callErr == nil {
				callErr = h.rejectConfiguredSecretEchoes(ctx, response.Data)
			}
			if callErr == nil && generation > 0 && h.requiresAuthenticationRefresh(response.Data) {
				if h.environment.AuthenticationSessions.Invalidate(ctx, resourcecap.AuthenticationSessionInvalidation{
					ChannelRef: h.configuration.Config.ChannelRef, Generation: generation,
				}) != nil {
					return gerror.New("confidential Provider authentication refresh failed")
				}
				return errAuthenticationRefreshRequired
			}
			return callErr
		}
		settings, settingsErr := decodeChannelAuthenticationSettings(h.configuration.Config.Settings)
		var err error
		if settingsErr == nil && (settings.Strategy == "per_request_form" || settings.Strategy == "per_request_headers" || settings.Strategy == "bt_panel_double_md5" || settings.Strategy == "per_request_basic") {
			err = consume(nil, 0)
		} else {
			err = h.accessSession(ctx, consume)
			if errors.Is(err, errAuthenticationRefreshRequired) {
				err = h.accessSession(ctx, func(token []byte, generation int64) error {
					callErr := consume(token, generation)
					if errors.Is(callErr, errAuthenticationRefreshRequired) {
						return bizerr.NewCode(resourcecap.CodeProviderAuthRejected)
					}
					return callErr
				})
			}
		}
		if err != nil {
			return nil, err
		}
		return marshalConfidentialHTTPResponse(*response)
	default:
		return nil, gerror.New("confidential Provider HTTP step kind is invalid")
	}
}

func (h *confidentialHTTPHandler) performBusiness(ctx context.Context, request resourcecap.ConfidentialHTTPRequest, token []byte) (*resourcecap.ConfidentialHTTPResponse, error) {
	var response *resourcecap.ConfidentialHTTPResponse
	if err := h.withChannelAuthentication(ctx, &request, func() error {
		var callErr error
		response, callErr = h.performBusinessWithOperationSecrets(ctx, request, token)
		return callErr
	}); err != nil {
		return nil, err
	}
	return response, nil
}

func (h *confidentialHTTPHandler) performBusinessWithOperationSecrets(ctx context.Context, request resourcecap.ConfidentialHTTPRequest, token []byte) (*resourcecap.ConfidentialHTTPResponse, error) {
	fields := sortedMapKeys(request.SecretFormFields)
	var response *resourcecap.ConfidentialHTTPResponse
	var visit func(int) error
	visit = func(index int) error {
		if index == len(fields) {
			var err error
			response, err = h.perform(ctx, request, token)
			return err
		}
		upstreamField := fields[index]
		handle, ok := confidentialBinding(h.operationSecrets, request.SecretFormFields[upstreamField])
		if !ok {
			return bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
		}
		purpose, err := secretcap.NewPurpose("provider.operation-secret")
		if err != nil {
			return err
		}
		access, err := secretcap.NewAccessRequest(handle, purpose)
		if err != nil {
			return err
		}
		return accessConfidentialProviderSecret(ctx, h.environment.Secrets, access, func(value []byte) error {
			if request.Form == nil {
				request.Form = make(map[string]string)
			}
			request.Form[upstreamField] = string(value)
			defer delete(request.Form, upstreamField)
			if err := visit(index + 1); err != nil {
				return err
			}
			if response != nil && confidentialPlaintextAppears(response.Data, value) {
				return gerror.New("confidential Provider HTTP response contains operation material")
			}
			return nil
		})
	}
	if err := visit(0); err != nil {
		return nil, err
	}
	if response != nil {
		if requestToken := request.Form["request_token"]; requestToken != "" && confidentialPlaintextAppears(response.Data, []byte(requestToken)) {
			return nil, gerror.New("confidential Provider HTTP response contains derived authentication material")
		}
	}
	return response, nil
}

type channelAuthenticationSettings struct {
	Strategy             string            `json:"authStrategy"`
	Fields               map[string]string `json:"authFields"`
	TokenJSONPath        []string          `json:"authTokenJSONPath"`
	TokenHeader          string            `json:"authTokenHeader"`
	TokenScheme          string            `json:"authTokenScheme"`
	SerializeInvocations bool              `json:"serializeInvocations"`
	RefreshJSONPath      []string          `json:"authRefreshJSONPath"`
	RefreshJSONValues    []string          `json:"authRefreshJSONValues"`
	SessionTTLSeconds    int               `json:"authSessionTTLSeconds"`
	LoginPath            string            `json:"authLoginPath"`
}

func decodeChannelAuthenticationSettings(payload json.RawMessage) (channelAuthenticationSettings, error) {
	profile, err := resourcecap.DecodeHostAuthenticationSettings(payload)
	if err != nil {
		return channelAuthenticationSettings{}, err
	}
	return channelAuthenticationSettings{
		Strategy: profile.Strategy, Fields: profile.Fields, TokenJSONPath: profile.TokenJSONPath,
		TokenHeader: profile.TokenHeader, TokenScheme: profile.TokenScheme,
		SerializeInvocations: profile.SerializeInvocations, RefreshJSONPath: profile.RefreshJSONPath,
		RefreshJSONValues: profile.RefreshJSONValues, SessionTTLSeconds: profile.SessionTTLSeconds,
		LoginPath: profile.LoginPath,
	}, nil
}

var allowedAuthenticationTokenHeaders = map[string]struct{}{
	"Authorization": {}, "Access-Token": {}, "X-Auth-Token": {},
}

func (h *confidentialHTTPHandler) withChannelAuthentication(
	ctx context.Context,
	request *resourcecap.ConfidentialHTTPRequest,
	action func() error,
) error {
	settings, settingsErr := decodeChannelAuthenticationSettings(h.configuration.Config.Settings)
	if len(h.configuration.Config.Settings) == 0 || settingsErr != nil || settings.Strategy == "" {
		return action()
	}
	if settings.Strategy == "login_session" {
		return action()
	}
	if settings.Strategy == "per_request_basic" {
		return h.withBasicAuthentication(ctx, request, settings, action)
	}
	if settings.Strategy == "bt_panel_double_md5" {
		return h.withBTPanelAuthentication(ctx, request, settings, action)
	}
	if (settings.Strategy != "per_request_form" && settings.Strategy != "per_request_headers") ||
		len(settings.Fields) == 0 || len(settings.Fields) > 8 {
		return bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
	}
	fields := sortedMapKeys(settings.Fields)
	var visit func(int) error
	visit = func(index int) error {
		if index == len(fields) {
			return action()
		}
		upstreamField := fields[index]
		if !confidentialHTTPFieldPattern.MatchString(strings.ReplaceAll(upstreamField, "-", "_")) {
			return bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
		}
		handle, ok := confidentialBinding(h.configuration.Config.SecretBindings, settings.Fields[upstreamField])
		if !ok {
			return bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
		}
		purpose, err := secretcap.NewPurpose("provider.channel-authentication")
		if err != nil {
			return err
		}
		access, err := secretcap.NewAccessRequest(handle, purpose)
		if err != nil {
			return err
		}
		return accessConfidentialProviderSecret(ctx, h.environment.Secrets, access, func(value []byte) error {
			switch settings.Strategy {
			case "per_request_form":
				if request.Form == nil {
					request.Form = make(map[string]string)
				}
				if _, exists := request.Form[upstreamField]; exists {
					return bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
				}
				request.Form[upstreamField] = string(value)
				defer delete(request.Form, upstreamField)
			case "per_request_headers":
				if request.Headers == nil {
					request.Headers = make(map[string]string)
				}
				request.Headers[upstreamField] = string(value)
				defer delete(request.Headers, upstreamField)
			}
			return visit(index + 1)
		})
	}
	return visit(0)
}

func (h *confidentialHTTPHandler) withBTPanelAuthentication(
	ctx context.Context,
	request *resourcecap.ConfidentialHTTPRequest,
	settings channelAuthenticationSettings,
	action func() error,
) error {
	if request.Method != http.MethodPost || len(settings.Fields) != 1 || settings.Fields["material"] == "" {
		return bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
	}
	if h.environment.Secrets == nil {
		return bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
	}
	if request.Form == nil {
		request.Form = make(map[string]string)
	}
	if _, exists := request.Form["request_time"]; exists {
		return gerror.New("resource channel BT Panel request time is host-owned")
	}
	if _, exists := request.Form["request_token"]; exists {
		return gerror.New("resource channel BT Panel request token is host-owned")
	}
	handle, ok := confidentialBinding(h.configuration.Config.SecretBindings, settings.Fields["material"])
	if !ok {
		return bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
	}
	purpose, err := secretcap.NewPurpose("provider.channel-authentication")
	if err != nil {
		return err
	}
	access, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return err
	}
	return accessConfidentialProviderSecret(ctx, h.environment.Secrets, access, func(value []byte) error {
		requestTime := strconv.FormatInt(time.Now().Unix(), 10)
		inner := md5.Sum(value)
		innerHex := make([]byte, hex.EncodedLen(len(inner)))
		hex.Encode(innerHex, inner[:])
		signatureInput := append([]byte(requestTime), innerHex...)
		outer := md5.Sum(signatureInput)
		requestToken := make([]byte, hex.EncodedLen(len(outer)))
		hex.Encode(requestToken, outer[:])
		clear(innerHex)
		clear(signatureInput)
		request.Form["request_time"] = requestTime
		request.Form["request_token"] = string(requestToken)
		clear(requestToken)
		defer delete(request.Form, "request_time")
		defer delete(request.Form, "request_token")
		return action()
	})
}

func (h *confidentialHTTPHandler) rejectConfiguredSecretEchoes(ctx context.Context, content []byte) error {
	if len(h.configuration.Config.SecretBindings) == 0 {
		return nil
	}
	if h.environment.Secrets == nil {
		return bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
	}
	purpose, err := secretcap.NewPurpose("provider.response-screening")
	if err != nil {
		return gerror.New("confidential Provider response screening is unavailable")
	}
	for _, binding := range h.configuration.Config.SecretBindings {
		access, accessErr := secretcap.NewAccessRequest(binding.Handle, purpose)
		if accessErr != nil {
			return gerror.New("confidential Provider response screening is unavailable")
		}
		accessErr = accessConfidentialProviderSecret(ctx, h.environment.Secrets, access, func(value []byte) error {
			if confidentialPlaintextAppears(content, value) {
				return gerror.New("confidential Provider HTTP response contains credential material")
			}
			return nil
		})
		if accessErr != nil {
			return gerror.New("confidential Provider HTTP response screening failed")
		}
	}
	return nil
}

func confidentialPlaintextAppears(content, secret []byte) bool {
	if len(content) == 0 || len(secret) == 0 || !json.Valid(content) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return false
	}
	plain := string(secret)
	var appears func(any) bool
	appears = func(candidate any) bool {
		switch typed := candidate.(type) {
		case string:
			return typed == plain || (len(secret) >= 8 && strings.Contains(typed, plain))
		case []any:
			for _, item := range typed {
				if appears(item) {
					return true
				}
			}
		case map[string]any:
			for _, item := range typed {
				if appears(item) {
					return true
				}
			}
		}
		return false
	}
	return appears(value)
}

// authenticate bypasses shared session caches for explicit connection tests.
// The probe token is cleared without replacing or invalidating business sessions.
func (h *confidentialHTTPHandler) authenticate(ctx context.Context, request resourcecap.ConfidentialHTTPRequest) error {
	h.login = &request
	if h.forceFreshAuthentication {
		token, err := h.performLogin(ctx, request)
		clear(token)
		return err
	}
	return h.accessSession(ctx, func([]byte, int64) error { return nil })
}

func (h *confidentialHTTPHandler) accessSession(ctx context.Context, consume resourcecap.AuthenticationTokenConsumer) error {
	if h.login == nil || h.environment.AuthenticationSessions == nil {
		return bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
	}
	settings, err := h.authenticationSettings()
	if err != nil || settings.Strategy != "login_session" {
		return bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
	}
	return h.environment.AuthenticationSessions.Access(ctx, resourcecap.AuthenticationSessionRequest{
		ChannelRef: h.configuration.Config.ChannelRef,
		TTL:        time.Duration(settings.SessionTTLSeconds) * time.Second,
	}, func(store resourcecap.SecretConsumer) error {
		response, err := h.performLogin(ctx, *h.login)
		if err != nil {
			return err
		}
		defer clear(response)
		return store(response)
	}, consume)
}

func (h *confidentialHTTPHandler) performLogin(ctx context.Context, request resourcecap.ConfidentialHTTPRequest) ([]byte, error) {
	settings, err := h.authenticationSettings()
	canonicalPath := confidentialCanonicalLoginPath(h.configuration.Config.Endpoint, settings.LoginPath)
	if err != nil || settings.Strategy != "login_session" ||
		(request.Path != settings.LoginPath && request.Path != canonicalPath) ||
		(len(request.CredentialFields) > 0 && !equalStringMap(request.CredentialFields, settings.Fields)) {
		return nil, bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
	}
	// The owner already holds the frozen credential mapping. Guests may omit
	// it so password-named fields never cross the governed JSON boundary.
	request.CredentialFields = settings.Fields
	// Older frozen declarations contain an absolute API prefix. The configured
	// endpoint already owns that API root; dispatch its exact relative tail.
	request.Path = canonicalPath
	values := url.Values{}
	fields := sortedMapKeys(request.CredentialFields)
	var tokenResult []byte
	var visit func(int) error
	visit = func(index int) error {
		if index == len(fields) {
			response, err := h.perform(ctx, resourcecap.ConfidentialHTTPRequest{
				Kind: resourcecap.ConfidentialHTTPKindRequest, Method: request.Method, Path: request.Path,
				Form: firstValues(values),
			}, nil)
			if err != nil {
				return err
			}
			if response.StatusCode == http.StatusBadRequest {
				return bizerr.NewCode(resourcecap.CodeProviderAuthRejected)
			}
			if response.StatusCode >= http.StatusInternalServerError || response.StatusCode == http.StatusTooManyRequests {
				return bizerr.NewCode(resourcecap.CodeProviderNetwork)
			}
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				return bizerr.NewCode(resourcecap.CodeProviderInvalidResponse)
			}
			token, tokenErr := extractAuthenticationToken(response.Data, settings.TokenJSONPath)
			if tokenErr != nil {
				return confidentialAuthenticationResponseError(response.Data)
			}
			tokenResult = []byte(token)
			return nil
		}
		field := fields[index]
		handle, ok := confidentialBinding(h.configuration.Config.SecretBindings, request.CredentialFields[field])
		if !ok {
			return bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
		}
		purpose, err := secretcap.NewPurpose("provider.authentication")
		if err != nil {
			return err
		}
		access, err := secretcap.NewAccessRequest(handle, purpose)
		if err != nil {
			return err
		}
		return accessConfidentialProviderSecret(ctx, h.environment.Secrets, access, func(value []byte) error {
			values.Set(field, string(value))
			defer values.Del(field)
			return visit(index + 1)
		})
	}
	if err := visit(0); err != nil {
		return nil, err
	}
	return tokenResult, nil
}

func (h *confidentialHTTPHandler) perform(ctx context.Context, request resourcecap.ConfidentialHTTPRequest, token []byte) (*resourcecap.ConfidentialHTTPResponse, error) {
	base, err := url.Parse(h.configuration.Config.Endpoint)
	if err != nil {
		return nil, err
	}
	target := *base
	target.Path = strings.TrimRight(base.Path, "/") + request.Path
	query := target.Query()
	for key, values := range request.Query {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	target.RawQuery = query.Encode()
	body := url.Values{}
	for key, value := range request.Form {
		body.Set(key, value)
	}
	var requestBody io.Reader = strings.NewReader(body.Encode())
	if len(request.JSONBody) > 0 {
		requestBody = bytes.NewReader(request.JSONBody)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, request.Method, target.String(), requestBody)
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Accept", "application/json")
	for key, value := range request.Headers {
		httpRequest.Header.Set(key, value)
	}
	if len(body) > 0 {
		httpRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if len(request.JSONBody) > 0 {
		httpRequest.Header.Set("Content-Type", "application/json")
	}
	if len(token) > 0 {
		settings, settingsErr := h.authenticationSettings()
		if settingsErr != nil || settings.Strategy != "login_session" {
			return nil, bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
		}
		value := string(token)
		if settings.TokenScheme != "" {
			value = settings.TokenScheme + " " + value
		}
		httpRequest.Header.Set(settings.TokenHeader, value)
		defer httpRequest.Header.Del(settings.TokenHeader)
	}
	response, err := h.environment.HTTPClient.Do(httpRequest)
	if err != nil {
		return nil, confidentialTransportError(err)
	}
	if response == nil || response.Body == nil {
		return nil, bizerr.NewCode(resourcecap.CodeProviderInvalidResponse)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, bizerr.NewCode(resourcecap.CodeProviderEndpointNotFound)
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, bizerr.NewCode(resourcecap.CodeProviderAuthRejected)
	}
	if response.StatusCode >= http.StatusMultipleChoices && response.StatusCode < http.StatusBadRequest {
		return nil, bizerr.NewCode(resourcecap.CodeProviderInvalidResponse)
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, maximumConfidentialHTTPResponse+1))
	if err != nil {
		clear(content)
		return nil, confidentialTransportError(err)
	}
	if len(content) > maximumConfidentialHTTPResponse || !json.Valid(content) {
		clear(content)
		return nil, bizerr.NewCode(resourcecap.CodeProviderInvalidResponse)
	}
	if slices.Contains(request.ResponseFields, "key") && validatePeerImportSchemaKeys(request, content) != nil {
		clear(content)
		return nil, gerror.New("confidential schema field key is invalid")
	}
	for name, value := range request.Headers {
		if strings.EqualFold(name, "Authorization") && confidentialPlaintextAppears(content, []byte(value)) {
			clear(content)
			return nil, gerror.New("confidential Provider HTTP response contains authentication material")
		}
	}
	// An empty allowlist is reachable only from the host-internal login call;
	// guest-originated business requests are rejected without one.
	if len(request.ResponseFields) == 0 {
		return &resourcecap.ConfidentialHTTPResponse{StatusCode: response.StatusCode, Data: content}, nil
	}
	projected, err := projectConfidentialHTTPResponse(content, request.ResponseFields)
	clear(content)
	if err != nil {
		return nil, bizerr.NewCode(resourcecap.CodeProviderInvalidResponse)
	}
	return &resourcecap.ConfidentialHTTPResponse{StatusCode: response.StatusCode, Data: projected}, nil
}

func (h *confidentialHTTPHandler) authenticationSettings() (channelAuthenticationSettings, error) {
	if h == nil {
		return channelAuthenticationSettings{}, bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
	}
	settings, err := decodeChannelAuthenticationSettings(h.configuration.Config.Settings)
	if err != nil {
		return settings, bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
	}
	if settings.Strategy != "login_session" {
		return settings, nil
	}
	if len(settings.TokenJSONPath) == 0 || len(settings.TokenJSONPath) > 4 {
		return settings, bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
	}
	for index, part := range settings.TokenJSONPath {
		if part == "$" && len(settings.TokenJSONPath) == 1 && index == 0 {
			continue
		}
		if !confidentialHTTPFieldPattern.MatchString(part) {
			return settings, bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
		}
	}
	if _, ok := allowedAuthenticationTokenHeaders[settings.TokenHeader]; !ok {
		return settings, bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
	}
	if settings.TokenScheme != "" && settings.TokenScheme != "Bearer" {
		return settings, bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
	}
	if settings.SessionTTLSeconds < 60 || settings.SessionTTLSeconds > 3600 {
		return settings, bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
	}
	if (len(settings.RefreshJSONPath) == 0) != (len(settings.RefreshJSONValues) == 0) ||
		len(settings.RefreshJSONPath) > 4 || len(settings.RefreshJSONValues) > 8 {
		return settings, bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
	}
	for _, part := range settings.RefreshJSONPath {
		if !confidentialHTTPFieldPattern.MatchString(part) {
			return settings, bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
		}
	}
	for _, value := range settings.RefreshJSONValues {
		if value == "" || len(value) > 64 || strings.ContainsAny(value, "\r\n\x00") {
			return settings, bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
		}
	}
	return settings, nil
}

func equalStringMap(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func extractAuthenticationToken(content []byte, path []string) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return "", gerror.New("authentication response is invalid")
	}
	if len(path) == 1 && path[0] == "$" {
		// The root must be a JSON string; no object field is guessed.
	} else {
		for _, part := range path {
			object, ok := value.(map[string]any)
			if !ok {
				return "", gerror.New("authentication token path is unavailable")
			}
			value, ok = object[part]
			if !ok {
				return "", gerror.New("authentication token path is unavailable")
			}
		}
	}
	token, ok := value.(string)
	if !ok || token == "" || token != strings.TrimSpace(token) || len(token) > 8192 || strings.ContainsAny(token, "\r\n\x00") {
		return "", gerror.New("authentication token is invalid")
	}
	return token, nil
}

func (h *confidentialHTTPHandler) requiresAuthenticationRefresh(content []byte) bool {
	settings, err := h.authenticationSettings()
	if err != nil || settings.Strategy != "login_session" ||
		len(settings.RefreshJSONPath) == 0 || len(settings.RefreshJSONPath) > 4 ||
		len(settings.RefreshJSONValues) == 0 || len(settings.RefreshJSONValues) > 8 {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return false
	}
	for _, part := range settings.RefreshJSONPath {
		if !confidentialHTTPFieldPattern.MatchString(part) {
			return false
		}
		object, ok := value.(map[string]any)
		if !ok {
			return false
		}
		value, ok = object[part]
		if !ok {
			return false
		}
	}
	var actual string
	switch typed := value.(type) {
	case string:
		actual = typed
	case json.Number:
		actual = typed.String()
	default:
		return false
	}
	for _, expected := range settings.RefreshJSONValues {
		if expected == actual {
			return true
		}
	}
	return false
}

func validateConfidentialHTTPRequest(request resourcecap.ConfidentialHTTPRequest) error {
	if err := validateConfidentialJSONBody(request); err != nil {
		return err
	}
	if request.Path == "" || request.Path[0] != '/' || strings.Contains(request.Path, "..") || strings.ContainsAny(request.Path, "?#\\") || len(request.Path) > 1024 {
		return gerror.New("confidential Provider path is invalid")
	}
	if request.Method != http.MethodGet && request.Method != http.MethodPost && request.Method != http.MethodPut && request.Method != http.MethodPatch && request.Method != http.MethodDelete {
		return gerror.New("confidential Provider method is invalid")
	}
	if len(request.Query) > 32 || len(request.Form) > 128 || len(request.CredentialFields) > 8 || len(request.SecretFormFields) > 8 || len(request.ResponseFields) > 128 {
		return gerror.New("confidential Provider HTTP fields exceed the limit")
	}
	if request.Kind == resourcecap.ConfidentialHTTPKindAuthenticate {
		if request.Method != http.MethodPost || len(request.SecretFormFields) != 0 || len(request.ResponseFields) != 0 || request.UseAuthentication {
			return gerror.New("confidential Provider authentication step is invalid")
		}
	} else if request.Kind == resourcecap.ConfidentialHTTPKindRequest {
		if len(request.CredentialFields) != 0 || len(request.ResponseFields) == 0 || !request.UseAuthentication {
			return gerror.New("confidential Provider request step is invalid")
		}
	}
	fields := request.ResponseFields
	if peerImportSchemaRequest(request) {
		fields = slices.DeleteFunc(slices.Clone(fields), func(field string) bool { return field == "key" })
	}
	if err := validateConfidentialResponseFields(fields); err != nil {
		return err
	}
	for upstreamField, slotName := range request.SecretFormFields {
		if upstreamField == "" || slotName == "" || len(upstreamField) > 128 || len(slotName) > 128 {
			return gerror.New("confidential Provider operation secret field is invalid")
		}
		if _, exists := request.Form[upstreamField]; exists {
			return gerror.New("confidential Provider operation secret field conflicts with public form")
		}
	}
	return nil
}

// validateConfidentialResponseFields limits projection to explicitly allowed public fields.
func validateConfidentialResponseFields(fields []string) error {
	for _, field := range fields {
		normalized := strings.ReplaceAll(strings.ToLower(field), "_", "")
		if !confidentialHTTPFieldPattern.MatchString(field) {
			return gerror.New("confidential Provider response field is invalid")
		}
		if _, forbidden := forbiddenConfidentialHTTPFields[normalized]; forbidden {
			return gerror.New("confidential Provider response field is forbidden")
		}
	}
	return nil
}

func validateConfidentialJSONBody(request resourcecap.ConfidentialHTTPRequest) error {
	if len(request.JSONBody) > 0 {
		if len(request.JSONBody) > maximumConfidentialHTTPResponse || len(request.Form) > 0 || len(request.SecretFormFields) > 0 || len(request.CredentialFields) > 0 || request.Kind != resourcecap.ConfidentialHTTPKindRequest || request.Method == http.MethodGet {
			return gerror.New("confidential Provider JSON body is invalid")
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(request.JSONBody, &object) != nil || object == nil || len(object) > 128 {
			return gerror.New("confidential Provider JSON object is invalid")
		}
	}
	return nil
}

func projectConfidentialHTTPResponse(content []byte, fields []string) (json.RawMessage, error) {
	allowed := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		allowed[field] = struct{}{}
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return nil, bizerr.NewCode(resourcecap.CodeProviderInvalidResponse)
	}
	if !boundedConfidentialHTTPValue(value, allowed, 0) {
		return nil, gerror.New("confidential Provider HTTP response exceeds structural limits")
	}
	projected, ok := projectConfidentialHTTPValue(value, allowed, 0)
	if !ok {
		return nil, gerror.New("confidential Provider HTTP response projection is empty")
	}
	result, err := json.Marshal(projected)
	if err != nil || len(result) > maximumConfidentialHTTPResponse {
		return nil, gerror.New("confidential Provider HTTP response projection is invalid")
	}
	return result, nil
}

func projectConfidentialHTTPValue(value any, allowed map[string]struct{}, depth int) (any, bool) {
	if depth > 32 {
		return nil, false
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, child := range typed {
			if _, ok := allowed[key]; !ok {
				continue
			}
			if projected, ok := projectConfidentialHTTPValue(child, allowed, depth+1); ok {
				result[key] = projected
			}
		}
		return result, len(result) != 0
	case []any:
		if len(typed) > 4096 {
			return nil, false
		}
		result := make([]any, 0, len(typed))
		for _, child := range typed {
			if projected, ok := projectConfidentialHTTPValue(child, allowed, depth+1); ok {
				result = append(result, projected)
			}
		}
		return result, true
	case nil, bool, json.Number, string:
		return typed, true
	default:
		return nil, false
	}
}

// Validate permitted structure before projection so an oversized price or
// candidate array cannot disappear while the rest appears complete.
func boundedConfidentialHTTPValue(value any, allowed map[string]struct{}, depth int) bool {
	if depth > 32 {
		return false
	}
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if _, ok := allowed[key]; ok && !boundedConfidentialHTTPValue(child, allowed, depth+1) {
				return false
			}
		}
	case []any:
		if len(typed) > 4096 {
			return false
		}
		for _, child := range typed {
			if !boundedConfidentialHTTPValue(child, allowed, depth+1) {
				return false
			}
		}
	}
	return true
}

func confidentialBinding(bindings []resourcecap.SecretBinding, name string) (secretcap.Handle, bool) {
	for _, binding := range bindings {
		if binding.Name == name && !binding.Handle.IsZero() {
			return binding.Handle, true
		}
	}
	return secretcap.Handle{}, false
}

func sortedMapKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func firstValues(values url.Values) map[string]string {
	result := make(map[string]string, len(values))
	for key := range values {
		result[key] = values.Get(key)
	}
	return result
}

func marshalConfidentialHTTPResponse(response resourcecap.ConfidentialHTTPResponse) (*pluginhost.ConfidentialOutboundResult, error) {
	payload, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	return &pluginhost.ConfidentialOutboundResult{Payload: payload}, nil
}
