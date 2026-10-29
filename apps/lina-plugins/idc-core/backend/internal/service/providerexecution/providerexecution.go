// Package providerexecution separates a saved business contract from its
// executable package. It resolves a missing package only to an enabled provider
// with the identical declaration; every invocation still uses real host evidence.
package providerexecution

import (
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/pluginhost"
)

// registrar retains the single startup-injected host authority without caches.
type registrar struct {
	pluginhost.BusinessExtensionRegistrar
}

// New adapts historical contract references to compatible current packages.
// It owns no state, does not edit snapshots and never changes host governance.
func New(host pluginhost.BusinessExtensionRegistrar) (pluginhost.BusinessExtensionRegistrar, error) {
	if host == nil {
		return nil, gerror.New("provider execution requires the host registrar")
	}
	return &registrar{BusinessExtensionRegistrar: host}, nil
}
