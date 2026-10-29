// This file schedules lookup-fenced upstream account preparation with bounded single batches.

package composition

import (
	"context"
	"sync/atomic"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-resource/backend/internal/service/principalprovision"
)

const (
	principalProvisionJobPattern     = "@every 1s"
	principalProvisionJobName        = "idc-resource-principal-provision"
	principalProvisionJobDisplayName = "IDC Resource Upstream Principal Provisioning"
	principalProvisionJobDescription = "Performs exact lookup, bounded lookup-fenced creates, verification, and secret cleanup."
	principalProvisionBatchLimit     = 8
)

// RegisterPrincipalProvisionJob binds the owner state machine to the host job lifecycle.
func RegisterPrincipalProvisionJob(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker *principalprovision.Worker,
) error {
	if ctx == nil || ctx.Err() != nil || isNilInterface(registrar) || worker == nil {
		return gerror.New("resource principal provisioning job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	// Coalesce ticks during an active batch; database leases remain the
	// authority across primary-node changes and process failures.
	var running atomic.Bool
	return registrar.AddWithMetadata(
		ctx, principalProvisionJobPattern, principalProvisionJobName,
		principalProvisionJobDisplayName, principalProvisionJobDescription,
		func(jobCtx context.Context) error {
			if !running.CompareAndSwap(false, true) {
				return nil
			}
			defer running.Store(false)
			for index := 0; index < principalProvisionBatchLimit; index++ {
				worked, err := worker.RunOne(jobCtx)
				if err != nil || !worked {
					return err
				}
			}
			return nil
		},
	)
}
