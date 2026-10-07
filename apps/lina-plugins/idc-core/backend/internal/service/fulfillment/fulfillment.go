// Package fulfillment consumes financially settled commerce events into
// core-owned service and business-operation intents before Provider execution.
package fulfillment

import (
	"context"
	"errors"

	"lina-plugin-idc-core/backend/internal/service/outbox"
)

const (
	// FinanciallySettledEventType is the only event contract accepted here.
	FinanciallySettledEventType = "commerce.order.financially_settled"
	financiallySettledVersion   = 1
	maximumSettlementSources    = 1000
)

// Delivery is the append-only local consumption receipt projection.
type Delivery struct {
	DeliveryNo        string
	EventNo           string
	SettlementNo      string
	OrderNo           string
	FulfillmentCount  int
	ReadyCount        int
	WaitingGateCount  int
	ManualReviewCount int
	Reused            bool
}

// Consumer materializes one claimed financial settlement event atomically.
type Consumer interface {
	Consume(ctx context.Context, event outbox.ClaimedEvent) (*Delivery, error)
}

// SettlementEvent carries the strictly decoded event coordinates. Consumers
// must still verify them against durable billing and business evidence.
type SettlementEvent struct {
	EventNo, SettlementNo, OrderNo, InvoiceNo, CustomerNo, CurrencyCode, BasisCode string
	TotalAmountMinor, SettledAtUnixMS                                              int64
	SourceCount                                                                    int
}

// AddonSettlementConsumer recognizes only persisted server-addon intents.
// A recognized but invalid addon must return an error, never fall through to
// generic product fulfillment.
type AddonSettlementConsumer interface {
	// ConsumeAddonSettlement returns handled=false only for a durable
	// non-addon order. Evidence failures must remain errors, and an already
	// consumed event returns the original receipt without repeating execution.
	ConsumeAddonSettlement(context.Context, SettlementEvent) (*Delivery, bool, error)
}

type consumptionError struct {
	code      string
	retryable bool
	cause     error
}

func (e *consumptionError) Error() string { return e.code }
func (e *consumptionError) Unwrap() error { return e.cause }

func permanent(code string) error {
	return &consumptionError{code: code, retryable: false}
}

// FailureClass returns stable redacted worker diagnostics.
func FailureClass(err error) (code string, summary string, retryable bool) {
	var classified *consumptionError
	if errors.As(err, &classified) {
		return classified.code, "Financial settlement delivery could not be completed", classified.retryable
	}
	return "financial_settlement_delivery_unavailable",
		"Financial settlement delivery could not be completed", true
}
