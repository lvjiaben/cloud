package notificationcap

import (
	"context"
	"time"
)

const NotificationRequestedEventType = "idc.notification.requested.v1"

type RecipientKind string

const (
	RecipientCustomerContact     RecipientKind = "customer_contact"
	RecipientStaffUser           RecipientKind = "staff_user"
	RecipientVerifiedDestination RecipientKind = "verified_destination"
)

// Recipient identifies exactly one source-authoritative audience. It never
// carries a raw email address, phone number, provider binding, or template
// choice across the plugin boundary.
type Recipient struct {
	Kind          RecipientKind
	CustomerNo    string
	ContactNo     string
	StaffUserID   int
	DestinationNo string
}

// EventRequest is one pure business notification intent. The source plugin
// supplies an authoritative recipient plus allowlisted business variables;
// notify alone chooses channels, locale, and immutable template versions.
type EventRequest struct {
	EventCode             string
	Recipient             Recipient
	BusinessObjectType    BusinessObjectType
	BusinessObjectNo      string
	BusinessObjectVersion int64
	MarketingSnapshotNo   string
	Variables             map[string]string
	CorrelationNo         string
	IdempotencyKey        string
}

type EventReceipt struct {
	EventNo       string
	SourcePlugin  string
	Status        string
	OccurredAt    time.Time
	AvailableAt   time.Time
	CorrelationNo string
}

// EventSink is bound to one source plugin at construction time. Enqueue must
// run in the source business transaction; a context without that transaction
// is rejected instead of producing a non-atomic notification event.
type EventSink interface {
	Enqueue(context.Context, EventRequest) (EventReceipt, error)
}

type EventClaimInput struct {
	WorkerID string
	Limit    int
}

type EventLease struct {
	EventNo    string
	LeaseToken string
	AttemptNo  int
}

type ClaimedEvent struct {
	EventReceipt
	Request        EventRequest
	Lease          EventLease
	LeaseExpiresAt time.Time
}

type EventReleaseInput struct {
	Lease        EventLease
	Retryable    bool
	ErrorCode    string
	ErrorSummary string
}

// EventFeed is exposed only to the notify owner. Claim is skip-locked and
// lease-fenced; Ack and Release reject stale, foreign, expired, or replayed
// attempts through one safe failure path.
type EventFeed interface {
	Claim(context.Context, EventClaimInput) ([]ClaimedEvent, error)
	Ack(context.Context, EventLease) (EventReceipt, error)
	Release(context.Context, EventReleaseInput) (EventReceipt, error)
}
