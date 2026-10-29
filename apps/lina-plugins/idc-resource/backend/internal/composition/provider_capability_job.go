// This file schedules bounded capability discovery through the managed job lifecycle.
package composition

import (
	"context"
	"errors"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-resource/backend/internal/service/providercapability"
)

const providerCapabilityBatchLimit = 8

// RegisterProviderCapabilityJob keeps collection inside the resource owner;
// disabling the plugin stops the job while preserving historical snapshots.
func RegisterProviderCapabilityJob(ctx context.Context, registrar pluginhost.JobsRegistrar, collector providercapability.Service) error {
	if ctx == nil || ctx.Err() != nil || isNilInterface(registrar) || collector == nil {
		return gerror.New("provider capability job dependencies are required")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(ctx, "@every 30s", "idc-resource-provider-capability-refresh", "IDC Resource Provider Capabilities", "Collects complete enabled connector capabilities without supplier writes.", func(ctx context.Context) error {
		var failures []error
		for range providerCapabilityBatchLimit {
			worked, err := collector.RunOne(ctx)
			if err != nil {
				if worked && errors.Is(err, providercapability.ErrDiscoveryFailed) {
					failures = append(failures, err)
					continue
				}
				return errors.Join(append(failures, err)...)
			}
			if !worked {
				break
			}
		}
		return errors.Join(failures...)
	})
}
