// This file reads the exact local management notification after staff detail
// authorization. It exposes bounded categories and never calls a supplier.
package productfulfillment

import (
	"context"
	"lina-plugin-idc-core/backend/internal/service/entitlementprogress"
)

const preparationPendingActivation = "pending_activation"

func (s *serviceImpl) loadManagementPreparation(ctx context.Context, intentNo, customerNo, ownerID string) (*ManagementPreparation, error) {
	var row *struct {
		StatusCode    string
		AttemptCount  int
		LastErrorCode string
	}
	if err := s.database.GetScan(ctx, &row, `SELECT event.status_code,event.attempt_count,event.last_error_code
 FROM plugin_idc_core_entitlement_created_outbox event
 JOIN plugin_idc_core_service_entitlement entitlement
 ON entitlement.tenant_id=event.tenant_id AND entitlement.entitlement_no=event.entitlement_no
 AND entitlement.source_intent_no=event.intent_no AND entitlement.customer_no=event.customer_no
 AND entitlement.entitlement_owner_id=event.owner_id
 WHERE event.tenant_id=0 AND event.intent_no=? AND event.customer_no=? AND event.owner_id=? AND entitlement.status_code<>?
 LIMIT 1`, intentNo, customerNo, ownerID, preparationPendingActivation); err != nil {
		return nil, err
	}
	if row == nil {
		return nil, nil
	}
	return &ManagementPreparation{StatusCode: string(entitlementprogress.FromDelivery(row.StatusCode)), AttemptCount: row.AttemptCount, FailureCode: entitlementprogress.SafeFailure(row.LastErrorCode)}, nil
}
