// This file exercises submission, callback and query races through real payment
// migrations in an isolated schema. Adapters and core funding are local fakes;
// no supplier or existing account is contacted.
package backend

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/billingcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
	"lina-plugin-idc-payment/backend/internal/service/paymentattempt"
	"lina-plugin-idc-payment/backend/internal/service/paymentflow"
)

// submissionBillingFake includes a real balance hold in the prepared contract.
type submissionBillingFake struct{ *paymentBillingFake }

func (f *submissionBillingFake) PrepareChannelPayment(ctx context.Context, input billingcap.PrepareChannelPayment) (*billingcap.PreparedChannelPayment, error) {
	p, err := f.paymentBillingFake.PrepareChannelPayment(ctx, input)
	if err != nil {
		return nil, err
	}
	p.BalanceAmountMinor = input.BalanceAmountMinor
	p.ChannelAmountMinor = p.TotalAmountMinor - input.BalanceAmountMinor
	if input.BalanceAmountMinor > 0 {
		p.BalanceHoldNo = "bhd_submission_test"
	}
	return p, nil
}

// submissionAdapterFake invokes callbacks inside Create to deterministically
// reproduce providers whose notification arrives before the request returns.
type submissionAdapterFake struct {
	paymentRefundAdapterFake
	create   func(context.Context, paymentcap.CreateRequest) (*paymentcap.Redirect, error)
	callback *paymentcap.VerifiedResult
	query    *paymentcap.VerifiedResult
}

func (a *submissionAdapterFake) Capabilities() paymentcap.Capabilities {
	capabilities := a.paymentRefundAdapterFake.Capabilities()
	capabilities.Operations[paymentcap.OperationQuery] = true
	return capabilities
}
func (a *submissionAdapterFake) Create(ctx context.Context, _ paymentcap.Credentials, input paymentcap.CreateRequest) (*paymentcap.Redirect, error) {
	if a.create != nil {
		return a.create(ctx, input)
	}
	return &paymentcap.Redirect{Method: "POST", URL: "https://payment.example.test/pay"}, nil
}
func (a *submissionAdapterFake) VerifyCallback(context.Context, paymentcap.Credentials, paymentcap.CallbackRequest) (*paymentcap.VerifiedResult, error) {
	return a.callback, nil
}
func (a *submissionAdapterFake) Query(context.Context, paymentcap.Credentials, paymentcap.QueryRequest) (*paymentcap.VerifiedResult, error) {
	return a.query, nil
}

func newSubmissionServices(t *testing.T) (gdb.DB, *paymentattempt.Store, *submissionBillingFake, *channelconfig.Service, *submissionAdapterFake, *paymentflow.Service) {
	t.Helper()
	var (
		db      = newPaymentPagesDatabase(t)
		billing = &submissionBillingFake{paymentBillingFake: &paymentBillingFake{}}
		adapter = &submissionAdapterFake{}
	)
	handle, err := secretcap.NewHandle("payment-test-secret-ref")
	if err != nil {
		t.Fatal(err)
	}
	secret := &paymentSecretFake{handle: handle, plaintext: []byte(`{"MerchantID":"test-merchant","secret":"test-secret"}`)}
	channels, err := channelconfig.New(db, secret, secret, []paymentcap.Adapter{adapter}, func(context.Context, paymentcap.ProviderCode, paymentcap.Credentials) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(context.Background(), `INSERT INTO plugin_idc_payment_channel(channel_no,provider_code,display_name,endpoint_origin,merchant_reference,secret_ref,supported_modes,supported_currencies,capability_snapshot,status_code,verified_at) VALUES('submission-channel','alipay','Test channel','https://payment.example.test','test-merchant','payment-test-secret-ref','["alipay_page"]','["CNY"]','{"create":true,"query":true,"refund":true}','enabled',CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	store, err := paymentattempt.New(db, billing)
	if err != nil {
		t.Fatal(err)
	}
	flows, err := paymentflow.New(store, channels, paymentflow.Config{PublicOrigin: "https://merchant.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	return db, store, billing, channels, adapter, flows
}

func submissionResult(attemptNo string, amount int64) paymentcap.VerifiedResult {
	return paymentcap.VerifiedResult{AttemptNo: attemptNo, ProviderEventReference: "event-" + attemptNo, ExternalTradeReference: "trade-" + attemptNo, Status: "succeeded", Currency: "CNY", AmountMinor: amount, EvidenceDigest: strings.Repeat("a", 64)}
}

func TestPaymentSubmissionPreservesFastCallbackPostgreSQL(t *testing.T) {
	for _, confirmBeforeReturn := range []bool{false, true} {
		t.Run(map[bool]string{false: "callback queued", true: "already posted"}[confirmBeforeReturn], func(t *testing.T) {
			db, store, billing, _, adapter, flows := newSubmissionServices(t)
			adapter.create = func(ctx context.Context, input paymentcap.CreateRequest) (*paymentcap.Redirect, error) {
				if input.NotifyURL != "https://merchant.example.test/x/idc-payment/api/v1/callbacks/payments/submission-channel" || input.ReturnURL != "https://merchant.example.test/payment/return/"+input.AttemptNo {
					t.Fatalf("generated public payment URLs do not match actual routes: notify=%q return=%q", input.NotifyURL, input.ReturnURL)
				}
				row, err := db.GetOne(ctx, `SELECT status_code FROM plugin_idc_payment_attempt WHERE attempt_no=?`, input.AttemptNo)
				if err != nil || row["status_code"].String() != "submitted" {
					t.Fatalf("submission was not durable before provider call: %v %v", row, err)
				}
				if _, err = store.RecordVerifiedCallback(ctx, paymentattempt.CallbackInput{ChannelNo: "submission-channel", PayloadDigest: strings.Repeat("b", 64), Result: submissionResult(input.AttemptNo, input.AmountMinor)}); err != nil {
					return nil, err
				}
				if confirmBeforeReturn {
					work, claimErr := store.ClaimConfirmation(ctx, time.Minute)
					if claimErr != nil || work == nil {
						t.Fatalf("confirmation claim: %v", claimErr)
					}
					if err = store.CompleteConfirmation(ctx, *work); err != nil {
						return nil, err
					}
				}
				return &paymentcap.Redirect{Method: "POST", URL: "https://payment.example.test/pay"}, nil
			}
			created, err := flows.Create(context.Background(), paymentflow.CreateInput{PaymentContextNo: "context-test", ChannelNo: "submission-channel", Mode: paymentcap.ModeAlipayPage, BalanceAmountMinor: 900})
			if err != nil || created == nil {
				t.Fatalf("fast callback broke submit: %v", err)
			}
			if !confirmBeforeReturn {
				work, claimErr := store.ClaimConfirmation(context.Background(), time.Minute)
				if claimErr != nil || work == nil {
					t.Fatalf("confirmation claim: %v", claimErr)
				}
				if err = store.CompleteConfirmation(context.Background(), *work); err != nil {
					t.Fatal(err)
				}
			}
			row, err := db.GetOne(context.Background(), `SELECT status_code FROM plugin_idc_payment_attempt WHERE attempt_no=?`, created.AttemptNo)
			if err != nil || row["status_code"].String() != "succeeded" || billing.calls != 1 || billing.releaseCalls != 0 {
				t.Fatalf("posted payment did not converge: %v billing=%d release=%d err=%v", row, billing.calls, billing.releaseCalls, err)
			}
			if err = store.FailAndRelease(context.Background(), created.AttemptNo); err != nil {
				t.Fatal(err)
			}
			if billing.releaseCalls != 0 {
				t.Fatal("success balance hold was released")
			}
		})
	}
}

func TestPaymentSubmissionAmbiguityRetainsHoldPostgreSQL(t *testing.T) {
	cases := []struct {
		name      string
		callback  bool
		redirect  *paymentcap.Redirect
		createErr error
	}{
		{name: "transport loss", createErr: errors.New("accepted request response lost")},
		{name: "missing redirect"},
		{name: "invalid redirect", redirect: &paymentcap.Redirect{Method: "POST", URL: "http://unsafe.example.test"}},
		{name: "callback before transport loss", callback: true, createErr: errors.New("accepted request response lost")},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			db, store, billing, _, adapter, flows := newSubmissionServices(t)
			var attemptNo string
			adapter.create = func(ctx context.Context, input paymentcap.CreateRequest) (*paymentcap.Redirect, error) {
				attemptNo = input.AttemptNo
				if test.callback {
					if _, err := store.RecordVerifiedCallback(ctx, paymentattempt.CallbackInput{ChannelNo: "submission-channel", PayloadDigest: strings.Repeat("b", 64), Result: submissionResult(input.AttemptNo, input.AmountMinor)}); err != nil {
						return nil, err
					}
				}
				return test.redirect, test.createErr
			}
			if result, err := flows.Create(context.Background(), paymentflow.CreateInput{PaymentContextNo: "context-test", ChannelNo: "submission-channel", Mode: paymentcap.ModeAlipayPage, BalanceAmountMinor: 900}); err == nil || result != nil {
				t.Fatal("unsafe or ambiguous result was accepted")
			}
			row, err := db.GetOne(context.Background(), `SELECT status_code,result_unknown FROM plugin_idc_payment_attempt WHERE attempt_no=?`, attemptNo)
			if err != nil || billing.releaseCalls != 0 {
				t.Fatalf("ambiguous submission released a hold: %v release=%d", err, billing.releaseCalls)
			}
			if test.callback {
				if row["status_code"].String() != "confirming" {
					t.Fatalf("verified success was downgraded: %v", row)
				}
				if err = store.FailAndRelease(context.Background(), attemptNo); err != nil {
					t.Fatal(err)
				}
				if billing.releaseCalls != 0 {
					t.Fatal("confirmed callback hold was released")
				}
			} else {
				if row["status_code"].String() != "manual_review" || !row["result_unknown"].Bool() {
					t.Fatalf("unknown outcome was lost: %v", row)
				}
				work, claimErr := store.ClaimQuery(context.Background(), time.Minute)
				if claimErr != nil || work == nil || work.AttemptNo != attemptNo {
					t.Fatalf("verified query recovery was not queued: %v", claimErr)
				}
				if err = store.CompleteQuery(context.Background(), *work, paymentcap.VerifiedResult{AttemptNo: attemptNo, Status: "closed", EvidenceDigest: strings.Repeat("c", 64)}); err != nil {
					t.Fatal(err)
				}
				if billing.releaseCalls != 1 {
					t.Fatal("authoritative closure did not release the retained hold")
				}
			}
		})
	}
}

func TestPaymentQueryLeaseFencesAllEffectsPostgreSQL(t *testing.T) {
	for _, test := range []struct {
		name          string
		status        string
		expire        bool
		forgedSubject bool
	}{
		{name: "wrong token closed", status: "closed"}, {name: "wrong token success", status: "succeeded"},
		{name: "expired token", status: "succeeded", expire: true}, {name: "wrong subject", status: "closed", forgedSubject: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, store, billing, _, _, flows := newSubmissionServices(t)
			created, err := flows.Create(context.Background(), paymentflow.CreateInput{PaymentContextNo: "context-test", ChannelNo: "submission-channel", Mode: paymentcap.ModeAlipayPage, BalanceAmountMinor: 900})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = store.ObserveReturn(context.Background(), created.AttemptNo, billingcap.PrincipalReferenceDigest(101)); err != nil {
				t.Fatal(err)
			}
			work, err := store.ClaimQuery(context.Background(), time.Minute)
			if err != nil || work == nil {
				t.Fatalf("query claim: %v", err)
			}
			if test.expire {
				if _, err = db.Exec(context.Background(), `UPDATE plugin_idc_payment_work_item SET lease_expires_at=CURRENT_TIMESTAMP-INTERVAL '1 second' WHERE work_no=?`, work.WorkNo); err != nil {
					t.Fatal(err)
				}
			} else if test.forgedSubject {
				work.AttemptNo = "foreign-attempt"
			} else {
				work.LeaseToken = "wrong-token"
			}
			result := submissionResult(work.AttemptNo, 11445)
			result.Status = test.status
			if err = store.CompleteQuery(context.Background(), *work, result); err == nil {
				t.Fatal("invalid lease was accepted")
			}
			row, err := db.GetOne(context.Background(), `SELECT status_code,(SELECT COUNT(*) FROM plugin_idc_payment_query_observation) AS observations,(SELECT COUNT(*) FROM plugin_idc_payment_callback_receipt) AS receipts,(SELECT COUNT(*) FROM plugin_idc_payment_work_item WHERE work_type='confirm_payment') AS confirmations FROM plugin_idc_payment_attempt WHERE attempt_no=?`, created.AttemptNo)
			if err != nil || row["status_code"].String() != "submitted" || row["observations"].Int() != 0 || row["receipts"].Int() != 0 || row["confirmations"].Int() != 0 || billing.releaseCalls != 0 {
				t.Fatalf("stale lease changed money or evidence: %v release=%d err=%v", row, billing.releaseCalls, err)
			}
		})
	}
}

func TestDisabledPaymentChannelSettlesExistingTransactionsPostgreSQL(t *testing.T) {
	db, store, billing, channels, adapter, flows := newSubmissionServices(t)
	ctx := context.Background()
	created, err := flows.Create(ctx, paymentflow.CreateInput{PaymentContextNo: "context-test", ChannelNo: "submission-channel", Mode: paymentcap.ModeAlipayPage, BalanceAmountMinor: 900})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.ObserveReturn(ctx, created.AttemptNo, billingcap.PrincipalReferenceDigest(101)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_payment_channel SET status_code='disabled' WHERE channel_no='submission-channel'`); err != nil {
		t.Fatal(err)
	}
	invoked := false
	if err = channels.WithCreateCredentials(ctx, "submission-channel", func(paymentcap.Adapter, paymentcap.Credentials) error { invoked = true; return nil }); err == nil || invoked {
		t.Fatal("disabled channel accepted new payment")
	}
	if result, createErr := flows.Create(ctx, paymentflow.CreateInput{PaymentContextNo: "new-context", ChannelNo: "submission-channel", Mode: paymentcap.ModeAlipayPage}); createErr == nil || result != nil {
		t.Fatal("disabled channel appeared available for new checkout")
	}
	callback := submissionResult(created.AttemptNo, 11445)
	adapter.callback = &callback
	if outcome, _, callbackErr := flows.Callback(ctx, "submission-channel", paymentcap.CallbackRequest{Body: []byte("synthetic verified callback")}); callbackErr != nil || outcome == nil || outcome.Status != "queued" {
		t.Fatalf("disabled channel rejected existing callback: %v", callbackErr)
	}
	closed := callback
	closed.Status = "closed"
	closed.EvidenceDigest = strings.Repeat("c", 64)
	adapter.query = &closed
	if ran, queryErr := flows.RunOneQuery(ctx); queryErr != nil || !ran {
		t.Fatalf("disabled channel rejected existing query: %v", queryErr)
	}
	row, err := db.GetOne(ctx, `SELECT status_code FROM plugin_idc_payment_attempt WHERE attempt_no=?`, created.AttemptNo)
	if err != nil || row["status_code"].String() != "confirming" || billing.releaseCalls != 0 {
		t.Fatal("query closure downgraded authoritative callback", row, err)
	}
	work, err := store.ClaimConfirmation(ctx, time.Minute)
	if err != nil || work == nil {
		t.Fatalf("confirmation claim: %v", err)
	}
	if err = store.CompleteConfirmation(ctx, *work); err != nil {
		t.Fatal(err)
	}
	if err = channels.WithCredentials(ctx, "submission-channel", func(a paymentcap.Adapter, credentials paymentcap.Credentials) error {
		_, refundErr := a.Refund(ctx, credentials, paymentcap.RefundRequest{RefundAttemptNo: "test-refund", Currency: "CNY", AmountMinor: 100})
		return refundErr
	}); err != nil || adapter.refundCalls != 1 {
		t.Fatal("disabled channel rejected existing refund", err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_payment_channel SET status_code='deleted',deleted_at=CURRENT_TIMESTAMP WHERE channel_no='submission-channel'`); err != nil {
		t.Fatal(err)
	}
	if err = channels.WithCredentials(ctx, "submission-channel", func(paymentcap.Adapter, paymentcap.Credentials) error { return nil }); err == nil {
		t.Fatal("deleted channel allowed credential use")
	}
}

func TestPaymentUnknownSubmissionQueryRecoversSuccessPostgreSQL(t *testing.T) {
	db, store, billing, _, adapter, flows := newSubmissionServices(t)
	ctx := context.Background()
	var attemptNo string
	adapter.create = func(_ context.Context, input paymentcap.CreateRequest) (*paymentcap.Redirect, error) {
		attemptNo = input.AttemptNo
		return nil, errors.New("response lost after acceptance")
	}
	if _, err := flows.Create(ctx, paymentflow.CreateInput{PaymentContextNo: "context-test", ChannelNo: "submission-channel", Mode: paymentcap.ModeAlipayPage, BalanceAmountMinor: 900}); err == nil {
		t.Fatal("expected provider response loss")
	}
	success := submissionResult(attemptNo, 11445)
	adapter.query = &success
	if ran, err := flows.RunOneQuery(ctx); err != nil || !ran {
		t.Fatalf("authoritative query did not recover unknown submission: %v", err)
	}
	work, err := store.ClaimConfirmation(ctx, time.Minute)
	if err != nil || work == nil {
		t.Fatalf("query did not queue confirmation: %v", err)
	}
	if err = store.CompleteConfirmation(ctx, *work); err != nil {
		t.Fatal(err)
	}
	row, err := db.GetOne(ctx, `SELECT status_code,result_unknown FROM plugin_idc_payment_attempt WHERE attempt_no=?`, attemptNo)
	if err != nil || row["status_code"].String() != "succeeded" || row["result_unknown"].Bool() || billing.calls != 1 || billing.releaseCalls != 0 {
		t.Fatalf("unknown submission did not converge exactly once: %v billing=%d release=%d err=%v", row, billing.calls, billing.releaseCalls, err)
	}
}

func TestPaymentUnknownSubmissionDoesNotInventQueryCapabilityPostgreSQL(t *testing.T) {
	db, _, billing, _, adapter, flows := newSubmissionServices(t)
	ctx := context.Background()
	if _, err := db.Exec(ctx, `UPDATE plugin_idc_payment_channel SET capability_snapshot='{"create":true}' WHERE channel_no='submission-channel'`); err != nil {
		t.Fatal(err)
	}
	adapter.create = func(context.Context, paymentcap.CreateRequest) (*paymentcap.Redirect, error) {
		return nil, errors.New("response lost")
	}
	if _, err := flows.Create(ctx, paymentflow.CreateInput{PaymentContextNo: "context-test", ChannelNo: "submission-channel", Mode: paymentcap.ModeAlipayPage, BalanceAmountMinor: 900}); err == nil {
		t.Fatal("expected provider response loss")
	}
	row, err := db.GetOne(ctx, `SELECT status_code,result_unknown,(SELECT COUNT(*) FROM plugin_idc_payment_work_item WHERE work_type='query_payment') AS queries FROM plugin_idc_payment_attempt`)
	if err != nil || row["status_code"].String() != "manual_review" || !row["result_unknown"].Bool() || row["queries"].Int() != 0 || billing.releaseCalls != 0 {
		t.Fatalf("unverified query capability was inferred: %v err=%v", row, err)
	}
}

// TestPaymentReturnProjectsOnlyOwnedOrderPostgreSQL proves the return page
// receives its exact local navigation target, without disclosing another payer.
func TestPaymentReturnProjectsOnlyOwnedOrderPostgreSQL(t *testing.T) {
	_, store, _, _, _, flows := newSubmissionServices(t)
	ctx := context.Background()
	created, err := flows.Create(ctx, paymentflow.CreateInput{PaymentContextNo: "context-test", ChannelNo: "submission-channel", Mode: paymentcap.ModeAlipayPage})
	if err != nil {
		t.Fatal(err)
	}
	status, err := store.ObserveReturn(ctx, created.AttemptNo, billingcap.PrincipalReferenceDigest(101))
	if err != nil || status.OrderNo != "order-1" || status.InvoiceNo != "invoice-1" || status.StatusCode == "succeeded" {
		t.Fatalf("return projection did not preserve unconfirmed owned order: %+v %v", status, err)
	}
	if other, err := store.ObserveReturn(ctx, created.AttemptNo, billingcap.PrincipalReferenceDigest(102)); err == nil || other != nil {
		t.Fatal("return exposed another customer's order")
	}
}
