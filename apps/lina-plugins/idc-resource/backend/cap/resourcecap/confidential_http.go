// This file declares secret-free confidential HTTP instructions. Secret field
// names cross governed boundaries as values, never credential-named JSON keys.
package resourcecap

import (
	"encoding/json"
	"sort"
)

const (
	ConfidentialHTTPKindAuthenticate = "authenticate"
	ConfidentialHTTPKindRequest      = "request"
)

// ConfidentialHTTPRequest is the dynamic-provider-visible, secret-free
// instruction for one host-confidential HTTP step.
type ConfidentialHTTPRequest struct {
	Kind   string              `json:"kind"`
	Method string              `json:"method"`
	Path   string              `json:"path"`
	Query  map[string][]string `json:"query,omitempty"`
	Form   map[string]string   `json:"form,omitempty"`
	// FormFields carries public HTTP form names as values, so bracket notation
	// does not become an invalid object key at the governed JSON boundary.
	FormFields []ConfidentialFormField `json:"formFields,omitempty"`
	// JSONBody carries public configuration in an object request body. It cannot
	// be combined with forms or secret form bindings; credentials remain host-owned.
	JSONBody         json.RawMessage   `json:"jsonBody,omitempty"`
	CredentialFields map[string]string `json:"credentialFields,omitempty"`
	// SecretFormFields is retained for trusted-owner internal instructions.
	// Guests use SecretFormBindings so sensitive field names are JSON values.
	SecretFormFields   map[string]string               `json:"secretFormFields,omitempty"`
	SecretFormBindings []ConfidentialSecretFormBinding `json:"secretFormBindings,omitempty"`
	// ResponseFields is the complete supplier JSON field allowlist for this step.
	// Container keys must also be listed.
	ResponseFields    []string `json:"responseFields,omitempty"`
	UseAuthentication bool     `json:"useAuthentication,omitempty"`
	// Headers is populated only inside the resource owner after decoding. It is
	// never accepted from or serialized to a managed guest.
	Headers map[string]string `json:"-"`
}

// ConfidentialHTTPResponse exposes only a safe business projection. Login
// tokens and raw authentication responses are retained by the host.
type ConfidentialHTTPResponse struct {
	StatusCode    int             `json:"StatusCode"`
	Data          json.RawMessage `json:"Data,omitempty"`
	Authenticated bool            `json:"Authenticated,omitempty"`
}

// ConfidentialSecretFormBinding names an upstream field and an existing operation
// secret slot. It contains no plaintext credential or secret handle.
type ConfidentialSecretFormBinding struct {
	Field string `json:"field"`
	Slot  string `json:"slot"`
}

// ConfidentialFormField carries one public form entry, never a secret binding.
// The resource owner validates names, limits and duplicates before any HTTP call.
type ConfidentialFormField struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

// NewConfidentialFormFields preserves upstream names in deterministic order
// without placing supplier-specific field syntax in governed JSON object keys.
func NewConfidentialFormFields(fields map[string]string) []ConfidentialFormField {
	keys := make([]string, 0, len(fields))
	for field := range fields {
		keys = append(keys, field)
	}
	sort.Strings(keys)
	items := make([]ConfidentialFormField, 0, len(keys))
	for _, field := range keys {
		items = append(items, ConfidentialFormField{Field: field, Value: fields[field]})
	}
	return items
}

// NewConfidentialSecretFormBindings creates a deterministic secret-free wire
// projection. The resource owner validates and resolves every slot on receipt.
func NewConfidentialSecretFormBindings(fields map[string]string) []ConfidentialSecretFormBinding {
	if len(fields) == 0 {
		return nil
	}
	keys := make([]string, 0, len(fields))
	for field := range fields {
		keys = append(keys, field)
	}
	sort.Strings(keys)
	bindings := make([]ConfidentialSecretFormBinding, 0, len(keys))
	for _, field := range keys {
		bindings = append(bindings, ConfidentialSecretFormBinding{Field: field, Slot: fields[field]})
	}
	return bindings
}
