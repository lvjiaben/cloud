package backend

import (
	"context"
	"os"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
)

func TestRuntimeRechargeEvidence(t *testing.T) {
	attemptNo := os.Getenv("LINAPRO_PAYMENT_RUNTIME_ATTEMPT_NO")
	if attemptNo == "" {
		t.Skip("set LINAPRO_PAYMENT_RUNTIME_ATTEMPT_NO after the browser creates a recharge")
	}
	ctx := context.Background()
	database := g.DB()
	var evidence struct {
		AttemptStatus      string `orm:"attempt_status"`
		PurposeCode        string `orm:"purpose_code"`
		AttemptAmount      int64  `orm:"attempt_amount"`
		SignatureValid     bool   `orm:"signature_valid"`
		MerchantMatch      bool   `orm:"merchant_match"`
		AmountMatch        bool   `orm:"amount_match"`
		CurrencyMatch      bool   `orm:"currency_match"`
		CallbackProcessing string `orm:"callback_processing"`
		WorkStatus         string `orm:"work_status"`
		ContextStatus      string `orm:"context_status"`
		ContextAmount      int64  `orm:"context_amount"`
		TransactionType    string `orm:"transaction_type"`
		TransactionSource  string `orm:"transaction_source"`
		EntryAmount        int64  `orm:"entry_amount"`
		SnapshotCredit     int64  `orm:"snapshot_credit"`
		ClosingBalance     int64  `orm:"closing_balance"`
	}
	err := database.GetScan(ctx, &evidence, `
SELECT attempt.status_code AS attempt_status,
       attempt.purpose_code,
       attempt.amount_minor AS attempt_amount,
       receipt.signature_valid,
       receipt.merchant_match,
       receipt.amount_match,
       receipt.currency_match,
       receipt.processing_code AS callback_processing,
       work.status_code AS work_status,
       recharge.status_code AS context_status,
       recharge.amount_minor AS context_amount,
       transaction.transaction_type_code AS transaction_type,
       transaction.source_reference AS transaction_source,
       entry.amount_minor AS entry_amount,
       snapshot.credit_amount_minor AS snapshot_credit,
       snapshot.closing_balance_minor AS closing_balance
FROM plugin_idc_payment_attempt attempt
JOIN plugin_idc_payment_callback_receipt receipt
  ON receipt.attempt_no=attempt.attempt_no
JOIN plugin_idc_payment_work_item work
  ON work.work_type='confirm_payment' AND work.subject_no=attempt.attempt_no
JOIN plugin_idc_core_balance_recharge_context recharge
  ON recharge.payment_attempt_no=attempt.attempt_no
JOIN plugin_idc_core_billing_ledger_transaction transaction
  ON transaction.transaction_no=recharge.ledger_transaction_no
JOIN plugin_idc_core_billing_ledger_entry entry
  ON entry.transaction_no=transaction.transaction_no
 AND entry.entry_role_code='balance'
JOIN plugin_idc_core_billing_balance_snapshot snapshot
  ON snapshot.transaction_no=transaction.transaction_no
 AND snapshot.account_no=entry.account_no
WHERE attempt.attempt_no=?`, attemptNo)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.AttemptStatus != "succeeded" ||
		evidence.PurposeCode != "balance_recharge" ||
		evidence.AttemptAmount != 1234 ||
		!evidence.SignatureValid || !evidence.MerchantMatch || !evidence.AmountMatch || !evidence.CurrencyMatch ||
		evidence.CallbackProcessing != "queued" ||
		evidence.WorkStatus != "succeeded" ||
		evidence.ContextStatus != "confirmed" ||
		evidence.ContextAmount != 1234 ||
		evidence.TransactionType != "recharge" ||
		evidence.TransactionSource != attemptNo ||
		evidence.EntryAmount != 1234 ||
		evidence.SnapshotCredit != 1234 ||
		evidence.ClosingBalance < 1234 {
		t.Fatalf("incomplete runtime recharge evidence: %+v", evidence)
	}
	t.Logf("runtime recharge confirmed: attempt=%s balance=%d", attemptNo, evidence.ClosingBalance)
}
