package billingcap

import (
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

type PrepareBalanceRecharge struct {
	PaymentAttemptNo string `json:"paymentAttemptNo"`
	CurrencyCode     string `json:"currencyCode"`
	AmountMinor      int64  `json:"amountMinor"`
}

type PreparedBalanceRecharge struct {
	PaymentAttemptNo  string    `json:"paymentAttemptNo"`
	RechargeContextNo string    `json:"rechargeContextNo"`
	CustomerRefDigest string    `json:"customerRefDigest"`
	CurrencyCode      string    `json:"currencyCode"`
	AmountMinor       int64     `json:"amountMinor"`
	ExpiresAt         time.Time `json:"expiresAt"`
}

type ConfirmedBalanceRecharge struct {
	PaymentAttemptNo  string `json:"paymentAttemptNo"`
	RechargeContextNo string `json:"rechargeContextNo"`
	CurrencyCode      string `json:"currencyCode"`
	AmountMinor       int64  `json:"amountMinor"`
	EvidenceDigest    string `json:"evidenceDigest"`
}

type BalanceRechargeResult struct {
	PaymentAttemptNo  string `json:"paymentAttemptNo"`
	RechargeContextNo string `json:"rechargeContextNo"`
	CurrencyCode      string `json:"currencyCode"`
	AmountMinor       int64  `json:"amountMinor"`
	TransactionNo     string `json:"transactionNo"`
	BalanceMinor      int64  `json:"balanceMinor"`
	AvailableMinor    int64  `json:"availableMinor"`
	Reused            bool   `json:"reused"`
}

func ValidatePrepareBalanceRecharge(input PrepareBalanceRecharge) error {
	if !businessReferencePattern.MatchString(input.PaymentAttemptNo) || !currencyCodePattern.MatchString(input.CurrencyCode) || input.AmountMinor <= 0 {
		return gerror.New("billingcap: balance recharge preparation is invalid")
	}
	return nil
}

func ValidatePreparedBalanceRecharge(result PreparedBalanceRecharge) error {
	if !businessReferencePattern.MatchString(result.PaymentAttemptNo) || !businessReferencePattern.MatchString(result.RechargeContextNo) || !digestPattern.MatchString(result.CustomerRefDigest) || !currencyCodePattern.MatchString(result.CurrencyCode) || result.AmountMinor <= 0 || result.ExpiresAt.IsZero() {
		return gerror.New("billingcap: prepared balance recharge is invalid")
	}
	return nil
}

func ValidateConfirmedBalanceRecharge(input ConfirmedBalanceRecharge) error {
	if !businessReferencePattern.MatchString(input.PaymentAttemptNo) || !businessReferencePattern.MatchString(input.RechargeContextNo) || !currencyCodePattern.MatchString(input.CurrencyCode) || input.AmountMinor <= 0 || !digestPattern.MatchString(input.EvidenceDigest) {
		return gerror.New("billingcap: confirmed balance recharge is invalid")
	}
	return nil
}

func ValidateBalanceRechargeResult(result BalanceRechargeResult) error {
	if !businessReferencePattern.MatchString(result.PaymentAttemptNo) || !businessReferencePattern.MatchString(result.RechargeContextNo) || !currencyCodePattern.MatchString(result.CurrencyCode) || result.AmountMinor <= 0 || !transactionNoPattern.MatchString(result.TransactionNo) || result.BalanceMinor < result.AmountMinor || result.AvailableMinor < 0 || result.AvailableMinor > result.BalanceMinor {
		return gerror.New("billingcap: balance recharge result is invalid")
	}
	return nil
}
