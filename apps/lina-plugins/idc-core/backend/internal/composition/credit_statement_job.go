// This file registers the primary-node bounded credit statement scheduler.

package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

const (
	creditStatementJobPattern     = "@every 1m"
	creditStatementJobName        = "idc-core-credit-statements"
	creditStatementJobDisplayName = "IDC Credit Statements"
	creditStatementJobDescription = "Issues one bounded merchant-timezone credit statement batch."
	creditStatementJobBatchLimit  = 32
)

type creditStatementBatchWorker interface {
	RunCreditStatementBatch(context.Context, int) (*billing.CreditStatementBatch, error)
}

// RegisterCreditStatementJob registers one primary-node billing worker and
// propagates worker failures so the host retains an observable job result.
func RegisterCreditStatementJob(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker creditStatementBatchWorker,
) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("credit statement job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(
		ctx, creditStatementJobPattern, creditStatementJobName,
		creditStatementJobDisplayName, creditStatementJobDescription,
		func(jobCtx context.Context) error {
			_, err := worker.RunCreditStatementBatch(jobCtx, creditStatementJobBatchLimit)
			return err
		},
	)
}
