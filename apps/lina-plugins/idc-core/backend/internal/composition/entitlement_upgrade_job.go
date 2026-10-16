// This file registers bounded durable upgrade reconciliation through the host's public job capability.
package composition

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/entitlementupgrade"
)

// RegisterEntitlementUpgradeJob shares one startup-owned worker and delegates primary-node scheduling.
func RegisterEntitlementUpgradeJob(ctx context.Context, registrar pluginhost.JobsRegistrar, worker entitlementupgrade.Worker) error {
	if registrar == nil || worker == nil {
		return gerror.New("upgrade job dependencies unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(ctx, "@every 10s", "idc-core-entitlement-upgrade", "IDC Configuration Upgrades", "Reconcile paid service upgrades without repeating payment.", func(ctx context.Context) error { _, err := worker.Run(ctx, 20); return err })
}
