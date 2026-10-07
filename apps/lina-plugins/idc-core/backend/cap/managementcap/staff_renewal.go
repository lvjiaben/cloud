package managementcap

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

const MaximumStaffRenewalBatch = 20

// StaffRenewalQuoteInput contains only the customer and entitlement selected
// from a business owner's governed list. Amounts, expiry, and Provider facts
// are always resolved again by the core renewal owner.
type StaffRenewalQuoteInput struct {
	CustomerNo, EntitlementNo, IdempotencyKey string
	RenewalCycles                             int
}

type StaffRenewalQuote struct {
	RequestNo, QuoteNo, PriceSnapshotNo, EntitlementNo, CurrencyCode, RenewalMode  string
	RenewalCycles                                                                  int
	OriginalExpiresAt, ProjectedExpiresAt, IssuedAt, ExpiresAt                     time.Time
	SubtotalMinor, DiscountMinor, NetAmountMinor, TaxAmountMinor, TotalAmountMinor int64
	Reused                                                                         bool
}

type StaffRenewalConfirmInput struct {
	CustomerNo, QuoteNo, IdempotencyKey string
	ActorUserID                         int
}

type StaffRenewalOperation struct {
	OperationNo, QuoteNo, InvoiceNo, EntitlementNo, CurrencyCode, Status, RenewalMode string
	TotalAmountMinor                                                                  int64
	PaymentDueAt                                                                      time.Time
	Reused                                                                            bool
}

// StaffRenewalManager is the narrow core-owned seam used by a business plugin
// after it has authorized the employee and selected entitlements from its own
// list. It deliberately exposes no generic search or caller-supplied amount.
type StaffRenewalManager interface {
	QuoteForStaff(context.Context, StaffRenewalQuoteInput) (*StaffRenewalQuote, error)
	ConfirmForStaff(context.Context, StaffRenewalConfirmInput) (*StaffRenewalOperation, error)
}

func ValidateStaffRenewalQuoteInput(input StaffRenewalQuoteInput) error {
	if !targetNumberPattern.MatchString(input.CustomerNo) ||
		!targetNumberPattern.MatchString(input.EntitlementNo) ||
		len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 128 ||
		input.RenewalCycles < 1 || input.RenewalCycles > 120 {
		return gerror.New("IDC staff renewal quote request is invalid")
	}
	return nil
}

func ValidateStaffRenewalConfirmInput(input StaffRenewalConfirmInput) error {
	if !targetNumberPattern.MatchString(input.CustomerNo) ||
		!targetNumberPattern.MatchString(input.QuoteNo) ||
		len(input.IdempotencyKey) < 8 || len(input.IdempotencyKey) > 128 ||
		input.ActorUserID <= 0 {
		return gerror.New("IDC staff renewal confirmation is invalid")
	}
	return nil
}
