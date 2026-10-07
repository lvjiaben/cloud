package billingcap

import (
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

type PrepareCreditRepayment struct {
	PaymentAttemptNo string `json:"paymentAttemptNo"`
	StatementNo      string `json:"statementNo"`
	AmountMinor      int64  `json:"amountMinor"`
}

type PreparedCreditRepayment struct {
	PaymentAttemptNo  string    `json:"paymentAttemptNo"`
	StatementNo       string    `json:"statementNo"`
	CustomerRefDigest string    `json:"customerRefDigest"`
	CurrencyCode      string    `json:"currencyCode"`
	AmountMinor       int64     `json:"amountMinor"`
	ExpiresAt         time.Time `json:"expiresAt"`
}

type ConfirmedCreditRepayment struct {
	PaymentAttemptNo string `json:"paymentAttemptNo"`
	StatementNo      string `json:"statementNo"`
	CurrencyCode     string `json:"currencyCode"`
	AmountMinor      int64  `json:"amountMinor"`
	EvidenceDigest   string `json:"evidenceDigest"`
}

type CreditRepaymentResult struct {
	PaymentAttemptNo          string `json:"paymentAttemptNo"`
	StatementNo               string `json:"statementNo"`
	CurrencyCode              string `json:"currencyCode"`
	AmountMinor               int64  `json:"amountMinor"`
	TransactionNo             string `json:"transactionNo"`
	RepaymentNo               string `json:"repaymentNo"`
	StatementStatusCode       string `json:"statementStatusCode"`
	StatementOutstandingMinor int64  `json:"statementOutstandingMinor"`
	Reused                    bool   `json:"reused"`
}

func ValidatePrepareCreditRepayment(input PrepareCreditRepayment) error {
	if !businessReferencePattern.MatchString(input.PaymentAttemptNo) || !businessReferencePattern.MatchString(input.StatementNo) || input.AmountMinor <= 0 {
		return gerror.New("billingcap: credit repayment preparation is invalid")
	}
	return nil
}

func ValidatePreparedCreditRepayment(result PreparedCreditRepayment) error {
	if !businessReferencePattern.MatchString(result.PaymentAttemptNo) || !businessReferencePattern.MatchString(result.StatementNo) || !digestPattern.MatchString(result.CustomerRefDigest) || !currencyCodePattern.MatchString(result.CurrencyCode) || result.AmountMinor <= 0 || result.ExpiresAt.IsZero() {
		return gerror.New("billingcap: prepared credit repayment is invalid")
	}
	return nil
}

func ValidateConfirmedCreditRepayment(input ConfirmedCreditRepayment) error {
	if !businessReferencePattern.MatchString(input.PaymentAttemptNo) || !businessReferencePattern.MatchString(input.StatementNo) || !currencyCodePattern.MatchString(input.CurrencyCode) || input.AmountMinor <= 0 || !digestPattern.MatchString(input.EvidenceDigest) {
		return gerror.New("billingcap: confirmed credit repayment is invalid")
	}
	return nil
}

func ValidateCreditRepaymentResult(result CreditRepaymentResult) error {
	if !businessReferencePattern.MatchString(result.PaymentAttemptNo) || !businessReferencePattern.MatchString(result.StatementNo) || !currencyCodePattern.MatchString(result.CurrencyCode) || result.AmountMinor <= 0 || !transactionNoPattern.MatchString(result.TransactionNo) || result.RepaymentNo == "" || result.StatementOutstandingMinor < 0 || (result.StatementStatusCode != "open" && result.StatementStatusCode != "partially_paid" && result.StatementStatusCode != "paid" && result.StatementStatusCode != "overdue") {
		return gerror.New("billingcap: credit repayment result is invalid")
	}
	return nil
}
