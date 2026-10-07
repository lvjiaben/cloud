package supplierinterfacecap

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

var outboundNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

var forbiddenOutboundResponseFields = map[string]struct{}{
	"accesskey": {}, "accesstoken": {}, "apikey": {}, "authorization": {},
	"cookie": {}, "credential": {}, "credentials": {}, "password": {},
	"privatekey": {}, "refreshtoken": {}, "secret": {}, "secretkey": {},
	"sessiontoken": {}, "token": {},
}

// OutboundHTTPRequest is a secret-free instruction for one host-owned
// supplier HTTP call. SecretHeaders maps an upstream header name to a secret
// interface field key; it never contains the secret value.
type OutboundHTTPRequest struct {
	Method         string            `json:"method"`
	Path           string            `json:"path"`
	Query          map[string]string `json:"query,omitempty"`
	Body           json.RawMessage   `json:"body,omitempty"`
	EndpointField  string            `json:"endpointField"`
	SecretHeaders  map[string]string `json:"secretHeaders,omitempty"`
	ResponseFields []string          `json:"responseFields"`
}

// OutboundHTTPResponse contains only the field-whitelisted supplier response.
type OutboundHTTPResponse struct {
	StatusCode int             `json:"statusCode"`
	Data       json.RawMessage `json:"data"`
}

func ValidateOutboundHTTPRequest(request OutboundHTTPRequest) error {
	if request.Method != http.MethodGet && request.Method != http.MethodPost && request.Method != http.MethodPut && request.Method != http.MethodPatch && request.Method != http.MethodDelete {
		return gerror.New("supplier interface outbound method is invalid")
	}
	if request.Path == "" || request.Path[0] != '/' || len(request.Path) > 1024 || strings.Contains(request.Path, "..") || strings.ContainsAny(request.Path, "?#\\") {
		return gerror.New("supplier interface outbound path is invalid")
	}
	if !outboundNamePattern.MatchString(request.EndpointField) || len(request.Query) > 32 || len(request.SecretHeaders) > 8 || len(request.ResponseFields) == 0 || len(request.ResponseFields) > 128 || len(request.Body) > MaximumOutboundPayload {
		return gerror.New("supplier interface outbound fields are invalid")
	}
	if len(request.Body) > 0 && !json.Valid(request.Body) {
		return gerror.New("supplier interface outbound body is invalid")
	}
	seenResponseFields := make(map[string]struct{}, len(request.ResponseFields))
	for _, field := range request.ResponseFields {
		if !outboundNamePattern.MatchString(field) {
			return gerror.New("supplier interface outbound response field is invalid")
		}
		normalized := strings.ReplaceAll(strings.ToLower(field), "_", "")
		if _, forbidden := forbiddenOutboundResponseFields[normalized]; forbidden {
			return gerror.New("supplier interface outbound response field is forbidden")
		}
		if _, exists := seenResponseFields[field]; exists {
			return gerror.New("supplier interface outbound response field is duplicated")
		}
		seenResponseFields[field] = struct{}{}
	}
	for header, field := range request.SecretHeaders {
		if !validSecretHeader(header) || !outboundNamePattern.MatchString(field) {
			return gerror.New("supplier interface outbound secret header is invalid")
		}
	}
	for key, value := range request.Query {
		if !outboundNamePattern.MatchString(key) || len(value) > 8192 || strings.ContainsAny(value, "\x00\r\n") {
			return gerror.New("supplier interface outbound query is invalid")
		}
	}
	return nil
}

func ValidateOutboundHTTPResponse(response *OutboundHTTPResponse) error {
	if response == nil || response.StatusCode < 100 || response.StatusCode > 599 || len(response.Data) == 0 || len(response.Data) > MaximumOutboundPayload || !json.Valid(response.Data) {
		return gerror.New("supplier interface outbound response is invalid")
	}
	return nil
}

func validSecretHeader(value string) bool {
	if value == "" || len(value) > 128 || strings.ContainsAny(value, "\x00\r\n:") {
		return false
	}
	for _, character := range value {
		if character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' {
			continue
		}
		return false
	}
	return true
}
