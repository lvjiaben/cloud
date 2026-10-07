// This file builds stable online-refund audit evidence and snapshots.

package billing

import (
	"context"

	"lina-plugin-idc-core/backend/cap/idccap"
)

func (s *onlineRefundService) appendRefundAudit(
	ctx context.Context,
	transition idccap.BusinessAuditTransition,
) error {
	return s.audit.AppendBusinessTransition(ctx, transition)
}

func refundAuditSnapshot(value *OnlineRefund) map[string]any {
	if value == nil {
		return nil
	}
	return map[string]any{
		"state": value.Status, "currency_code": value.CurrencyCode,
		"amount_minor": value.AmountMinor, "approval_mode": value.ApprovalMode,
		"destination":                      value.Destination,
		"requires_entitlement_termination": value.RequiresEntitlementTermination,
		"has_resource_impact":              value.HasResourceImpact,
		"has_dangerous_operation":          value.HasDangerousOperation,
		"high_amount":                      value.HighAmount, "version": value.Version,
	}
}
