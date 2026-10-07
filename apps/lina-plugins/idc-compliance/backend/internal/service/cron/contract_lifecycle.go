package cron

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-compliance/backend/internal/service/contractlifecycle"
)

const (
	contractLifecyclePattern     = "@every 1m"
	contractLifecycleName        = "idc-compliance-contract-lifecycle"
	contractLifecycleDisplayName = "IDC Contract Lifecycle"
	contractLifecycleDescription = "Consumes one bounded batch of exact Tencent contract status deliveries from the installation-local gateway."
)

// RegisterContractLifecycle registers one primary-node exact-status worker.
func RegisterContractLifecycle(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker contractlifecycle.Service,
) error {
	if ctx == nil || ctx.Err() != nil || nilDependency(registrar) || nilDependency(worker) {
		return gerror.New("contract lifecycle job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(
		ctx, contractLifecyclePattern, contractLifecycleName,
		contractLifecycleDisplayName, contractLifecycleDescription,
		func(jobCtx context.Context) error {
			_, err := worker.Consume(jobCtx, contractlifecycle.MaximumBatchSize)
			return err
		},
	)
}
