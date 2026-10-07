package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

const (
	productAutomationSecretRecoveryJobPattern = "@every 30s"
	productAutomationSecretRecoveryJobName    = "idc-core-product-automation-secret-recovery"
	productAutomationSecretRecoveryBatchLimit = 8
)

// RegisterProductAutomationSecretRecoveryJob installs bounded, primary-node
// convergence for owner-private secret operation records.
func RegisterProductAutomationSecretRecoveryJob(ctx context.Context, registrar pluginhost.JobsRegistrar, worker productautomation.SecretOperationRecoveryWorker) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil {
		return gerror.New("product automation secret recovery registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	if worker == nil {
		return gerror.New("product automation secret recovery worker is unavailable")
	}
	return registrar.AddWithMetadata(
		ctx,
		productAutomationSecretRecoveryJobPattern,
		productAutomationSecretRecoveryJobName,
		"Product automation secret recovery",
		"Converges owner-private interface secret operations after uncertain or interrupted writes.",
		func(jobCtx context.Context) error {
			_, err := worker.Run(jobCtx, productAutomationSecretRecoveryBatchLimit)
			return err
		},
	)
}
