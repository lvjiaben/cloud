// Persisted commercial history and bound accessory billing are source-only
// read projections. Amounts use decimal minor-unit strings without JSON number
// rounding; provider identifiers, parameters and digests are never exposed.

package servercommercialcap

import (
	"context"
	"time"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

// OperationListRequest scopes a descending, bounded history page to its current
// entitlement owner. OperationNo optionally selects one exact persisted intent.
type OperationListRequest struct {
	EntitlementNo, CustomerNo, OperationNo, Cursor string
	Limit                                          int
}

// OperationHistory preserves the actual commercial state, including uncertainty.
// An empty financial reference means no current-owner document is visible.
type OperationHistory struct {
	OperationNo, QuoteNo, OrderNo, InvoiceNo, CurrencyCode, TotalAmountMinor string
	Status, LastErrorCode                                                    string
	Action                                                                   serveroperationresourcecap.Action
	CreatedAt, UpdatedAt                                                     time.Time
	CompletedAt                                                              *time.Time
}

// OperationPage contains no count of records outside the current owner's scope.
type OperationPage struct {
	Items      []OperationHistory
	NextCursor string
}

// AccessoryBillingListRequest selects only real bound objects, optionally by
// exact local ObjectNo and kind. Empty Kind selects snapshot and backup records.
type AccessoryBillingListRequest struct {
	EntitlementNo, CustomerNo, ObjectNo, Cursor string
	Kind                                        serveroperationresourcecap.AccessoryKind
	Limit                                       int
}

// AccessoryBilling is the retained billing record for a verified local object.
// Nil pending amount means no visible frozen invoice, never a current quote.
// PendingAmountMinor is that invoice's frozen total, not its remaining debt.
// Free objects retain nil current-period dates rather than invented paid terms.
type AccessoryBilling struct {
	AccessoryEntitlementNo, ObjectNo, BillingMode, Status, CurrencyCode, PeriodUnit string
	Kind                                                                            serveroperationresourcecap.AccessoryKind
	PeriodCount                                                                     int
	CurrentPeriodStartAt, CurrentPeriodEndAt                                        *time.Time
	AutoRenewEnabled                                                                bool
	PendingInvoiceNo, PendingInvoiceStatus                                          string
	PendingAmountMinor                                                              *string
	CreatedAt, UpdatedAt                                                            time.Time
}

// AccessoryBillingPage uses the same bounded descending pagination as history.
type AccessoryBillingPage struct {
	Items      []AccessoryBilling
	NextCursor string
}

// CommercialReadAccess independently validates principal and current ownership;
// paused or expired parents remain readable without a fresh provider snapshot.
type CommercialReadAccess interface {
	// ListOperations returns persisted native addon, resize and IP change history.
	ListOperations(context.Context, OperationListRequest) (*OperationPage, error)
	// ListAccessoryBilling returns billing periods only for real bound objects.
	ListAccessoryBilling(context.Context, AccessoryBillingListRequest) (*AccessoryBillingPage, error)
}
