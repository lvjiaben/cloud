// Native resource writes and refunds share the parent entitlement fence.
// A durable executing/unknown operation retains the fence after the admission
// transaction commits and until resource evidence proves a terminal result.
package billing

import (
	"context"
	"fmt"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"sort"
)

func (s *onlineRefundService) ensureNativeCommercialRefundIdle(ctx context.Context, preview *RefundPreview) error {
	if preview == nil || gdb.TXFromCtx(ctx, s.database.GetGroup()) == nil {
		return bizerr.NewCode(CodeRefundIntegrity)
	}
	owners := map[string]bool{}
	for _, item := range preview.Items {
		for _, impact := range item.Impacts {
			if impact.EntitlementNo != "" {
				owners[impact.EntitlementNo] = true
			}
		}
	}
	numbers := make([]string, 0, len(owners))
	for number := range owners {
		numbers = append(numbers, number)
	}
	sort.Strings(numbers)
	args := make([]any, len(numbers))
	for i, number := range numbers {
		args[i] = number
	}
	if len(numbers) > 0 {
		// Explicitly finish locking parents before the following statement takes
		// its fresh snapshot of operation states. A single joined SELECT could
		// have evaluated a stale EXISTS before waiting for a concurrent claim.
		var locked []struct{ EntitlementNo string }
		query := fmt.Sprintf(`SELECT entitlement_no FROM plugin_idc_core_service_entitlement WHERE tenant_id=0 AND entitlement_no IN (%s) ORDER BY entitlement_no FOR UPDATE`, refundPlaceholders(len(numbers)))
		if err := s.database.GetScan(ctx, &locked, query, args...); err != nil {
			return err
		}
		if len(locked) != len(numbers) {
			return bizerr.NewCode(CodeRefundIntegrity)
		}
	}
	query := `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_business_operation op WHERE op.tenant_id=0
AND op.subject_type_code='entitlement' AND op.resource_owner_code='idc-resource'
AND op.operation_type_code IN ('addon','ip_change','resize')
AND (op.status_code='executing' OR (op.status_code='manual_review' AND (op.provider_task_no IS NOT NULL OR op.execution_started_at IS NOT NULL)))
AND (op.order_no=?`
	values := []any{preview.OrderNo}
	if len(numbers) > 0 {
		query += fmt.Sprintf(` OR op.subject_no IN (%s)`, refundPlaceholders(len(numbers)))
		values = append(values, args...)
	}
	query += `))`
	busy, err := s.database.GetValue(ctx, query, values...)
	if err != nil {
		return err
	}
	if busy.Bool() {
		return bizerr.NewCode(CodeRefundNotEligible)
	}
	if len(numbers) > 0 {
		query = fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM plugin_idc_core_native_server_write_fence
WHERE tenant_id=0 AND entitlement_no IN (%s) AND status_code IN ('executing','result_unknown'))`, refundPlaceholders(len(numbers)))
		busy, err = s.database.GetValue(ctx, query, args...)
		if err != nil {
			return err
		}
		if busy.Bool() {
			return bizerr.NewCode(CodeRefundNotEligible)
		}
	}
	return nil
}
