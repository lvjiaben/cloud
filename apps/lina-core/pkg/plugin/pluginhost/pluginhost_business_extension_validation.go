// This file implements source-owner registration for typed provider metadata validation.

package pluginhost

import (
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

func businessExtensionValidatorKey(point, contractVersion string) string {
	return strings.TrimSpace(point) + "\x00" + strings.TrimSpace(contractVersion)
}

// RegisterProviderValidator binds one validator to the current source owner.
func (r *sourcePluginBusinessExtensions) RegisterProviderValidator(
	point, contractVersion string,
	validator BusinessExtensionProviderValidator,
) error {
	if r == nil || r.plugin == nil {
		return gerror.New("pluginhost: source plugin business extension facade is nil")
	}
	point = strings.TrimSpace(point)
	contractVersion = strings.TrimSpace(contractVersion)
	if point == "" || contractVersion == "" || validator == nil {
		return gerror.New("pluginhost: business extension provider validator is invalid")
	}
	key := businessExtensionValidatorKey(point, contractVersion)
	r.plugin.businessExtensionMu.Lock()
	defer r.plugin.businessExtensionMu.Unlock()
	if _, exists := r.plugin.businessExtensionValidators[key]; exists {
		return gerror.Newf("pluginhost: duplicate business extension provider validator: %s@%s", point, contractVersion)
	}
	r.plugin.businessExtensionValidators[key] = validator
	return nil
}

// GetBusinessExtensionProviderValidator returns one immutable registration.
func (p *sourcePlugin) GetBusinessExtensionProviderValidator(
	point, contractVersion string,
) (BusinessExtensionProviderValidator, bool) {
	if p == nil {
		return nil, false
	}
	p.businessExtensionMu.RLock()
	defer p.businessExtensionMu.RUnlock()
	validator, ok := p.businessExtensionValidators[businessExtensionValidatorKey(point, contractVersion)]
	return validator, ok && validator != nil
}
