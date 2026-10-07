package cron

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-compliance/backend/internal/service/contractarchive"
)

const (
	contractArchivePattern     = "@every 1m"
	contractArchiveName        = "idc-compliance-contract-archive"
	contractArchiveDisplayName = "IDC Contract Archive"
	contractArchiveDescription = "Archives completed Tencent contract files through the installation-local gateway into private object storage."
)

// RegisterContractArchive registers the bounded primary-node archive worker.
func RegisterContractArchive(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker contractarchive.Service,
) error {
	if ctx == nil || ctx.Err() != nil || nilDependency(registrar) || nilDependency(worker) {
		return gerror.New("contract archive job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(
		ctx, contractArchivePattern, contractArchiveName,
		contractArchiveDisplayName, contractArchiveDescription,
		func(jobCtx context.Context) error {
			_, err := worker.Run(jobCtx, contractarchive.MaximumBatchSize)
			return err
		},
	)
}
