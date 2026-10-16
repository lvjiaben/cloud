// Package commission owns commission accrual snapshots and refund reversals.
// Payment-time attribution and immutable account entries share invoice-level idempotency.
package commission

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"

	"github.com/gogf/gf/v2/database/gdb"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/saleschannel"
)

const (
	RefundReversalEventType = "billing.refund.commission_reversal_requested"
	AccrualEventType        = "commerce.order.commission_requested"
	maximumWorkerBatch      = 50
)

var referencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)

type refundPayload struct {
	RefundNo     string `json:"refundNo"`
	InvoiceNo    string `json:"invoiceNo"`
	CustomerNo   string `json:"customerNo"`
	CurrencyCode string `json:"currencyCode"`
}

type reversalSummary struct {
	Count  int   `orm:"reversal_count"`
	Amount int64 `orm:"reversed_amount_minor"`
}

// Consumer owns idempotent payment accrual and proportional refund reversal.
type Consumer interface {
	// Consume rereads the authoritative invoice and posts an event once.
	Consume(context.Context, outbox.ClaimedEvent) error
	// AccrueRenewals processes a bounded batch of paid renewal invoices lacking receipts.
	AccrueRenewals(context.Context, int) (int, error)
	// Mature moves due pending commissions using database time.
	Mature(context.Context, int) (int, error)
}

type consumer struct {
	database gdb.DB
	rules    saleschannel.Service
}

func NewConsumer(database gdb.DB, rules saleschannel.Service) (Consumer, error) {
	if database == nil || rules == nil {
		return nil, gerror.New("commission consumer requires database and commission calculator")
	}
	return &consumer{database: database, rules: rules}, nil
}

func (c *consumer) Consume(ctx context.Context, event outbox.ClaimedEvent) error {
	if event.EventType == AccrualEventType {
		return c.consumeAccrual(ctx, event)
	}
	return c.consumeRefund(ctx, event)
}

func decodeRefundEvent(event outbox.ClaimedEvent) (refundPayload, string, error) {
	if event.EventType != RefundReversalEventType || event.EventVersion != 1 ||
		event.Topic != RefundReversalEventType || event.AggregateType != "refund_request" ||
		!validReference(event.EventNo, 40) || !validReference(event.AggregateNo, 40) {
		return refundPayload{}, "", permanent("commission_refund_event_invalid")
	}
	var payload refundPayload
	decoder := json.NewDecoder(bytes.NewReader(event.Payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return refundPayload{}, "", permanent("commission_refund_event_invalid")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF ||
		!validReference(payload.RefundNo, 40) || !validReference(payload.InvoiceNo, 40) ||
		!validReference(payload.CustomerNo, 40) || len(payload.CurrencyCode) != 3 ||
		payload.RefundNo != event.AggregateNo {
		return refundPayload{}, "", permanent("commission_refund_event_invalid")
	}
	sum := sha256.Sum256(event.Payload)
	return payload, hex.EncodeToString(sum[:]), nil
}

func validReference(value string, maximum int) bool {
	return len(value) >= 1 && len(value) <= maximum && referencePattern.MatchString(value)
}

type classifiedError struct {
	code      string
	retryable bool
	cause     error
}

func (e *classifiedError) Error() string { return e.code }
func (e *classifiedError) Unwrap() error { return e.cause }

func permanent(code string) error { return &classifiedError{code: code} }
func retryable(cause error) error {
	return &classifiedError{code: "commission_refund_unavailable", retryable: true, cause: cause}
}

func failureClass(err error) (string, bool) {
	var classified *classifiedError
	if errors.As(err, &classified) {
		return classified.code, classified.retryable
	}
	return "commission_refund_unavailable", true
}
