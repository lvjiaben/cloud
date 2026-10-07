package cron

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-compliance/backend/internal/service/contractfailure"
)

const (
	contractFailurePattern     = "@every 1m"
	contractFailureName        = "idc-compliance-contract-failure"
	contractFailureDisplayName = "IDC Contract Failure Compensation"
	contractFailureDescription = "Stops due required-contract orders and creates one core-owned refund handoff."
)

func RegisterContractFailure(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker contractfailure.Service,
) error {
	if ctx == nil || ctx.Err() != nil || nilDependency(registrar) || nilDependency(worker) {
		return gerror.New("contract failure job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(
		ctx, contractFailurePattern, contractFailureName,
		contractFailureDisplayName, contractFailureDescription,
		func(jobCtx context.Context) error {
			_, err := worker.ProcessDue(jobCtx, contractfailure.MaximumBatchSize)
			return err
		},
	)
}
