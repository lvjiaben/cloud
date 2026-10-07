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
