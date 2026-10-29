// This file projects completed unpaid closures onto their original accessory intent.

package serveroperation

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

// Completed unpaid cancellation releases only its pending accessory intent.
const (
	accessoryCancelled          accessoryOperationStatus      = "cancelled"
	accessoryCancellationReason accessorySettlementReasonCode = "order_cancelled"
)

type accessoryCancellationConsumer struct {
	database gdb.DB
	evidence commerce.CancellationEvidenceReader
}

// NewAccessoryCancellationConsumer shares the order owner's verified completion reader.
func NewAccessoryCancellationConsumer(database gdb.DB, evidence commerce.CancellationEvidenceReader) (productfulfillment.AddonCancellationConsumer, error) {
	if database == nil || evidence == nil {
		return nil, ErrServiceUnavailable
	}
	return &accessoryCancellationConsumer{database: database, evidence: evidence}, nil
}

type accessoryCancellationRow struct {
	OperationNo, ServiceNo, CustomerNo, InvoiceNo, QuoteNo, CurrencyCode, StatusCode string
	SettlementNo                                                                     string
	Version                                                                          int64
	FinanciallySettledAt, ExecutionStartedAt                                         *time.Time
	FinancialSettlementRequired                                                      bool
	SubjectTypeCode, SubjectNo                                                       string
}

// ConsumeAddonCancellation releases only the local pending intent. It never
// deletes history, dispatches a provider command, or refunds a paid order.
func (s *accessoryCancellationConsumer) ConsumeAddonCancellation(ctx context.Context, input productfulfillment.OrderCancellationInput) (*productfulfillment.OrderCancellationResult, bool, error) {
	if ctx == nil || !validReference(input.OrderNo, 128) || !validReference(input.CancellationNo, 128) {
		return nil, true, ErrInvalidInput
	}
	result := &productfulfillment.OrderCancellationResult{OrderNo: input.OrderNo}
	handled := false
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		// Both independent signals are necessary: a broken addon relation must
		// not be reclassified as a new purchase. Lock order before all other rows.
		var order *struct{ QuoteNo string }
		if err := s.database.Model(dao.CommerceOrder.Table()).Ctx(ctx).Where(do.CommerceOrder{TenantId: 0, OrderNo: input.OrderNo}).Fields("quote_no").LockUpdate().Scan(&order); err != nil {
			return err
		}
		if order == nil {
			return ErrOperationConflict
		}
		owned, err := s.database.GetValue(ctx, accessorySettlementClassificationSQL, order.QuoteNo, input.OrderNo)
		if err != nil || !owned.Bool() {
			return err
		}
		handled = true
		proof, err := s.evidence.GetCompletedCancellationEvidence(ctx, input.OrderNo, input.CancellationNo)
		if err != nil {
			return err
		}
		if proof == nil || proof.OrderNo != input.OrderNo || proof.CancellationNo != input.CancellationNo {
			return ErrOperationConflict
		}
		var rows []accessoryCancellationRow
		query, err := accessoryCancellationQuery(ctx, s.database, input.OrderNo)
		if err != nil {
			return err
		}
		if err = s.database.GetScan(ctx, &rows, query, input.OrderNo); err != nil {
			return err
		}
		if len(rows) != 1 {
			return ErrOperationConflict
		}
		row := rows[0]
		if row.CustomerNo != proof.CustomerNo || row.InvoiceNo != proof.InvoiceNo || row.QuoteNo != proof.QuoteNo || row.CurrencyCode != proof.CurrencyCode ||
			!row.FinancialSettlementRequired || row.SettlementNo != "" || row.FinanciallySettledAt != nil || row.ExecutionStartedAt != nil {
			return ErrOperationConflict
		}
		executed, err := s.database.Model(dao.ServerOperationExecution.Table()).Ctx(ctx).Where(do.ServerOperationExecution{TenantId: 0, OperationNo: row.OperationNo}).Exist()
		if err != nil {
			return err
		}
		if executed {
			return ErrOperationConflict
		}
		evidence, err := digestValue(proof)
		if err != nil {
			return err
		}
		if accessoryOperationStatus(row.StatusCode) == accessoryCancelled {
			verified, readErr := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_business_operation_event
WHERE tenant_id=0 AND operation_no=? AND event_sequence=? AND to_status_code='cancelled' AND reason_code=? AND actor_reference=? AND evidence_digest=?)`,
				row.OperationNo, row.Version, accessoryCancellationReason, proof.CancellationNo, evidence)
			if readErr != nil {
				return readErr
			}
			if !verified.Bool() {
				return ErrOperationConflict
			}
			result.Reused = true
			return nil
		}
		if accessoryOperationStatus(row.StatusCode) != accessoryAwaitingSettlement {
			return ErrOperationConflict
		}
		updated, err := s.database.Model(dao.BusinessOperation.Table()).Ctx(ctx).Where(do.BusinessOperation{TenantId: 0, OperationNo: row.OperationNo, Version: row.Version, StatusCode: accessoryAwaitingSettlement}).Data(do.BusinessOperation{StatusCode: accessoryCancelled, Version: row.Version + 1, CompletedAt: &proof.CompletedAt}).Update()
		if err != nil {
			return err
		}
		if count, countErr := updated.RowsAffected(); countErr != nil {
			return countErr
		} else if count != 1 {
			return ErrOperationConflict
		}
		// Mutual exclusion and package reservations read operation.status_code;
		// retaining the service's last-operation pointer preserves its history.
		if err = s.appendCancellationEvent(ctx, row, proof, evidence); err != nil {
			return err
		}
		result.Cancelled = 1
		return nil
	})
	return result, handled, err
}

const accessoryCancellationSQL = `SELECT op.operation_no,op.service_no,op.customer_no,op.invoice_no,op.quote_no,op.currency_code,
op.status_code,COALESCE(op.settlement_no,'') AS settlement_no,op.financially_settled_at,
op.execution_started_at,op.financial_settlement_required,op.version
FROM plugin_idc_core_business_operation op
JOIN plugin_idc_core_service_instance service ON service.tenant_id=op.tenant_id AND service.service_no=op.service_no
 AND service.customer_no=op.customer_no AND service.currency_code=op.currency_code
JOIN plugin_idc_core_server_accessory_quote quote ON quote.tenant_id=op.tenant_id AND quote.quote_no=op.quote_no
 AND quote.service_no=op.service_no AND quote.customer_no=op.customer_no AND quote.currency_code=op.currency_code
WHERE op.tenant_id=0 AND op.order_no=? AND op.operation_type_code='addon'
LIMIT 2 FOR UPDATE OF op,service`

// appendCancellationEvent uses the generated model and ORM automatic timestamps.
func (s *accessoryCancellationConsumer) appendCancellationEvent(ctx context.Context, row accessoryCancellationRow, proof *commerce.CompletedCancellationEvidence, evidence string) error {
	if row.SubjectTypeCode == nativeAccessorySubject {
		return appendNativeAccessoryEvent(ctx, s.database, row.OperationNo, row.SubjectNo, row.Version+1,
			row.StatusCode, string(accessoryCancelled), string(accessoryCancellationReason), proof.CancellationNo, evidence)
	}
	eventNo, err := deterministicReference("soe_", struct {
		OperationNo string
		Sequence    int
	}{row.OperationNo, int(row.Version + 1)})
	if err != nil {
		return err
	}
	content, err := digestValue(struct {
		EventNo, OperationNo, ServiceNo, CancellationNo, Evidence string
		Sequence                                                  int64
	}{eventNo, row.OperationNo, row.ServiceNo, proof.CancellationNo, evidence, row.Version + 1})
	if err != nil {
		return err
	}
	_, err = s.database.Model(dao.BusinessOperationEvent.Table()).Ctx(ctx).Data(do.BusinessOperationEvent{
		TenantId: 0, EventNo: eventNo, OperationNo: row.OperationNo, ServiceNo: row.ServiceNo, EventSequence: row.Version + 1,
		FromStatusCode: &row.StatusCode, ToStatusCode: accessoryCancelled, ReasonCode: accessoryCancellationReason,
		ActorTypeCode: commerce.OrderCancellationSourceSystem, ActorReference: proof.CancellationNo, EvidenceDigest: evidence, ContentDigest: content,
	}).Insert()
	return err
}
