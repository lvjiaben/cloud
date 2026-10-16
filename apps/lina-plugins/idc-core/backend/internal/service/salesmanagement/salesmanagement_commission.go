// This file defines administrator commission reads and offline payout recording.
// It never transfers money or writes customer spending balances.
package salesmanagement

import (
	"context"
	"time"
)

// CommissionService enforces staff permission and merchant data scope for every
// read and write. Monetary values are decimal strings of integer minor units.
type CommissionService interface {
	// ListCommissionAccounts returns bounded balances and beneficiary labels.
	ListCommissionAccounts(context.Context, PageInput) (CommissionAccountPage, error)
	// ListCommissionRecords returns immutable attribution and current maturity facts.
	ListCommissionRecords(context.Context, PageInput) (CommissionRecordPage, error)
	// ListCommissionPayouts returns recorded offline evidence; it sends no payments.
	ListCommissionPayouts(context.Context, PageInput) (CommissionPayoutPage, error)
	// RecordCommissionPayout atomically moves available commission to paid with
	// operator evidence and audit. Idempotency conflicts or insufficient funds fail.
	RecordCommissionPayout(context.Context, CommissionPayoutInput) (CommissionPayout, error)
}

// CommissionAccount exposes separate currency buckets, never a currency sum.
type CommissionAccount struct {
	AccountNo, BeneficiaryType, BeneficiaryReference, BeneficiaryName, CurrencyCode         string
	CreditedMinor, PendingMinor, AvailableMinor, PaidMinor, ReversedMinor, RecoveryDueMinor string
	Version                                                                                 int64
}

// CommissionAccountPage is an authorized bounded projection.
type CommissionAccountPage struct {
	Items []CommissionAccount
	Total int
}

// CommissionRecord describes the frozen source and current reversal state.
type CommissionRecord struct {
	CommissionNo, OrderNo, InvoiceNo, CustomerName, BeneficiaryType, BeneficiaryReference, BeneficiaryName, CurrencyCode, Status string
	BasisMinor, OriginalMinor, PendingMinor, MaturedMinor, ReversedMinor                                                         string
	MaturesAt                                                                                                                    *time.Time
}

// CommissionRecordPage is an authorized bounded accrual history.
type CommissionRecordPage struct {
	Items []CommissionRecord
	Total int
}

// CommissionPayout records a completed offline payment entered by an administrator.
type CommissionPayout struct {
	PayoutNo, AccountNo, BeneficiaryName, CurrencyCode, AmountMinor, EvidenceReference string
	PaidAt, CreatedAt                                                                  *time.Time
	RecordedBy                                                                         int
}

// CommissionPayoutPage is a bounded evidence history.
type CommissionPayoutPage struct {
	Items []CommissionPayout
	Total int
}

// CommissionPayoutInput binds one fresh request to an account revision and proof.
type CommissionPayoutInput struct {
	AccountNo, AmountMinor, EvidenceReference, IdempotencyKey string
	PaidAt                                                    time.Time
	ExpectedVersion                                           int64
}
