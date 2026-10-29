// Package cron registers the resource plugin's asynchronous supplier-option
// refresh. Shared PostgreSQL leases coordinate work across process restarts.
package cron

import (
	"context"
	"errors"
	"lina-plugin-idc-resource/backend/internal/service/customeroperations"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-resource/backend/internal/service/interfacecandidate"
	"lina-plugin-idc-resource/backend/internal/service/nativecommercial"
	"lina-plugin-idc-resource/backend/internal/service/serverfacts"
)

// Start attaches the already-bound candidate source to the host job lifecycle.
// No independent service graph, unmanaged goroutine or supplier write is created.
func Start(ctx context.Context, jobs pluginhost.JobsRegistrar, candidates *interfacecandidate.LazySource) error {
	if ctx == nil || ctx.Err() != nil || jobs == nil || candidates == nil {
		return gerror.New("supplier option refresh dependencies are unavailable")
	}
	if !jobs.IsPrimaryNode() {
		return nil
	}
	return jobs.AddWithMetadata(ctx, "@every 5s", "idc-resource-supplier-options", "Supplier option synchronization",
		"Refreshes shared secret-free region, node and image projections without blocking purchase pages.",
		func(jobCtx context.Context) error {
			_, err := candidates.RunCandidateRefresh(jobCtx)
			return err
		})
}

// StartServerFacts refreshes at most eight native mappings per scheduled batch.
// SQL leases coordinate concurrent workers; only read-only provider calls occur.
func StartServerFacts(ctx context.Context, jobs pluginhost.JobsRegistrar, facts serverfacts.Service) error {
	if ctx == nil || ctx.Err() != nil || jobs == nil || facts == nil {
		return gerror.New("native server facts job dependencies unavailable")
	}
	if !jobs.IsPrimaryNode() {
		return nil
	}
	return jobs.AddWithMetadata(ctx, "@every 5s", "idc-resource-native-server-facts", "Native server observations", "Refresh bounded historical server facts without blocking customer lists.", func(jobCtx context.Context) error {
		for index := 0; index < 8; index++ {
			worked, err := facts.RunOne(jobCtx)
			if err != nil || !worked {
				return err
			}
		}
		return nil
	})
}

// StartNativeCommerce connects ready intents and verified receipts to the single resource executor.
func StartNativeCommerce(ctx context.Context, jobs pluginhost.JobsRegistrar, worker *nativecommercial.Service) error {
	if ctx == nil || ctx.Err() != nil || jobs == nil || worker == nil {
		return gerror.New("native commercial job dependencies unavailable")
	}
	if !jobs.IsPrimaryNode() {
		return nil
	}
	return jobs.AddWithMetadata(ctx, "@every 2s", "idc-resource-native-commerce", "Native commercial resource tasks", "Binds settled native operations and reports durable outcomes without another provider executor.", func(jobCtx context.Context) error {
		readyErr := worker.RunReady(jobCtx)
		outcomeErr := worker.RunOutcomes(jobCtx)
		return errors.Join(readyErr, outcomeErr)
	})
}

// StartNativeWriteReceipts reconciles actual task outcomes without cloud calls.
func StartNativeWriteReceipts(ctx context.Context, jobs pluginhost.JobsRegistrar, worker *customeroperations.WriteReconciler) error {
	if ctx == nil || ctx.Err() != nil || jobs == nil || worker == nil {
		return gerror.New("native write receipt job dependencies unavailable")
	}
	if !jobs.IsPrimaryNode() {
		return nil
	}
	return jobs.AddWithMetadata(ctx, "@every 2s", "idc-resource-native-write-receipts", "Native management write receipts", "Reconciles durable management outcomes and retains uncertain execution fences.", worker.Run)
}
