package resourcecap

import "encoding/json"

const (
	ConfidentialHTTPKindAuthenticate = "authenticate"
	ConfidentialHTTPKindRequest      = "request"
)

// ConfidentialHTTPRequest is the dynamic-provider-visible, secret-free
// instruction for one host-confidential HTTP step.
type ConfidentialHTTPRequest struct {
	Kind             string              `json:"kind"`
	Method           string              `json:"method"`
	Path             string              `json:"path"`
	Query            map[string][]string `json:"query,omitempty"`
	Form             map[string]string   `json:"form,omitempty"`
	CredentialFields map[string]string   `json:"credentialFields,omitempty"`
	// SecretFormFields maps an upstream form field to an operation secret slot.
	// Only the slot name crosses the WASM boundary; the host resolves its handle.
	SecretFormFields map[string]string `json:"secretFormFields,omitempty"`
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
