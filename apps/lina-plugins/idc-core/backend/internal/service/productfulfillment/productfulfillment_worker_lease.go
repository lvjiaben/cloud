// This file keeps a claimed peer task leased while bounded configuration file
// streams are uploaded. Losing the lease cancels the outbound request; recovery
// retains the existing unknown-result fence rather than authorizing a replay.

package productfulfillment

import (
	"context"
	"time"
)

func (w *fulfillmentWorker) keepPeerLease(ctx context.Context, claimed *claimedFulfillment) (context.Context, func()) {
	callCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-callCtx.Done():
				return
			case <-ticker.C:
				if !w.refreshPeerLease(callCtx, claimed) {
					cancel()
					return
				}
			}
		}
	}()
	return callCtx, func() { cancel(); <-done }
}

func (w *fulfillmentWorker) refreshPeerLease(ctx context.Context, claimed *claimedFulfillment) bool {
	result, err := w.database.Exec(ctx, `UPDATE plugin_idc_core_product_fulfillment_task SET lease_expires_at=CURRENT_TIMESTAMP+INTERVAL '60 seconds' WHERE tenant_id=0 AND task_no=? AND lease_token=? AND status_code IN ('querying','submitting') AND lease_expires_at>CURRENT_TIMESTAMP`, claimed.TaskNo, claimed.LeaseToken)
	return err == nil && affected(result) == 1
}
