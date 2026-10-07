package composition

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-resource/backend/internal/service/principalprovision"
)

const (
	principalProvisionJobPattern     = "@every 5s"
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
	return registrar.AddWithMetadata(
		ctx, principalProvisionJobPattern, principalProvisionJobName,
		principalProvisionJobDisplayName, principalProvisionJobDescription,
		func(jobCtx context.Context) error {
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
