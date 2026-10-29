// Package servercommercialcap exposes source-only commercial admission for the resource owner.
// Resource facts must be constructed by the authenticated resource implementation,
// never decoded from a browser request. Core independently checks the entitlement,
// published prices, stock, settlement and accessory billing lifecycle.
package servercommercialcap

import (
	"context"
	"encoding/json"
	"time"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

// AllowedCallerPluginID owns resource facts, provider tasks and confirmed outcomes.
const AllowedCallerPluginID = "idc-resource"

// TargetSnapshot is an owner-verified, bounded local projection without provider secrets.
// ExpiresAt is capability freshness, never a traffic-package expiry promise.
type TargetSnapshot struct {
	EntitlementNo, CustomerNo, LocalResourceNo, ChannelNo, PoolNo string
	EntitlementVersion                                            int64
	RuntimeStatus                                                 string
	TrafficSuspended                                              bool
	SnapshotNo                                                    string
	ObservedAt, ExpiresAt                                         time.Time
	Actions                                                       []serveroperationresourcecap.Action
	Items                                                         []serveroperationresourcecap.AccessoryItem
}

// QuoteRequest freezes a merchant rule for exactly one verified server.
// Parameters contain only resource-validated public inputs; traffic capacity
// and all prices are derived by core from PackageNo, never from Parameters.
type QuoteRequest struct {
	Target                    TargetSnapshot
	Action                    serveroperationresourcecap.Action
	Parameters                json.RawMessage
	PackageNo, IdempotencyKey string
	Resize                    *ResizeInput
}

// ResizeInput selects the published catalog revision and billing term.
type ResizeInput struct {
	TargetQuoteNo                  string
	Revision, TermNo, CurrencyCode string
	Selections                     []ResizeSelection
}

type ResizeSelection struct {
	OptionNo string
	Quantity int
}

// Quote is the frozen inclusive price and target revision, with no supplier data.
type Quote struct {
	QuoteNo, EntitlementNo, CurrencyCode, PolicyNo                                string
	Action                                                                        serveroperationresourcecap.Action
	Kind                                                                          serveroperationresourcecap.AccessoryKind
	TotalAmountMinor, TaxAmountMinor, PolicyVersion                               int64
	FreeCountPerServer, ExistingObjectCount, ReservedCreateCount, ChargedQuantity int
	TrafficGB                                                                     int
	PeriodUnit                                                                    string
	PeriodCount                                                                   int
	ExpiresAt                                                                     time.Time
	Reused                                                                        bool
	Direction                                                                     string
	EffectiveAt                                                                   time.Time
	CurrentRecurringMinor, TargetRecurringMinor                                   int64
}

// CheckoutRequest rechecks current resource facts against the frozen quote.
type CheckoutRequest struct {
	QuoteNo, IdempotencyKey string
	Target                  TargetSnapshot
}

// Purchase never treats payment pending as authority to execute.
type Purchase struct {
	OperationNo, QuoteNo, EntitlementNo, OrderNo, InvoiceNo, Status, CurrencyCode string
	TotalAmountMinor                                                              int64
	LastErrorCode                                                                 string
	Reused                                                                        bool
}

// TrafficPackage is a published fixed package available to this exact product.
// Nil RemainingStock means unlimited sales; capacity and prices are core facts.
type TrafficPackage struct {
	PackageNo, NameZhCN, NameEnUS, CurrencyCode string
	TrafficGB                                   int
	AmountMinor, PolicyVersion                  int64
	RemainingStock                              *int64
	UnlimitedStock                              bool
}

// Operation is the settled or explicitly free immutable intent for resource-owned execution.
type Operation struct {
	OperationNo, EntitlementNo, CustomerNo, LocalResourceNo, ChannelNo, PoolNo string
	ExpectedEntitlementVersion                                                 int64
	Action                                                                     serveroperationresourcecap.Action
	Parameters                                                                 json.RawMessage
	ContentDigest                                                              string
	QuoteNo, Direction                                                         string
	EffectiveAt                                                                time.Time
	TargetConfigurationNo, TargetVersionNo, TargetBillingRuleNo                string
	ResourceReservationNo, ResourceSnapshotNo                                  string
	IPQuantity                                                                 int
}

// CompletionStatus describes the resource owner's evidence, not an HTTP success flag.
type CompletionStatus string

const (
	CompletionSucceeded CompletionStatus = "succeeded"
	CompletionFailed    CompletionStatus = "failed"
	CompletionUnknown   CompletionStatus = "result_unknown"
)

// Completion must come from the resource task reconciler after checking its
// durable task identity and provider evidence. ObjectNo is the resource-owned
// local object key; uncertain outcomes never release occupancy or permit replay.
type Completion struct {
	OperationNo, EntitlementNo, TaskNo, ObjectNo, EvidenceDigest string
	Status                                                       CompletionStatus
	ObservedAt                                                   time.Time
}

// ObjectRequest is scoped by a resource-owned server and object association.
type ObjectRequest struct {
	EntitlementNo, CustomerNo, ObjectNo string
	Kind                                serveroperationresourcecap.AccessoryKind
}

// Deletion is a resource reconciler's successful free deletion receipt.
// Deleting and restoring objects do not create a priced quote or new order.
type Deletion struct {
	ObjectRequest
	OperationNo, TaskNo, EvidenceDigest string
	ObservedAt                          time.Time
}

// TypedPurchaseValue mirrors the generic catalog's opaque typed public value.
// It contains no raw compiler key, supplier parameter, amount or customer identity.
type TypedPurchaseValue struct {
	FieldToken, Type                                                string
	TextValue, IntegerValue, DecimalValue                           *string
	BooleanValue                                                    *bool
	DateValue, DateTimeValue                                        *string
	OptionTokens, TreeOptionTokens, AddressOptionTokens, FileTokens []string
}

// StructuredTargetInput selects one published structured configuration for the
// source-verified server. Core derives the product, currency and paid cycles.
type StructuredTargetInput struct {
	Target                                  TargetSnapshot
	ConfigurationRevision, BillingTermToken string
	Values                                  []TypedPurchaseValue
}

// TargetQuote is a real owned change-purpose sales quote and frozen configuration.
type TargetQuote struct{ QuoteNo, QuoteItemNo, StructuredConfigurationNo string }

// CheckoutIntent exposes only the frozen customer-visible confirmation facts
// after checking current source target identity, core ownership and revision.
type CheckoutIntent struct {
	QuoteNo, EntitlementNo, LocalResourceNo, CurrencyCode string
	Action                                                serveroperationresourcecap.Action
	TotalAmountMinor                                      int64
	ExpiresAt                                             time.Time
}

// PurchaseReadRequest recovers an owned checkout using its original idempotency identity.
type PurchaseReadRequest struct{ QuoteNo, IdempotencyKey, EntitlementNo, CustomerNo string }

// AdmissionFailure records permanent unsent admission evidence; it never
// describes a provider result and cannot replace a bound or started task.
type AdmissionFailure struct{ OperationNo, EntitlementNo, ReasonCode, EvidenceDigest string }

// Service is injected only into the resource owner at startup. It is not a
// dynamic host service or direct HTTP API. The resource owner performs precise
// action/data-scope authorization before Quote, Checkout and CheckRestore.
type Service interface {
	// ListTrafficPackages returns at most 100 currently eligible published packages.
	ListTrafficPackages(context.Context, TargetSnapshot) ([]TrafficPackage, error)
	// QuoteTarget freezes an owned structured change target without changing the customer cart.
	QuoteTarget(context.Context, StructuredTargetInput) (*TargetQuote, error)
	// Quote freezes current merchant rules after validating core ownership and resource freshness.
	Quote(context.Context, QuoteRequest) (*Quote, error)
	// Checkout reserves stock/one object slot atomically and creates ordinary payable documents.
	Checkout(context.Context, CheckoutRequest) (*Purchase, error)
	// ReadPurchase returns the matching existing purchase or nil after current ownership authorization.
	ReadPurchase(context.Context, PurchaseReadRequest) (*Purchase, error)
	// ReadCheckout returns immutable confirmation facts after current target admission.
	ReadCheckout(context.Context, CheckoutRequest) (*CheckoutIntent, error)
	// RejectUnsent records a permanent local ready admission failure only before a task is bound or started.
	RejectUnsent(context.Context, AdmissionFailure) error
	// ReconcileReady makes bounded progress on ready operations with completed owner evidence.
	ReconcileReady(context.Context, int) (int, error)
	// ListReady returns at most 100 ready operation numbers for a bounded owner worker.
	ListReady(context.Context, int) ([]string, error)
	// WithReadyOperation rechecks entitlement/refund gates and calls enqueue in
	// the core transaction. enqueue may only persist one idempotent resource task
	// using that context; it must never call a provider. Its returned local task
	// number is frozen before committing the executing state.
	WithReadyOperation(context.Context, string, func(context.Context, Operation) (string, error)) error
	// WithQueuedOperation validates the already-bound local task and rechecks
	// settlement, entitlement and refund gates before the callback durably claims
	// provider execution in the shared transaction. The callback performs no
	// network I/O; the resource executor invokes the provider after commit under
	// its existing durable lease and unknown-result fence.
	WithQueuedOperation(context.Context, string, string, func(context.Context, Operation) error) error
	// CheckRestore rejects expired or payment-pending paid objects; it never charges for restore.
	CheckRestore(context.Context, ObjectRequest) error
	// CheckQueuedRestore uses the resource task's frozen identity without a
	// customer session. It requires the same database transaction in which the
	// resource owner has revalidated and claimed the task through its entitlement
	// gate; browser identities and provider network calls are never accepted here.
	CheckQueuedRestore(context.Context, ObjectRequest) error
	// Complete records a verified resource outcome and binds a created object or
	// stops renewal after proven deletion. Duplicate identical evidence is harmless.
	Complete(context.Context, Completion) error
	// CompleteDeletion releases a free slot or stops renewal only after the
	// resource owner durably confirms deletion; it never refunds the current cycle.
	CompleteDeletion(context.Context, Deletion) error
}
