// This file recovers owner-gated paid intents through the authoritative release command.
// Keyset rotation advances even on errors so an unavailable owner cannot starve later items.
package productfulfillment

import (
	"context"
	"errors"
	"sync"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

// GateRecoveryWorker revisits a bounded set of pending owner gates on the primary node.
type GateRecoveryWorker interface {
	// Run rechecks at most limit authoritative pending items (1..100), returning partial progress with joined failures.
	Run(context.Context, int) (GateRecoveryResult, error)
}

// GateRecoveryResult reports attempted records and formal release outcomes without details.
type GateRecoveryResult struct{ Selected, Queued, Manual, Failed int }

type gateRecoveryWorker struct {
	database         gdb.DB
	settlements      SettlementConsumer
	settlementCursor string
	owner            OwnerGateReleaser
	mutex            sync.Mutex
	order, item      string
}

// NewOwnerGateReleaser binds the same frozen validator and gates used by settlement delivery.
func NewOwnerGateReleaser(database gdb.DB, frozen productautomation.FrozenCustomerConfigurationValidator, gates OwnerGateValidator, reasons SystemTransitionReasonCatalog) (OwnerGateReleaser, error) {
	if database == nil || frozen == nil || gates == nil || reasons == nil {
		return nil, gerror.New("owner gate recovery requires database, frozen validator, gates and system reasons")
	}
	return &serviceImpl{database: database, frozen: frozen, gates: gates, systemReasons: reasons}, nil
}

// NewGateRecoveryWorker binds the authoritative owner command and shared database.
func NewGateRecoveryWorker(database gdb.DB, owner OwnerGateReleaser) (GateRecoveryWorker, error) {
	if database == nil || owner == nil {
		return nil, gerror.New("owner gate recovery requires database and releaser")
	}
	return &gateRecoveryWorker{database: database, owner: owner}, nil
}

type gateRecoveryItem struct{ OrderNo, OrderItemNo string }

// Run rotates the keyset after every attempted item, including unchanged and failed gates.
// Concurrent callbacks share one cursor; cross-instance retries converge under the owner's row locks.
func (w *gateRecoveryWorker) Run(ctx context.Context, limit int) (GateRecoveryResult, error) {
	result := GateRecoveryResult{}
	if w == nil || w.database == nil || w.owner == nil || ctx == nil || ctx.Err() != nil || limit < 1 || limit > 100 {
		return result, gerror.New("owner gate recovery batch invalid")
	}
	w.mutex.Lock()
	defer w.mutex.Unlock()
	rows, err := w.selectPending(ctx, limit)
	if err != nil {
		return result, err
	}
	if len(rows) == 0 && w.order != "" {
		w.order, w.item = "", ""
		rows, err = w.selectPending(ctx, limit)
		if err != nil {
			return result, err
		}
	}
	var failures []error
	if w.settlements != nil {
		recovered, recoveryErr := w.recoverSettlements(ctx, limit)
		result.Selected += recovered.Selected
		result.Queued += recovered.Queued
		result.Manual += recovered.Manual
		result.Failed += recovered.Failed
		if recoveryErr != nil {
			failures = append(failures, recoveryErr)
		}
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return result, errors.Join(append(failures, ctx.Err())...)
		}
		w.order, w.item = row.OrderNo, row.OrderItemNo
		result.Selected++
		released, releaseErr := w.owner.ReleaseOwnerGates(ctx, OwnerGateReleaseInput{OrderNo: row.OrderNo, OrderItemNos: []string{row.OrderItemNo}})
		if releaseErr != nil {
			result.Failed++
			failures = append(failures, releaseErr)
			continue
		}
		if released == nil {
			result.Failed++
			failures = append(failures, gerror.New("owner gate release returned no outcome"))
			continue
		}
		result.Queued += released.TaskCount
		result.Manual += released.ManualCount
	}
	return result, errors.Join(failures...)
}

func (w *gateRecoveryWorker) selectPending(ctx context.Context, limit int) ([]gateRecoveryItem, error) {
	var rows []gateRecoveryItem
	err := w.database.GetScan(ctx, &rows, `SELECT DISTINCT order_no,order_item_no FROM plugin_idc_core_product_fulfillment_intent
 WHERE tenant_id=0 AND strategy_code='payment_confirmed_plugin' AND status_code='gate_pending'
 AND (order_no,order_item_no)>(?::text,?::text) ORDER BY order_no,order_item_no LIMIT ?`, w.order, w.item, limit)
	return rows, err
}

// NewSettlementGateRecoveryWorker also repairs locally settled orders whose
// technical delivery failure exhausted its event budget before any intent existed.
// It reuses the authoritative consumer; it never replays an upstream write.
func NewSettlementGateRecoveryWorker(database gdb.DB, owner OwnerGateReleaser, settlements SettlementConsumer) (GateRecoveryWorker, error) {
	if database == nil || owner == nil || settlements == nil {
		return nil, gerror.New("settlement recovery requires database and authoritative consumers")
	}
	return &gateRecoveryWorker{database: database, owner: owner, settlements: settlements}, nil
}
