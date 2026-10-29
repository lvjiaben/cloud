// This file registers bounded product-fulfillment jobs with primary-node lifecycle governance.

package composition

import (
	"context"
	"sync/atomic"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

const productFulfillmentBatchLimit = 20

// productFulfillmentGateRecoveryBatchLimit bounds authoritative gate rechecks per pass.
const productFulfillmentGateRecoveryBatchLimit = 20

// RegisterProductFulfillmentGateRecoveryJob revisits paid intents after risk or contract approval.
func RegisterProductFulfillmentGateRecoveryJob(ctx context.Context, registrar pluginhost.JobsRegistrar, worker productfulfillment.GateRecoveryWorker) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("product fulfillment gate recovery registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(ctx, "@every 5s", "idc-core-product-fulfillment-gate-recovery", "IDC Product Fulfillment Gate Recovery", "Rechecks frozen owner gates for already-paid generic orders.", func(jobCtx context.Context) error {
		_, err := worker.Run(jobCtx, productFulfillmentGateRecoveryBatchLimit)
		return err
	})
}

const productFulfillmentCancellationBatchLimit = 20

const productFulfillmentResourceOutcomeBatchLimit = 20

func RegisterProductFulfillmentJob(ctx context.Context, registrar pluginhost.JobsRegistrar, worker productfulfillment.Worker) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("product fulfillment job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	// A faster clock must not stack concurrent batches during a slow provider call.
	var running atomic.Bool
	return registrar.AddWithMetadata(ctx, "@every 1s", "idc-core-product-fulfillment", "IDC Product Fulfillment", "Converges frozen generic product fulfillment tasks.", func(jobCtx context.Context) error {
		if !running.CompareAndSwap(false, true) {
			return nil
		}
		defer running.Store(false)
		_, err := worker.Run(jobCtx, productFulfillmentBatchLimit)
		return err
	})
}

// RegisterProductFulfillmentResourceOutcomeJob installs durable post-commit
// reports to the resource owner on the primary node.
func RegisterProductFulfillmentResourceOutcomeJob(ctx context.Context, registrar pluginhost.JobsRegistrar, worker productfulfillment.ResourceOutcomeWorker) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("product fulfillment resource outcome job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	// A faster clock must not stack concurrent batches during a slow provider call.
	var running atomic.Bool
	return registrar.AddWithMetadata(ctx, "@every 1s", "idc-core-product-fulfillment-resource-outcome", "IDC Product Fulfillment Resource Outcome", "Retries post-commit resource-owner outcome reports.", func(jobCtx context.Context) error {
		if !running.CompareAndSwap(false, true) {
			return nil
		}
		defer running.Store(false)
		_, err := worker.Run(jobCtx, productFulfillmentResourceOutcomeBatchLimit)
		return err
	})
}

func RegisterProductFulfillmentCancellationJob(ctx context.Context, registrar pluginhost.JobsRegistrar, worker productfulfillment.CancellationWorker) error {
	if ctx == nil || ctx.Err() != nil || registrar == nil || worker == nil {
		return gerror.New("product fulfillment cancellation job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(ctx, "@every 5s", "idc-core-product-fulfillment-cancellation", "IDC Product Fulfillment Cancellation", "Applies authoritative order cancellation facts to generic fulfillment.", func(jobCtx context.Context) error {
		_, err := worker.Run(jobCtx, productFulfillmentCancellationBatchLimit)
		return err
	})
}
