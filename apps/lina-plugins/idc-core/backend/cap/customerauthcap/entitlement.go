// This file exposes owner-scoped entitlement admission without sharing tables.
package customerauthcap

import (
	"context"
	"time"
)

// Entitlement is a customer-owned, generic service fact. Resource details and
// supplier identifiers remain exclusively in the resource owner.
type Entitlement struct {
	EntitlementNo, CustomerNo, Status, OrderNo string
	ProductNameZhCN, ProductNameEnUS           string
	PeriodUnit                                 string
	PeriodCount                                int
	StartedAt                                  time.Time
	ExpiresAt                                  *time.Time
	Version                                    int64
	CanOperate                                 bool
}

// EntitlementAccess is available only to the source resource owner. The write
// callback runs while core holds its own entitlement lock and refund fence;
// callers must pass this transaction context to their resource-owned writes.
type EntitlementAccess interface {
	ReadCurrentEntitlement(context.Context, string) (*Entitlement, error)
	WithCurrentEntitlement(context.Context, string, func(context.Context, *Entitlement) error) error
}

// QueuedEntitlementAccess admits an already authenticated resource-owner task.
// The immutable expected version fences transfer, expiry and refund changes.
// It is source-only and never grants an HTTP caller an identity bypass.
type QueuedEntitlementAccess interface {
	WithQueuedEntitlement(context.Context, string, int64, func(context.Context, *Entitlement) error) error
}
