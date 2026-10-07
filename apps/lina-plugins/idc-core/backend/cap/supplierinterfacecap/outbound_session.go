// This file defines the secret-free supplier authentication session contract.

package supplierinterfacecap

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	MaximumOutboundSessionSteps = 8
	OutboundTokenRootString     = "root_string"
	OutboundTokenField          = "field"
)

// OutboundSessionAuthentication describes one host-owned form login. Secret
// values are referenced by interface field key and never carried in this DTO.
type OutboundSessionAuthentication struct {
	Method           string            `json:"method"`
	Path             string            `json:"path"`
	Form             map[string]string `json:"form,omitempty"`
	SecretFormFields map[string]string `json:"secretFormFields"`
	TokenSelector    string            `json:"tokenSelector"`
	TokenField       string            `json:"tokenField,omitempty"`
	TokenHeader      string            `json:"tokenHeader"`
}

// OutboundSessionStep is one authenticated, exact-origin supplier request.
type OutboundSessionStep struct {
	Method           string            `json:"method"`
	Path             string            `json:"path"`
	Query            map[string]string `json:"query,omitempty"`
	Form             map[string]string `json:"form,omitempty"`
	SecretFormFields map[string]string `json:"secretFormFields,omitempty"`
	ResponseFields   []string          `json:"responseFields"`
}

// OutboundSessionRequest executes one login and a bounded sequence while the
// token remains only in host memory.
type OutboundSessionRequest struct {
	EndpointField  string                        `json:"endpointField"`
	Authentication OutboundSessionAuthentication `json:"authentication"`
	Steps          []OutboundSessionStep         `json:"steps"`
}

// OutboundSessionStepResponse contains one field-whitelisted response.
type OutboundSessionStepResponse struct {
	StatusCode int             `json:"statusCode"`
	Data       json.RawMessage `json:"data"`
}

// OutboundSessionResponse contains the ordered safe projections only.
type OutboundSessionResponse struct {
	Steps []OutboundSessionStepResponse `json:"steps"`
}

// ValidateOutboundSessionRequest validates the secret-free session plan.
func ValidateOutboundSessionRequest(request OutboundSessionRequest) error {
	if !outboundNamePattern.MatchString(request.EndpointField) ||
		request.Authentication.Method != http.MethodPost ||
		!validOutboundSessionPath(request.Authentication.Path) ||
		!validSecretHeader(request.Authentication.TokenHeader) ||
		len(request.Authentication.SecretFormFields) == 0 ||
		len(request.Authentication.SecretFormFields) > 8 ||
		len(request.Steps) == 0 || len(request.Steps) > MaximumOutboundSessionSteps {
		return gerror.New("supplier interface outbound session is invalid")
	}
	if request.Authentication.TokenSelector != OutboundTokenRootString && request.Authentication.TokenSelector != OutboundTokenField {
		return gerror.New("supplier interface outbound token selector is invalid")
	}
	if request.Authentication.TokenSelector == OutboundTokenField {
		if !outboundNamePattern.MatchString(request.Authentication.TokenField) {
			return gerror.New("supplier interface outbound token field is invalid")
		}
	} else if request.Authentication.TokenField != "" {
		return gerror.New("supplier interface outbound token field is invalid")
	}
	if !validOutboundSessionForm(request.Authentication.Form, request.Authentication.SecretFormFields) {
		return gerror.New("supplier interface outbound authentication form is invalid")
	}
	for _, step := range request.Steps {
		if !validOutboundSessionMethod(step.Method) || !validOutboundSessionPath(step.Path) ||
			len(step.Query) > 32 || !validOutboundSessionForm(step.Form, step.SecretFormFields) ||
			len(step.ResponseFields) == 0 || len(step.ResponseFields) > 128 {
			return gerror.New("supplier interface outbound session step is invalid")
		}
		seen := make(map[string]struct{}, len(step.ResponseFields))
		for _, field := range step.ResponseFields {
			normalized := strings.ReplaceAll(strings.ToLower(field), "_", "")
			if !outboundNamePattern.MatchString(field) {
				return gerror.New("supplier interface outbound session response field is invalid")
			}
			if _, forbidden := forbiddenOutboundResponseFields[normalized]; forbidden {
				return gerror.New("supplier interface outbound session response field is forbidden")
			}
			if _, exists := seen[field]; exists {
				return gerror.New("supplier interface outbound session response field is duplicated")
			}
			seen[field] = struct{}{}
		}
		for key, value := range step.Query {
			if !outboundNamePattern.MatchString(key) || len(value) > 8192 || strings.ContainsAny(value, "\x00\r\n") {
				return gerror.New("supplier interface outbound session query is invalid")
			}
		}
	}
	return nil
}

// ValidateOutboundSessionResponse validates one bounded safe response.
func ValidateOutboundSessionResponse(response *OutboundSessionResponse) error {
	if response == nil || len(response.Steps) == 0 || len(response.Steps) > MaximumOutboundSessionSteps {
		return gerror.New("supplier interface outbound session response is invalid")
	}
	for _, step := range response.Steps {
		if step.StatusCode < 100 || step.StatusCode > 599 || len(step.Data) == 0 || len(step.Data) > MaximumOutboundPayload {
			return gerror.New("supplier interface outbound session response is invalid")
		}
	}
	return nil
}

func validOutboundSessionMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete
}

func validOutboundSessionPath(path string) bool {
	return path != "" && path[0] == '/' && len(path) <= 1024 && !strings.Contains(path, "..") && !strings.ContainsAny(path, "?#\\")
}

func validOutboundSessionForm(public, secret map[string]string) bool {
	if len(public) > 128 || len(secret) > 8 {
		return false
	}
	for key, value := range public {
		if !outboundNamePattern.MatchString(key) || len(value) > 8192 || strings.ContainsAny(value, "\x00\r\n") {
			return false
		}
		if _, duplicated := secret[key]; duplicated {
			return false
		}
	}
	for upstreamField, secretField := range secret {
		if !outboundNamePattern.MatchString(upstreamField) || !outboundNamePattern.MatchString(secretField) {
			return false
		}
	}
	return true
}
