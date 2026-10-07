package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/customeractivation"
)

func RegisterCustomerActivationExpiryJob(ctx context.Context, registrar pluginhost.JobsRegistrar, worker customeractivation.ExpiryWorker) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("customer activation expiry job requires registrar and worker")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(ctx, "@every 1m", "idc-core-customer-activation-expiry",
		"IDC Customer Activation Expiry", "Marks expired single-use customer activation invitations with authoritative database time.",
		func(jobCtx context.Context) error {
			_, err := worker.Expire(jobCtx, 100)
			return err
		})
}
