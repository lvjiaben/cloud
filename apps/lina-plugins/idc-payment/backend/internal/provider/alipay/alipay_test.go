package alipay

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-payment/backend/cap/paymentcap"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestAlipayPageAndWAPRequestsAreRSA2Signed(t *testing.T) {
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	credentials := paymentcap.Credentials{AppID: "app-1", PrivateKey: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})}
	adapter := New(nil)
	adapter.now = func() time.Time { return time.Date(2026, 8, 19, 12, 0, 0, 0, time.Local) }
	for _, mode := range []paymentcap.Mode{paymentcap.ModeAlipayPage, paymentcap.ModeAlipayWAP} {
		redirect, err := adapter.Create(context.Background(), credentials, paymentcap.CreateRequest{AttemptNo: "attempt-1", InvoiceNo: "invoice-1", Mode: mode, Currency: "CNY", AmountMinor: 10001, Subject: "服务器", NotifyURL: "https://merchant.example/callback", ReturnURL: "https://merchant.example/return", ExpiresAt: time.Now().Add(time.Hour)})
		if err != nil || redirect.Method != "POST" || redirect.URL != Gateway || redirect.Fields["sign_type"] != "RSA2" {
			t.Fatalf("mode=%s redirect=%+v err=%v", mode, redirect, err)
		}
		signature, err := base64.StdEncoding.DecodeString(redirect.Fields["sign"])
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256Sum(canonical(redirect.Fields))
		if err = rsa.VerifyPKCS1v15(&private.PublicKey, cryptoSHA256(), digest, signature); err != nil {
			t.Fatalf("mode=%s signature=%v", mode, err)
		}
	}
}

func TestAlipayCallbackResponseOperationsAndKeyRotation(t *testing.T) {
	merchantPrivate, _ := rsaKeyPair(t)
	oldPlatformPrivate, oldPlatformPublic := rsaKeyPair(t)
	newPlatformPrivate, newPlatformPublic := rsaKeyPair(t)
	credentials := paymentcap.Credentials{AppID: "app-1", MerchantID: "seller-1", PrivateKey: merchantPrivate, PublicKey: oldPlatformPublic}
	adapter := New(nil)
	callback := map[string]string{
		"app_id": "app-1", "seller_id": "seller-1", "trade_status": "TRADE_SUCCESS",
		"notify_id": "notify-1", "trade_no": "trade-1", "out_trade_no": "attempt-1",
		"total_amount": "100.01", "sign_type": "RSA2",
	}
	callback["sign"] = signText(t, oldPlatformPrivate, canonicalCallback(callback))
	verified, err := adapter.VerifyCallback(context.Background(), credentials, paymentcap.CallbackRequest{Values: callback})
	if err != nil || verified.AmountMinor != 10001 || verified.AttemptNo != "attempt-1" {
		t.Fatalf("callback=%+v err=%v", verified, err)
	}
	rotated := credentials
	rotated.PublicKey = newPlatformPublic
	if _, err = adapter.VerifyCallback(context.Background(), rotated, paymentcap.CallbackRequest{Values: callback}); err == nil {
		t.Fatal("callback signed by retired platform key was accepted after rotation")
	}
	callback["sign"] = signText(t, newPlatformPrivate, canonicalCallback(callback))
	if _, err = adapter.VerifyCallback(context.Background(), rotated, paymentcap.CallbackRequest{Values: callback}); err != nil {
		t.Fatalf("callback signed by rotated platform key was rejected: %v", err)
	}

	responses := map[string]map[string]string{
		"alipay.trade.query":                             {"code": "10000", "trade_no": "trade-1", "out_trade_no": "attempt-1", "trade_status": "TRADE_SUCCESS", "total_amount": "100.01"},
		"alipay.trade.close":                             {"code": "10000"},
		"alipay.trade.refund":                            {"code": "10000", "trade_no": "trade-1", "out_trade_no": "attempt-1", "refund_fee": "25.00", "fund_change": "Y"},
		"alipay.trade.fastpay.refund.query":              {"code": "10000", "trade_no": "trade-1", "out_trade_no": "attempt-1", "out_request_no": "refund-1", "refund_amount": "25.00", "refund_status": "REFUND_SUCCESS"},
		"alipay.data.dataservice.bill.downloadurl.query": {"code": "10000", "bill_download_url": "https://download.example/alipay.csv"},
	}
	adapter.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(request.Body)
		values, _ := urlParseQuery(string(raw))
		method := values.Get("method")
		payload, _ := json.Marshal(responses[method])
		signature := signBytes(t, oldPlatformPrivate, payload)
		envelope, _ := json.Marshal(map[string]json.RawMessage{
			strings.ReplaceAll(method, ".", "_") + "_response": payload,
			"sign": json.RawMessage(strconvQuote(signature)),
		})
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(envelope)), Header: make(http.Header)}, nil
	})}
	query, err := adapter.Query(context.Background(), credentials, paymentcap.QueryRequest{AttemptNo: "attempt-1"})
	if err != nil || query.Status != "succeeded" || query.AmountMinor != 10001 {
		t.Fatalf("query=%+v err=%v", query, err)
	}
	if err = adapter.Close(context.Background(), credentials, paymentcap.CloseRequest{AttemptNo: "attempt-1"}); err != nil {
		t.Fatalf("close: %v", err)
	}
	refund, err := adapter.Refund(context.Background(), credentials, paymentcap.RefundRequest{RefundAttemptNo: "refund-1", ExternalTradeReference: "trade-1", Currency: "CNY", AmountMinor: 2500})
	if err != nil || refund.Status != "succeeded" || refund.AmountMinor != 2500 ||
		refund.AttemptNo != "refund-1" || refund.ExternalTradeReference != "refund-1" {
		t.Fatalf("refund=%+v err=%v", refund, err)
	}
	refundQuery, err := adapter.QueryRefund(context.Background(), credentials, paymentcap.RefundQueryRequest{RefundAttemptNo: "refund-1", OriginalTradeReference: "trade-1"})
	if err != nil || refundQuery.Status != "succeeded" || refundQuery.AttemptNo != "refund-1" || refundQuery.AmountMinor != 2500 {
		t.Fatalf("refund query=%+v err=%v", refundQuery, err)
	}
	if _, err = adapter.VerifyRefundCallback(context.Background(), credentials, paymentcap.CallbackRequest{}); !errors.Is(err, paymentcap.ErrUnsupported) {
		t.Fatalf("unexpected Alipay refund callback support: %v", err)
	}
	statement, err := adapter.Statement(context.Background(), credentials, paymentcap.StatementRequest{BusinessDate: time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)})
	if err != nil || statement.DownloadURL != "https://download.example/alipay.csv" {
		t.Fatalf("statement=%+v err=%v", statement, err)
	}
}

func rsaKeyPair(t *testing.T) ([]byte, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}), pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
}

func signText(t *testing.T, privatePEM []byte, value string) string {
	return signBytes(t, privatePEM, []byte(value))
}
func signBytes(t *testing.T, privatePEM, value []byte) string {
	t.Helper()
	key, err := parsePrivateKey(privatePEM)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(value)
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(signature)
}

func urlParseQuery(value string) (url.Values, error) { return url.ParseQuery(value) }
func strconvQuote(value string) []byte               { raw, _ := json.Marshal(value); return raw }

// Tiny wrappers keep the assertion focused while avoiding package-level test state.
func sha256Sum(value string) []byte { sum := sha256.Sum256([]byte(value)); return sum[:] }
func cryptoSHA256() crypto.Hash     { return crypto.SHA256 }
