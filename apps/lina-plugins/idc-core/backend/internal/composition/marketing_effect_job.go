package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

const (
	marketingEffectJobPattern    = "@every 10s"
	marketingEffectJobName       = "idc-core-marketing-effect-reconciliation"
	marketingEffectJobBatchLimit = 100
)

func RegisterMarketingEffectJob(ctx context.Context, registrar pluginhost.JobsRegistrar, worker marketing.EffectReconciler) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("marketing effect job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(ctx, marketingEffectJobPattern, marketingEffectJobName,
		"IDC Marketing Effect Reconciliation",
		"Projects authoritative local campaign, order, payment, refund, and cost facts without external calls.",
		func(jobCtx context.Context) error {
			_, err := worker.Run(jobCtx, marketingEffectJobBatchLimit)
			return err
		})
}
