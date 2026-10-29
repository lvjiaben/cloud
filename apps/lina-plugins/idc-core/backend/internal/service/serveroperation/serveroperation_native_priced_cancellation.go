// Completed unpaid cancellation closes the original native IP/resize intent
// and releases only its own frozen local capacity. Paid/unknown tasks are fenced.

package serveroperation

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

type serverOperationCancellationConsumer struct {
	database gdb.DB
	evidence commerce.CancellationEvidenceReader
	capacity serveroperationresourcecap.IPCapacity
	addons   productfulfillment.AddonCancellationConsumer
}

// NewServerOperationCancellationConsumer reuses the actual order owner and the
// startup resource capacity owner. It has no provider command or payment writer.
func NewServerOperationCancellationConsumer(database gdb.DB, evidence commerce.CancellationEvidenceReader, capacity serveroperationresourcecap.IPCapacity) (productfulfillment.AddonCancellationConsumer, error) {
	if nilServerOperationDependency(database) || nilServerOperationDependency(evidence) {
		return nil, ErrServiceUnavailable
	}
	addons, err := NewAccessoryCancellationConsumer(database, evidence)
	if err != nil {
		return nil, err
	}
	return &serverOperationCancellationConsumer{database: database, evidence: evidence, capacity: capacity, addons: addons}, nil
}

func (s *serverOperationCancellationConsumer) ConsumeAddonCancellation(ctx context.Context, input productfulfillment.OrderCancellationInput) (*productfulfillment.OrderCancellationResult, bool, error) {
	owned, err := nativePricedOrderOwned(ctx, s.database, input.OrderNo)
	if err != nil {
		return nil, true, err
	}
	if !owned {
		return s.addons.ConsumeAddonCancellation(ctx, input)
	}
	if ctx == nil || ctx.Err() != nil || !validReference(input.OrderNo, 128) || !validReference(input.CancellationNo, 128) {
		return nil, true, ErrInvalidInput
	}
	result := &productfulfillment.OrderCancellationResult{OrderNo: input.OrderNo}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, err := s.database.GetValue(ctx, `SELECT order_no FROM plugin_idc_core_commerce_order WHERE tenant_id=0 AND order_no=? FOR UPDATE`, input.OrderNo); err != nil {
			return err
		}
		proof, err := s.evidence.GetCompletedCancellationEvidence(ctx, input.OrderNo, input.CancellationNo)
		if err != nil {
			return err
		}
		if proof == nil || proof.OrderNo != input.OrderNo || proof.CancellationNo != input.CancellationNo {
			return ErrOperationConflict
		}
		var rows []struct {
			OperationNo, QuoteNo, CustomerNo, CurrencyCode, InvoiceNo, StatusCode, SettlementNo, ReservationNo string
			Version                                                                                            int64
			Financial                                                                                          bool
			FinanciallySettledAt, ExecutionStartedAt                                                           *time.Time
			TaskNo                                                                                             string
		}
		if err = s.database.GetScan(ctx, &rows, `SELECT op.operation_no,op.quote_no,op.customer_no,op.currency_code,op.invoice_no,op.status_code,op.version,op.financial_settlement_required AS financial,COALESCE(op.settlement_no,'') AS settlement_no,op.financially_settled_at,op.execution_started_at,COALESCE(op.provider_task_no,'') AS task_no,COALESCE(q.resource_reservation_no,'') AS reservation_no FROM plugin_idc_core_business_operation op JOIN plugin_idc_core_channel_service_quote q ON q.tenant_id=op.tenant_id AND q.quote_no=op.quote_no AND q.source_code='native' AND q.service_no=op.subject_no AND q.service_owner_customer_no=op.customer_no JOIN plugin_idc_core_service_entitlement e ON e.tenant_id=op.tenant_id AND e.entitlement_no=op.subject_no AND e.customer_no=op.customer_no AND e.currency_code=op.currency_code AND e.entitlement_owner_id='idc-resource' WHERE op.tenant_id=0 AND op.order_no=? AND op.subject_type_code='entitlement' AND op.service_no IS NULL AND op.operation_type_code=q.action_code LIMIT 2 FOR UPDATE OF op,e`, input.OrderNo); err != nil {
			return err
		}
		if len(rows) != 1 {
			return ErrOperationConflict
		}
		row := rows[0]
		if row.QuoteNo != proof.QuoteNo || row.CustomerNo != proof.CustomerNo || row.CurrencyCode != proof.CurrencyCode || row.InvoiceNo != proof.InvoiceNo || !row.Financial || row.SettlementNo != "" || row.FinanciallySettledAt != nil || row.ExecutionStartedAt != nil || row.TaskNo != "" {
			return ErrOperationConflict
		}
		evidence, err := digestValue(proof)
		if err != nil {
			return err
		}
		if row.StatusCode == "cancelled" {
			valid, readErr := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_business_operation_event WHERE tenant_id=0 AND operation_no=? AND event_sequence=? AND to_status_code='cancelled' AND reason_code='order_cancelled' AND actor_reference=? AND evidence_digest=?)`, row.OperationNo, row.Version, proof.CancellationNo, evidence)
			if readErr != nil {
				return readErr
			}
			if !valid.Bool() {
				return ErrOperationConflict
			}
			result.Reused = true
		} else {
			if row.StatusCode != "awaiting_financial_settlement" {
				return ErrOperationConflict
			}
			updated, updateErr := s.database.Model(dao.BusinessOperation.Table()).Ctx(ctx).Where(do.BusinessOperation{TenantId: 0, OperationNo: row.OperationNo, Version: row.Version, StatusCode: "awaiting_financial_settlement"}).Data(do.BusinessOperation{StatusCode: "cancelled", Version: row.Version + 1, CompletedAt: &proof.CompletedAt}).Update()
			if updateErr != nil {
				return updateErr
			}
			count, countErr := updated.RowsAffected()
			if countErr != nil {
				return countErr
			}
			if count != 1 {
				return ErrOperationConflict
			}
			subject, err := s.database.GetValue(ctx, `SELECT subject_no FROM plugin_idc_core_business_operation WHERE tenant_id=0 AND operation_no=?`, row.OperationNo)
			if err != nil {
				return err
			}
			if err = appendNativeAccessoryEvent(ctx, s.database, row.OperationNo, subject.String(), row.Version+1, row.StatusCode, "cancelled", "order_cancelled", proof.CancellationNo, evidence); err != nil {
				return err
			}
			result.Cancelled = 1
		}
		return s.releaseNativePricedReservation(ctx, row.ReservationNo, row.OperationNo)
	})
	return result, true, err
}

// releaseNativePricedReservation is local capacity compensation only after the
// order owner's completed unpaid cancellation has been verified transactionally.
func (s *serverOperationCancellationConsumer) releaseNativePricedReservation(ctx context.Context, reservationNo, operationNo string) error {
	if reservationNo != "" {
		capacity, ok := s.capacity.(serveroperationresourcecap.IPCommercialHold)
		if !ok || nilServerOperationDependency(capacity) {
			return ErrServiceUnavailable
		}
		released, releaseErr := capacity.ReleaseIPCommercial(ctx, serveroperationresourcecap.IPCommercialReleaseRequest{ReservationNo: reservationNo, OperationNo: operationNo, ReasonCode: "core_order_cancelled"})
		if releaseErr != nil {
			return releaseErr
		}
		if released == nil || released.ReservationNo != reservationNo || released.Status != serveroperationresourcecap.IPReservationReleased {
			return ErrOperationConflict
		}
	}
	return nil
}
