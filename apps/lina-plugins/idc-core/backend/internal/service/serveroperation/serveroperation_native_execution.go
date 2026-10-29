// Native execution binds one core intent to one resource task. The callbacks
// are transaction-local persistence, never provider network operations.
package serveroperation

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type nativeExecutionRow struct {
	OperationNo, SubjectNo, CustomerNo, CurrencyCode, OperationTypeCode                  string
	OrderNo, InvoiceNo, QuoteNo, SettlementNo, StatusCode, ProviderTaskNo, ContentDigest string
	Version, ExpectedSubjectVersion                                                      int64
	FinancialSettlementRequired                                                          bool
	FinanciallySettledAt                                                                 *time.Time
	Operation                                                                            servercommercialcap.Operation
	Target                                                                               servercommercialcap.TargetSnapshot
	Priced                                                                               *nativePricedOperation
	QuoteContentDigest                                                                   string
	AmountMinor                                                                          int64
}

func (s *nativeCommercialService) ListReady(ctx context.Context, limit int) ([]string, error) {
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > 100 || gdb.TXFromCtx(ctx, s.database.GetGroup()) != nil {
		return nil, PublicError(ErrInvalidInput)
	}
	var rows []struct{ OperationNo string }
	err := s.database.GetScan(ctx, &rows, `WITH candidates AS (SELECT op.operation_no FROM plugin_idc_core_business_operation op
JOIN plugin_idc_core_service_entitlement e ON e.tenant_id=op.tenant_id AND e.entitlement_no=op.subject_no
AND e.customer_no=op.customer_no AND e.currency_code=op.currency_code AND e.entitlement_owner_id='idc-resource'
WHERE op.tenant_id=0 AND op.subject_type_code='entitlement' AND op.resource_owner_code='idc-resource'
AND e.status_code='active' AND e.version=op.expected_subject_version AND (e.expires_at IS NULL OR e.expires_at>CURRENT_TIMESTAMP)
AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_refund_entitlement_lock l WHERE l.tenant_id=e.tenant_id AND l.entitlement_no=e.entitlement_no AND l.released_at IS NULL)
AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_refund_request r WHERE r.tenant_id=op.tenant_id AND r.order_no=op.order_no AND r.status_code<>'rejected')
AND op.status_code='ready' AND (op.operation_type_code='addon' OR (op.operation_type_code IN ('resize','ip_change')
AND EXISTS(SELECT 1 FROM plugin_idc_core_channel_service_quote q WHERE q.tenant_id=op.tenant_id AND q.quote_no=op.quote_no AND q.source_code='native' AND q.effective_at<=CURRENT_TIMESTAMP)))
ORDER BY op.updated_at,op.operation_no LIMIT ? FOR UPDATE OF op SKIP LOCKED)
UPDATE plugin_idc_core_business_operation op SET updated_at=clock_timestamp()
FROM candidates WHERE op.tenant_id=0 AND op.operation_no=candidates.operation_no RETURNING op.operation_no`, limit)
	if err != nil {
		return nil, PublicError(err)
	}
	result := make([]string, len(rows))
	for i := range rows {
		result[i] = rows[i].OperationNo
	}
	return result, nil
}

func (s *nativeCommercialService) WithReadyOperation(ctx context.Context, number string, enqueue func(context.Context, servercommercialcap.Operation) (string, error)) error {
	if ctx == nil || ctx.Err() != nil || enqueue == nil || !validReference(number, 40) {
		return PublicError(ErrInvalidInput)
	}
	return PublicError(s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := s.lockNativeExecution(ctx, number)
		if err != nil {
			return err
		}
		if row.StatusCode == "executing" && row.ProviderTaskNo != "" {
			return nil
		}
		if row.StatusCode != "ready" || row.ProviderTaskNo != "" {
			return ErrOperationConflict
		}
		if err = s.admitNativeExecution(ctx, row); err != nil {
			return err
		}
		if row.Priced != nil {
			if err = s.nativePreparePricedExecution(ctx, row.Priced, ""); err != nil {
				return err
			}
		}
		task, err := enqueue(ctx, row.Operation)
		if err != nil {
			return err
		}
		if !validReference(task, 64) {
			return ErrOperationConflict
		}
		if row.Priced != nil {
			if err = s.nativePreparePricedExecution(ctx, row.Priced, task); err != nil {
				return err
			}
		}
		evidence, evidenceNo := row.QuoteContentDigest, row.QuoteNo
		if err = execOne(ctx, s.database, `UPDATE plugin_idc_core_business_operation SET status_code='executing',
execution_mode_code='provider_confirmed',channel_no=?,capability_evidence_no=?,capability_evidence_digest=?,
provider_task_no=?,execution_started_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND operation_no=? AND status_code='ready' AND version=?`, row.Target.ChannelNo, evidenceNo, evidence, task, number, row.Version); err != nil {
			return err
		}
		return appendNativeAccessoryEvent(ctx, s.database, number, row.SubjectNo, row.Version+1, "ready", "executing", "server_commercial.task_bound", task, evidence)
	}))
}

func (s *nativeCommercialService) WithQueuedOperation(ctx context.Context, number, task string, claim func(context.Context, servercommercialcap.Operation) error) error {
	if ctx == nil || ctx.Err() != nil || claim == nil || !validReference(number, 40) || !validReference(task, 64) {
		return PublicError(ErrInvalidInput)
	}
	return PublicError(s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := s.lockNativeExecution(ctx, number)
		if err != nil {
			return err
		}
		if row.StatusCode != "executing" || row.ProviderTaskNo != task {
			return ErrOperationConflict
		}
		if err = s.admitNativeExecution(ctx, row); err != nil {
			return err
		}
		return claim(ctx, row.Operation)
	}))
}

const nativeExecutionSQL = `SELECT operation_no,subject_no,customer_no,currency_code,operation_type_code,
COALESCE(order_no,'') AS order_no,COALESCE(invoice_no,'') AS invoice_no,quote_no,
COALESCE(settlement_no,'') AS settlement_no,status_code,COALESCE(provider_task_no,'') AS provider_task_no,
content_digest,version,expected_subject_version,financial_settlement_required,financially_settled_at
FROM plugin_idc_core_business_operation WHERE tenant_id=0 AND operation_no=?
AND subject_type_code='entitlement' AND service_no IS NULL AND resource_owner_code='idc-resource'
AND operation_type_code IN ('addon','resize','ip_change')`

// lockNativeExecution uses the financial owner's order/invoice/customer locks,
// then parent entitlement and operation. Immutable identity is compared again
// after taking the locks so a stale pre-lock projection cannot choose the scope.
func (s *nativeCommercialService) lockNativeExecution(ctx context.Context, number string) (*nativeExecutionRow, error) {
	var first *nativeExecutionRow
	if err := s.database.GetScan(ctx, &first, nativeExecutionSQL, number); err != nil {
		return nil, err
	}
	if first == nil {
		return nil, ErrOperationConflict
	}
	if first.OrderNo != "" {
		if _, err := s.database.GetValue(ctx, `SELECT order_no FROM plugin_idc_core_commerce_order WHERE tenant_id=0 AND order_no=? FOR UPDATE`, first.OrderNo); err != nil {
			return nil, err
		}
		if _, err := s.database.GetValue(ctx, `SELECT invoice_no FROM plugin_idc_core_billing_invoice WHERE tenant_id=0 AND invoice_no=? AND order_no=? FOR UPDATE`, first.InvoiceNo, first.OrderNo); err != nil {
			return nil, err
		}
	}
	if _, err := s.database.GetValue(ctx, `SELECT customer_no FROM plugin_idc_core_customer_subject WHERE tenant_id=0 AND customer_no=? FOR UPDATE`, first.CustomerNo); err != nil {
		return nil, err
	}
	if _, err := s.database.GetValue(ctx, `SELECT entitlement_no FROM plugin_idc_core_service_entitlement WHERE tenant_id=0 AND entitlement_no=? AND customer_no=? AND entitlement_owner_id='idc-resource' FOR UPDATE`, first.SubjectNo, first.CustomerNo); err != nil {
		return nil, err
	}
	var row *nativeExecutionRow
	if err := s.database.GetScan(ctx, &row, nativeExecutionSQL+" FOR UPDATE", number); err != nil {
		return nil, err
	}
	if row == nil || first.SubjectNo != row.SubjectNo || first.CustomerNo != row.CustomerNo || first.OrderNo != row.OrderNo || first.InvoiceNo != row.InvoiceNo || first.QuoteNo != row.QuoteNo {
		return nil, ErrOperationConflict
	}
	if row.OperationTypeCode != "addon" {
		priced, err := s.loadNativePricedOperation(ctx, number)
		if err != nil {
			return nil, err
		}
		if priced == nil {
			return nil, ErrOperationConflict
		}
		row.Priced, row.Operation, row.Target, row.AmountMinor = priced, priced.Operation, priced.Target, priced.Price.TotalAmountMinor
		row.QuoteContentDigest = priced.Quote.ContentDigest
		return row, nil
	}
	var q *struct {
		NativeTargetSnapshot, Parameters, ActionCode, ContentDigest string
		TotalAmountMinor, ExpectedSubjectVersion                    int64
	}
	if err := s.database.GetScan(ctx, &q, `SELECT native_target_snapshot::TEXT AS native_target_snapshot,parameters::TEXT AS parameters,action_code,total_amount_minor,expected_subject_version,content_digest
FROM plugin_idc_core_server_accessory_quote WHERE tenant_id=0 AND quote_no=? AND subject_type_code='entitlement' AND subject_no=? AND customer_no=? AND currency_code=? AND service_no IS NULL`, row.QuoteNo, row.SubjectNo, row.CustomerNo, row.CurrencyCode); err != nil {
		return nil, err
	}
	if q == nil || json.Unmarshal([]byte(q.NativeTargetSnapshot), &row.Target) != nil || row.Target.EntitlementNo != row.SubjectNo || row.Target.CustomerNo != row.CustomerNo || row.Target.EntitlementVersion != row.ExpectedSubjectVersion || q.ExpectedSubjectVersion != row.ExpectedSubjectVersion || row.Target.LocalResourceNo == "" {
		return nil, ErrOperationConflict
	}
	digest, err := nativeAccessoryIntentDigest(number, row.QuoteNo, row.OrderNo, row.InvoiceNo, row.SubjectNo, q.ActionCode, q.Parameters, row.FinancialSettlementRequired, row.ExpectedSubjectVersion)
	if err != nil || digest != row.ContentDigest {
		return nil, ErrOperationConflict
	}
	row.AmountMinor = q.TotalAmountMinor
	row.QuoteContentDigest = q.ContentDigest
	row.Operation = servercommercialcap.Operation{OperationNo: number, EntitlementNo: row.SubjectNo, CustomerNo: row.CustomerNo, LocalResourceNo: row.Target.LocalResourceNo, ChannelNo: row.Target.ChannelNo, PoolNo: row.Target.PoolNo, ExpectedEntitlementVersion: row.ExpectedSubjectVersion, Action: serveroperationresourcecap.Action(q.ActionCode), Parameters: json.RawMessage(q.Parameters), ContentDigest: row.ContentDigest, QuoteNo: row.QuoteNo}
	return row, nil
}

func (s *nativeCommercialService) admitNativeExecution(ctx context.Context, row *nativeExecutionRow) error {
	allowed, err := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_service_entitlement e
JOIN plugin_idc_core_customer_subject c ON c.tenant_id=e.tenant_id AND c.customer_no=e.customer_no
WHERE e.tenant_id=0 AND e.entitlement_no=? AND e.customer_no=? AND e.currency_code=? AND e.version=?
AND e.entitlement_owner_id='idc-resource' AND e.status_code='active' AND c.status_code='active'
AND (e.expires_at IS NULL OR e.expires_at>CURRENT_TIMESTAMP)
AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_refund_entitlement_lock l WHERE l.tenant_id=e.tenant_id AND l.entitlement_no=e.entitlement_no AND l.released_at IS NULL))`, row.SubjectNo, row.CustomerNo, row.CurrencyCode, row.ExpectedSubjectVersion)
	if err != nil {
		return err
	}
	if !allowed.Bool() {
		return ErrActionUnavailable
	}
	busy, err := s.nativeCommercialBusy(ctx, row.SubjectNo, row.OperationNo)
	if err != nil {
		return err
	}
	if busy {
		return ErrOperationConflict
	}
	if !row.FinancialSettlementRequired {
		if row.AmountMinor != 0 || row.OrderNo != "" || row.InvoiceNo != "" || row.SettlementNo != "" || row.FinanciallySettledAt != nil {
			return ErrOperationConflict
		}
		return nil
	}
	if row.AmountMinor <= 0 || row.FinanciallySettledAt == nil || row.SettlementNo == "" || s.evidence == nil {
		return ErrOperationConflict
	}
	proof, err := s.evidence.GetSettledOrderEvidence(ctx, row.OrderNo)
	if err != nil {
		return err
	}
	if proof == nil || proof.SettlementNo != row.SettlementNo || proof.OrderNo != row.OrderNo || proof.InvoiceNo != row.InvoiceNo || proof.CustomerNo != row.CustomerNo || proof.CurrencyCode != row.CurrencyCode || proof.TotalAmountMinor != row.AmountMinor {
		return ErrOperationConflict
	}
	clean, err := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_commerce_order o
JOIN plugin_idc_core_billing_invoice i ON i.tenant_id=o.tenant_id AND i.invoice_no=? AND i.order_no=o.order_no
WHERE o.tenant_id=0 AND o.order_no=? AND o.status_code='paid' AND i.status_code IN ('paid','credit_settled'))
AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_refund_request r WHERE r.tenant_id=0 AND r.order_no=? AND r.status_code<>'rejected')`, row.InvoiceNo, row.OrderNo, row.OrderNo)
	if err != nil {
		return err
	}
	if !clean.Bool() {
		return ErrOperationConflict
	}
	return nil
}
