package billingcap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

const MaximumChannelPaymentPayloadBytes = 4096

var (
	businessReferencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,39}$`)
	currencyCodePattern      = regexp.MustCompile(`^[A-Z]{3}$`)
	digestPattern            = regexp.MustCompile(`^[0-9a-f]{64}$`)
	transactionNoPattern     = regexp.MustCompile(`^btx_[0-9a-f]{32}$`)
	allocationNoPattern      = regexp.MustCompile(`^bpa_[0-9a-f]{32}$`)
	settlementNoPattern      = regexp.MustCompile(`^fst_[0-9a-f]{32}$`)
)

// ConfirmedChannelPayment is a payment-owner assertion after cryptographic
// callback verification or an authoritative active query. Customer, order,
// merchant accounts, and current receivable are deliberately not accepted.
type ConfirmedChannelPayment struct {
	PaymentAttemptNo   string `json:"paymentAttemptNo"`
	InvoiceNo          string `json:"invoiceNo"`
	CurrencyCode       string `json:"currencyCode"`
	AmountMinor        int64  `json:"amountMinor"`
	TotalAmountMinor   int64  `json:"totalAmountMinor"`
	BalanceHoldNo      string `json:"balanceHoldNo,omitempty"`
	BalanceAmountMinor int64  `json:"balanceAmountMinor"`
	EvidenceDigest     string `json:"evidenceDigest"`
}

// PrepareChannelPayment asks the core owner to re-read one authenticated
// payment context and optionally reserve same-currency customer balance.
type PrepareChannelPayment struct {
	PaymentAttemptNo   string `json:"paymentAttemptNo"`
	PaymentContextNo   string `json:"paymentContextNo"`
	BalanceAmountMinor int64  `json:"balanceAmountMinor"`
}

type PreparedChannelPayment struct {
	PaymentAttemptNo   string    `json:"paymentAttemptNo"`
	PaymentContextNo   string    `json:"paymentContextNo"`
	InvoiceNo          string    `json:"invoiceNo"`
	OrderNo            string    `json:"orderNo"`
	CustomerRefDigest  string    `json:"customerRefDigest"`
	CurrencyCode       string    `json:"currencyCode"`
	TotalAmountMinor   int64     `json:"totalAmountMinor"`
	ChannelAmountMinor int64     `json:"channelAmountMinor"`
	BalanceHoldNo      string    `json:"balanceHoldNo,omitempty"`
	BalanceAmountMinor int64     `json:"balanceAmountMinor"`
	ExpiresAt          time.Time `json:"expiresAt"`
}

type ReleaseChannelPayment struct {
	PaymentAttemptNo string `json:"paymentAttemptNo"`
	BalanceHoldNo    string `json:"balanceHoldNo"`
}

type ReleasedChannelPayment struct {
	PaymentAttemptNo string `json:"paymentAttemptNo"`
	BalanceHoldNo    string `json:"balanceHoldNo"`
	Status           string `json:"status"`
	Reused           bool   `json:"reused"`
}

// AllocateExceptionFund asks the core owner to apply one already verified
// late channel payment to a different, customer-created unpaid order.
// Customer ownership, current eligibility and the exact outstanding amount
// are derived again by the owner; the payment plugin cannot assert them.
type AllocateExceptionFund struct {
	ExceptionNo       string `json:"exceptionNo"`
	PaymentAttemptNo  string `json:"paymentAttemptNo"`
	TargetOrderNo     string `json:"targetOrderNo"`
	CustomerRefDigest string `json:"customerRefDigest"`
	CurrencyCode      string `json:"currencyCode"`
	AmountMinor       int64  `json:"amountMinor"`
	EvidenceDigest    string `json:"evidenceDigest"`
}

type ExceptionFundAllocation struct {
	ExceptionNo      string `json:"exceptionNo"`
	PaymentAttemptNo string `json:"paymentAttemptNo"`
	TargetOrderNo    string `json:"targetOrderNo"`
	InvoiceNo        string `json:"invoiceNo"`
	AllocationNo     string `json:"allocationNo"`
	SettlementNo     string `json:"settlementNo"`
	CurrencyCode     string `json:"currencyCode"`
	AmountMinor      int64  `json:"amountMinor"`
	Reused           bool   `json:"reused"`
}

type PaymentClosureWork struct {
	Found          bool   `json:"found"`
	CancellationNo string `json:"cancellationNo,omitempty"`
	OrderNo        string `json:"orderNo,omitempty"`
}

type ReportPaymentClosure struct {
	CancellationNo string `json:"cancellationNo"`
	OutcomeCode    string `json:"outcomeCode"`
	Reference      string `json:"reference"`
	EvidenceDigest string `json:"evidenceDigest"`
}

type PaymentClosureReport struct {
	CancellationNo string `json:"cancellationNo"`
	StatusCode     string `json:"statusCode"`
	OutcomeCode    string `json:"outcomeCode"`
}

type PaymentActor struct {
	PrincipalRefDigest string `json:"principalRefDigest"`
}

// ChannelPaymentResult separates the durable channel-money allocation from
// the optional order settlement that follows it.
type ChannelPaymentResult struct {
	PaymentAttemptNo     string `json:"paymentAttemptNo"`
	TransactionNo        string `json:"transactionNo"`
	AllocationNo         string `json:"allocationNo"`
	InvoiceNo            string `json:"invoiceNo"`
	OrderNo              string `json:"orderNo"`
	CurrencyCode         string `json:"currencyCode"`
	AmountMinor          int64  `json:"amountMinor"`
	TotalAmountMinor     int64  `json:"totalAmountMinor"`
	BalanceAmountMinor   int64  `json:"balanceAmountMinor"`
	BalanceTransactionNo string `json:"balanceTransactionNo,omitempty"`
	BalanceAllocationNo  string `json:"balanceAllocationNo,omitempty"`
	PaidAmountMinor      int64  `json:"paidAmountMinor"`
	InvoiceStatus        string `json:"invoiceStatus"`
	SettlementNo         string `json:"settlementNo,omitempty"`
	FundingReused        bool   `json:"fundingReused"`
	SettlementReused     bool   `json:"settlementReused"`
}

type Service interface {
	RefundExecutionService
	CurrentPaymentActor(context.Context) (*PaymentActor, error)
	PrepareChannelPayment(context.Context, PrepareChannelPayment) (*PreparedChannelPayment, error)
	// ConfirmChannelPayment atomically posts and allocates one payment-owner
	// confirmation, then independently requests settlement when the invoice is
	// fully funded. A settlement failure never reverses confirmed money.
	ConfirmChannelPayment(context.Context, ConfirmedChannelPayment) (*ChannelPaymentResult, error)
	ReleaseChannelPayment(context.Context, ReleaseChannelPayment) (*ReleasedChannelPayment, error)
	AllocateExceptionFund(context.Context, AllocateExceptionFund) (*ExceptionFundAllocation, error)
	ClaimPaymentClosure(context.Context) (*PaymentClosureWork, error)
	ReportPaymentClosure(context.Context, ReportPaymentClosure) (*PaymentClosureReport, error)
	PrepareCreditRepayment(context.Context, PrepareCreditRepayment) (*PreparedCreditRepayment, error)
	ConfirmCreditRepayment(context.Context, ConfirmedCreditRepayment) (*CreditRepaymentResult, error)
	PrepareBalanceRecharge(context.Context, PrepareBalanceRecharge) (*PreparedBalanceRecharge, error)
	ConfirmBalanceRecharge(context.Context, ConfirmedBalanceRecharge) (*BalanceRechargeResult, error)
}

func ValidatePaymentClosureWork(work PaymentClosureWork) error {
	if !work.Found {
		if work.CancellationNo != "" || work.OrderNo != "" {
			return gerror.New("billingcap: empty payment closure work is invalid")
		}
		return nil
	}
	if !businessReferencePattern.MatchString(work.CancellationNo) || !businessReferencePattern.MatchString(work.OrderNo) {
		return gerror.New("billingcap: payment closure work is invalid")
	}
	return nil
}

func ValidateReportPaymentClosure(input ReportPaymentClosure) error {
	if !businessReferencePattern.MatchString(input.CancellationNo) || !businessReferencePattern.MatchString(input.Reference) || !digestPattern.MatchString(input.EvidenceDigest) || (input.OutcomeCode != "closed" && input.OutcomeCode != "no_attempts" && input.OutcomeCode != "confirmed_success" && input.OutcomeCode != "unknown") {
		return gerror.New("billingcap: payment closure report is invalid")
	}
	return nil
}

func ValidateAllocateExceptionFund(input AllocateExceptionFund) error {
	if !businessReferencePattern.MatchString(input.ExceptionNo) ||
		!businessReferencePattern.MatchString(input.PaymentAttemptNo) ||
		!businessReferencePattern.MatchString(input.TargetOrderNo) ||
		!digestPattern.MatchString(input.CustomerRefDigest) ||
		!currencyCodePattern.MatchString(input.CurrencyCode) || input.AmountMinor <= 0 ||
		!digestPattern.MatchString(input.EvidenceDigest) {
		return gerror.New("billingcap: exception fund allocation is invalid")
	}
	return nil
}

func ValidateExceptionFundAllocation(result ExceptionFundAllocation) error {
	if !businessReferencePattern.MatchString(result.ExceptionNo) ||
		!businessReferencePattern.MatchString(result.PaymentAttemptNo) ||
		!businessReferencePattern.MatchString(result.TargetOrderNo) ||
		!businessReferencePattern.MatchString(result.InvoiceNo) ||
		!allocationNoPattern.MatchString(result.AllocationNo) ||
		!settlementNoPattern.MatchString(result.SettlementNo) ||
		!currencyCodePattern.MatchString(result.CurrencyCode) || result.AmountMinor <= 0 {
		return gerror.New("billingcap: exception fund allocation result is invalid")
	}
	return nil
}

func ValidatePaymentActor(actor PaymentActor) error {
	if !digestPattern.MatchString(actor.PrincipalRefDigest) {
		return gerror.New("billingcap: payment actor is invalid")
	}
	return nil
}

func ValidatePrepareChannelPayment(input PrepareChannelPayment) error {
	if !businessReferencePattern.MatchString(input.PaymentAttemptNo) ||
		!businessReferencePattern.MatchString(input.PaymentContextNo) || input.BalanceAmountMinor < 0 {
		return gerror.New("billingcap: channel payment preparation is invalid")
	}
	return nil
}

func ValidatePreparedChannelPayment(result PreparedChannelPayment) error {
	if !businessReferencePattern.MatchString(result.PaymentAttemptNo) ||
		!businessReferencePattern.MatchString(result.PaymentContextNo) ||
		!businessReferencePattern.MatchString(result.InvoiceNo) ||
		!businessReferencePattern.MatchString(result.OrderNo) ||
		!digestPattern.MatchString(result.CustomerRefDigest) ||
		!currencyCodePattern.MatchString(result.CurrencyCode) || result.TotalAmountMinor <= 0 ||
		result.ChannelAmountMinor <= 0 || result.BalanceAmountMinor < 0 ||
		result.ChannelAmountMinor+result.BalanceAmountMinor != result.TotalAmountMinor || result.ExpiresAt.IsZero() ||
		((result.BalanceAmountMinor == 0) != (result.BalanceHoldNo == "")) ||
		(result.BalanceHoldNo != "" && !businessReferencePattern.MatchString(result.BalanceHoldNo)) {
		return gerror.New("billingcap: prepared channel payment result is invalid")
	}
	return nil
}

func ValidateReleaseChannelPayment(input ReleaseChannelPayment) error {
	if !businessReferencePattern.MatchString(input.PaymentAttemptNo) || !businessReferencePattern.MatchString(input.BalanceHoldNo) {
		return gerror.New("billingcap: channel payment release is invalid")
	}
	return nil
}

func CustomerReferenceDigest(customerNo string) string {
	digest := sha256.Sum256([]byte("billingcap-customer-reference\x00" + customerNo))
	return hex.EncodeToString(digest[:])
}

// PrincipalReferenceDigest binds a payment projection to the authenticated
// one-to-one customer login without exposing the customer or user identifier.
func PrincipalReferenceDigest(userID int) string {
	digest := sha256.Sum256([]byte("billingcap-principal-reference\x00" + strconv.Itoa(userID)))
	return hex.EncodeToString(digest[:])
}

func ValidateConfirmedChannelPayment(input ConfirmedChannelPayment) error {
	if !businessReferencePattern.MatchString(input.PaymentAttemptNo) ||
		!businessReferencePattern.MatchString(input.InvoiceNo) ||
		!currencyCodePattern.MatchString(input.CurrencyCode) || input.AmountMinor <= 0 ||
		input.TotalAmountMinor < input.AmountMinor || input.BalanceAmountMinor < 0 ||
		input.AmountMinor+input.BalanceAmountMinor != input.TotalAmountMinor ||
		((input.BalanceAmountMinor == 0) != (input.BalanceHoldNo == "")) ||
		(input.BalanceHoldNo != "" && !businessReferencePattern.MatchString(input.BalanceHoldNo)) ||
		!digestPattern.MatchString(input.EvidenceDigest) {
		return gerror.New("billingcap: confirmed channel payment is invalid")
	}
	return nil
}

func ValidateChannelPaymentResult(result ChannelPaymentResult) error {
	if !businessReferencePattern.MatchString(result.PaymentAttemptNo) ||
		!transactionNoPattern.MatchString(result.TransactionNo) ||
		!allocationNoPattern.MatchString(result.AllocationNo) ||
		!businessReferencePattern.MatchString(result.InvoiceNo) ||
		!businessReferencePattern.MatchString(result.OrderNo) ||
		!currencyCodePattern.MatchString(result.CurrencyCode) ||
		result.AmountMinor <= 0 || result.TotalAmountMinor < result.AmountMinor ||
		result.BalanceAmountMinor < 0 || result.AmountMinor+result.BalanceAmountMinor != result.TotalAmountMinor ||
		result.PaidAmountMinor < result.TotalAmountMinor ||
		(result.InvoiceStatus != "partially_paid" && result.InvoiceStatus != "paid") {
		return gerror.New("billingcap: channel payment result is invalid")
	}
	if result.SettlementNo != "" {
		if result.InvoiceStatus != "paid" || !settlementNoPattern.MatchString(result.SettlementNo) {
			return gerror.New("billingcap: channel payment settlement result is invalid")
		}
	} else if result.SettlementReused {
		return gerror.New("billingcap: channel payment settlement replay is invalid")
	}
	if result.BalanceAmountMinor == 0 {
		if result.BalanceTransactionNo != "" || result.BalanceAllocationNo != "" {
			return gerror.New("billingcap: channel payment balance result is invalid")
		}
	} else if !transactionNoPattern.MatchString(result.BalanceTransactionNo) ||
		!allocationNoPattern.MatchString(result.BalanceAllocationNo) {
		return gerror.New("billingcap: channel payment balance result is invalid")
	}
	return nil
}
