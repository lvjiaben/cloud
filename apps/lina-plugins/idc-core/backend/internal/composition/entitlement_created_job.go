package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

func RegisterEntitlementCreatedJob(ctx context.Context, registrar pluginhost.JobsRegistrar, worker productfulfillment.EntitlementCreatedWorker) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("entitlement-created job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(ctx, "@every 5s", "idc-core-entitlement-created", "IDC Entitlement Projection", "Delivers committed generic entitlement facts to an authorized owner.", func(jobCtx context.Context) error { _, err := worker.Run(jobCtx, 20); return err })
}
