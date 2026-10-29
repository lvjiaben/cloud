// This file repairs missing local delivery after a technical settlement-event failure.
// Unknown outcomes and orders that already have an intent are never resubmitted.
package productfulfillment

import (
	"context"
	"errors"
	"lina-core/pkg/logger"
)

type settlementRecoveryRow struct{ EventNo, SettlementNo, OrderNo string }

// recoverSettlements reads one bounded local page, then applies the same frozen
// evidence, billing authority, gates, row locks and idempotency as first delivery.
func (w *gateRecoveryWorker) recoverSettlements(ctx context.Context, limit int) (GateRecoveryResult, error) {
	result := GateRecoveryResult{}
	rows, err := w.pendingSettlements(ctx, limit)
	if err != nil {
		return result, err
	}
	if len(rows) == 0 && w.settlementCursor != "" {
		w.settlementCursor = ""
		rows, err = w.pendingSettlements(ctx, limit)
		if err != nil {
			return result, err
		}
	}
	var failures []error
	for _, row := range rows {
		w.settlementCursor = row.EventNo
		result.Selected++
		got, consumeErr := w.settlements.ConsumeFinanciallySettled(ctx, SettlementInput{SettlementNo: row.SettlementNo, OrderNo: row.OrderNo})
		if consumeErr != nil {
			result.Failed++
			failures = append(failures, consumeErr)
			continue
		}
		if got != nil {
			result.Queued += got.TaskCount
			result.Manual += got.ManualCount
		}
		logger.Infof(ctx, "settled order local fulfillment recovered order=%s", row.OrderNo)
	}
	return result, errors.Join(failures...)
}
func (w *gateRecoveryWorker) pendingSettlements(ctx context.Context, limit int) ([]settlementRecoveryRow, error) {
	var rows []settlementRecoveryRow
	err := w.database.GetScan(ctx, &rows, `SELECT event.event_no,settlement.settlement_no,settlement.order_no
 FROM plugin_idc_core_outbox_event event
 JOIN plugin_idc_core_financial_settlement settlement ON settlement.tenant_id=event.tenant_id AND settlement.order_no=event.aggregate_no AND settlement.settlement_no=event.correlation_no
 JOIN plugin_idc_core_commerce_order orders ON orders.tenant_id=settlement.tenant_id AND orders.order_no=settlement.order_no
 JOIN plugin_idc_core_commerce_quote_snapshot quote ON quote.tenant_id=orders.tenant_id AND quote.quote_no=orders.quote_no
 WHERE event.tenant_id=0 AND event.event_type='commerce.order.financially_settled'
 AND event.status_code='dead_letter' AND event.last_error_code='financial_settlement_delivery_unavailable'
 AND orders.status_code='paid' AND quote.purpose_code='new_purchase'
 AND event.event_no>?
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_intent intent WHERE intent.tenant_id=settlement.tenant_id AND intent.order_no=settlement.order_no)
 ORDER BY event.event_no LIMIT ?`, w.settlementCursor, limit)
	return rows, err
}
