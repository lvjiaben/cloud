package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/billingcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
	"lina-plugin-idc-payment/backend/internal/provider/epay"
	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
	"lina-plugin-idc-payment/backend/internal/service/exceptionfund"
	"lina-plugin-idc-payment/backend/internal/service/paymentattempt"
	"lina-plugin-idc-payment/backend/internal/service/paymentflow"
	"lina-plugin-idc-payment/backend/internal/service/reconciliation"
)

const paymentTestDSN = "LINAPRO_IDC_PAYMENT_TEST_DSN"

type paymentReconciliationLoaderFake struct{}

func (paymentReconciliationLoaderFake) Load(context.Context, string, time.Time, int) ([]reconciliation.SourceRecord, string, error) {
	return nil, strings.Repeat("f", 64), nil
}

type paymentBillingFake struct {
	calls          int
	creditCalls    int
	rechargeCalls  int
	releaseCalls   int
	last           billingcap.ConfirmedChannelPayment
	refundWork     *billingcap.RefundExecutionWork
	lastRefundWork *billingcap.RefundExecutionWork
	refundReports  []billingcap.ReportRefundExecution
}

func (f *paymentBillingFake) ClaimRefundExecution(
	context.Context,
	billingcap.ClaimRefundExecution,
) (*billingcap.RefundExecutionWork, error) {
	if f.refundWork == nil {
		return &billingcap.RefundExecutionWork{}, nil
	}
	work := *f.refundWork
	f.refundWork = nil
	f.lastRefundWork = &work
	return &work, nil
}

func (f *paymentBillingFake) ReportRefundExecution(
	_ context.Context,
	input billingcap.ReportRefundExecution,
) (*billingcap.RefundExecutionReport, error) {
	f.refundReports = append(f.refundReports, input)
	refundStatus := "executing"
	if input.ResultCode == "original_channel_unsupported" {
		refundStatus = "awaiting_balance_consent"
	}
	if input.Status == billingcap.RefundExecutionResultReady && f.lastRefundWork != nil {
		next := *f.lastRefundWork
		next.Phase = billingcap.RefundExecutionPhaseExecute
		next.Attempt++
		next.LeaseExpiresAt = time.Now().Add(time.Minute)
		f.refundWork = &next
	}
	return &billingcap.RefundExecutionReport{
		CommandNo: input.CommandNo, RefundNo: "rfd_payment_fake_0001",
		CommandStatusCode: string(input.Status), RefundStatusCode: refundStatus,
	}, nil
}

func (f *paymentBillingFake) PrepareChannelPayment(_ context.Context, input billingcap.PrepareChannelPayment) (*billingcap.PreparedChannelPayment, error) {
	return &billingcap.PreparedChannelPayment{PaymentAttemptNo: input.PaymentAttemptNo, PaymentContextNo: input.PaymentContextNo, InvoiceNo: "invoice-1", OrderNo: "order-1", CustomerRefDigest: billingcap.PrincipalReferenceDigest(101), CurrencyCode: "CNY", TotalAmountMinor: 12345, ChannelAmountMinor: 12345, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (f *paymentBillingFake) CurrentPaymentActor(context.Context) (*billingcap.PaymentActor, error) {
	return &billingcap.PaymentActor{PrincipalRefDigest: billingcap.PrincipalReferenceDigest(101)}, nil
}

func (f *paymentBillingFake) PrepareCreditRepayment(_ context.Context, input billingcap.PrepareCreditRepayment) (*billingcap.PreparedCreditRepayment, error) {
	return &billingcap.PreparedCreditRepayment{PaymentAttemptNo: input.PaymentAttemptNo, StatementNo: input.StatementNo, CustomerRefDigest: billingcap.PrincipalReferenceDigest(101), CurrencyCode: "CNY", AmountMinor: input.AmountMinor, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (f *paymentBillingFake) ConfirmCreditRepayment(_ context.Context, input billingcap.ConfirmedCreditRepayment) (*billingcap.CreditRepaymentResult, error) {
	f.creditCalls++
	return &billingcap.CreditRepaymentResult{PaymentAttemptNo: input.PaymentAttemptNo, StatementNo: input.StatementNo, CurrencyCode: input.CurrencyCode, AmountMinor: input.AmountMinor, TransactionNo: "btx_3123456789abcdef0123456789abcdef", RepaymentNo: "crr_payment_0001", StatementStatusCode: "paid"}, nil
}

func (f *paymentBillingFake) PrepareBalanceRecharge(_ context.Context, input billingcap.PrepareBalanceRecharge) (*billingcap.PreparedBalanceRecharge, error) {
	return &billingcap.PreparedBalanceRecharge{PaymentAttemptNo: input.PaymentAttemptNo, RechargeContextNo: "BRC_payment_0001", CustomerRefDigest: billingcap.PrincipalReferenceDigest(101), CurrencyCode: input.CurrencyCode, AmountMinor: input.AmountMinor, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (f *paymentBillingFake) ConfirmBalanceRecharge(_ context.Context, input billingcap.ConfirmedBalanceRecharge) (*billingcap.BalanceRechargeResult, error) {
	f.rechargeCalls++
	return &billingcap.BalanceRechargeResult{PaymentAttemptNo: input.PaymentAttemptNo, RechargeContextNo: input.RechargeContextNo, CurrencyCode: input.CurrencyCode, AmountMinor: input.AmountMinor, TransactionNo: "btx_5123456789abcdef0123456789abcdef", BalanceMinor: input.AmountMinor, AvailableMinor: input.AmountMinor}, nil
}

type paymentSecretFake struct {
	plaintext []byte
	handle    secretcap.Handle
	metadata  secretcap.Metadata
}

type paymentRefundAdapterFake struct {
	refundNotifyURL string
	refundCalls     int
	queryCalls      int
	refundStatus    string
	queryStatus     string
	callbackValue   *paymentcap.VerifiedResult
}

type paymentRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn paymentRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func (a *paymentRefundAdapterFake) Provider() paymentcap.ProviderCode {
	return paymentcap.ProviderAlipay
}
func (a *paymentRefundAdapterFake) Capabilities() paymentcap.Capabilities {
	return paymentcap.Capabilities{
		Modes:      map[paymentcap.Mode]bool{paymentcap.ModeAlipayPage: true},
		Currencies: map[string]bool{"CNY": true},
		Operations: map[paymentcap.Operation]bool{
			paymentcap.OperationCreate: true, paymentcap.OperationRefund: true,
			paymentcap.OperationRefundCallback: true, paymentcap.OperationRefundQuery: true,
		},
	}
}
func (a *paymentRefundAdapterFake) Create(context.Context, paymentcap.Credentials, paymentcap.CreateRequest) (*paymentcap.Redirect, error) {
	return &paymentcap.Redirect{Method: "POST", URL: "https://example.test/pay"}, nil
}
func (a *paymentRefundAdapterFake) VerifyCallback(context.Context, paymentcap.Credentials, paymentcap.CallbackRequest) (*paymentcap.VerifiedResult, error) {
	return nil, paymentcap.ErrUnsupported
}
func (a *paymentRefundAdapterFake) Query(context.Context, paymentcap.Credentials, paymentcap.QueryRequest) (*paymentcap.VerifiedResult, error) {
	return nil, paymentcap.ErrUnsupported
}
func (a *paymentRefundAdapterFake) Close(context.Context, paymentcap.Credentials, paymentcap.CloseRequest) error {
	return paymentcap.ErrUnsupported
}
func (a *paymentRefundAdapterFake) Refund(_ context.Context, _ paymentcap.Credentials, input paymentcap.RefundRequest) (*paymentcap.VerifiedResult, error) {
	a.refundCalls++
	a.refundNotifyURL = input.NotifyURL
	status := a.refundStatus
	if status == "" {
		status = "succeeded"
	}
	return &paymentcap.VerifiedResult{
		ExternalTradeReference: "provider-" + input.RefundAttemptNo, AttemptNo: input.RefundAttemptNo,
		Currency: input.Currency, AmountMinor: input.AmountMinor,
		Status: status, EvidenceDigest: strings.Repeat("e", 64),
	}, nil
}
func (a *paymentRefundAdapterFake) VerifyRefundCallback(context.Context, paymentcap.Credentials, paymentcap.CallbackRequest) (*paymentcap.VerifiedResult, error) {
	if a.callbackValue == nil {
		return nil, paymentcap.ErrUnsupported
	}
	result := *a.callbackValue
	return &result, nil
}
func (a *paymentRefundAdapterFake) QueryRefund(_ context.Context, _ paymentcap.Credentials, input paymentcap.RefundQueryRequest) (*paymentcap.VerifiedResult, error) {
	a.queryCalls++
	if a.queryStatus == "" {
		return nil, paymentcap.ErrUnsupported
	}
	return &paymentcap.VerifiedResult{
		ExternalTradeReference: "provider-" + input.RefundAttemptNo, AttemptNo: input.RefundAttemptNo,
		Currency: "CNY", AmountMinor: 1200, Status: a.queryStatus,
		EvidenceDigest: strings.Repeat("d", 64),
	}, nil
}
func (a *paymentRefundAdapterFake) Statement(context.Context, paymentcap.Credentials, paymentcap.StatementRequest) (*paymentcap.StatementFile, error) {
	return nil, paymentcap.ErrUnsupported
}
func (a *paymentRefundAdapterFake) VerifyConnection(context.Context, paymentcap.Credentials) error {
	return nil
}

func (f *paymentSecretFake) Create(_ context.Context, _ string, produce secretcap.PlaintextProducer) (secretcap.CreateResult, error) {
	var value []byte
	err := produce(func(plaintext []byte) error { value = append([]byte(nil), plaintext...); return nil })
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	if f.handle.IsZero() {
		f.handle, _ = secretcap.NewHandle("payment-test-secret-ref")
		f.metadata = secretcap.Metadata{Configured: true, MaskedTail: "test", UpdatedAt: time.Now().UTC()}
	}
	f.plaintext = value
	return secretcap.CreateResult{Handle: f.handle, Metadata: f.metadata}, nil
}
func (f *paymentSecretFake) RecoverCreate(context.Context, string) (secretcap.CreateResult, error) {
	return secretcap.CreateResult{Handle: f.handle, Metadata: f.metadata, Reused: true}, nil
}
func (f *paymentSecretFake) Replace(context.Context, secretcap.Handle, time.Time, string, secretcap.PlaintextProducer) (secretcap.MutationResult, error) {
	return secretcap.MutationResult{}, errors.New("not used")
}
func (f *paymentSecretFake) Metadata(context.Context, secretcap.Handle) (secretcap.Metadata, error) {
	return f.metadata, nil
}
func (f *paymentSecretFake) Revoke(context.Context, secretcap.Handle, time.Time) (secretcap.Metadata, error) {
	return f.metadata, nil
}
func (f *paymentSecretFake) Rotate(context.Context, int) (secretcap.RotationResult, error) {
	return secretcap.RotationResult{}, nil
}
func (f *paymentSecretFake) Access(_ context.Context, request secretcap.AccessRequest, consume secretcap.Consumer) error {
	if request.Purpose != secretcap.Purpose("payment.channel-use") {
		return errors.New("ungranted payment credential purpose")
	}
	if !request.Handle.Equal(f.handle) {
		return errors.New("foreign handle")
	}
	return consume(append([]byte(nil), f.plaintext...))
}

func (f *paymentBillingFake) ConfirmChannelPayment(_ context.Context, input billingcap.ConfirmedChannelPayment) (*billingcap.ChannelPaymentResult, error) {
	f.calls++
	f.last = input
	result := &billingcap.ChannelPaymentResult{
		PaymentAttemptNo: input.PaymentAttemptNo,
		TransactionNo:    "btx_0123456789abcdef0123456789abcdef",
		AllocationNo:     "bpa_0123456789abcdef0123456789abcdef",
		InvoiceNo:        input.InvoiceNo, OrderNo: "order-1", CurrencyCode: input.CurrencyCode,
		AmountMinor: input.AmountMinor, TotalAmountMinor: input.TotalAmountMinor, PaidAmountMinor: input.TotalAmountMinor, InvoiceStatus: "paid",
		SettlementNo: "fst_0123456789abcdef0123456789abcdef",
	}
	if input.BalanceAmountMinor > 0 {
		result.BalanceAmountMinor = input.BalanceAmountMinor
		result.BalanceTransactionNo = "btx_1123456789abcdef0123456789abcdef"
		result.BalanceAllocationNo = "bpa_1123456789abcdef0123456789abcdef"
	}
	return result, nil
}

func (f *paymentBillingFake) ReleaseChannelPayment(_ context.Context, input billingcap.ReleaseChannelPayment) (*billingcap.ReleasedChannelPayment, error) {
	f.releaseCalls++
	return &billingcap.ReleasedChannelPayment{PaymentAttemptNo: input.PaymentAttemptNo, BalanceHoldNo: input.BalanceHoldNo, Status: "released"}, nil
}

func (f *paymentBillingFake) AllocateExceptionFund(_ context.Context, input billingcap.AllocateExceptionFund) (*billingcap.ExceptionFundAllocation, error) {
	return &billingcap.ExceptionFundAllocation{ExceptionNo: input.ExceptionNo, PaymentAttemptNo: input.PaymentAttemptNo, TargetOrderNo: input.TargetOrderNo, InvoiceNo: "invoice-exception", AllocationNo: "bpa_0123456789abcdef0123456789abcdef", SettlementNo: "fst_0123456789abcdef0123456789abcdef", CurrencyCode: input.CurrencyCode, AmountMinor: input.AmountMinor}, nil
}
func (f *paymentBillingFake) ClaimPaymentClosure(context.Context) (*billingcap.PaymentClosureWork, error) {
	return &billingcap.PaymentClosureWork{}, nil
}
func (f *paymentBillingFake) ReportPaymentClosure(_ context.Context, input billingcap.ReportPaymentClosure) (*billingcap.PaymentClosureReport, error) {
	return &billingcap.PaymentClosureReport{CancellationNo: input.CancellationNo, StatusCode: "completed", OutcomeCode: input.OutcomeCode}, nil
}

func TestPaymentFoundationMigrationPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv(paymentTestDSN))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL payment migration tests", paymentTestDSN)
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "linapro_payment_test_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quotePaymentIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if !strings.HasPrefix(schema, "linapro_payment_test_") {
			t.Fatalf("unsafe schema %q", schema)
		}
		if _, dropErr := admin.Exec(ctx, "DROP SCHEMA "+quotePaymentIdentifier(schema)+" CASCADE"); dropErr != nil {
			t.Errorf("drop schema: %v", dropErr)
		}
	}()
	database, err := gdb.New(gdb.ConfigNode{Link: appendPaymentDSN(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(ctx)
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration path")
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(file), "..", "manifest", "sql", "[0-9][0-9][0-9]-*.sql"))
	if err != nil || len(paths) != 6 {
		t.Fatalf("migration paths=%v err=%v", paths, err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		for _, path := range paths {
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if _, err = database.Exec(ctx, string(content)); err != nil {
				t.Fatalf("migration %s pass %d: %v", filepath.Base(path), attempt+1, err)
			}
		}
	}
	row, err := database.GetOne(ctx, `
SELECT count(*) AS table_count
FROM information_schema.tables
WHERE table_schema = current_schema()
  AND table_name LIKE 'plugin_idc_payment_%'`)
	if err != nil || row["table_count"].Int() != 16 {
		t.Fatalf("table count row=%v err=%v", row, err)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_payment_channel
  (channel_no, provider_code, display_name, endpoint_origin, merchant_reference, secret_ref,
   supported_modes, supported_currencies, capability_snapshot, status_code, verified_at)
VALUES
  ('channel-epay', 'epay', '易支付', 'https://www.ezfp.cn', 'merchant-1', 'secret-ref-1',
   '["epay_cashier"]', '["CNY"]', '{"create":true,"callback":true}', 'enabled', CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_payment_attempt
  (attempt_no, channel_no, invoice_no, order_no, customer_ref_digest, currency_code,
   amount_minor, mode_code, request_digest, expires_at)
VALUES
  ('attempt-1', 'channel-epay', 'invoice-1', 'order-1', repeat('a',64), 'USD',
   100, 'epay_cashier', repeat('b',64), CURRENT_TIMESTAMP + INTERVAL '1 hour')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_payment_callback_receipt
  (receipt_no, channel_no, attempt_no, provider_event_reference, payload_digest,
   signature_valid, merchant_match, amount_match, currency_match, processing_code, evidence_digest)
VALUES ('receipt-1', 'channel-epay', 'attempt-1', 'trade-1', repeat('c',64), TRUE, TRUE, TRUE, TRUE, 'queued', repeat('d',64))`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_payment_callback_receipt SET processing_code='applied' WHERE receipt_no='receipt-1'`); err == nil {
		t.Fatal("append-only callback history accepted update")
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_payment_refund_attempt
  (refund_attempt_no, refund_request_no, payment_attempt_no, channel_no, currency_code,
   amount_minor, request_digest, capability_code, status_code, result_unknown)
VALUES ('refund-1','request-1','attempt-1','channel-epay','USD',10,repeat('e',64),'manual','result_unknown',FALSE)`); err == nil {
		t.Fatal("refund result_unknown status accepted inconsistent projection")
	}
	billing := &paymentBillingFake{}
	store, err := paymentattempt.New(database, billing)
	if err != nil {
		t.Fatal(err)
	}
	create := paymentattempt.CreateInput{
		AttemptNo: "attempt-service-1", ChannelNo: "channel-epay", InvoiceNo: "invoice-service-1", OrderNo: "order-1",
		CustomerRefDigest: strings.Repeat("1", 64), CurrencyCode: "CNY", AmountMinor: 9900,
		ModeCode: "epay_cashier", RequestDigest: strings.Repeat("2", 64), ExpiresAt: time.Now().Add(time.Hour),
	}
	if err = store.Create(ctx, create); err != nil {
		t.Fatal(err)
	}
	callback := paymentattempt.CallbackInput{ChannelNo: "channel-epay", PayloadDigest: strings.Repeat("3", 64), Result: paymentcap.VerifiedResult{
		ProviderEventReference: "trade-service-1", ExternalTradeReference: "trade-service-1",
		AttemptNo: create.AttemptNo, Currency: "CNY", AmountMinor: 9900, Status: "succeeded", EvidenceDigest: strings.Repeat("4", 64),
	}}
	outcome, err := store.RecordVerifiedCallback(ctx, callback)
	if err != nil || outcome.Status != "queued" || outcome.Duplicate {
		t.Fatalf("callback=%+v err=%v", outcome, err)
	}
	replay, err := store.RecordVerifiedCallback(ctx, callback)
	if err != nil || !replay.Duplicate || replay.ReceiptNo != outcome.ReceiptNo {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	for index, mismatch := range []struct {
		name     string
		currency string
		amount   int64
	}{
		{name: "amount", currency: "CNY", amount: 9901},
		{name: "currency", currency: "USD", amount: 9900},
		{name: "overpayment", currency: "CNY", amount: 19900},
	} {
		mismatchAttempt := create
		mismatchAttempt.AttemptNo = "attempt-mismatch-" + mismatch.name
		mismatchAttempt.InvoiceNo = "invoice-mismatch-" + mismatch.name
		mismatchAttempt.RequestDigest = strings.Repeat(string(rune('c'+index)), 64)
		if err = store.Create(ctx, mismatchAttempt); err != nil {
			t.Fatal(err)
		}
		mismatchCallback := callback
		mismatchCallback.PayloadDigest = strings.Repeat(string(rune('a'+index)), 64)
		mismatchCallback.Result.AttemptNo = mismatchAttempt.AttemptNo
		mismatchCallback.Result.ProviderEventReference = "trade-mismatch-" + mismatch.name
		mismatchCallback.Result.ExternalTradeReference = mismatchCallback.Result.ProviderEventReference
		mismatchCallback.Result.Currency = mismatch.currency
		mismatchCallback.Result.AmountMinor = mismatch.amount
		mismatchCallback.Result.EvidenceDigest = strings.Repeat(string(rune('1'+index)), 64)
		mismatchOutcome, mismatchErr := store.RecordVerifiedCallback(ctx, mismatchCallback)
		if mismatchErr != nil || mismatchOutcome.Status != "manual_review" {
			t.Fatalf("%s mismatch outcome=%+v err=%v", mismatch.name, mismatchOutcome, mismatchErr)
		}
		mismatchState, stateErr := database.GetOne(ctx, `SELECT status_code,result_unknown FROM plugin_idc_payment_attempt WHERE attempt_no=?`, mismatchAttempt.AttemptNo)
		if stateErr != nil || mismatchState["status_code"].String() != "manual_review" || !mismatchState["result_unknown"].Bool() {
			t.Fatalf("%s mismatch state=%v err=%v", mismatch.name, mismatchState, stateErr)
		}
	}
	work, err := store.ClaimConfirmation(ctx, time.Minute)
	if err != nil || work == nil || work.AttemptNo != create.AttemptNo {
		t.Fatalf("work=%+v err=%v", work, err)
	}
	if err = store.CompleteConfirmation(ctx, *work); err != nil {
		t.Fatal(err)
	}
	if billing.calls != 1 {
		t.Fatalf("billing calls=%d", billing.calls)
	}
	state, err := database.GetOne(ctx, `SELECT status_code FROM plugin_idc_payment_attempt WHERE attempt_no=?`, create.AttemptNo)
	if err != nil || state["status_code"].String() != "succeeded" {
		t.Fatalf("state=%v err=%v", state, err)
	}
	late := create
	late.AttemptNo = "attempt-service-late"
	late.InvoiceNo = "invoice-service-late"
	late.RequestDigest = strings.Repeat("5", 64)
	if err = store.Create(ctx, late); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_payment_attempt SET status_code='closed', closed_at=CURRENT_TIMESTAMP WHERE attempt_no=?`, late.AttemptNo); err != nil {
		t.Fatal(err)
	}
	lateCallback := callback
	lateCallback.PayloadDigest = strings.Repeat("6", 64)
	lateCallback.Result.AttemptNo = late.AttemptNo
	lateCallback.Result.ProviderEventReference = "trade-service-late"
	lateCallback.Result.ExternalTradeReference = "trade-service-late"
	lateCallback.Result.EvidenceDigest = strings.Repeat("7", 64)
	lateOutcome, err := store.RecordVerifiedCallback(ctx, lateCallback)
	if err != nil || lateOutcome.Status != "late_success" {
		t.Fatalf("late=%+v err=%v", lateOutcome, err)
	}
	exception, err := database.GetOne(ctx, `SELECT status_code,reason_code FROM plugin_idc_payment_exception_fund WHERE attempt_no=?`, late.AttemptNo)
	if err != nil || exception["status_code"].String() != "pending" || exception["reason_code"].String() != "late_success" {
		t.Fatalf("exception=%v err=%v", exception, err)
	}
	exceptionService, err := exceptionfund.New(database, billing)
	if err != nil {
		t.Fatal(err)
	}
	lateException, err := database.GetOne(ctx, `SELECT exception_no FROM plugin_idc_payment_exception_fund WHERE attempt_no=?`, late.AttemptNo)
	if err != nil || lateException.IsEmpty() {
		t.Fatalf("late exception=%v err=%v", lateException, err)
	}
	allocatedFund, err := exceptionService.Allocate(ctx, lateException["exception_no"].String(), "order-new-unpaid", 101)
	if err != nil || allocatedFund.StatusCode != "allocated" || allocatedFund.ActionStatus != "completed" || allocatedFund.TargetOrderNo != "order-new-unpaid" {
		t.Fatalf("allocated fund=%+v err=%v", allocatedFund, err)
	}
	allocatedReplay, err := exceptionService.Allocate(ctx, lateException["exception_no"].String(), "order-new-unpaid", 101)
	if err != nil || allocatedReplay.StatusCode != "allocated" || allocatedReplay.ActionNo != allocatedFund.ActionNo || allocatedReplay.Version != allocatedFund.Version {
		t.Fatalf("allocated fund replay=%+v err=%v", allocatedReplay, err)
	}
	lateState, err := database.GetOne(ctx, `SELECT status_code FROM plugin_idc_payment_attempt WHERE attempt_no=?`, late.AttemptNo)
	if err != nil || lateState["status_code"].String() != "late_success" {
		t.Fatalf("old order attempt was revived: state=%v err=%v", lateState, err)
	}
	manual := late
	manual.AttemptNo = "attempt-late-manual-refund"
	manual.InvoiceNo = "invoice-late-manual-refund"
	manual.OrderNo = "order-late-manual-refund"
	manual.RequestDigest = strings.Repeat("d", 64)
	if err = store.Create(ctx, manual); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_payment_attempt SET status_code='late_success',external_trade_reference='trade-late-manual',funding_result_digest=repeat('e',64),confirmed_at=CURRENT_TIMESTAMP WHERE attempt_no=?`, manual.AttemptNo); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_payment_exception_fund(exception_no,attempt_no,channel_no,currency_code,amount_minor,reason_code,evidence_digest) VALUES('pex_late_manual_refund',?,'channel-epay','CNY',?,'late_success',repeat('e',64))`, manual.AttemptNo, manual.AmountMinor); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_payment_channel SET capability_snapshot='{"refund":false}'::jsonb WHERE channel_no='channel-epay'`); err != nil {
		t.Fatal(err)
	}
	manualFund, err := exceptionService.RequestOriginalRefund(ctx, "pex_late_manual_refund", 101)
	if err != nil || manualFund.ActionStatus != "manual_pending" {
		t.Fatalf("manual refund=%+v err=%v", manualFund, err)
	}
	manualFund, err = exceptionService.ConfirmManualRefund(ctx, "pex_late_manual_refund", "host-file:123", strings.Repeat("f", 64), 101, manualFund.Version)
	if err != nil || manualFund.StatusCode != "refunded" || manualFund.ActionStatus != "completed" {
		t.Fatalf("confirmed manual refund=%+v err=%v", manualFund, err)
	}
	manualReplay, err := exceptionService.RequestOriginalRefund(ctx, "pex_late_manual_refund", 101)
	if err != nil || manualReplay.StatusCode != "refunded" || manualReplay.ActionNo != manualFund.ActionNo || manualReplay.Version != manualFund.Version {
		t.Fatalf("manual refund replay=%+v err=%v", manualReplay, err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_payment_channel SET capability_snapshot='{"create":true,"callback":true}'::jsonb WHERE channel_no='channel-epay'`); err != nil {
		t.Fatal(err)
	}
	closureAttempt := paymentattempt.CreateInput{AttemptNo: "attempt-closure-safe", ChannelNo: "channel-epay", InvoiceNo: "invoice-closure-safe", OrderNo: "order-closure-safe", CustomerRefDigest: strings.Repeat("1", 64), CurrencyCode: "CNY", AmountMinor: 500, ModeCode: string(paymentcap.ModeEPayCashier), RequestDigest: strings.Repeat("2", 64), ExpiresAt: time.Now().Add(time.Hour)}
	if err = store.Create(ctx, closureAttempt); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_payment_attempt SET status_code='submitted',external_trade_reference='trade-closure-safe' WHERE attempt_no=?`, closureAttempt.AttemptNo); err != nil {
		t.Fatal(err)
	}
	closeCalls := 0
	closureOutcome, _, _, err := store.CloseOrderAttempts(ctx, closureAttempt.OrderNo, func(context.Context, paymentattempt.ClosureAttempt) error {
		closeCalls++
		return nil
	})
	if err != nil || closureOutcome != "closed" || closeCalls != 1 {
		t.Fatalf("safe closure outcome=%s calls=%d err=%v", closureOutcome, closeCalls, err)
	}
	closureOutcome, _, _, err = store.CloseOrderAttempts(ctx, closureAttempt.OrderNo, func(context.Context, paymentattempt.ClosureAttempt) error {
		closeCalls++
		return nil
	})
	if err != nil || closureOutcome != "closed" || closeCalls != 1 {
		t.Fatalf("safe closure replay outcome=%s calls=%d err=%v", closureOutcome, closeCalls, err)
	}
	unknownAttempt := closureAttempt
	unknownAttempt.AttemptNo = "attempt-closure-unknown"
	unknownAttempt.InvoiceNo = "invoice-closure-unknown"
	unknownAttempt.OrderNo = "order-closure-unknown"
	unknownAttempt.RequestDigest = strings.Repeat("3", 64)
	if err = store.Create(ctx, unknownAttempt); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_payment_attempt SET status_code='submitted',external_trade_reference='trade-closure-unknown' WHERE attempt_no=?`, unknownAttempt.AttemptNo); err != nil {
		t.Fatal(err)
	}
	unknownCalls := 0
	closureOutcome, _, _, err = store.CloseOrderAttempts(ctx, unknownAttempt.OrderNo, func(context.Context, paymentattempt.ClosureAttempt) error {
		unknownCalls++
		return errors.New("provider timeout")
	})
	if err != nil || closureOutcome != "unknown" || unknownCalls != 1 {
		t.Fatalf("unknown closure outcome=%s calls=%d err=%v", closureOutcome, unknownCalls, err)
	}
	closureOutcome, _, _, err = store.CloseOrderAttempts(ctx, unknownAttempt.OrderNo, func(context.Context, paymentattempt.ClosureAttempt) error {
		unknownCalls++
		return nil
	})
	if err != nil || closureOutcome != "unknown" || unknownCalls != 1 {
		t.Fatalf("unknown closure replay outcome=%s calls=%d err=%v", closureOutcome, unknownCalls, err)
	}
	combination := paymentattempt.CreateInput{AttemptNo: "attempt-combination", ChannelNo: "channel-epay", InvoiceNo: "invoice-combination", OrderNo: "order-combination", CustomerRefDigest: strings.Repeat("8", 64), CurrencyCode: "CNY", AmountMinor: 12345, BalanceLockedMinor: 2345, BalanceLockReference: "bhd_combination_0001", ModeCode: string(paymentcap.ModeEPayCashier), RequestDigest: strings.Repeat("9", 64), ExpiresAt: time.Now().Add(time.Hour)}
	if err = store.Create(ctx, combination); err != nil {
		t.Fatal(err)
	}
	combinationCallback := paymentattempt.CallbackInput{ChannelNo: "channel-epay", PayloadDigest: strings.Repeat("a", 64), Result: paymentcap.VerifiedResult{ProviderEventReference: "trade-combination", ExternalTradeReference: "trade-combination", AttemptNo: combination.AttemptNo, Currency: "CNY", AmountMinor: 10000, Status: "succeeded", EvidenceDigest: strings.Repeat("b", 64)}}
	if result, callbackErr := store.RecordVerifiedCallback(ctx, combinationCallback); callbackErr != nil || result.Status != "queued" {
		t.Fatalf("combination callback=%+v err=%v", result, callbackErr)
	}
	combinationWork, err := store.ClaimConfirmation(ctx, time.Minute)
	if err != nil || combinationWork.AttemptNo != combination.AttemptNo {
		t.Fatalf("combination work=%+v err=%v", combinationWork, err)
	}
	if err = store.CompleteConfirmation(ctx, *combinationWork); err != nil {
		t.Fatal(err)
	}
	if billing.last.AmountMinor != 10000 || billing.last.TotalAmountMinor != 12345 || billing.last.BalanceAmountMinor != 2345 || billing.last.BalanceHoldNo != "bhd_combination_0001" {
		t.Fatalf("combination core input=%+v", billing.last)
	}
	secret := &paymentSecretFake{}
	epayAdapter, err := epay.NewWithClient("", &http.Client{Transport: paymentRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusMethodNotAllowed, Body: io.NopCloser(strings.NewReader("method not allowed")), Header: make(http.Header)}, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	channels, err := channelconfig.New(database, secret, secret, []paymentcap.Adapter{epayAdapter}, func(_ context.Context, provider paymentcap.ProviderCode, credentials paymentcap.Credentials) error {
		if provider != paymentcap.ProviderEPay || credentials.MerchantID != "1001" || string(credentials.Secret) != "merchant-key" {
			return errors.New("bad verification material")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	configured, err := channels.Save(ctx, channelconfig.SaveInput{ChannelNo: "channel-configured", Provider: paymentcap.ProviderEPay, DisplayName: "备用易支付", EndpointOrigin: "https://www.ezfp.cn/submit.php", MerchantReference: "1001", Credential: channelconfig.CredentialEnvelope{MerchantID: "1001", Secret: "merchant-key"}, IdempotencyKey: "configure-1", ActorID: 1})
	if err != nil || configured.StatusCode != "disabled" || !configured.VerifiedAt.IsZero() || len(configured.Modes) != 3 || len(configured.Currencies) != 1 {
		t.Fatalf("configured=%+v err=%v", configured, err)
	}
	configured, err = channels.VerifyConnection(ctx, configured.ChannelNo, configured.Version, 1)
	if err != nil || configured.StatusCode != "disabled" || configured.VerifiedAt.IsZero() {
		t.Fatalf("verified=%+v err=%v", configured, err)
	}
	configured, err = channels.SetStatus(ctx, configured.ChannelNo, "enabled", configured.Version, 1)
	if err != nil || configured.StatusCode != "enabled" {
		t.Fatalf("enabled=%+v err=%v", configured, err)
	}
	stored, err := database.GetOne(ctx, `SELECT secret_ref,capability_snapshot::text AS capabilities FROM plugin_idc_payment_channel WHERE channel_no='channel-configured'`)
	if err != nil || stored["secret_ref"].String() == "merchant-key" || strings.Contains(stored["capabilities"].String(), "merchant-key") {
		t.Fatalf("stored=%v err=%v", stored, err)
	}
	called := false
	err = channels.WithCredentials(ctx, "channel-configured", func(adapter paymentcap.Adapter, credentials paymentcap.Credentials) error {
		called = true
		if adapter.Provider() != paymentcap.ProviderEPay || string(credentials.Secret) != "merchant-key" {
			return errors.New("credential mismatch")
		}
		redirect, createErr := adapter.Create(ctx, credentials, paymentcap.CreateRequest{AttemptNo: "configured-attempt", InvoiceNo: "configured-invoice", Mode: paymentcap.ModeEPayCashier, Currency: "CNY", AmountMinor: 100, Subject: "test", NotifyURL: "https://merchant.example/callback", ReturnURL: "https://merchant.example/return", ExpiresAt: time.Now().Add(time.Hour)})
		if createErr != nil || redirect.URL != "https://www.ezfp.cn/submit.php" {
			return errors.New("configured endpoint was not bound")
		}
		return nil
	})
	if err != nil || !called {
		t.Fatalf("credential access called=%v err=%v", called, err)
	}
	flows, err := paymentflow.New(store, channels, paymentflow.Config{PublicOrigin: "https://customer.example"})
	if err != nil {
		t.Fatal(err)
	}
	billing.refundWork = &billingcap.RefundExecutionWork{
		HasWork: true, CommandNo: "rfc_payment_preflight_0001",
		RefundNo: "rfd_payment_preflight_0001", RefundSourceNo: "rfs_payment_preflight_0001",
		PaymentAttemptNo: create.AttemptNo, CurrencyCode: "CNY", AmountMinor: 1000,
		RequestDigest: strings.Repeat("f", 64), Phase: billingcap.RefundExecutionPhasePreflight,
		Attempt: 1, LeaseExpiresAt: time.Now().Add(time.Minute),
	}
	worked, err := flows.RunOneRefund(ctx)
	if err != nil || !worked || len(billing.refundReports) != 1 ||
		billing.refundReports[0].Status != billingcap.RefundExecutionResultFailed ||
		billing.refundReports[0].ResultCode != "original_channel_unsupported" {
		t.Fatalf("unsupported refund preflight worked=%v reports=%+v err=%v", worked, billing.refundReports, err)
	}
	refundAttemptCount, err := database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_payment_refund_attempt`)
	if err != nil || refundAttemptCount.Int64() != 0 {
		t.Fatalf("unsupported refund reached money execution count=%d err=%v", refundAttemptCount.Int64(), err)
	}
	preflightEvidenceCount, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM plugin_idc_payment_evidence
WHERE subject_no='rfc_payment_preflight_0001' AND evidence_type='refund_preflight'`)
	if err != nil || preflightEvidenceCount.Int64() != 1 {
		t.Fatalf("refund preflight evidence count=%d err=%v", preflightEvidenceCount.Int64(), err)
	}
	refundAdapter := &paymentRefundAdapterFake{}
	refundSecret := &paymentSecretFake{}
	refundChannels, err := channelconfig.New(
		database, refundSecret, refundSecret, []paymentcap.Adapter{refundAdapter},
		func(_ context.Context, provider paymentcap.ProviderCode, credentials paymentcap.Credentials) error {
			if provider != paymentcap.ProviderAlipay || credentials.MerchantID != "refund-merchant" {
				return errors.New("bad refund verification material")
			}
			return nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	refundChannel, err := refundChannels.Save(ctx, channelconfig.SaveInput{
		ChannelNo: "channel-refund-alipay", Provider: paymentcap.ProviderAlipay,
		DisplayName: "退款支付宝", EndpointOrigin: "https://openapi.alipay.com/gateway.do",
		MerchantReference: "refund-merchant",
		Credential:        channelconfig.CredentialEnvelope{MerchantID: "refund-merchant"},
		EnabledModes:      []paymentcap.Mode{paymentcap.ModeAlipayPage},
		IdempotencyKey:    "configure-refund-1", ActorID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	refundChannel, err = refundChannels.VerifyConnection(ctx, refundChannel.ChannelNo, refundChannel.Version, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = refundChannels.SetStatus(ctx, refundChannel.ChannelNo, "enabled", refundChannel.Version, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_payment_attempt
    (attempt_no,channel_no,invoice_no,order_no,customer_ref_digest,currency_code,
     amount_minor,mode_code,request_digest,external_trade_reference,status_code,
     expires_at,confirmed_at,funding_result_digest)
VALUES ('attempt-exception-auto-refund','channel-refund-alipay','invoice-exception-auto-refund',
        'order-exception-auto-refund',repeat('1',64),'CNY',700,'alipay_page',
        repeat('2',64),'provider-trade-exception-auto-refund','late_success',
        CURRENT_TIMESTAMP+INTERVAL '1 hour',CURRENT_TIMESTAMP,repeat('3',64))`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_payment_exception_fund(exception_no,attempt_no,channel_no,currency_code,amount_minor,reason_code,evidence_digest) VALUES('pex_auto_refund','attempt-exception-auto-refund','channel-refund-alipay','CNY',700,'late_success',repeat('3',64))`); err != nil {
		t.Fatal(err)
	}
	autoFund, err := exceptionService.RequestOriginalRefund(ctx, "pex_auto_refund", 101)
	if err != nil || autoFund.ActionStatus != "automatic_pending" {
		t.Fatalf("automatic refund request=%+v err=%v", autoFund, err)
	}
	autoReplay, err := exceptionService.RequestOriginalRefund(ctx, "pex_auto_refund", 101)
	if err != nil || autoReplay.ActionStatus != "automatic_pending" || autoReplay.ActionNo != autoFund.ActionNo || autoReplay.Version != autoFund.Version {
		t.Fatalf("automatic refund request replay=%+v err=%v", autoReplay, err)
	}
	refundCallsBeforeException := refundAdapter.refundCalls
	worked, err = exceptionService.RunOneAutomaticRefund(ctx, refundChannels)
	if err != nil || !worked || refundAdapter.refundCalls != refundCallsBeforeException+1 {
		t.Fatalf("automatic exception refund worked=%v calls=%d err=%v", worked, refundAdapter.refundCalls, err)
	}
	autoFund, err = exceptionService.Get(ctx, "pex_auto_refund")
	if err != nil || autoFund.StatusCode != "refunded" || autoFund.ActionStatus != "completed" {
		t.Fatalf("automatic exception refund result=%+v err=%v", autoFund, err)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_payment_attempt
    (attempt_no,channel_no,invoice_no,order_no,customer_ref_digest,currency_code,
     amount_minor,mode_code,request_digest,external_trade_reference,status_code,
     expires_at,confirmed_at)
VALUES ('attempt-refund-success','channel-refund-alipay','invoice-refund-success',
        'order-refund-success',repeat('1',64),'CNY',5000,'alipay_page',
        repeat('2',64),'provider-trade-refund-success','succeeded',
        CURRENT_TIMESTAMP+INTERVAL '1 hour',CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	refundFlow, err := paymentflow.New(store, refundChannels, paymentflow.Config{PublicOrigin: "https://customer.example"})
	if err != nil {
		t.Fatal(err)
	}
	billing.refundWork = &billingcap.RefundExecutionWork{
		HasWork: true, CommandNo: "rfc_payment_success_0001",
		RefundNo: "rfd_payment_success_0001", RefundSourceNo: "rfs_payment_success_0001",
		PaymentAttemptNo: "attempt-refund-success", CurrencyCode: "CNY", AmountMinor: 1200,
		RequestDigest: strings.Repeat("3", 64), Phase: billingcap.RefundExecutionPhasePreflight,
		Attempt: 1, LeaseExpiresAt: time.Now().Add(time.Minute),
	}
	if worked, err = refundFlow.RunOneRefund(ctx); err != nil || !worked ||
		len(billing.refundReports) != 2 ||
		billing.refundReports[1].Status != billingcap.RefundExecutionResultReady {
		t.Fatalf("supported refund preflight worked=%v reports=%+v err=%v", worked, billing.refundReports, err)
	}
	if worked, err = refundFlow.RunOneRefund(ctx); err != nil || !worked ||
		refundAdapter.refundCalls != refundCallsBeforeException+2 || len(billing.refundReports) != 3 ||
		billing.refundReports[2].Status != billingcap.RefundExecutionResultSucceeded {
		t.Fatalf("supported refund execution worked=%v calls=%d reports=%+v err=%v", worked, refundAdapter.refundCalls, billing.refundReports, err)
	}
	if refundAdapter.refundNotifyURL != "https://customer.example/x/idc-payment/api/v1/callbacks/refunds/channel-refund-alipay" {
		t.Fatalf("generated refund notification misses registered plugin route: %q", refundAdapter.refundNotifyURL)
	}
	refundState, err := database.GetOne(ctx, `
SELECT status_code,result_code,result_evidence_digest
FROM plugin_idc_payment_refund_attempt
WHERE refund_request_no='rfs_payment_success_0001'`)
	if err != nil || refundState["status_code"].String() != "succeeded" ||
		refundState["result_code"].String() != "refund_succeeded" ||
		len(refundState["result_evidence_digest"].String()) != 64 {
		t.Fatalf("successful refund attempt state=%v err=%v", refundState, err)
	}
	billing.refundWork = &billingcap.RefundExecutionWork{
		HasWork: true, CommandNo: "rfc_payment_mismatch_0001",
		RefundNo: "rfd_payment_mismatch_0001", RefundSourceNo: "rfs_payment_mismatch_0001",
		PaymentAttemptNo: "attempt-refund-success", CurrencyCode: "CNY", AmountMinor: 6000,
		RequestDigest: strings.Repeat("5", 64), Phase: billingcap.RefundExecutionPhasePreflight,
		Attempt: 1, LeaseExpiresAt: time.Now().Add(time.Minute),
	}
	if worked, err = refundFlow.RunOneRefund(ctx); err != nil || !worked ||
		refundAdapter.refundCalls != refundCallsBeforeException+2 || len(billing.refundReports) != 4 ||
		billing.refundReports[3].Status != billingcap.RefundExecutionResultManualConfirmation ||
		billing.refundReports[3].ResultCode != "payment_context_mismatch" {
		t.Fatalf("mismatched refund preflight worked=%v calls=%d reports=%+v err=%v", worked, refundAdapter.refundCalls, billing.refundReports, err)
	}
	mismatchEvidenceCount, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM plugin_idc_payment_evidence
WHERE subject_no='rfc_payment_mismatch_0001' AND evidence_type='refund_preflight'
  AND summary_code='payment_context_mismatch'`)
	if err != nil || mismatchEvidenceCount.Int64() != 1 {
		t.Fatalf("refund mismatch evidence count=%d err=%v", mismatchEvidenceCount.Int64(), err)
	}
	refundAdapter.refundStatus = "pending"
	billing.refundWork = &billingcap.RefundExecutionWork{
		HasWork: true, CommandNo: "rfc_payment_callback_0001",
		RefundNo: "rfd_payment_callback_0001", RefundSourceNo: "rfs_payment_callback_0001",
		PaymentAttemptNo: "attempt-refund-success", CurrencyCode: "CNY", AmountMinor: 1200,
		RequestDigest: strings.Repeat("6", 64), Phase: billingcap.RefundExecutionPhasePreflight,
		Attempt: 1, LeaseExpiresAt: time.Now().Add(time.Minute),
	}
	if worked, err = refundFlow.RunOneRefund(ctx); err != nil || !worked || len(billing.refundReports) != 5 {
		t.Fatalf("callback refund preflight worked=%v reports=%+v err=%v", worked, billing.refundReports, err)
	}
	if worked, err = refundFlow.RunOneRefund(ctx); err != nil || !worked || len(billing.refundReports) != 5 {
		t.Fatalf("pending refund execution worked=%v reports=%+v err=%v", worked, billing.refundReports, err)
	}
	pendingCallbackAttempt, err := database.GetOne(ctx, `
SELECT refund_attempt_no,status_code FROM plugin_idc_payment_refund_attempt
WHERE refund_request_no='rfs_payment_callback_0001'`)
	if err != nil || pendingCallbackAttempt["status_code"].String() != "submitted" {
		t.Fatalf("pending callback refund state=%v err=%v", pendingCallbackAttempt, err)
	}
	callbackAttemptNo := pendingCallbackAttempt["refund_attempt_no"].String()
	refundAdapter.callbackValue = &paymentcap.VerifiedResult{
		ProviderEventReference: "refund-callback-1", ExternalTradeReference: "provider-" + callbackAttemptNo,
		AttemptNo: callbackAttemptNo, Currency: "CNY", AmountMinor: 1200,
		Status: "succeeded", EvidenceDigest: strings.Repeat("7", 64),
	}
	callbackOutcome, callbackProvider, callbackErr := refundFlow.RefundCallback(
		ctx, "channel-refund-alipay", paymentcap.CallbackRequest{Body: []byte(`{"verified":true}`)},
	)
	if callbackErr != nil || callbackProvider != paymentcap.ProviderAlipay || callbackOutcome.StatusCode != "succeeded" || callbackOutcome.Reused {
		t.Fatalf("refund callback outcome=%+v provider=%s err=%v", callbackOutcome, callbackProvider, callbackErr)
	}
	replayedOutcome, _, callbackErr := refundFlow.RefundCallback(
		ctx, "channel-refund-alipay", paymentcap.CallbackRequest{Body: []byte(`{"verified":true}`)},
	)
	if callbackErr != nil || !replayedOutcome.Reused {
		t.Fatalf("refund callback replay=%+v err=%v", replayedOutcome, callbackErr)
	}
	callbackWork := *billing.lastRefundWork
	callbackWork.Attempt++
	callbackWork.LeaseExpiresAt = time.Now().Add(time.Minute)
	billing.refundWork = &callbackWork
	if worked, err = refundFlow.RunOneRefund(ctx); err != nil || !worked || len(billing.refundReports) != 6 ||
		billing.refundReports[5].Status != billingcap.RefundExecutionResultSucceeded {
		t.Fatalf("callback refund report worked=%v reports=%+v err=%v", worked, billing.refundReports, err)
	}
	refundAdapter.callbackValue = nil

	billing.refundWork = &billingcap.RefundExecutionWork{
		HasWork: true, CommandNo: "rfc_payment_query_0001",
		RefundNo: "rfd_payment_query_0001", RefundSourceNo: "rfs_payment_query_0001",
		PaymentAttemptNo: "attempt-refund-success", CurrencyCode: "CNY", AmountMinor: 1200,
		RequestDigest: strings.Repeat("8", 64), Phase: billingcap.RefundExecutionPhasePreflight,
		Attempt: 1, LeaseExpiresAt: time.Now().Add(time.Minute),
	}
	if worked, err = refundFlow.RunOneRefund(ctx); err != nil || !worked || len(billing.refundReports) != 7 {
		t.Fatalf("query refund preflight worked=%v reports=%+v err=%v", worked, billing.refundReports, err)
	}
	if worked, err = refundFlow.RunOneRefund(ctx); err != nil || !worked || len(billing.refundReports) != 7 {
		t.Fatalf("query refund submission worked=%v reports=%+v err=%v", worked, billing.refundReports, err)
	}
	refundAdapter.queryStatus = "succeeded"
	queryRefundWork := *billing.lastRefundWork
	queryRefundWork.Attempt++
	queryRefundWork.LeaseExpiresAt = time.Now().Add(time.Minute)
	billing.refundWork = &queryRefundWork
	if worked, err = refundFlow.RunOneRefund(ctx); err != nil || !worked || refundAdapter.queryCalls != 1 ||
		len(billing.refundReports) != 8 || billing.refundReports[7].Status != billingcap.RefundExecutionResultSucceeded {
		t.Fatalf("queried refund worked=%v queryCalls=%d reports=%+v err=%v", worked, refundAdapter.queryCalls, billing.refundReports, err)
	}
	queryRefundEvidence, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM plugin_idc_payment_evidence evidence
JOIN plugin_idc_payment_refund_attempt attempt ON attempt.refund_attempt_no=evidence.subject_no
WHERE attempt.refund_request_no='rfs_payment_query_0001' AND evidence.evidence_type='refund_result'`)
	if err != nil || queryRefundEvidence.Int64() < 2 {
		t.Fatalf("query refund evidence count=%d err=%v", queryRefundEvidence.Int64(), err)
	}
	refundAdapter.queryStatus = "pending"
	billing.refundWork = &billingcap.RefundExecutionWork{
		HasWork: true, CommandNo: "rfc_payment_unknown_0001",
		RefundNo: "rfd_payment_unknown_0001", RefundSourceNo: "rfs_payment_unknown_0001",
		PaymentAttemptNo: "attempt-refund-success", CurrencyCode: "CNY", AmountMinor: 1200,
		RequestDigest: strings.Repeat("9", 64), Phase: billingcap.RefundExecutionPhasePreflight,
		Attempt: 1, LeaseExpiresAt: time.Now().Add(time.Minute),
	}
	if worked, err = refundFlow.RunOneRefund(ctx); err != nil || !worked || len(billing.refundReports) != 9 {
		t.Fatalf("unknown refund preflight worked=%v reports=%+v err=%v", worked, billing.refundReports, err)
	}
	if worked, err = refundFlow.RunOneRefund(ctx); err != nil || !worked || len(billing.refundReports) != 9 {
		t.Fatalf("unknown refund submission worked=%v reports=%+v err=%v", worked, billing.refundReports, err)
	}
	unknownRefundWork := *billing.lastRefundWork
	unknownRefundWork.Attempt = 8
	unknownRefundWork.LeaseExpiresAt = time.Now().Add(time.Minute)
	billing.refundWork = &unknownRefundWork
	if worked, err = refundFlow.RunOneRefund(ctx); err != nil || !worked || len(billing.refundReports) != 10 ||
		billing.refundReports[9].Status != billingcap.RefundExecutionResultUnknown {
		t.Fatalf("unknown refund result worked=%v reports=%+v err=%v", worked, billing.refundReports, err)
	}
	unknownRefundState, err := database.GetOne(ctx, `
SELECT status_code,result_unknown,result_code FROM plugin_idc_payment_refund_attempt
WHERE refund_request_no='rfs_payment_unknown_0001'`)
	if err != nil || unknownRefundState["status_code"].String() != "result_unknown" ||
		!unknownRefundState["result_unknown"].Bool() || unknownRefundState["result_code"].String() != "provider_result_unknown" {
		t.Fatalf("unknown refund state=%v err=%v", unknownRefundState, err)
	}
	created, err := flows.Create(ctx, paymentflow.CreateInput{PaymentContextNo: "PAY_context_0001", ChannelNo: "channel-configured", Mode: paymentcap.ModeEPayCashier})
	if err != nil || created.StatusCode != "confirming" || created.Redirect.URL != "https://www.ezfp.cn/submit.php" {
		t.Fatalf("created flow=%+v err=%v", created, err)
	}
	observed, err := flows.Status(ctx, created.AttemptNo)
	if err != nil || observed.StatusCode != "submitted" || observed.QueryQueued {
		t.Fatalf("observed flow=%+v err=%v", observed, err)
	}
	creditCreated, err := flows.CreateCreditRepayment(ctx, paymentflow.CreditCreateInput{StatementNo: "credit-statement-1", ChannelNo: "channel-configured", AmountMinor: 2500, Mode: paymentcap.ModeEPayCashier})
	if err != nil || creditCreated.Redirect.URL != "https://www.ezfp.cn/submit.php" {
		t.Fatalf("credit payment=%+v err=%v", creditCreated, err)
	}
	creditObserved, err := flows.Status(ctx, creditCreated.AttemptNo)
	if err != nil || creditObserved.PurposeCode != "credit_statement" || creditObserved.PurposeReference != "credit-statement-1" || creditObserved.TotalAmountMinor != 2500 {
		t.Fatalf("credit payment status=%+v err=%v", creditObserved, err)
	}
	creditCallback := paymentattempt.CallbackInput{ChannelNo: "channel-configured", PayloadDigest: strings.Repeat("1", 64), Result: paymentcap.VerifiedResult{ProviderEventReference: "credit-trade-1", ExternalTradeReference: "credit-trade-1", AttemptNo: creditCreated.AttemptNo, Currency: "CNY", AmountMinor: 2500, Status: "succeeded", EvidenceDigest: strings.Repeat("2", 64)}}
	if outcome, callbackErr := store.RecordVerifiedCallback(ctx, creditCallback); callbackErr != nil || outcome.Status != "queued" {
		t.Fatalf("credit callback=%+v err=%v", outcome, callbackErr)
	}
	creditWork, err := store.ClaimConfirmation(ctx, time.Minute)
	if err != nil || creditWork == nil || creditWork.PurposeCode != "credit_statement" {
		t.Fatalf("credit work=%+v err=%v", creditWork, err)
	}
	if err = store.CompleteConfirmation(ctx, *creditWork); err != nil || billing.creditCalls != 1 {
		t.Fatalf("complete credit payment calls=%d err=%v", billing.creditCalls, err)
	}
	rechargeCreated, err := flows.CreateBalanceRecharge(ctx, paymentflow.RechargeCreateInput{ChannelNo: "channel-configured", CurrencyCode: "CNY", AmountMinor: 8800, Mode: paymentcap.ModeEPayCashier})
	if err != nil || rechargeCreated.Redirect.URL != "https://www.ezfp.cn/submit.php" {
		t.Fatalf("balance recharge=%+v err=%v", rechargeCreated, err)
	}
	rechargeObserved, err := flows.Status(ctx, rechargeCreated.AttemptNo)
	if err != nil || rechargeObserved.PurposeCode != "balance_recharge" || rechargeObserved.PurposeReference != "BRC_payment_0001" || rechargeObserved.TotalAmountMinor != 8800 {
		t.Fatalf("balance recharge status=%+v err=%v", rechargeObserved, err)
	}
	rechargeCallback := paymentattempt.CallbackInput{ChannelNo: "channel-configured", PayloadDigest: strings.Repeat("3", 64), Result: paymentcap.VerifiedResult{ProviderEventReference: "recharge-trade-1", ExternalTradeReference: "recharge-trade-1", AttemptNo: rechargeCreated.AttemptNo, Currency: "CNY", AmountMinor: 8800, Status: "succeeded", EvidenceDigest: strings.Repeat("4", 64)}}
	if outcome, callbackErr := store.RecordVerifiedCallback(ctx, rechargeCallback); callbackErr != nil || outcome.Status != "queued" {
		t.Fatalf("recharge callback=%+v err=%v", outcome, callbackErr)
	}
	rechargeWork, err := store.ClaimConfirmation(ctx, time.Minute)
	if err != nil || rechargeWork == nil || rechargeWork.PurposeCode != "balance_recharge" {
		t.Fatalf("recharge work=%+v err=%v", rechargeWork, err)
	}
	if err = store.CompleteConfirmation(ctx, *rechargeWork); err != nil || billing.rechargeCalls != 1 {
		t.Fatalf("complete recharge calls=%d err=%v", billing.rechargeCalls, err)
	}
	unconfigured, err := paymentflow.New(store, channels, paymentflow.Config{PublicOrigin: "https://localhost.invalid"})
	if err != nil {
		t.Fatal(err)
	}
	if result, createErr := unconfigured.Create(ctx, paymentflow.CreateInput{PaymentContextNo: "PAY_context_unconfigured", ChannelNo: "channel-configured", Mode: paymentcap.ModeEPayCashier}); createErr == nil || result != nil {
		t.Fatalf("unconfigured public origin created payment=%+v err=%v", result, createErr)
	}

	queryAttempt := create
	queryAttempt.AttemptNo = "attempt-query-closed"
	queryAttempt.InvoiceNo = "invoice-query-closed"
	queryAttempt.RequestDigest = strings.Repeat("c", 64)
	queryAttempt.BalanceLockedMinor = 900
	queryAttempt.BalanceLockReference = "bhd_query_closed_0001"
	if err = store.Create(ctx, queryAttempt); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_payment_work_item(work_no,work_type,subject_no,evidence_digest,status_code,attempt_count,lease_token_digest,lease_expires_at) VALUES('query-work-expired','query_payment',?,repeat('d',64),'leased',1,repeat('e',64),CURRENT_TIMESTAMP-INTERVAL '1 second')`, queryAttempt.AttemptNo); err != nil {
		t.Fatal(err)
	}
	queryWork, err := store.ClaimQuery(ctx, time.Minute)
	if err != nil || queryWork == nil || queryWork.AttemptNo != queryAttempt.AttemptNo {
		t.Fatalf("reclaimed query work=%+v err=%v", queryWork, err)
	}
	if err = store.CompleteQuery(ctx, *queryWork, paymentcap.VerifiedResult{ProviderEventReference: "query-closed", AttemptNo: queryAttempt.AttemptNo, Currency: "CNY", Status: "closed", EvidenceDigest: strings.Repeat("f", 64)}); err != nil {
		t.Fatal(err)
	}
	queryState, err := database.GetOne(ctx, `SELECT attempt.status_code,work.status_code AS work_status FROM plugin_idc_payment_attempt attempt JOIN plugin_idc_payment_work_item work ON work.subject_no=attempt.attempt_no AND work.work_type='query_payment' WHERE attempt.attempt_no=?`, queryAttempt.AttemptNo)
	if err != nil || queryState["status_code"].String() != "failed" || queryState["work_status"].String() != "succeeded" || billing.releaseCalls != 1 {
		t.Fatalf("closed query state=%v releaseCalls=%d err=%v", queryState, billing.releaseCalls, err)
	}

	reconciliationAttempt := create
	reconciliationAttempt.AttemptNo = "attempt-reconciliation"
	reconciliationAttempt.InvoiceNo = "invoice-reconciliation"
	reconciliationAttempt.RequestDigest = strings.Repeat("7", 64)
	reconciliationAttempt.AmountMinor = 4567
	if err = store.Create(ctx, reconciliationAttempt); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_payment_attempt SET status_code='submitted',submitted_at=CURRENT_TIMESTAMP WHERE attempt_no=?`, reconciliationAttempt.AttemptNo); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_payment_reconciliation_batch(batch_no,channel_no,business_date,source_digest,status_code,daily_scope_key,merchant_timezone,source_type_code,capability_code,total_count,difference_count,payment_count,finished_at) VALUES('batch-reconciliation','channel-epay',CURRENT_DATE,repeat('8',64),'differences','channel-epay:test-day','Asia/Shanghai','authoritative_query','authoritative_query',1,1,1,CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_payment_reconciliation_source(source_no,batch_no,record_type_code,local_subject_no,provider_event_reference,external_reference,customer_ref_digest,currency_code,amount_minor,observed_status_code,evidence_digest,observed_at) VALUES('source-reconciliation','batch-reconciliation','payment',?,'event-reconciliation','trade-reconciliation',?,'CNY',4567,'succeeded',repeat('9',64),CURRENT_TIMESTAMP)`, reconciliationAttempt.AttemptNo, reconciliationAttempt.CustomerRefDigest); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_payment_reconciliation_line(line_no,batch_no,attempt_no,external_reference,difference_code,local_amount_minor,channel_amount_minor,currency_code,evidence_digest,record_type_code,customer_ref_digest,local_status_code,channel_status_code) VALUES('line-reconciliation','batch-reconciliation',?,'trade-reconciliation','local_missing',4567,4567,'CNY',repeat('9',64),'payment',?,'submitted','succeeded')`, reconciliationAttempt.AttemptNo, reconciliationAttempt.CustomerRefDigest); err != nil {
		t.Fatal(err)
	}
	reconciliationService, err := reconciliation.New(database, channels, store, paymentReconciliationLoaderFake{})
	if err != nil {
		t.Fatal(err)
	}
	mismatches := []struct {
		name, customer, currency string
		amount                   int64
		digest                   string
	}{
		{name: "ownership", customer: strings.Repeat("0", 64), currency: "CNY", amount: 4567, digest: strings.Repeat("a", 64)},
		{name: "currency", customer: reconciliationAttempt.CustomerRefDigest, currency: "USD", amount: 4567, digest: strings.Repeat("b", 64)},
		{name: "amount", customer: reconciliationAttempt.CustomerRefDigest, currency: "CNY", amount: 4568, digest: strings.Repeat("c", 64)},
	}
	for _, mismatch := range mismatches {
		lineNo, external := "line-mismatch-"+mismatch.name, "trade-mismatch-"+mismatch.name
		if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_payment_reconciliation_source(source_no,batch_no,record_type_code,local_subject_no,provider_event_reference,external_reference,customer_ref_digest,currency_code,amount_minor,observed_status_code,evidence_digest,observed_at) VALUES(?, 'batch-reconciliation','payment',?,?,?,?,?,?,'succeeded',?,CURRENT_TIMESTAMP)`, "source-mismatch-"+mismatch.name, reconciliationAttempt.AttemptNo, "event-mismatch-"+mismatch.name, external, mismatch.customer, mismatch.currency, mismatch.amount, mismatch.digest); err != nil {
			t.Fatal(err)
		}
		if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_payment_reconciliation_line(line_no,batch_no,attempt_no,external_reference,difference_code,local_amount_minor,channel_amount_minor,currency_code,evidence_digest,record_type_code,customer_ref_digest,local_status_code,channel_status_code) VALUES(?,'batch-reconciliation',?,?,'local_missing',4567,?,?,?,'payment',?,'submitted','succeeded')`, lineNo, reconciliationAttempt.AttemptNo, external, mismatch.amount, mismatch.currency, mismatch.digest, mismatch.customer); err != nil {
			t.Fatal(err)
		}
		if invalid, submitErr := reconciliationService.SubmitSupplement(ctx, lineNo, 101, 1); submitErr == nil || invalid != nil {
			t.Fatalf("%s mismatch accepted supplement=%+v err=%v", mismatch.name, invalid, submitErr)
		}
	}
	supplement, err := reconciliationService.SubmitSupplement(ctx, "line-reconciliation", 101, 1)
	if err != nil || supplement == nil || supplement.StatusCode != "pending_review" {
		t.Fatalf("supplement=%+v err=%v", supplement, err)
	}
	if _, err = reconciliationService.ReviewSupplement(ctx, supplement.RequestNo, 101, true, "", supplement.Version); err == nil {
		t.Fatal("supplement submitter reviewed the same request")
	}
	supplement, err = reconciliationService.ReviewSupplement(ctx, supplement.RequestNo, 202, true, "", supplement.Version)
	if err != nil || supplement.StatusCode != "applied" || supplement.ReviewerUserID != 202 {
		t.Fatalf("reviewed supplement=%+v err=%v", supplement, err)
	}
	reconciled, err := database.GetOne(ctx, `SELECT a.status_code,a.external_trade_reference,w.status_code AS work_status FROM plugin_idc_payment_attempt a JOIN plugin_idc_payment_work_item w ON w.subject_no=a.attempt_no AND w.work_type='confirm_payment' WHERE a.attempt_no=?`, reconciliationAttempt.AttemptNo)
	if err != nil || reconciled["status_code"].String() != "confirming" || reconciled["external_trade_reference"].String() != "trade-reconciliation" || reconciled["work_status"].String() != "pending" {
		t.Fatalf("reconciled payment=%v err=%v", reconciled, err)
	}
}

func appendPaymentDSN(dsn, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}
func quotePaymentIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
