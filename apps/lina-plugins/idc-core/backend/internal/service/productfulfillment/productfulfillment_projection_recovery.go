// This file resumes only the local server-binding projection during an audited
// provisioning recovery. Previous delivery attempts stay in the same business
// audit; no provider task, payment or sold entitlement identity is rewritten.

package productfulfillment

import (
	"context"

	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
)

type projectionDeliveryState string

const (
	projectionPending         projectionDeliveryState = "pending"
	projectionRetryWait       projectionDeliveryState = "retry_wait"
	projectionDelivering      projectionDeliveryState = "delivering"
	projectionManualAttention projectionDeliveryState = "manual_attention"
)

// projectionResumeFact retains both technical delivery rounds in the staff
// action audit. These are local projection attempts, never supplier attempts.
type projectionResumeFact struct {
	EventNo           string                  `json:"eventNo"`
	PreviousState     projectionDeliveryState `json:"previousState"`
	PreviousAttempts  int                     `json:"previousAttempts"`
	PreviousErrorCode string                  `json:"previousErrorCode"`
	ResumedAttempts   int                     `json:"resumedAttempts"`
}

// resumeResourceProjection runs inside the already-authorized recovery
// transaction, after its payment and failed-provider checks. Command replay
// returns before reaching it, so one recovery grants at most one local round.
func (s *serviceImpl) resumeResourceProjection(ctx context.Context, current *IntentDetail) (*projectionResumeFact, error) {
	var rows []struct {
		EventNo       string
		StatusCode    projectionDeliveryState
		AttemptCount  int
		LastErrorCode string
		LiveLease     bool
	}
	if err := s.database.GetScan(ctx, &rows, `SELECT event.event_no,event.status_code,event.attempt_count,event.last_error_code,
 (event.lease_token<>'' AND event.lease_expires_at>CURRENT_TIMESTAMP) AS live_lease
 FROM plugin_idc_core_entitlement_created_outbox event
 JOIN plugin_idc_core_service_entitlement entitlement
  ON entitlement.tenant_id=event.tenant_id AND entitlement.entitlement_no=event.entitlement_no
  AND entitlement.source_intent_no=event.intent_no AND entitlement.customer_no=event.customer_no
 WHERE event.tenant_id=0 AND event.intent_no=? AND event.customer_no=? AND event.owner_id=?
  AND entitlement.status_code='pending_activation'
  AND event.status_code IN (?,?,?,?)
 LIMIT 2 FOR UPDATE OF event`, current.IntentNo, current.CustomerNo, provisionresourcecap.OwnerID, projectionPending, projectionRetryWait, projectionDelivering, projectionManualAttention); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if len(rows) != 1 || rows[0].LiveLease {
		return nil, errorsNew("product_fulfillment_action_conflict")
	}
	row := rows[0]
	attempts := row.AttemptCount
	if row.StatusCode == projectionManualAttention || row.StatusCode == projectionDelivering {
		attempts = 0
	}
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_entitlement_created_outbox
 SET status_code=?,attempt_count=?,next_attempt_at=CURRENT_TIMESTAMP,
  lease_token='',lease_expires_at=NULL,last_error_code='',updated_at=CURRENT_TIMESTAMP
 WHERE tenant_id=0 AND event_no=? AND status_code=? AND attempt_count=?`, projectionPending, attempts, row.EventNo, row.StatusCode, row.AttemptCount)
	if err != nil {
		return nil, err
	}
	if affected(result) != 1 {
		return nil, errorsNew("product_fulfillment_action_conflict")
	}
	return &projectionResumeFact{EventNo: row.EventNo, PreviousState: row.StatusCode, PreviousAttempts: row.AttemptCount, PreviousErrorCode: row.LastErrorCode, ResumedAttempts: attempts}, nil
}
