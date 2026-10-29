// This file adds an optional, paired confidential principal bridge to v1.
package resourcecap

import "github.com/gogf/gf/v2/errors/gerror"

const (
	DynamicProviderMethodLookupPrincipal = "principal.lookup"
	DynamicProviderMethodCreatePrincipal = "principal.create"
)

// DynamicPrincipalRequest contains slot names only; principal credentials and
// personally identifying profile values remain in resource-owned secretcap.
type DynamicPrincipalRequest struct {
	RequestID              string   `json:"requestId"`
	ConfidentialInputNames []string `json:"confidentialInputNames"`
}

func (r DynamicPrincipalRequest) Validate(create bool) error {
	if validateRequiredReference("principal request ID", r.RequestID) != nil {
		return gerror.New("principal bridge request is invalid")
	}
	seen := map[string]bool{}
	for _, name := range r.ConfidentialInputNames {
		if seen[name] {
			return gerror.New("principal bridge slot is duplicated")
		}
		switch name {
		case PrincipalBindingUsername, PrincipalBindingPassword, PrincipalBindingPhone, PrincipalBindingEmail, PrincipalBindingRealName:
		default:
			return gerror.New("principal bridge slot is invalid")
		}
		seen[name] = true
	}
	if !seen[PrincipalBindingUsername] || create && !seen[PrincipalBindingPassword] || !create && len(seen) != 1 {
		return gerror.New("principal bridge required slots are missing")
	}
	return nil
}
