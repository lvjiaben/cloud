// Package entitlementprogress projects local owner notification progress for
// readers. It never changes delivery, lifecycle, payments or supplier tasks.
package entitlementprogress

// Status describes management readiness independently from service lifecycle.
type Status string

const (
	Preparing Status = "preparing"
	Ready     Status = "ready"
	Attention Status = "needs_attention"
)

// Notification values are persisted by the existing entitlement-created worker.
const (
	notificationPending    = "pending"
	notificationRetryWait  = "retry_wait"
	notificationDelivering = "delivering"
	notificationDelivered  = "delivered"
	ownerUnavailable       = "entitlement_owner_unavailable"
	deliveryUnavailable    = "owner_delivery_unavailable"
	deliveryUnknown        = "owner_delivery_result_unknown"
)

// FromDelivery maps durable notification states to safe display progress. An
// absent historical notification stays absent; unknown recorded states fail closed.
func FromDelivery(state string) Status {
	switch state {
	case "":
		return ""
	case notificationPending, notificationRetryWait, notificationDelivering:
		return Preparing
	case notificationDelivered:
		return Ready
	default:
		return Attention
	}
}

// SafeFailure exposes only reviewed local categories, never stored free text.
func SafeFailure(code string) string {
	switch code {
	case ownerUnavailable, deliveryUnavailable, deliveryUnknown:
		return code
	default:
		return ""
	}
}
