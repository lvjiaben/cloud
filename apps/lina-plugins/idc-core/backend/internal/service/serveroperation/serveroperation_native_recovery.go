// Native ready recovery removes undispatched financial or parent blockers from
// the dispatch queue. Only an owner's completed, fully posted refund releases
// capacity; provider-unknown operations are never retried or compensated here.

package serveroperation

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

type nativeReadyReason string
type nativeReadyParentState string

const nativeReadyParentActive nativeReadyParentState = "active"

const (
	nativeReadyPrefix                      = "server_commercial.ready_blocked."
	nativeReadyRefund    nativeReadyReason = nativeReadyPrefix + "refund_pending"
	nativeReadyParent    nativeReadyReason = nativeReadyPrefix + "parent_inactive"
	nativeReadyCustomer  nativeReadyReason = nativeReadyPrefix + "customer_inactive"
	nativeReadyVersion   nativeReadyReason = nativeReadyPrefix + "version_changed"
	nativeReadyFinancial nativeReadyReason = nativeReadyPrefix + "financial_unavailable"
	nativeReadyResumed                     = "server_commercial.ready_resumed"
	nativeReadyRefunded                    = "server_commercial.refund_cancelled"
)

// ReconcileReady checks a bounded, rotating batch before dispatch. It returns
// the number of committed state changes, together with any independent errors.
// Transactions reuse execution's order/invoice/customer/parent/operation locks.
func (s *nativeCommercialService) ReconcileReady(ctx context.Context, limit int) (int, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil || limit < 1 || limit > 100 || gdb.TXFromCtx(ctx, s.database.GetGroup()) != nil {
		return 0, PublicError(ErrInvalidInput)
	}
	var candidates []struct{ OperationNo string }
	if err := s.database.GetScan(ctx, &candidates, `SELECT operation_no FROM plugin_idc_core_business_operation
WHERE tenant_id=0 AND subject_type_code='entitlement' AND service_no IS NULL
 AND resource_owner_code='idc-resource' AND operation_type_code IN ('addon','resize','ip_change')
 AND status_code IN ('ready','manual_review') AND provider_task_no IS NULL AND execution_started_at IS NULL
ORDER BY updated_at,operation_no LIMIT ?`, limit); err != nil {
		return 0, PublicError(err)
	}
	var count int
	var failures []error
	for _, candidate := range candidates {
		changed := false
		err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
			var err error
			changed, err = s.reconcileNativeReady(ctx, candidate.OperationNo)
			return err
		})
		if err != nil {
			// A failed owner's read or release must not monopolize every next
			// bounded batch. Rotate metadata only after its transaction rolled
			// back; no financial state or task is changed by this statement.
			failures = append(failures, errors.Join(err, s.touchNativeReady(ctx, candidate.OperationNo)))
		} else if changed {
			count++
		}
	}
	return count, PublicError(errors.Join(failures...))
}

func (s *nativeCommercialService) reconcileNativeReady(ctx context.Context, number string) (bool, error) {
	row, err := s.lockNativeExecution(ctx, number)
	if err != nil {
		return false, err
	}
	var fence struct {
		LastErrorCode      string
		ExecutionStartedAt *time.Time
	}
	if err = s.database.GetScan(ctx, &fence, `SELECT last_error_code,execution_started_at FROM plugin_idc_core_business_operation WHERE tenant_id=0 AND operation_no=?`, number); err != nil {
		return false, err
	}
	if (row.StatusCode != string(accessoryExecutionReady) && row.StatusCode != string(nativeCreateManual)) || row.ProviderTaskNo != "" || fence.ExecutionStartedAt != nil {
		return false, nil
	}
	refund, err := s.nativeReadyRefundProof(ctx, row)
	if err != nil {
		return false, err
	}
	if row.FinancialSettlementRequired && row.AmountMinor > 0 && refund.CompletedAmountMinor == row.AmountMinor {
		return true, s.cancelRefundedNativeReady(ctx, row, refund)
	}
	// Only this reconciler's affirmative no-dispatch blockers may resume. An
	// unrelated manual conclusion or permanent admission error stays fenced.
	if row.StatusCode == string(nativeCreateManual) && !strings.HasPrefix(fence.LastErrorCode, nativeReadyPrefix) {
		return false, s.touchNativeReady(ctx, number)
	}
	reason, err := s.nativeReadyBlockReason(ctx, row, refund)
	if err != nil {
		return false, err
	}
	if (reason == "" && row.StatusCode == string(accessoryExecutionReady)) || (reason != "" && row.StatusCode == string(nativeCreateManual)) {
		if string(reason) != fence.LastErrorCode && reason != "" {
			if _, err = s.database.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET last_error_code=? WHERE tenant_id=0 AND operation_no=?`, reason, number); err != nil {
				return false, err
			}
		}
		return false, s.touchNativeReady(ctx, number)
	}
	to, event := string(nativeCreateManual), string(reason)
	if reason == "" {
		to, event = string(accessoryExecutionReady), nativeReadyResumed
	}
	evidence, err := digestValue(struct {
		Operation, From, To, Reason string
		Version                     int64
	}{number, row.StatusCode, to, event, row.Version})
	if err != nil {
		return false, err
	}
	if err = execOne(ctx, s.database, `UPDATE plugin_idc_core_business_operation SET status_code=?,last_error_code=?,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND operation_no=? AND version=?`, to, reason, number, row.Version); err != nil {
		return false, err
	}
	return true, appendNativeAccessoryEvent(ctx, s.database, number, row.SubjectNo, row.Version+1, row.StatusCode, to, event, number, evidence)
}

// nativeReadyBlockReason reads the locked aggregate's current owner state; it
// never rewrites the frozen version or treats a parent's refund as addon money.
func (s *nativeCommercialService) nativeReadyBlockReason(ctx context.Context, row *nativeExecutionRow, refund nativeReadyRefundEvidence) (nativeReadyReason, error) {
	if refund.HasRequest {
		return nativeReadyRefund, nil
	}
	var parent *struct {
		StatusCode, CustomerStatus string
		Version                    int64
		Expired, RefundLocked      bool
	}
	if err := s.database.GetScan(ctx, &parent, `SELECT e.status_code,e.version,c.status_code AS customer_status,
 (e.expires_at IS NOT NULL AND e.expires_at<=CURRENT_TIMESTAMP) AS expired,
 EXISTS(SELECT 1 FROM plugin_idc_core_refund_entitlement_lock l WHERE l.tenant_id=0 AND l.entitlement_no=e.entitlement_no AND l.released_at IS NULL) AS refund_locked
FROM plugin_idc_core_service_entitlement e JOIN plugin_idc_core_customer_subject c ON c.tenant_id=e.tenant_id AND c.customer_no=e.customer_no
WHERE e.tenant_id=0 AND e.entitlement_no=? AND e.customer_no=? AND e.currency_code=? AND e.entitlement_owner_id='idc-resource'`, row.SubjectNo, row.CustomerNo, row.CurrencyCode); err != nil {
		return "", err
	}
	if parent == nil || nativeReadyParentState(parent.StatusCode) != nativeReadyParentActive || parent.Expired || parent.RefundLocked {
		return nativeReadyParent, nil
	}
	if customer.Status(parent.CustomerStatus) != customer.StatusActive {
		return nativeReadyCustomer, nil
	}
	if parent.Version != row.ExpectedSubjectVersion {
		return nativeReadyVersion, nil
	}
	// Settlement and invoice eligibility are revalidated by the same authority
	// as dispatch. No payment, provider task or approval is created by recovery.
	if err := s.admitNativeExecution(ctx, row); err != nil {
		if errors.Is(err, ErrOperationConflict) || errors.Is(err, ErrActionUnavailable) {
			return nativeReadyFinancial, nil
		}
		return "", err
	}
	return "", nil
}

// Every checked row moves behind the next bounded batch, including permanent
// manual blockers. This prevents a few old failures starving later operations.
func (s *nativeCommercialService) touchNativeReady(ctx context.Context, number string) error {
	_, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND operation_no=? AND subject_type_code='entitlement' AND resource_owner_code='idc-resource'
 AND status_code IN ('ready','manual_review') AND provider_task_no IS NULL AND execution_started_at IS NULL`, number)
	return err
}
