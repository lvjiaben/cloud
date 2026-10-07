// Package commission owns commission accrual snapshots and refund reversals.
// Stage 10 intentionally wires only the reversal of amounts that have not matured.
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

	"lina-plugin-idc-core/backend/internal/service/outbox"
)

const (
	RefundReversalEventType = "billing.refund.commission_reversal_requested"
	maximumWorkerBatch      = 50
)

var referencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)

type refundPayload struct {
	RefundNo     string `json:"refundNo"`
	InvoiceNo    string `json:"invoiceNo"`
	CustomerNo   string `json:"customerNo"`
	CurrencyCode string `json:"currencyCode"`
}

type receiptRow struct {
	PayloadDigest string `orm:"payload_digest"`
}

type reversalSummary struct {
	Count  int   `orm:"reversal_count"`
	Amount int64 `orm:"reversed_amount_minor"`
}

// Consumer proportionally reverses only pending commission for one verified refund.
type Consumer interface {
	Consume(context.Context, outbox.ClaimedEvent) error
}

type consumer struct{ database gdb.DB }

func NewConsumer(database gdb.DB) (Consumer, error) {
	if database == nil {
		return nil, errors.New("commission refund consumer requires database")
	}
	return &consumer{database: database}, nil
}

func (c *consumer) Consume(ctx context.Context, event outbox.ClaimedEvent) error {
	payload, digest, err := decodeRefundEvent(event)
	if err != nil {
		return err
	}
	return c.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var receipt *receiptRow
		if scanErr := tx.GetScan(&receipt, `
SELECT payload_digest
FROM plugin_idc_core_commission_refund_receipt
WHERE tenant_id=0 AND (event_no=? OR refund_no=?)
FOR UPDATE`, event.EventNo, payload.RefundNo); scanErr != nil {
			return retryable(scanErr)
		}
		if receipt != nil {
			if receipt.PayloadDigest != digest {
				return permanent("commission_refund_replay_conflict")
			}
			return nil
		}

		var summary reversalSummary
		if scanErr := tx.GetScan(&summary, `
WITH refunded AS (
    SELECT allocation.invoice_item_no,
           SUM(CASE allocation.direction_code
               WHEN 'apply' THEN allocation.amount_minor
               ELSE -allocation.amount_minor END) AS amount_minor
    FROM plugin_idc_core_billing_refund_allocation allocation
    JOIN plugin_idc_core_refund_execution_command command
      ON command.command_no=allocation.refund_source_reference
     AND command.refund_no=?
    WHERE allocation.tenant_id=0 AND allocation.invoice_no=?
    GROUP BY allocation.invoice_item_no
), locked AS (
    SELECT accrual.commission_no,accrual.original_amount_minor,
           accrual.invoice_item_amount_minor,accrual.pending_amount_minor,
           accrual.reversed_amount_minor,refunded.amount_minor AS refunded_minor
    FROM plugin_idc_core_commission_accrual accrual
    JOIN refunded ON refunded.invoice_item_no=accrual.invoice_item_no
    WHERE accrual.tenant_id=0 AND accrual.invoice_no=?
      AND accrual.pending_amount_minor>0
    ORDER BY accrual.id
    FOR UPDATE OF accrual
), inserted AS (
    INSERT INTO plugin_idc_core_commission_refund_reversal
        (tenant_id,reversal_no,commission_no,refund_no,source_event_no,amount_minor)
    SELECT 0,'crv_' || left(md5(locked.commission_no || ':' || ?),32),
           locked.commission_no,?,?,
           LEAST(locked.pending_amount_minor,
               GREATEST(0,
                   LEAST(locked.original_amount_minor,
                       locked.original_amount_minor * locked.refunded_minor
                           / locked.invoice_item_amount_minor)
                   - locked.reversed_amount_minor))
    FROM locked
    WHERE LEAST(locked.pending_amount_minor,
              GREATEST(0,
                  LEAST(locked.original_amount_minor,
                      locked.original_amount_minor * locked.refunded_minor
                          / locked.invoice_item_amount_minor)
                  - locked.reversed_amount_minor)) > 0
    ON CONFLICT (commission_no,refund_no) DO NOTHING
    RETURNING commission_no,amount_minor
), updated AS (
    UPDATE plugin_idc_core_commission_accrual accrual
    SET pending_amount_minor=accrual.pending_amount_minor-inserted.amount_minor,
        reversed_amount_minor=accrual.reversed_amount_minor+inserted.amount_minor,
        status_code=CASE
            WHEN accrual.pending_amount_minor=inserted.amount_minor THEN 'reversed'
            ELSE 'partially_reversed' END,
        updated_at=CURRENT_TIMESTAMP
    FROM inserted
    WHERE accrual.commission_no=inserted.commission_no
    RETURNING inserted.amount_minor
)
SELECT COUNT(*)::int AS reversal_count,
       COALESCE(SUM(amount_minor),0)::bigint AS reversed_amount_minor
FROM updated`, payload.RefundNo, payload.InvoiceNo, payload.InvoiceNo,
			payload.RefundNo, payload.RefundNo, event.EventNo); scanErr != nil {
			return retryable(scanErr)
		}
		if _, insertErr := tx.Exec(`
INSERT INTO plugin_idc_core_commission_refund_receipt
    (tenant_id,event_no,refund_no,payload_digest,reversal_count,reversed_amount_minor)
VALUES (0,?,?,?,?,?)`, event.EventNo, payload.RefundNo, digest,
			summary.Count, summary.Amount); insertErr != nil {
			return retryable(insertErr)
		}
		return nil
	})
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
