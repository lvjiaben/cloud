// This file registers the bounded generic refund-entitlement termination worker.

package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

const (
	refundEntitlementJobPattern     = "@every 10s"
	refundEntitlementJobName        = "idc-core-refund-entitlements"
	refundEntitlementJobDisplayName = "IDC Refund Entitlement Termination"
	refundEntitlementJobDescription = "Terminates bounded generic long-lived entitlements after confirmed refunds."
	refundEntitlementJobBatchLimit  = 20
)

// RegisterRefundEntitlementJob installs the durable consumer only on the primary node.
func RegisterRefundEntitlementJob(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker billing.RefundEntitlementWorker,
) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("refund entitlement job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(
		ctx, refundEntitlementJobPattern, refundEntitlementJobName,
		refundEntitlementJobDisplayName, refundEntitlementJobDescription,
		func(jobCtx context.Context) error {
			_, err := worker.Run(jobCtx, refundEntitlementJobBatchLimit)
			return err
		},
	)
}
