// This file admits two reviewed guest connection labels only for the managed
// cloud connector's exact authenticated resource read. Global secret-field
// restrictions remain unchanged for actions, catalogs and other providers.
package composition

import (
	"net/http"
	"regexp"
	"slices"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

var cloudResourceDetailPath = regexp.MustCompile(`^/clouds/[1-9][0-9]{0,18}$`)

// validateConfidentialRequest applies the ordinary validator to every field
// except the two source-documented guest facts within this exact read boundary.
func (h *confidentialHTTPHandler) validateConfidentialRequest(request resourcecap.ConfidentialHTTPRequest) error {
	if h.configuration.PluginID == "idc-zjmfcloud" && h.providerMethod == resourcecap.DynamicProviderMethodQueryResource &&
		request.Kind == resourcecap.ConfidentialHTTPKindRequest && request.Method == http.MethodGet && request.UseAuthentication &&
		cloudResourceDetailPath.MatchString(request.Path) && len(request.ResponseFields) <= 128 && len(request.Headers) == 0 && len(request.Query) == 0 && len(request.Form) == 0 && len(request.FormFields) == 0 &&
		len(request.JSONBody) == 0 && len(request.CredentialFields) == 0 && len(request.SecretFormFields) == 0 && len(request.SecretFormBindings) == 0 {
		request.ResponseFields = slices.DeleteFunc(slices.Clone(request.ResponseFields), func(field string) bool { return field == "hostname" || field == "osuser" })
	}
	return validateConfidentialHTTPRequest(request)
}
