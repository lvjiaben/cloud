// This file restores public form names only inside the resource owner. Bracket
// notation is accepted for supplier arrays; credential names remain forbidden.
package composition

import (
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

var confidentialPublicFormName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.:-]*(\[[A-Za-z0-9_.:-]*\])*$`)

// normalizeConfidentialPublicFormFields validates the entire wire projection
// before mutation. Public entries cannot overlap the trusted secret-slot path.
func normalizeConfidentialPublicFormFields(request *resourcecap.ConfidentialHTTPRequest) error {
	if request == nil {
		return gerror.New("confidential public form is invalid")
	}
	if len(request.FormFields) == 0 {
		return nil
	}
	if len(request.Form) != 0 || len(request.FormFields) > 128 || request.Kind != resourcecap.ConfidentialHTTPKindRequest {
		return gerror.New("confidential public form conflicts or exceeds limit")
	}
	fields := make(map[string]string, len(request.FormFields))
	for _, item := range request.FormFields {
		if len(item.Field) > 128 || !confidentialPublicFormName.MatchString(item.Field) || !safeDiagnosticJSONKey(item.Field) {
			return gerror.New("confidential public form field is invalid")
		}
		for _, part := range strings.FieldsFunc(item.Field, func(r rune) bool { return r == '[' || r == ']' }) {
			if !safeDiagnosticJSONKey(part) {
				return gerror.New("confidential public form field is forbidden")
			}
		}
		if _, exists := fields[item.Field]; exists {
			return gerror.New("confidential public form field is duplicated")
		}
		fields[item.Field] = item.Value
	}
	request.Form, request.FormFields = fields, nil
	return nil
}
