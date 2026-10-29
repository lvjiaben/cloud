package epay

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-payment/backend/cap/paymentcap"
)

func TestEPayCreatesDocumentedPOSTAndVerifiesOnlySignedSuccess(t *testing.T) {
	adapter, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	credentials := paymentcap.Credentials{MerchantID: "1001", Secret: []byte("merchant-key")}
	redirect, err := adapter.Create(context.Background(), credentials, paymentcap.CreateRequest{
		AttemptNo: "pay_attempt_0001", InvoiceNo: "invoice_0001", Mode: paymentcap.ModeEPayCashier,
		Currency: "CNY", AmountMinor: 12345, Subject: strings.Repeat("服务器套餐", 30),
		NotifyURL: "https://merchant.example/callbacks/epay", ReturnURL: "https://merchant.example/account/payments/return",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil || redirect.Method != "POST" || redirect.URL != DefaultEndpoint || redirect.Fields["money"] != "123.45" || len([]byte(redirect.Fields["name"])) > 127 || redirect.Fields["sign"] == "" {
		t.Fatalf("unexpected EPay redirect=%+v err=%v", redirect, err)
	}
	callback := map[string]string{
		"pid": "1001", "trade_no": "channel-trade-1", "out_trade_no": "pay_attempt_0001",
		"type": "alipay", "name": "服务器套餐", "money": "123.45", "trade_status": "TRADE_SUCCESS",
		"param": "invoice_0001", "sign_type": "MD5",
	}
	callback["sign"] = sign(callback, credentials.Secret)
	verified, err := adapter.VerifyCallback(context.Background(), credentials, paymentcap.CallbackRequest{Values: callback})
	if err != nil || verified.AttemptNo != "pay_attempt_0001" || verified.AmountMinor != 12345 || len(verified.EvidenceDigest) != 64 {
		t.Fatalf("unexpected EPay callback=%+v err=%v", verified, err)
	}
	callback["money"] = "123.46"
	if _, err = adapter.VerifyCallback(context.Background(), credentials, paymentcap.CallbackRequest{Values: callback}); err == nil {
		t.Fatal("changed EPay callback passed the original signature")
	}
}

func TestEPayKeepsUndocumentedOperationsClosed(t *testing.T) {
	adapter, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = adapter.Query(context.Background(), paymentcap.Credentials{}, paymentcap.QueryRequest{}); !errors.Is(err, paymentcap.ErrUnsupported) {
		t.Fatalf("query=%v", err)
	}
	if err = adapter.Close(context.Background(), paymentcap.Credentials{}, paymentcap.CloseRequest{}); !errors.Is(err, paymentcap.ErrUnsupported) {
		t.Fatalf("close=%v", err)
	}
	if _, err = adapter.Refund(context.Background(), paymentcap.Credentials{}, paymentcap.RefundRequest{}); !errors.Is(err, paymentcap.ErrUnsupported) {
		t.Fatalf("refund=%v", err)
	}
	if _, err = adapter.VerifyRefundCallback(context.Background(), paymentcap.Credentials{}, paymentcap.CallbackRequest{}); !errors.Is(err, paymentcap.ErrUnsupported) {
		t.Fatalf("refund callback=%v", err)
	}
	if _, err = adapter.QueryRefund(context.Background(), paymentcap.Credentials{}, paymentcap.RefundQueryRequest{}); !errors.Is(err, paymentcap.ErrUnsupported) {
		t.Fatalf("refund query=%v", err)
	}
	if _, err = adapter.Statement(context.Background(), paymentcap.Credentials{}, paymentcap.StatementRequest{}); !errors.Is(err, paymentcap.ErrUnsupported) {
		t.Fatalf("statement=%v", err)
	}
	capabilities := adapter.Capabilities()
	if capabilities.Supports(paymentcap.ModeEPayCashier, "USD", paymentcap.OperationCreate) ||
		capabilities.Operations[paymentcap.OperationRefund] || capabilities.Operations[paymentcap.OperationRefundQuery] ||
		capabilities.Operations[paymentcap.OperationRefundCallback] {
		t.Fatalf("unsupported capability became enabled: %+v", capabilities)
	}
}

// TestEPayUsesConfiguredLoopbackBrowserReturn verifies the real adapter consumes
// the same development return URL accepted by merchant address settings.
func TestEPayUsesConfiguredLoopbackBrowserReturn(t *testing.T) {
	adapter, err := New("https://epay.example/submit.php")
	if err != nil {
		t.Fatal(err)
	}
	input := paymentcap.CreateRequest{AttemptNo: "PAT_loopback", InvoiceNo: "INV_loopback", Mode: paymentcap.ModeEPayCashier, Currency: "CNY", AmountMinor: 100, Subject: "isolated payment", NotifyURL: "https://baidu.com/x/idc-payment/api/v1/callbacks/payments/EPAY_MAIN", ReturnURL: "http://127.0.0.1:6032/payment/return/PAT_loopback", ExpiresAt: time.Now().Add(time.Hour)}
	redirect, err := adapter.Create(context.Background(), paymentcap.Credentials{MerchantID: "1108", Secret: []byte("isolated-test-secret")}, input)
	if err != nil || redirect.Fields["return_url"] != input.ReturnURL || redirect.Fields["notify_url"] != input.NotifyURL || redirect.Fields["money"] != "1.00" || redirect.Fields["sign"] == "" {
		t.Fatalf("configured browser return failed: err=%v", err)
	}
}

// TestEPayDirectMethodsIncludeTypeInSignature covers providers without a
// standalone cashier.php: explicit methods remain normal signed POST requests.
func TestEPayDirectMethodsIncludeTypeInSignature(t *testing.T) {
	adapter, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	for mode, kind := range map[paymentcap.Mode]string{paymentcap.ModeEPayAlipay: "alipay", paymentcap.ModeEPayWechat: "wxpay"} {
		t.Run(string(mode), func(t *testing.T) {
			credentials := paymentcap.Credentials{MerchantID: "1108", Secret: []byte("isolated-test-secret")}
			result, createErr := adapter.Create(context.Background(), credentials, paymentcap.CreateRequest{AttemptNo: "PAT_direct", InvoiceNo: "INV_direct", Mode: mode, Currency: "CNY", AmountMinor: 100, Subject: "isolated payment", NotifyURL: "https://notify.example/callback", ReturnURL: "http://127.0.0.1:6032/payment/return/PAT_direct", ExpiresAt: time.Now().Add(time.Hour)})
			if createErr != nil || result.Fields["type"] != kind || result.Fields["sign"] != sign(result.Fields, credentials.Secret) {
				t.Fatalf("direct EPay request failed: %v", createErr)
			}
			result.Fields["type"] = "changed"
			if result.Fields["sign"] == sign(result.Fields, credentials.Secret) {
				t.Fatal("payment type not covered by signature")
			}
		})
	}
}
