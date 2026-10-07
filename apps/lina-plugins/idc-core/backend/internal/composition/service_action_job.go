package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/serviceaction"
)

func RegisterManagedServiceActionJob(ctx context.Context, registrar pluginhost.JobsRegistrar, worker serviceaction.Worker) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("managed service action job is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(ctx, "@every 5s", "idc-core-managed-service-actions", "IDC Managed Service Actions", "Converges frozen actions on long-lived services.", func(jobCtx context.Context) error {
		_, err := worker.Run(jobCtx, 20)
		return err
	})
}
