// This file verifies financial settlement concurrency, evidence eligibility,
// transactional rollback, immutability, and migration safety on PostgreSQL.

package commerce

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/billingcap"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
	billingservice "lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type failingFinancialSettlementEventWriter struct{}
type paymentClosureOwnerTestStub struct{}

func (paymentClosureOwnerTestStub) ClaimPaymentClosure(context.Context) (*billingcap.PaymentClosureWork, error) {
	return &billingcap.PaymentClosureWork{}, nil
}
func (paymentClosureOwnerTestStub) ReportPaymentClosure(_ context.Context, input billingcap.ReportPaymentClosure) (*billingcap.PaymentClosureReport, error) {
	return &billingcap.PaymentClosureReport{CancellationNo: input.CancellationNo, StatusCode: "completed", OutcomeCode: input.OutcomeCode}, nil
}

func newChannelPaymentOwnerTestService(
	t *testing.T,
	database gdb.DB,
	settlements billingservice.FinancialSettlementService,
	readers ...principal.Reader,
) (billingcap.Service, error) {
	t.Helper()
	events, err := outbox.NewEnqueuer(database, outbox.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := asynctask.New(asynctask.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return billingservice.NewChannelPaymentOwnerService(
		database, settlements, events, tasks, paymentClosureOwnerTestStub{}, readers...,
	)
}

func newLocalRefundWorkerTestService(
	t *testing.T,
	database gdb.DB,
) (billingservice.LocalRefundWorker, error) {
	t.Helper()
	events, err := outbox.NewEnqueuer(database, outbox.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := asynctask.New(asynctask.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return billingservice.NewLocalRefundWorker(database, events, tasks)
}

func (failingFinancialSettlementEventWriter) Enqueue(
	context.Context,
	outbox.EnqueueInput,
) (*outbox.Event, error) {
	return nil, errors.New("injected financial settlement outbox failure")
}


func TestFinancialSettlementPostgreSQLSerializesBalanceFunding(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "settlement-balance-checkout-0001",
	})
	if err != nil {
		t.Fatalf("checkout balance settlement order: %v", err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	balance, err := billingservice.NewBalanceService(scenario.database)
	if err != nil {
		t.Fatalf("create balance service: %v", err)
	}
	if _, err = balance.PostConfirmedRecharge(scenario.ctx, billingservice.ConfirmedRechargeInput{
		CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY",
		AmountMinor: checkout.TotalAmountMinor, SourceReference: "settlement-recharge-0001",
		SourceSnapshotDigest: strings.Repeat("a", 64),
	}); err != nil {
		t.Fatalf("fund settlement balance: %v", err)
	}
	invoices, err := billingservice.NewInvoiceService(scenario.database)
	if err != nil {
		t.Fatalf("create invoice service: %v", err)
	}
	if _, err = invoices.PayFromBalance(scenario.ctx, billingservice.InvoiceBalancePaymentInput{
		InvoiceNo: checkout.InvoiceNo, SourceReference: "settlement-balance-payment-0001",
		SourceSnapshotDigest: strings.Repeat("b", 64),
	}); err != nil {
		t.Fatalf("pay settlement invoice from balance: %v", err)
	}
	settlements := newFinancialSettlementTestService(t, scenario)

	const workerCount = 8
	results := make([]*billingservice.FinancialSettlement, workerCount)
	errorsFound := make([]error, workerCount)
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for index := range workerCount {
		go func() {
			defer workers.Done()
			<-start
			results[index], errorsFound[index] = settlements.SettleOrder(
				scenario.ctx, checkout.OrderNo,
			)
		}()
	}
	close(start)
	workers.Wait()
	settlementNo := ""
	createdCount := 0
	for index := range workerCount {
		if errorsFound[index] != nil || results[index] == nil {
			t.Fatalf("settlement worker %d result=%+v err=%v", index, results[index], errorsFound[index])
		}
		if settlementNo == "" {
			settlementNo = results[index].SettlementNo
		} else if results[index].SettlementNo != settlementNo {
			t.Fatalf("settlement workers diverged: %q != %q", settlementNo, results[index].SettlementNo)
		}
		if !results[index].Reused {
			createdCount++
		}
		if results[index].Basis != billingservice.SettlementBasisFundedInvoice ||
			results[index].PaidAmountMinor != checkout.TotalAmountMinor ||
			len(results[index].Sources) != 1 ||
			results[index].Sources[0].SourceType !=
				billingservice.FinancialSettlementSourceBalance {
			t.Fatalf("unexpected funded settlement projection: %+v", results[index])
		}
	}
	if createdCount != 1 {
		t.Fatalf("concurrent settlement created count=%d want=1", createdCount)
	}
	assertFinancialSettlementState(t, scenario, checkout.OrderNo, settlementNo, 1)
	if _, err = scenario.database.Exec(scenario.ctx, `
UPDATE plugin_idc_core_financial_settlement
SET paid_amount_minor = paid_amount_minor + 1
WHERE settlement_no = ?`, settlementNo); err == nil {
		t.Fatal("immutable financial settlement accepted an update")
	}
}

func TestAccountBalancePaymentPreservesConfirmedFundsWhenSettlementRetries(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "settlement-balance-owner-checkout-0001",
	})
	if err != nil {
		t.Fatalf("checkout balance owner order: %v", err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	balance, err := billingservice.NewBalanceService(scenario.database)
	if err != nil {
		t.Fatalf("create balance service: %v", err)
	}
	if _, err = balance.PostConfirmedRecharge(scenario.ctx, billingservice.ConfirmedRechargeInput{
		CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY",
		AmountMinor:          checkout.TotalAmountMinor + 500,
		SourceReference:      "settlement-balance-owner-recharge-0001",
		SourceSnapshotDigest: strings.Repeat("a", 64),
	}); err != nil {
		t.Fatalf("fund balance owner account: %v", err)
	}
	commands, err := billingservice.NewAccountFinanceCommandService(
		scenario.database,
		commercePrincipalStub{audience: &principal.Audience{
			UserID: 101, Code: principal.AudienceCustomer,
			Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-1",
		}},
		newFinancialSettlementTestService(t, scenario),
	)
	if err != nil {
		t.Fatalf("create account financial commands: %v", err)
	}
	input := billingservice.AccountBalancePaymentInput{
		InvoiceNo: checkout.InvoiceNo, IdempotencyKey: "settlement-balance-owner-payment-0001",
	}
	if result, paymentErr := commands.PayAccountInvoiceFromBalance(scenario.ctx, input); paymentErr == nil || result != nil {
		t.Fatalf("payment without verified identity result=%+v err=%v", result, paymentErr)
	}
	var waiting struct {
		InvoiceStatus  string `orm:"invoice_status"`
		PaidMinor      int64  `orm:"paid_minor"`
		BalanceMinor   int64  `orm:"balance_minor"`
		OrderStatus    string `orm:"order_status"`
		SettlementRows int64  `orm:"settlement_rows"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &waiting, `
SELECT invoice.status_code AS invoice_status,
       invoice.paid_amount_minor AS paid_minor,
       COALESCE((
         SELECT SUM(CASE entry.direction_code
                      WHEN 'credit' THEN entry.amount_minor
                      ELSE -entry.amount_minor END)
         FROM plugin_idc_core_billing_ledger_entry entry
         WHERE entry.account_no = account.account_no
       ), 0) AS balance_minor,
       order_row.status_code AS order_status,
       (SELECT COUNT(*) FROM plugin_idc_core_financial_settlement settlement
         WHERE settlement.order_no = order_row.order_no) AS settlement_rows
FROM plugin_idc_core_billing_invoice invoice
JOIN plugin_idc_core_commerce_order order_row
  ON order_row.order_no = invoice.order_no
JOIN plugin_idc_core_billing_ledger_account account
  ON account.customer_no = invoice.customer_no
 AND account.currency_code = invoice.currency_code
 AND account.account_type_code = 'customer_balance'
WHERE invoice.invoice_no = ?`, checkout.InvoiceNo); err != nil {
		t.Fatalf("load preserved balance payment state: %v", err)
	}
	if waiting.InvoiceStatus != "paid" || waiting.PaidMinor != checkout.TotalAmountMinor ||
		waiting.BalanceMinor != 500 || waiting.OrderStatus != "pending_payment" ||
		waiting.SettlementRows != 0 {
		t.Fatalf("confirmed funds were not preserved independently: %+v", waiting)
	}

	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	replayed, err := commands.PayAccountInvoiceFromBalance(scenario.ctx, input)
	if err != nil || replayed == nil || !replayed.Reused ||
		replayed.InvoiceNo != checkout.InvoiceNo || replayed.BalanceMinor != 500 {
		t.Fatalf("retry preserved balance payment result=%+v err=%v", replayed, err)
	}
	settlementNo, err := scenario.database.GetValue(scenario.ctx, `
SELECT settlement_no
FROM plugin_idc_core_financial_settlement
WHERE order_no = ?`, checkout.OrderNo)
	if err != nil || settlementNo.String() == "" {
		t.Fatalf("load retried financial settlement number: value=%q err=%v", settlementNo.String(), err)
	}
	assertFinancialSettlementState(t, scenario, checkout.OrderNo, settlementNo.String(), 1)
}

func TestFinancialSettlementPostgreSQLAcceptsReviewedManualReceipt(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "settlement-manual-checkout-0001",
	})
	if err != nil {
		t.Fatalf("checkout manual settlement order: %v", err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	adjustments, err := billingservice.NewFinancialAdjustmentService(scenario.database)
	if err != nil {
		t.Fatalf("create manual receipt service: %v", err)
	}
	if _, err = adjustments.ConfigureApprovalPolicy(scenario.ctx, billingservice.AdjustmentPolicyInput{
		CurrencyCode: "CNY", ReviewThresholdMinor: 1, ConfiguredBy: 1001,
	}); err != nil {
		t.Fatalf("configure manual receipt approval: %v", err)
	}
	request, err := adjustments.CreateManualReceipt(scenario.ctx, billingservice.ManualReceiptInput{
		InvoiceNo: checkout.InvoiceNo, AmountMinor: checkout.TotalAmountMinor,
		ReasonCode: "verified_bank_receipt", ProofReference: "proof-private-0001",
		ReceivingAccountReference: "merchant-bank-private-0001",
		BusinessPurposeCode:       "server_order", EvidenceDigest: strings.Repeat("c", 64),
		IdempotencyKey: "settlement-manual-receipt-0001", RequestedBy: 1001,
	})
	if err != nil || request.Status != billingservice.AdjustmentStatusPendingReview {
		t.Fatalf("create reviewed manual receipt result=%+v err=%v", request, err)
	}
	posted, err := adjustments.ReviewAdjustment(scenario.ctx, billingservice.AdjustmentReviewInput{
		AdjustmentNo: request.AdjustmentNo,
		Decision:     billingservice.AdjustmentDecisionApprove, ReviewerID: 1002,
		CommentDigest: strings.Repeat("d", 64), EvidenceDigest: strings.Repeat("e", 64),
	})
	if err != nil || posted.Status != billingservice.AdjustmentStatusPosted || posted.Invoice == nil {
		t.Fatalf("post reviewed manual receipt result=%+v err=%v", posted, err)
	}
	settlement, err := newFinancialSettlementTestService(t, scenario).SettleOrder(
		scenario.ctx, checkout.OrderNo,
	)
	if err != nil || settlement.Basis != billingservice.SettlementBasisFundedInvoice ||
		len(settlement.Sources) != 1 ||
		settlement.Sources[0].SourceType !=
			billingservice.FinancialSettlementSourceManualReceipt {
		t.Fatalf("settle reviewed manual receipt result=%+v err=%v", settlement, err)
	}
	assertFinancialSettlementState(t, scenario, checkout.OrderNo, settlement.SettlementNo, 1)
}

func TestChannelPaymentOwnerPostgreSQLSerializesFundingAndSettlement(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "settlement-channel-checkout-0001",
	})
	if err != nil {
		t.Fatalf("checkout channel settlement order: %v", err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	owner, err := newChannelPaymentOwnerTestService(t,
		scenario.database, newFinancialSettlementTestService(t, scenario),
	)
	if err != nil {
		t.Fatalf("create channel payment owner: %v", err)
	}
	input := billingcap.ConfirmedChannelPayment{
		PaymentAttemptNo: "PAT_channel_0001", InvoiceNo: checkout.InvoiceNo,
		CurrencyCode: "CNY", AmountMinor: checkout.TotalAmountMinor, TotalAmountMinor: checkout.TotalAmountMinor,
		EvidenceDigest: strings.Repeat("7", 64),
	}
	const workerCount = 8
	results := make([]*billingcap.ChannelPaymentResult, workerCount)
	errorsFound := make([]error, workerCount)
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for index := range workerCount {
		go func() {
			defer workers.Done()
			<-start
			results[index], errorsFound[index] = owner.ConfirmChannelPayment(scenario.ctx, input)
		}()
	}
	close(start)
	workers.Wait()
	transactionNo, allocationNo, settlementNo := "", "", ""
	createdFunding, createdSettlement := 0, 0
	for index := range workerCount {
		result := results[index]
		if errorsFound[index] != nil || result == nil ||
			result.AmountMinor != checkout.TotalAmountMinor ||
			result.PaidAmountMinor != checkout.TotalAmountMinor ||
			result.InvoiceStatus != "paid" || result.SettlementNo == "" {
			t.Fatalf("channel worker %d result=%+v err=%v", index, result, errorsFound[index])
		}
		if transactionNo == "" {
			transactionNo, allocationNo, settlementNo = result.TransactionNo, result.AllocationNo, result.SettlementNo
		} else if result.TransactionNo != transactionNo || result.AllocationNo != allocationNo ||
			result.SettlementNo != settlementNo {
			t.Fatalf("channel workers diverged: first=%q/%q/%q current=%+v",
				transactionNo, allocationNo, settlementNo, result)
		}
		if !result.FundingReused {
			createdFunding++
		}
		if !result.SettlementReused {
			createdSettlement++
		}
	}
	if createdFunding != 1 || createdSettlement != 1 {
		t.Fatalf("channel creation counts funding=%d settlement=%d", createdFunding, createdSettlement)
	}
	var counts struct {
		Transactions int64 `orm:"transactions"`
		Allocations  int64 `orm:"allocations"`
		Settlements  int64 `orm:"settlements"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &counts, `
SELECT
  (SELECT COUNT(*) FROM plugin_idc_core_billing_ledger_transaction
    WHERE source_type_code = 'payment_attempt' AND source_reference = ?) AS transactions,
  (SELECT COUNT(*) FROM plugin_idc_core_billing_payment_allocation
    WHERE payment_source_type_code = 'channel_payment'
      AND payment_source_reference = ?) AS allocations,
  (SELECT COUNT(*) FROM plugin_idc_core_financial_settlement
    WHERE order_no = ?) AS settlements`, input.PaymentAttemptNo, input.PaymentAttemptNo, checkout.OrderNo); err != nil {
		t.Fatalf("load channel payment counts: %v", err)
	}
	if counts.Transactions != 1 || counts.Allocations != 1 || counts.Settlements != 1 {
		t.Fatalf("channel payment was not exactly once: %+v", counts)
	}
	assertFinancialSettlementState(t, scenario, checkout.OrderNo, settlementNo, 1)
}

func TestExceptionFundAllocationRevalidatesCustomerAmountAndPayability(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	if _, err := scenario.database.Exec(scenario.ctx, `INSERT INTO plugin_idc_core_customer_identity_binding(binding_no,sys_user_id,customer_no,source_code,source_reference_no,created_by,updated_by) VALUES('BND_exception_fund_0001',101,'customer-commerce-service-1','migration','exception-fund-test',101,101)`); err != nil {
		t.Fatalf("insert exception fund customer binding: %v", err)
	}
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "exception-fund-checkout-0001",
	})
	if err != nil {
		t.Fatalf("checkout exception target: %v", err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	owner, err := newChannelPaymentOwnerTestService(t, scenario.database, newFinancialSettlementTestService(t, scenario))
	if err != nil {
		t.Fatal(err)
	}
	input := billingcap.AllocateExceptionFund{
		ExceptionNo: "PEX_exception_fund_0001", PaymentAttemptNo: "PAT_exception_fund_0001",
		TargetOrderNo: checkout.OrderNo, CustomerRefDigest: billingcap.PrincipalReferenceDigest(101),
		CurrencyCode: "CNY", AmountMinor: checkout.TotalAmountMinor, EvidenceDigest: strings.Repeat("a", 64),
	}
	foreignCustomer := input
	foreignCustomer.CustomerRefDigest = billingcap.PrincipalReferenceDigest(202)
	if result, allocateErr := owner.AllocateExceptionFund(scenario.ctx, foreignCustomer); allocateErr == nil || result != nil {
		t.Fatalf("foreign customer exception fund was accepted: result=%+v err=%v", result, allocateErr)
	}
	wrongAmount := input
	wrongAmount.AmountMinor--
	if result, allocateErr := owner.AllocateExceptionFund(scenario.ctx, wrongAmount); allocateErr == nil || result != nil {
		t.Fatalf("wrong amount exception fund was accepted: result=%+v err=%v", result, allocateErr)
	}
	wrongCurrency := input
	wrongCurrency.CurrencyCode = "USD"
	if result, allocateErr := owner.AllocateExceptionFund(scenario.ctx, wrongCurrency); allocateErr == nil || result != nil {
		t.Fatalf("wrong currency exception fund was accepted: result=%+v err=%v", result, allocateErr)
	}
	const workers = 8
	results := make([]*billingcap.ExceptionFundAllocation, workers)
	errorsFound := make([]error, workers)
	start := make(chan struct{})
	var group sync.WaitGroup
	group.Add(workers)
	for index := range workers {
		go func() {
			defer group.Done()
			<-start
			results[index], errorsFound[index] = owner.AllocateExceptionFund(scenario.ctx, input)
		}()
	}
	close(start)
	group.Wait()
	allocated := results[0]
	for index, result := range results {
		if errorsFound[index] != nil || result == nil || result.TargetOrderNo != checkout.OrderNo ||
			result.InvoiceNo != checkout.InvoiceNo || result.AmountMinor != checkout.TotalAmountMinor ||
			result.AllocationNo == "" || result.SettlementNo == "" || allocated == nil ||
			result.AllocationNo != allocated.AllocationNo || result.SettlementNo != allocated.SettlementNo {
			t.Fatalf("concurrent exception allocation worker=%d result=%+v first=%+v err=%v", index, result, allocated, errorsFound[index])
		}
	}
	assertFinancialSettlementState(t, scenario, checkout.OrderNo, allocated.SettlementNo, 1)
	if replayed, replayErr := owner.AllocateExceptionFund(scenario.ctx, input); replayErr != nil || replayed == nil ||
		replayed.AllocationNo != allocated.AllocationNo || replayed.SettlementNo != allocated.SettlementNo || !replayed.Reused {
		t.Fatalf("exact exception fund replay diverged: result=%+v err=%v", replayed, replayErr)
	}
}

func TestChannelPaymentOwnerAtomicallyConsumesBalanceHoldAndReleasesOnFailure(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "settlement-combination-checkout-0001"})
	if err != nil || checkout.TotalAmountMinor < 2 {
		t.Fatalf("checkout combination order=%+v err=%v", checkout, err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	balances, err := billingservice.NewBalanceService(scenario.database)
	if err != nil {
		t.Fatal(err)
	}
	balanceMinor := checkout.TotalAmountMinor / 3
	if _, err = balances.PostConfirmedRecharge(scenario.ctx, billingservice.ConfirmedRechargeInput{CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", AmountMinor: balanceMinor * 2, SourceReference: "combination-recharge-0001", SourceSnapshotDigest: strings.Repeat("1", 64)}); err != nil {
		t.Fatalf("fund combination balance: %v", err)
	}
	hold, err := balances.CreateHold(scenario.ctx, billingservice.HoldCreateInput{CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", AmountMinor: balanceMinor, SourceType: "payment_attempt", SourceReference: "PAT_combination_0001", SourceSnapshotDigest: strings.Repeat("2", 64), ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatalf("create combination hold: %v", err)
	}
	owner, err := newChannelPaymentOwnerTestService(t, scenario.database, newFinancialSettlementTestService(t, scenario))
	if err != nil {
		t.Fatal(err)
	}
	channelMinor := checkout.TotalAmountMinor - balanceMinor
	result, err := owner.ConfirmChannelPayment(scenario.ctx, billingcap.ConfirmedChannelPayment{PaymentAttemptNo: "PAT_combination_0001", InvoiceNo: checkout.InvoiceNo, CurrencyCode: "CNY", AmountMinor: channelMinor, TotalAmountMinor: checkout.TotalAmountMinor, BalanceHoldNo: hold.HoldNo, BalanceAmountMinor: balanceMinor, EvidenceDigest: strings.Repeat("3", 64)})
	if err != nil || result == nil || result.InvoiceStatus != "paid" || result.BalanceTransactionNo == "" || result.BalanceAllocationNo == "" || result.SettlementNo == "" {
		t.Fatalf("confirm combination result=%+v err=%v", result, err)
	}
	var state struct {
		HoldStatus  string `orm:"hold_status"`
		Consumed    int64  `orm:"consumed"`
		Allocations int64  `orm:"allocations"`
		Settlements int64  `orm:"settlements"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &state, `SELECT h.status_code AS hold_status,h.consumed_amount_minor AS consumed,(SELECT COUNT(*) FROM plugin_idc_core_billing_payment_allocation WHERE invoice_no=? AND payment_source_reference IN (?,?)) AS allocations,(SELECT COUNT(*) FROM plugin_idc_core_financial_settlement WHERE order_no=?) AS settlements FROM plugin_idc_core_billing_fund_hold h WHERE h.hold_no=?`, checkout.InvoiceNo, "PAT_combination_0001", channelBalanceReferenceForTest("PAT_combination_0001"), checkout.OrderNo, hold.HoldNo); err != nil {
		t.Fatal(err)
	}
	if state.HoldStatus != "consumed" || state.Consumed != balanceMinor || state.Allocations != 2 || state.Settlements != 1 {
		t.Fatalf("combination state=%+v", state)
	}

	releaseHold, err := balances.CreateHold(scenario.ctx, billingservice.HoldCreateInput{CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", AmountMinor: balanceMinor, SourceType: "payment_attempt", SourceReference: "PAT_combination_release_0001", SourceSnapshotDigest: strings.Repeat("4", 64), ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	released, err := owner.ReleaseChannelPayment(scenario.ctx, billingcap.ReleaseChannelPayment{PaymentAttemptNo: "PAT_combination_release_0001", BalanceHoldNo: releaseHold.HoldNo})
	if err != nil || released.Status != "released" {
		t.Fatalf("release=%+v err=%v", released, err)
	}
}

func TestChannelPaymentOwnerPreparesOnlyCurrentCustomerAndReusesBalanceHold(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "settlement-preparation-checkout-0001"})
	if err != nil || checkout.TotalAmountMinor < 2 {
		t.Fatalf("checkout=%+v err=%v", checkout, err)
	}
	reader := commercePrincipalStub{audience: &principal.Audience{UserID: 101, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-1"}}
	preparations, err := NewPaymentPreparationService(scenario.database, reader)
	if err != nil {
		t.Fatal(err)
	}
	contextResult, err := preparations.PrepareAccountOrderPayment(scenario.ctx, checkout.OrderNo)
	if err != nil || !contextResult.Ready {
		t.Fatalf("payment context=%+v err=%v", contextResult, err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	balances, err := billingservice.NewBalanceService(scenario.database)
	if err != nil {
		t.Fatal(err)
	}
	balanceMinor := checkout.TotalAmountMinor / 4
	if _, err = balances.PostConfirmedRecharge(scenario.ctx, billingservice.ConfirmedRechargeInput{CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", AmountMinor: balanceMinor, SourceReference: "preparation-recharge-0001", SourceSnapshotDigest: strings.Repeat("5", 64)}); err != nil {
		t.Fatal(err)
	}
	owner, err := newChannelPaymentOwnerTestService(t, scenario.database, newFinancialSettlementTestService(t, scenario), reader)
	if err != nil {
		t.Fatal(err)
	}
	input := billingcap.PrepareChannelPayment{PaymentAttemptNo: "PAT_preparation_0001", PaymentContextNo: contextResult.ContextNo, BalanceAmountMinor: balanceMinor}
	prepared, err := owner.PrepareChannelPayment(scenario.ctx, input)
	if err != nil || prepared.BalanceHoldNo == "" || prepared.ChannelAmountMinor+prepared.BalanceAmountMinor != checkout.TotalAmountMinor || prepared.CustomerRefDigest == "customer-commerce-service-1" {
		t.Fatalf("prepared=%+v err=%v", prepared, err)
	}
	orderState, err := scenario.database.GetValue(scenario.ctx, `SELECT status_code FROM plugin_idc_core_commerce_order WHERE order_no=?`, checkout.OrderNo)
	if err != nil || orderState.String() != "payment_processing" {
		t.Fatalf("order did not enter payment processing state=%s err=%v", orderState.String(), err)
	}
	replayed, err := owner.PrepareChannelPayment(scenario.ctx, input)
	if err != nil || replayed.BalanceHoldNo != prepared.BalanceHoldNo {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	changed := input
	changed.BalanceAmountMinor--
	if result, changedErr := owner.PrepareChannelPayment(scenario.ctx, changed); changedErr == nil || result != nil {
		t.Fatalf("changed preparation=%+v err=%v", result, changedErr)
	}
}

func channelBalanceReferenceForTest(attemptNo string) string {
	digest := sha256.Sum256([]byte("channel-balance\x00" + attemptNo))
	return "pbc_" + hex.EncodeToString(digest[:16])
}

func TestChannelPaymentOwnerPreservesFundingWhenSettlementRetries(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "settlement-channel-retry-checkout-0001",
	})
	if err != nil {
		t.Fatalf("checkout channel retry order: %v", err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	creditUsage, err := billingservice.NewCreditUsageService(scenario.database)
	if err != nil {
		t.Fatalf("create channel retry credit service: %v", err)
	}
	failingSettlements, err := billingservice.NewFinancialSettlementService(
		scenario.database, failingFinancialSettlementEventWriter{}, creditUsage,
	)
	if err != nil {
		t.Fatalf("create failing channel settlement: %v", err)
	}
	failingOwner, err := newChannelPaymentOwnerTestService(t, scenario.database, failingSettlements)
	if err != nil {
		t.Fatalf("create failing channel owner: %v", err)
	}
	input := billingcap.ConfirmedChannelPayment{
		PaymentAttemptNo: "PAT_channel_retry_0001", InvoiceNo: checkout.InvoiceNo,
		CurrencyCode: "CNY", AmountMinor: checkout.TotalAmountMinor, TotalAmountMinor: checkout.TotalAmountMinor,
		EvidenceDigest: strings.Repeat("8", 64),
	}
	if result, confirmErr := failingOwner.ConfirmChannelPayment(scenario.ctx, input); confirmErr == nil || result != nil {
		t.Fatalf("failing channel settlement result=%+v err=%v", result, confirmErr)
	}
	var waiting struct {
		InvoiceStatus string `orm:"invoice_status"`
		PaidMinor     int64  `orm:"paid_minor"`
		OrderStatus   string `orm:"order_status"`
		Transactions  int64  `orm:"transactions"`
		Allocations   int64  `orm:"allocations"`
		Settlements   int64  `orm:"settlements"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &waiting, `
SELECT invoice.status_code AS invoice_status,
       invoice.paid_amount_minor AS paid_minor,
       order_row.status_code AS order_status,
       (SELECT COUNT(*) FROM plugin_idc_core_billing_ledger_transaction
         WHERE source_type_code = 'payment_attempt' AND source_reference = ?) AS transactions,
       (SELECT COUNT(*) FROM plugin_idc_core_billing_payment_allocation
         WHERE payment_source_type_code = 'channel_payment'
           AND payment_source_reference = ?) AS allocations,
       (SELECT COUNT(*) FROM plugin_idc_core_financial_settlement
         WHERE order_no = order_row.order_no) AS settlements
FROM plugin_idc_core_billing_invoice invoice
JOIN plugin_idc_core_commerce_order order_row
  ON order_row.order_no = invoice.order_no
WHERE invoice.invoice_no = ?`, input.PaymentAttemptNo, input.PaymentAttemptNo, checkout.InvoiceNo); err != nil {
		t.Fatalf("load preserved channel payment: %v", err)
	}
	if waiting.InvoiceStatus != "paid" || waiting.PaidMinor != checkout.TotalAmountMinor ||
		waiting.OrderStatus != "pending_payment" || waiting.Transactions != 1 ||
		waiting.Allocations != 1 || waiting.Settlements != 0 {
		t.Fatalf("channel funding was not preserved independently: %+v", waiting)
	}
	workingOwner, err := newChannelPaymentOwnerTestService(t,
		scenario.database, newFinancialSettlementTestService(t, scenario),
	)
	if err != nil {
		t.Fatalf("create retrying channel owner: %v", err)
	}
	retried, err := workingOwner.ConfirmChannelPayment(scenario.ctx, input)
	if err != nil || retried == nil || !retried.FundingReused ||
		retried.SettlementNo == "" || retried.SettlementReused {
		t.Fatalf("retry channel payment result=%+v err=%v", retried, err)
	}
	assertFinancialSettlementState(t, scenario, checkout.OrderNo, retried.SettlementNo, 1)
}

func TestChannelPaymentOwnerSupportsPartialFundingAndRejectsChangedReplay(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "settlement-channel-partial-checkout-0001",
	})
	if err != nil || checkout.TotalAmountMinor < 2 {
		t.Fatalf("checkout channel partial order result=%+v err=%v", checkout, err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	owner, err := newChannelPaymentOwnerTestService(t,
		scenario.database, newFinancialSettlementTestService(t, scenario),
	)
	if err != nil {
		t.Fatalf("create partial channel owner: %v", err)
	}
	firstAmount := checkout.TotalAmountMinor / 2
	firstInput := billingcap.ConfirmedChannelPayment{
		PaymentAttemptNo: "PAT_channel_partial_0001", InvoiceNo: checkout.InvoiceNo,
		CurrencyCode: "CNY", AmountMinor: firstAmount, TotalAmountMinor: firstAmount,
		EvidenceDigest: strings.Repeat("9", 64),
	}
	first, err := owner.ConfirmChannelPayment(scenario.ctx, firstInput)
	if err != nil || first == nil || first.InvoiceStatus != "partially_paid" ||
		first.PaidAmountMinor != firstAmount || first.SettlementNo != "" || first.FundingReused {
		t.Fatalf("first partial channel payment result=%+v err=%v", first, err)
	}
	changed := firstInput
	changed.AmountMinor++
	if result, changedErr := owner.ConfirmChannelPayment(scenario.ctx, changed); changedErr == nil || result != nil {
		t.Fatalf("changed channel replay result=%+v err=%v", result, changedErr)
	}
	secondInput := billingcap.ConfirmedChannelPayment{
		PaymentAttemptNo: "PAT_channel_partial_0002", InvoiceNo: checkout.InvoiceNo,
		CurrencyCode: "CNY", AmountMinor: checkout.TotalAmountMinor - firstAmount, TotalAmountMinor: checkout.TotalAmountMinor - firstAmount,
		EvidenceDigest: strings.Repeat("a", 64),
	}
	second, err := owner.ConfirmChannelPayment(scenario.ctx, secondInput)
	if err != nil || second == nil || second.InvoiceStatus != "paid" ||
		second.PaidAmountMinor != checkout.TotalAmountMinor || second.SettlementNo == "" {
		t.Fatalf("second partial channel payment result=%+v err=%v", second, err)
	}
	var counts struct {
		Transactions int64 `orm:"transactions"`
		Allocations  int64 `orm:"allocations"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &counts, `
SELECT
  (SELECT COUNT(*) FROM plugin_idc_core_billing_ledger_transaction
    WHERE source_type_code = 'payment_attempt') AS transactions,
  (SELECT COUNT(*) FROM plugin_idc_core_billing_payment_allocation
    WHERE invoice_no = ? AND payment_source_type_code = 'channel_payment') AS allocations`,
		checkout.InvoiceNo); err != nil {
		t.Fatalf("load partial channel counts: %v", err)
	}
	if counts.Transactions != 2 || counts.Allocations != 2 {
		t.Fatalf("changed partial replay created money: %+v", counts)
	}
	assertFinancialSettlementState(t, scenario, checkout.OrderNo, second.SettlementNo, 1, 2)
}

func TestFinancialSettlementPostgreSQLSerializesCreditSettlement(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "settlement-credit-checkout-0001",
	})
	if err != nil {
		t.Fatalf("checkout credit settlement order: %v", err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	activateFinancialSettlementCredit(
		t, scenario, "customer-commerce-service-1", checkout.TotalAmountMinor+20_000,
		"serialize",
	)
	settlements := newFinancialSettlementTestService(t, scenario)
	input := billingservice.CreditOrderSettlementInput{
		OrderNo: checkout.OrderNo, ActorType: "customer", ActorID: 101,
	}
	const workerCount = 8
	results := make([]*billingservice.FinancialSettlement, workerCount)
	errorsFound := make([]error, workerCount)
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for index := range workerCount {
		go func() {
			defer workers.Done()
			<-start
			results[index], errorsFound[index] = settlements.SettleOrderWithCredit(
				scenario.ctx, input,
			)
		}()
	}
	close(start)
	workers.Wait()
	createdCount := 0
	settlementNo := ""
	for index := range workerCount {
		result := results[index]
		if errorsFound[index] != nil || result == nil ||
			result.Basis != billingservice.SettlementBasisCredit ||
			result.CreditAmountMinor != checkout.TotalAmountMinor ||
			result.PaidAmountMinor != 0 || len(result.Sources) != 1 ||
			result.Sources[0].SourceType != billingservice.FinancialSettlementSourceCredit {
			t.Fatalf("credit settlement worker %d result=%+v err=%v",
				index, result, errorsFound[index])
		}
		if settlementNo == "" {
			settlementNo = result.SettlementNo
		} else if result.SettlementNo != settlementNo {
			t.Fatalf("credit settlement numbers diverged: %q != %q",
				result.SettlementNo, settlementNo)
		}
		if !result.Reused {
			createdCount++
		}
	}
	if createdCount != 1 {
		t.Fatalf("credit settlement created count=%d want=1", createdCount)
	}
	var state struct {
		OrderStatus      string `orm:"order_status"`
		InvoiceStatus    string `orm:"invoice_status"`
		PaidMinor        int64  `orm:"paid_minor"`
		CreditMinor      int64  `orm:"credit_minor"`
		AvailableMinor   int64  `orm:"available_minor"`
		UnbilledMinor    int64  `orm:"unbilled_minor"`
		ExposureCount    int64  `orm:"exposure_count"`
		ReserveCount     int64  `orm:"reserve_count"`
		FulfillmentCount int64  `orm:"fulfillment_count"`
		PendingUnits     int64  `orm:"pending_units"`
		ReservationCount int64  `orm:"reservation_count"`
		HeldReservations int64  `orm:"held_reservations"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &state, `
SELECT order_row.status_code AS order_status,
       invoice.status_code AS invoice_status,
       invoice.paid_amount_minor AS paid_minor,
       invoice.credit_amount_minor AS credit_minor,
       account.available_limit_minor AS available_minor,
       account.unbilled_principal_minor AS unbilled_minor,
       (SELECT COUNT(*) FROM plugin_idc_core_credit_exposure exposure
         WHERE exposure.order_no = order_row.order_no) AS exposure_count,
       (SELECT COUNT(*) FROM plugin_idc_core_credit_account_movement movement
         WHERE movement.credit_account_no = account.credit_account_no
           AND movement.movement_type_code = 'reserve'
           AND movement.source_reference = order_row.order_no) AS reserve_count,
       (SELECT COUNT(*) FROM plugin_idc_core_commerce_fulfillment_unit unit
         WHERE unit.order_no = order_row.order_no) AS fulfillment_count,
       (SELECT COUNT(*) FROM plugin_idc_core_commerce_fulfillment_unit unit
         WHERE unit.order_no = order_row.order_no
           AND unit.status_code = 'pending_payment') AS pending_units,
       (SELECT COUNT(*) FROM plugin_idc_core_commerce_inventory_reservation reservation
         WHERE reservation.order_no = order_row.order_no) AS reservation_count,
       (SELECT COUNT(*) FROM plugin_idc_core_commerce_inventory_reservation reservation
         WHERE reservation.order_no = order_row.order_no
           AND reservation.status_code = 'held') AS held_reservations
FROM plugin_idc_core_commerce_order order_row
JOIN plugin_idc_core_billing_invoice invoice
  ON invoice.order_no = order_row.order_no
JOIN plugin_idc_core_credit_account account
  ON account.customer_no = order_row.customer_no
 AND account.currency_code = order_row.currency_code
WHERE order_row.order_no = ?`, checkout.OrderNo); err != nil {
		t.Fatalf("load credit settlement state: %v", err)
	}
	if state.OrderStatus != "paid" || state.InvoiceStatus != "credit_settled" ||
		state.PaidMinor != 0 || state.CreditMinor != checkout.TotalAmountMinor ||
		state.AvailableMinor != 20_000 || state.UnbilledMinor != checkout.TotalAmountMinor ||
		state.ExposureCount != 1 || state.ReserveCount != 1 ||
		state.FulfillmentCount != 0 || state.PendingUnits != 0 ||
		state.ReservationCount != int64(checkout.ItemCount) ||
		state.HeldReservations != state.ReservationCount {
		t.Fatalf("unexpected credit settlement state: %+v checkout=%+v", state, checkout)
	}
	assertFinancialSettlementState(t, scenario, checkout.OrderNo, settlementNo, 1)
}

func TestAccountCreditSettlementDerivesOwnershipAndReplays(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "settlement-credit-account-checkout-0001",
	})
	if err != nil {
		t.Fatalf("checkout account credit order: %v", err)
	}
	activateFinancialSettlementCredit(
		t, scenario, "customer-commerce-service-1", checkout.TotalAmountMinor+10_000,
		"account-command",
	)
	commands, err := billingservice.NewAccountFinanceCommandService(
		scenario.database,
		commercePrincipalStub{audience: &principal.Audience{
			UserID: 101, Code: principal.AudienceCustomer,
			Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-1",
		}},
		newFinancialSettlementTestService(t, scenario),
	)
	if err != nil {
		t.Fatalf("create account credit commands: %v", err)
	}
	created, err := commands.PayAccountInvoiceFromCredit(scenario.ctx, checkout.InvoiceNo)
	if err != nil || created == nil || created.Reused ||
		created.InvoiceNo != checkout.InvoiceNo || created.OrderNo != checkout.OrderNo ||
		created.CurrencyCode != "CNY" ||
		created.CreditAmountMinor != checkout.TotalAmountMinor ||
		created.StatusCode != "credit_settled" {
		t.Fatalf("create account credit settlement result=%+v err=%v", created, err)
	}
	replayed, err := commands.PayAccountInvoiceFromCredit(scenario.ctx, checkout.InvoiceNo)
	if err != nil || replayed == nil || !replayed.Reused ||
		replayed.SettlementNo != created.SettlementNo ||
		replayed.CreditAmountMinor != created.CreditAmountMinor {
		t.Fatalf("replay account credit settlement result=%+v err=%v", replayed, err)
	}
	foreign, err := billingservice.NewAccountFinanceCommandService(
		scenario.database,
		commercePrincipalStub{audience: &principal.Audience{
			UserID: 202, Code: principal.AudienceCustomer,
			Status: principal.AudienceStatusActive, CustomerNo: "customer-foreign",
		}},
		newFinancialSettlementTestService(t, scenario),
	)
	if err != nil {
		t.Fatalf("create foreign account credit commands: %v", err)
	}
	if result, foreignErr := foreign.PayAccountInvoiceFromCredit(
		scenario.ctx, checkout.InvoiceNo,
	); result != nil || !bizerr.Is(foreignErr, billingservice.CodeDirectoryNotAccessible) {
		t.Fatalf("foreign account credit settlement result=%+v err=%v", result, foreignErr)
	}
	assertFinancialSettlementState(t, scenario, checkout.OrderNo, created.SettlementNo, 1)
}

func TestFinancialSettlementPostgreSQLRollsBackCreditReserveWithOutbox(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "settlement-credit-rollback-0001",
	})
	if err != nil {
		t.Fatalf("checkout credit rollback order: %v", err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	limitMinor := checkout.TotalAmountMinor + 10_000
	activateFinancialSettlementCredit(
		t, scenario, "customer-commerce-service-1", limitMinor, "rollback",
	)
	failing, err := billingservice.NewFinancialSettlementService(
		scenario.database, failingFinancialSettlementEventWriter{},
		newFinancialSettlementCreditService(t, scenario),
	)
	if err != nil {
		t.Fatalf("create failing credit settlement service: %v", err)
	}
	if _, err = failing.SettleOrderWithCredit(
		scenario.ctx,
		billingservice.CreditOrderSettlementInput{
			OrderNo: checkout.OrderNo, ActorType: "customer", ActorID: 101,
		},
	); !bizerr.Is(err, billingservice.CodeSettlementUnavailable) {
		t.Fatalf("credit settlement outbox failure error=%v", err)
	}
	var state struct {
		AvailableMinor  int64  `orm:"available_minor"`
		UnbilledMinor   int64  `orm:"unbilled_minor"`
		AccountVersion  int64  `orm:"account_version"`
		InvoiceStatus   string `orm:"invoice_status"`
		CreditMinor     int64  `orm:"credit_minor"`
		OrderStatus     string `orm:"order_status"`
		ExposureCount   int64  `orm:"exposure_count"`
		ReserveCount    int64  `orm:"reserve_count"`
		SettlementCount int64  `orm:"settlement_count"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &state, `
SELECT account.available_limit_minor AS available_minor,
       account.unbilled_principal_minor AS unbilled_minor,
       account.version AS account_version,
       invoice.status_code AS invoice_status,
       invoice.credit_amount_minor AS credit_minor,
       order_row.status_code AS order_status,
       (SELECT COUNT(*) FROM plugin_idc_core_credit_exposure exposure
         WHERE exposure.order_no = order_row.order_no) AS exposure_count,
       (SELECT COUNT(*) FROM plugin_idc_core_credit_account_movement movement
         WHERE movement.movement_type_code = 'reserve'
           AND movement.source_reference = order_row.order_no) AS reserve_count,
       (SELECT COUNT(*) FROM plugin_idc_core_financial_settlement settlement
         WHERE settlement.order_no = order_row.order_no) AS settlement_count
FROM plugin_idc_core_commerce_order order_row
JOIN plugin_idc_core_billing_invoice invoice
  ON invoice.order_no = order_row.order_no
JOIN plugin_idc_core_credit_account account
  ON account.customer_no = order_row.customer_no
 AND account.currency_code = order_row.currency_code
WHERE order_row.order_no = ?`, checkout.OrderNo); err != nil {
		t.Fatalf("load rolled-back credit state: %v", err)
	}
	if state.AvailableMinor != limitMinor || state.UnbilledMinor != 0 ||
		state.AccountVersion != 1 || state.InvoiceStatus != "pending_payment" ||
		state.CreditMinor != 0 || state.OrderStatus != "pending_payment" ||
		state.ExposureCount != 0 || state.ReserveCount != 0 ||
		state.SettlementCount != 0 {
		t.Fatalf("credit reserve was not rolled back with settlement: %+v", state)
	}
}

func TestFinancialSettlementPostgreSQLHandlesZeroAndFullDiscount(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		withMarketing bool
		wantBasis     billingservice.FinancialSettlementBasis
	}{
		{name: "legal zero amount", wantBasis: billingservice.SettlementBasisZeroAmount},
		{name: "full discount", withMarketing: true, wantBasis: billingservice.SettlementBasisFullDiscount},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
			if testCase.withMarketing {
				if _, err := scenario.database.Exec(scenario.ctx, `
UPDATE plugin_idc_core_marketing_benefit_rule
SET amount_minor = 12000
WHERE benefit_no = 'benefit-commerce-service'`); err != nil {
					t.Fatalf("configure full discount: %v", err)
				}
			} else if _, err := scenario.database.Exec(scenario.ctx, `
UPDATE plugin_idc_core_catalog_price
SET amount_minor = 0, setup_fee_minor = 0, renewal_amount_minor = 0
WHERE version_no = 'version-commerce-service-1'`); err != nil {
				t.Fatalf("configure legal zero price: %v", err)
			}
			service, quote := newCheckoutCustomer(
				t, scenario.ctx, scenario.database, scenario.resolver, scenario.purchase,
				"customer-commerce-service-2", 202, 1, testCase.withMarketing,
			)
			if quote.TotalAmountMinor != 0 {
				t.Fatalf("synthetic settlement quote total=%d want=0", quote.TotalAmountMinor)
			}
			insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-2")
			checkout, err := service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
				QuoteNo:        quote.QuoteNo,
				IdempotencyKey: "settlement-synthetic-checkout-" + strings.ReplaceAll(testCase.name, " ", "-"),
			})
			if err != nil {
				t.Fatalf("checkout synthetic settlement order: %v", err)
			}
			settlement, err := newFinancialSettlementTestService(t, scenario).SettleOrder(
				scenario.ctx, checkout.OrderNo,
			)
			if err != nil || settlement.Basis != testCase.wantBasis ||
				settlement.TotalAmountMinor != 0 || len(settlement.Sources) != 1 {
				t.Fatalf("synthetic settlement result=%+v err=%v", settlement, err)
			}
			assertFinancialSettlementState(t, scenario, checkout.OrderNo, settlement.SettlementNo, 1)
		})
	}
}

func TestFinancialSettlementPostgreSQLRejectsInvoiceWaiver(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "settlement-waiver-checkout-0001",
	})
	if err != nil {
		t.Fatalf("checkout waiver settlement order: %v", err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	adjustments, err := billingservice.NewFinancialAdjustmentService(scenario.database)
	if err != nil {
		t.Fatalf("create waiver service: %v", err)
	}
	request, err := adjustments.CreateInvoiceWaiver(scenario.ctx, billingservice.InvoiceWaiverInput{
		InvoiceNo: checkout.InvoiceNo, AmountMinor: checkout.TotalAmountMinor,
		ReasonCode: "commercial_waiver", EvidenceDigest: strings.Repeat("f", 64),
		IdempotencyKey: "settlement-waiver-0001", RequestedBy: 1001,
	})
	if err != nil || request.Status != billingservice.AdjustmentStatusPendingReview {
		t.Fatalf("create invoice waiver result=%+v err=%v", request, err)
	}
	if _, err = adjustments.ReviewAdjustment(scenario.ctx, billingservice.AdjustmentReviewInput{
		AdjustmentNo: request.AdjustmentNo,
		Decision:     billingservice.AdjustmentDecisionApprove, ReviewerID: 1002,
		CommentDigest: strings.Repeat("1", 64), EvidenceDigest: strings.Repeat("2", 64),
	}); err != nil {
		t.Fatalf("approve invoice waiver: %v", err)
	}
	if _, err = newFinancialSettlementTestService(t, scenario).SettleOrder(
		scenario.ctx, checkout.OrderNo,
	); !bizerr.Is(err, billingservice.CodeSettlementNotEligible) {
		t.Fatalf("invoice waiver settlement error=%v", err)
	}
	assertFinancialSettlementState(t, scenario, checkout.OrderNo, "", 0)
}

func TestFinancialSettlementPostgreSQLRollsBackWhenOutboxFails(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	if _, err := scenario.database.Exec(scenario.ctx, `
UPDATE plugin_idc_core_catalog_price
SET amount_minor = 0, setup_fee_minor = 0, renewal_amount_minor = 0
WHERE version_no = 'version-commerce-service-1'`); err != nil {
		t.Fatalf("configure rollback zero price: %v", err)
	}
	service, quote := newCheckoutCustomer(
		t, scenario.ctx, scenario.database, scenario.resolver, scenario.purchase,
		"customer-commerce-service-2", 202, 1, false,
	)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-2")
	settlements, err := billingservice.NewFinancialSettlementService(
		scenario.database, failingFinancialSettlementEventWriter{},
		newFinancialSettlementCreditService(t, scenario),
	)
	if err != nil {
		t.Fatalf("create failing settlement service: %v", err)
	}
	service.settlements = settlements
	if _, err = service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: quote.QuoteNo, IdempotencyKey: "settlement-outbox-rollback-0001",
	}); !bizerr.Is(
		err, billingservice.CodeSettlementUnavailable,
	) {
		t.Fatalf("automatic settlement outbox failure error=%v", err)
	}
	var state struct {
		OrderNo         string `orm:"order_no"`
		OrderStatus     string `orm:"order_status"`
		InvoiceStatus   string `orm:"invoice_status"`
		SettlementCount int64  `orm:"settlement_count"`
		SourceCount     int64  `orm:"source_count"`
		EventCount      int64  `orm:"event_count"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &state, `
SELECT order_row.order_no, order_row.status_code AS order_status,
       invoice.status_code AS invoice_status,
       (SELECT COUNT(*) FROM plugin_idc_core_financial_settlement
         WHERE order_no = order_row.order_no) AS settlement_count,
       (SELECT COUNT(*) FROM plugin_idc_core_financial_settlement_source
         WHERE order_no = order_row.order_no) AS source_count,
       (SELECT COUNT(*) FROM plugin_idc_core_outbox_event
         WHERE event_type = 'commerce.order.financially_settled'
           AND aggregate_no = order_row.order_no) AS event_count
FROM plugin_idc_core_commerce_order order_row
JOIN plugin_idc_core_billing_invoice invoice
  ON invoice.order_no = order_row.order_no
WHERE order_row.quote_no = ?`, quote.QuoteNo); err != nil {
		t.Fatalf("load rolled-back settlement state: %v", err)
	}
	if state.OrderStatus != "pending_payment" || state.InvoiceStatus != "pending_payment" ||
		state.OrderNo == "" || state.SettlementCount != 0 ||
		state.SourceCount != 0 || state.EventCount != 0 {
		t.Fatalf("financial settlement outbox failure did not roll back: %+v", state)
	}
}

func TestFinancialSettlementPostgreSQLRejectsHeaderWithoutSources(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "settlement-db-conservation-0001",
	})
	if err != nil {
		t.Fatalf("checkout settlement conservation order: %v", err)
	}
	err = scenario.database.Transaction(scenario.ctx, func(ctx context.Context, _ gdb.TX) error {
		_, insertErr := scenario.database.Exec(ctx, `
INSERT INTO plugin_idc_core_financial_settlement
    (settlement_no, order_no, invoice_no, customer_no, currency_code,
     basis_code, total_amount_minor, paid_amount_minor, credit_amount_minor,
     source_count, identity_evidence_digest, source_snapshot_digest,
     content_digest, settled_at)
VALUES
    ('fst_missing_sources', ?, ?, 'customer-commerce-service-1', 'CNY',
     'funded_invoice', ?, ?, 0, 1, repeat('1', 64), repeat('2', 64),
     repeat('3', 64), CURRENT_TIMESTAMP)`,
			checkout.OrderNo, checkout.InvoiceNo,
			checkout.TotalAmountMinor, checkout.TotalAmountMinor,
		)
		return insertErr
	})
	if err == nil {
		t.Fatal("database committed a financial settlement header without sources")
	}
}

func TestFinancialSettlementPostgreSQLMigrationIsIdempotent(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve financial settlement migration path")
	}
	path := filepath.Join(
		filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql",
		"035-financial-settlement.sql",
	)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read financial settlement migration: %v", err)
	}
	if _, err = scenario.database.Exec(scenario.ctx, string(content)); err != nil {
		t.Fatalf("reapply financial settlement migration: %v", err)
	}
}

func newFinancialSettlementTestService(
	t *testing.T,
	scenario checkoutTestScenario,
) billingservice.FinancialSettlementService {
	t.Helper()
	events, err := outbox.NewEnqueuer(scenario.database, outbox.Config{})
	if err != nil {
		t.Fatalf("create settlement outbox: %v", err)
	}
	service, err := billingservice.NewFinancialSettlementService(
		scenario.database, events, newFinancialSettlementCreditService(t, scenario),
	)
	if err != nil {
		t.Fatalf("create financial settlement service: %v", err)
	}
	return service
}

func newFinancialSettlementCreditService(
	t *testing.T,
	scenario checkoutTestScenario,
) billingservice.CreditUsageService {
	t.Helper()
	credits, err := billingservice.NewCreditUsageService(scenario.database)
	if err != nil {
		t.Fatalf("create settlement credit service: %v", err)
	}
	return credits
}

func seedFinancialSettlementMerchant(t *testing.T, scenario checkoutTestScenario) {
	t.Helper()
	if _, err := scenario.database.Exec(scenario.ctx, `
INSERT INTO plugin_idc_core_merchant_profile
    (merchant_no, display_name, legal_name, default_currency, enabled)
VALUES ('merchant-financial-settlement', 'Settlement Merchant',
        'Settlement Merchant Ltd.', 'CNY', TRUE)`); err != nil {
		t.Fatalf("seed financial settlement merchant: %v", err)
	}
}

func activateFinancialSettlementCredit(
	t *testing.T,
	scenario checkoutTestScenario,
	customerNo string,
	limitMinor int64,
	suffix string,
) {
	t.Helper()
	approval, err := billingservice.NewCreditApprovalService(scenario.database)
	if err != nil {
		t.Fatalf("create settlement credit approval service: %v", err)
	}
	if _, err = approval.ConfigureCreditPolicy(
		scenario.ctx,
		billingservice.CreditPolicyInput{
			CurrencyCode: "CNY", MaximumLimitMinor: limitMinor + 10_000,
			DualReviewThresholdMinor: limitMinor + 1, BillingDay: 15,
			RepaymentPeriodDays: 30, GracePeriodDays: 3,
			ConfiguredBy:   1001,
			IdempotencyKey: "settlement-credit-policy-" + suffix,
		},
	); err != nil {
		t.Fatalf("configure settlement credit policy: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	application, err := approval.SubmitCreditApplication(
		scenario.ctx,
		billingservice.CreditApplicationInput{
			CustomerNo: customerNo, CurrencyCode: "CNY",
			RequestedLimitMinor:   limitMinor,
			RequestedValidFrom:    now.Add(-time.Minute),
			RequestedValidUntil:   now.AddDate(1, 0, 0),
			RiskEvidenceReference: "private:settlement-credit/" + suffix,
			RiskEvidenceDigest:    strings.Repeat("7", 64),
			RequestActorType:      "customer", RequestedBy: 101,
			IdempotencyKey: "settlement-credit-application-" + suffix,
		},
	)
	if err != nil {
		t.Fatalf("submit settlement credit application: %v", err)
	}
	approved, err := approval.ReviewCreditApplication(
		scenario.ctx,
		billingservice.CreditReviewInput{
			ApplicationNo: application.ApplicationNo,
			Decision:      billingservice.CreditDecisionApprove, ReviewerID: 1002,
			CommentDigest:  strings.Repeat("8", 64),
			EvidenceDigest: strings.Repeat("9", 64),
		},
	)
	if err != nil || approved == nil || approved.StatusCode != "approved" {
		t.Fatalf("approve settlement credit application result=%+v err=%v", approved, err)
	}
}

func assertFinancialSettlementState(
	t *testing.T,
	scenario checkoutTestScenario,
	orderNo string,
	settlementNo string,
	wantSettlementCount int64,
	wantSourceCounts ...int64,
) {
	t.Helper()
	var state struct {
		OrderStatus       string `orm:"order_status"`
		InvoiceStatus     string `orm:"invoice_status"`
		SettlementCount   int64  `orm:"settlement_count"`
		SourceCount       int64  `orm:"source_count"`
		EventCount        int64  `orm:"event_count"`
		ReadyFulfillments int64  `orm:"ready_fulfillments"`
		HeldReservations  int64  `orm:"held_reservations"`
	}
	if err := scenario.database.GetScan(scenario.ctx, &state, `
SELECT order_row.status_code AS order_status,
       invoice.status_code AS invoice_status,
       (SELECT COUNT(*) FROM plugin_idc_core_financial_settlement settlement
         WHERE settlement.order_no = order_row.order_no) AS settlement_count,
       (SELECT COUNT(*) FROM plugin_idc_core_financial_settlement_source source
         WHERE source.settlement_no = ?) AS source_count,
       (SELECT COUNT(*) FROM plugin_idc_core_outbox_event event
         WHERE event.event_type = 'commerce.order.financially_settled'
           AND event.aggregate_no = order_row.order_no) AS event_count,
       (SELECT COUNT(*) FROM plugin_idc_core_commerce_fulfillment_unit unit
         WHERE unit.order_no = order_row.order_no
           AND unit.status_code <> 'pending_payment') AS ready_fulfillments,
       (SELECT COUNT(*) FROM plugin_idc_core_commerce_inventory_reservation reservation
         WHERE reservation.order_no = order_row.order_no
           AND reservation.status_code = 'held') AS held_reservations
FROM plugin_idc_core_commerce_order order_row
JOIN plugin_idc_core_billing_invoice invoice
  ON invoice.order_no = order_row.order_no
WHERE order_row.order_no = ?`, settlementNo, orderNo); err != nil {
		t.Fatalf("load financial settlement state: %v", err)
	}
	if state.SettlementCount != wantSettlementCount || state.EventCount != wantSettlementCount ||
		state.ReadyFulfillments != 0 || state.HeldReservations != 1 {
		t.Fatalf("unexpected financial settlement state: %+v", state)
	}
	if wantSettlementCount == 1 {
		wantSourceCount := int64(1)
		if len(wantSourceCounts) == 1 {
			wantSourceCount = wantSourceCounts[0]
		} else if len(wantSourceCounts) > 1 {
			t.Fatalf("financial settlement source expectation must have at most one value")
		}
		validInvoiceStatus := state.InvoiceStatus == "paid" ||
			state.InvoiceStatus == "credit_settled"
		if state.OrderStatus != "paid" || !validInvoiceStatus || state.SourceCount != wantSourceCount {
			t.Fatalf("settled financial state=%+v", state)
		}
	} else if state.OrderStatus != "pending_payment" || state.InvoiceStatus != "paid" {
		t.Fatalf("rejected financial state=%+v", state)
	}
}
