package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/supportdesk"
)

const (
	supportSLAJobPattern     = "@every 30s"
	supportSLAJobName        = "idc-core-support-sla-risk"
	supportSLAJobDisplayName = "IDC Support SLA Risk"
	supportSLAJobDescription = "Refreshes persisted current-round support SLA risk from PostgreSQL authority."
	supportSLAJobBatchLimit  = 200
)

func RegisterSupportSLAJob(ctx context.Context, registrar pluginhost.JobsRegistrar, worker supportdesk.SLAWorker) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil {
		return gerror.New("support SLA job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	if worker == nil {
		return gerror.New("support SLA worker is unavailable")
	}
	return registrar.AddWithMetadata(
		ctx,
		supportSLAJobPattern,
		supportSLAJobName,
		supportSLAJobDisplayName,
		supportSLAJobDescription,
		func(jobCtx context.Context) error {
			_, err := worker.RefreshRisk(jobCtx, supportSLAJobBatchLimit)
			return err
		},
	)
}
