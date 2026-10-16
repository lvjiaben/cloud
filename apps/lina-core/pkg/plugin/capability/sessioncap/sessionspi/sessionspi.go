// Package sessionspi exposes the same-process query-composition seam owned by
// the host session service. Query builders are deliberately excluded from the
// ordinary sessioncap contract and from the dynamic-plugin wire protocol.
package sessionspi

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/sessioncap"
)

// ScopeService composes governed presence into source-plugin queries without
// materializing all user IDs or exposing host table names to business code.
type ScopeService interface {
	// BuildOnlineUserExists returns a correlated subquery for EXISTS. The column
	// must be an alias-qualified SQL identifier. Tenant and role data scope,
	// enabled-user state, and runtime inactivity timeout are applied by the host.
	// empty means the caller must return an empty result. Missing dependencies,
	// authentication, or invalid identifiers fail closed. The model is request
	// local and must not be cached or have its host-owned predicates removed.
	BuildOnlineUserExists(ctx context.Context, userIDColumn string) (model *gdb.Model, empty bool, err error)
}

// Scope obtains the explicitly supported source query seam. Ordinary guest
// clients return unavailable; their bounded BatchGetUserOnlineStatus remains
// the serializable presence contract.
func Scope(service sessioncap.Service) (ScopeService, error) {
	if scope, ok := service.(ScopeService); ok && scope != nil {
		return scope, nil
	}
	return nil, bizerr.NewCode(capmodel.CodeCapabilityUnavailable, bizerr.P("capability", "session.scope"))
}
