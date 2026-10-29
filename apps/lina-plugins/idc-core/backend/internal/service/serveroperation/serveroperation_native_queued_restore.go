// Queued restore validates retained billing/object facts without a customer
// session. Only the trusted resource owner may call this source-only port after
// WithQueuedEntitlement verified the frozen task/customer and live lease in the
// same transaction. It never creates or impersonates a principal.

package serveroperation

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
)

// CheckQueuedRestore requires the shared core transaction established by the
// queued entitlement gate. The resource owner retains responsibility for exact
// task ownership, expected parent version and lease validation before this call.
// This port performs only local database reads and makes no provider call.
func (s *nativeCommercialService) CheckQueuedRestore(ctx context.Context, input servercommercialcap.ObjectRequest) error {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil {
		return PublicError(ErrInvalidInput)
	}
	if gdb.TXFromCtx(ctx, s.database.GetGroup()) == nil {
		return PublicError(ErrOperationConflict)
	}
	return PublicError(nativeCheckRestore(ctx, s.database, input))
}
