package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/supportdesk"
)

const (
	supportAutomationJobPattern     = "@every 1m"
	supportAutomationJobName        = "idc-core-support-automation"
	supportAutomationJobDisplayName = "IDC Support Automation"
	supportAutomationJobDescription = "Auto-closes resolved support tickets after the configured database-authoritative wait period."
	supportAutomationJobBatchLimit  = 200
)

func RegisterSupportAutomationJob(ctx context.Context, registrar pluginhost.JobsRegistrar, worker supportdesk.AutomationWorker) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil {
		return gerror.New("support automation job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	if worker == nil {
		return gerror.New("support automation worker is unavailable")
	}
	return registrar.AddWithMetadata(
		ctx,
		supportAutomationJobPattern,
		supportAutomationJobName,
		supportAutomationJobDisplayName,
		supportAutomationJobDescription,
		func(jobCtx context.Context) error {
			_, err := worker.AutoCloseResolved(jobCtx, supportAutomationJobBatchLimit)
			return err
		},
	)
}
