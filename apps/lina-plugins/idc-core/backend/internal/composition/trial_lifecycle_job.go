// This file registers bounded trial expiry processing on the primary node.

package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/triallifecycle"
)

// Trial job metadata is stable across plugin restarts.
const (
	trialLifecycleJobPattern     = "@every 30s"
	trialLifecycleJobName        = "idc-core-trial-lifecycle"
	trialLifecycleJobDisplayName = "IDC Trial Lifecycle"
	trialLifecycleJobDescription = "Suspends expired trials and sends customer reminders."
	trialLifecycleBatchLimit     = 32
)

// RegisterTrialLifecycleJob installs the primary-node trial convergence worker.
// The worker owns durable scheduling, resource observation, and notification
// retries, so the scheduler only invokes one bounded pass.
func RegisterTrialLifecycleJob(ctx context.Context, registrar pluginhost.JobsRegistrar, worker triallifecycle.Service) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("trial lifecycle job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(ctx, trialLifecycleJobPattern, trialLifecycleJobName,
		trialLifecycleJobDisplayName, trialLifecycleJobDescription,
		func(jobCtx context.Context) error {
			return worker.Run(jobCtx, trialLifecycleBatchLimit)
		})
}
