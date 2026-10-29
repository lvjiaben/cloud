// This file records bounded, credential-scanned HTTP response diagnostics in
// the resource owner before guest field projection can discard error messages.
package composition

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const diagnosticRedaction = "[REDACTED]"

// reviewedResponseStatus preserves known HTTP rejections even when reading the
// error body fails. Only documented validation failures prove non-acceptance.
func (h *confidentialHTTPHandler) reviewedResponseStatus(status int) (*resourcecap.ConfidentialHTTPResponse, error, bool) {
	switch status {
	case http.StatusNotFound:
		return nil, bizerr.NewCode(resourcecap.CodeProviderEndpointNotFound), true
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, bizerr.NewCode(resourcecap.CodeProviderAuthRejected), true
	}
	if h.providerMethod == resourcecap.DynamicProviderMethodCreatePrincipal && status == http.StatusBadRequest {
		return nil, &resourcecap.ProviderCallError{Code: resourcecap.ProviderCallErrorCodeInvalidResponse}, true
	}
	if confidentialOperationValidationRejected(h.configuration.PluginID, h.providerMethod, status) {
		return &resourcecap.ConfidentialHTTPResponse{StatusCode: status, Data: json.RawMessage(`{}`)}, nil, true
	}
	if status >= http.StatusMultipleChoices && status < http.StatusBadRequest {
		return nil, bizerr.NewCode(resourcecap.CodeProviderInvalidResponse), true
	}
	return nil, nil, false
}

// observeHTTPResponse never records login responses, headers, query strings or
// endpoints. Secret echoes are removed before any diagnostic can be retained.
func (h *confidentialHTTPHandler) observeHTTPResponse(ctx context.Context, request resourcecap.ConfidentialHTTPRequest, status int, content, token []byte) {
	if h.observeResponse == nil || h.providerMethod != resourcecap.DynamicProviderMethodExecute || request.Kind != resourcecap.ConfidentialHTTPKindRequest {
		return
	}
	text := string(content)
	secrets := append([]string{string(token)}, diagnosticSecretValues(request)...)
	for _, value := range secrets {
		if value != "" {
			text = strings.ReplaceAll(text, value, diagnosticRedaction)
		}
	}
	// Configured login credentials may not occur in this business request;
	// their decoded echoes are removed inside owner-only secret callbacks.
	evidence := resourcecap.Evidence{Kind: resourcecap.EvidenceKindHTTPResponse, ObservedAt: time.Now().UTC()}
	var body any
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	if decoder.Decode(&body) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		body = text
	}
	body = sanitizeHTTPDiagnosticValue(body, 0, secrets)
	body = h.redactConfiguredDiagnosticSecrets(ctx, body)
	value := struct {
		Method     string `json:"method"`
		Route      string `json:"route"`
		HTTPStatus int    `json:"httpStatus"`
		Response   any    `json:"response"`
		Truncated  bool   `json:"truncated,omitempty"`
	}{Method: request.Method, Route: request.Path, HTTPStatus: status, Response: body}
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	if len(raw) > 4000 {
		response, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			return
		}
		value.Response, value.Truncated = truncateDiagnostic(string(response), 512), true
		raw, err = json.Marshal(value)
		if err != nil {
			return
		}
	}
	evidence.Summary = string(raw)
	if evidence.Validate() == nil {
		h.observeResponse(evidence)
	}
}

// redactConfiguredDiagnosticSecrets consumes configured values only inside
// their authorized secret callback and retains only the sanitized response.
func (h *confidentialHTTPHandler) redactConfiguredDiagnosticSecrets(ctx context.Context, body any) any {
	if len(h.configuration.Config.SecretBindings) == 0 {
		return body
	}
	if h.environment.Secrets == nil {
		return diagnosticRedaction
	}
	purpose, err := secretcap.NewPurpose("provider.response-screening")
	if err != nil {
		return diagnosticRedaction
	}
	for _, binding := range h.configuration.Config.SecretBindings {
		access, accessErr := secretcap.NewAccessRequest(binding.Handle, purpose)
		if accessErr != nil {
			return diagnosticRedaction
		}
		if accessErr = accessConfidentialProviderSecret(ctx, h.environment.Secrets, access, func(value []byte) error {
			body = sanitizeHTTPDiagnosticValue(body, 0, []string{string(value)})
			return nil
		}); accessErr != nil {
			return diagnosticRedaction
		}
	}
	return body
}

// diagnosticSecretValues excludes the actual authentication and operation
// values supplied to this HTTP request, including derived request signatures.
func diagnosticSecretValues(request resourcecap.ConfidentialHTTPRequest) []string {
	values := make([]string, 0, len(request.Headers)+len(request.Form))
	for _, value := range request.Headers {
		values = append(values, value)
	}
	for key, value := range request.Form {
		if !safeDiagnosticJSONKey(key) {
			values = append(values, value)
		}
	}
	return values
}

// sanitizeHTTPDiagnosticValue retains response structure and error wording,
// with fixed bounds and credential scanning at every untrusted scalar/key.
func sanitizeHTTPDiagnosticValue(value any, depth int, secrets []string) any {
	if depth > 8 {
		return "[TRUNCATED]"
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any)
		count := 0
		for key, item := range typed {
			if count >= 64 {
				result["_truncated"] = true
				break
			}
			count++
			if !safeDiagnosticJSONKey(key) || redactDiagnosticString(key, secrets) != key {
				result["_redactedFields"] = true
				continue
			}
			result[key] = sanitizeHTTPDiagnosticValue(item, depth+1, secrets)
		}
		return result
	case []any:
		result := make([]any, 0, min(len(typed), 17))
		for i, item := range typed {
			if i >= 16 {
				result = append(result, "[TRUNCATED]")
				break
			}
			result = append(result, sanitizeHTTPDiagnosticValue(item, depth+1, secrets))
		}
		return result
	case string:
		typed = redactDiagnosticString(typed, secrets)
		probe := resourcecap.Evidence{Kind: resourcecap.EvidenceKindHTTPResponse, Summary: truncateDiagnostic(typed, 2048), ObservedAt: time.Now().UTC()}
		if probe.Validate() != nil {
			if len(typed) > 2048 {
				return diagnosticRedaction + " [TRUNCATED]"
			}
			return diagnosticRedaction
		}
		return probe.Summary
	case json.Number:
		if redactDiagnosticString(typed.String(), secrets) != typed.String() {
			return diagnosticRedaction
		}
		return typed
	default:
		return value
	}
}

// safeDiagnosticJSONKey reuses the owner's existing credential scanner rather
// than weakening it to permit credential-bearing fields with masked values.
func safeDiagnosticJSONKey(key string) bool {
	name := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
	if strings.Contains(name, "pass") || name == "username" {
		return false
	}
	raw, err := json.Marshal(map[string]string{key: "masked"})
	if err != nil {
		return false
	}
	return (resourcecap.Evidence{Kind: resourcecap.EvidenceKindHTTPResponse, Summary: string(raw), ObservedAt: time.Now().UTC()}).Validate() == nil
}

// redactDiagnosticString also screens decoded JSON text so escaped echoes of
// the submitted credentials cannot bypass literal byte replacement.
func redactDiagnosticString(text string, secrets []string) string {
	for _, value := range secrets {
		if value != "" {
			text = strings.ReplaceAll(text, value, diagnosticRedaction)
		}
	}
	return text
}

// truncateDiagnostic preserves UTF-8 and marks excerpts instead of silently
// passing a partial oversized response to storage.
func truncateDiagnostic(text string, maximum int) string {
	if len(text) <= maximum {
		return text
	}
	text = text[:maximum]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text + " [TRUNCATED]"
}

// appendHTTPResponseEvidence retains trusted owner observations independently
// of whether the guest returned a successful, rejected or uncertain result.
func appendHTTPResponseEvidence(target any, observations []resourcecap.Evidence) {
	if operation, ok := target.(*resourcecap.OperationResult); ok {
		operation.Evidence = append(operation.Evidence, observations...)
	}
}

// retainUncertainHTTPResponse preserves actual HTTP evidence even when the
// guest invocation itself failed. A response alone never proves acceptance.
func retainUncertainHTTPResponse(method string, target any, observations []resourcecap.Evidence) bool {
	if method != resourcecap.DynamicProviderMethodExecute || len(observations) == 0 {
		return false
	}
	operation, ok := target.(*resourcecap.OperationResult)
	if !ok {
		return false
	}
	*operation = resourcecap.OperationResult{Status: resourcecap.TaskStatusUnknown, Failure: &resourcecap.ProviderFailure{Code: "upstream_result_unknown", Message: "upstream operation requires reconciliation", ResultUnknown: true}}
	appendHTTPResponseEvidence(target, observations)
	return true
}
