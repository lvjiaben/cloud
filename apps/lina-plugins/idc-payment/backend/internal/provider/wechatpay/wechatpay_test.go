package wechatpay

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wechatpay-apiv3/wechatpay-go/utils"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func TestWechatPayCapabilityMatrixKeepsProductsAndCurrencyExact(t *testing.T) {
	capabilities := New(nil).Capabilities()
	for _, mode := range []paymentcap.Mode{paymentcap.ModeWechatNative, paymentcap.ModeWechatH5, paymentcap.ModeWechatJSAPI} {
		if !capabilities.Supports(mode, "CNY", paymentcap.OperationCreate) {
			t.Fatalf("mode %s missing", mode)
		}
	}
	if capabilities.Supports(paymentcap.ModeWechatH5, "USD", paymentcap.OperationCreate) {
		t.Fatal("wechat accepted unsupported currency")
	}
}
func TestWechatPayRejectsIncompleteMerchantCredentialsBeforeNetwork(t *testing.T) {
	adapter := New(nil)
	if _, _, err := adapter.client(t.Context(), paymentcap.Credentials{}); err == nil {
		t.Fatal("missing credentials accepted")
	}
	if _, err := adapter.Refund(t.Context(), paymentcap.Credentials{}, paymentcap.RefundRequest{AmountMinor: 101, TotalAmountMinor: 100, Currency: "CNY"}); err == nil || errors.Is(err, paymentcap.ErrUnsupported) {
		t.Fatalf("invalid refund=%v", err)
	}
}

func TestWechatPayOfficialProductsOperationsCallbackAndKeyRotation(t *testing.T) {
	merchantPrivate, _ := rsaKeyPair(t)
	platformPrivate, platformPublic := rsaKeyPair(t)
	apiKey := []byte("0123456789abcdef0123456789abcdef")
	credentials := paymentcap.Credentials{
		MerchantID: "1900000109", AppID: "wx-app-1", PrivateKey: merchantPrivate,
		PublicKey: platformPublic,
		Extra: map[string][]byte{
			extraCertificateSerial: []byte("MERCHANTSERIAL1"), extraPublicKeyID: []byte("PUB_KEY_ID_0114237487"), extraAPIV3Key: apiKey,
		},
	}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		path := request.URL.Path
		status := http.StatusOK
		var body string
		switch {
		case strings.HasSuffix(path, "/transactions/native"):
			body = `{"code_url":"weixin://wxpay/bizpayurl?pr=test"}`
		case strings.HasSuffix(path, "/transactions/h5"):
			body = `{"h5_url":"https://wx.tenpay.com/cgi-bin/mmpayweb-bin/checkmweb?prepay_id=test"}`
		case strings.HasSuffix(path, "/transactions/jsapi"):
			body = `{"prepay_id":"wx-prepay-1"}`
		case strings.Contains(path, "/transactions/out-trade-no/") && request.Method == http.MethodGet:
			body = `{"appid":"wx-app-1","mchid":"1900000109","out_trade_no":"attempt-1","transaction_id":"wx-trade-1","trade_state":"SUCCESS","amount":{"total":10001,"payer_total":10001,"currency":"CNY","payer_currency":"CNY"}}`
		case strings.Contains(path, "/transactions/out-trade-no/") && request.Method == http.MethodPost:
			status = http.StatusNoContent
		case strings.HasSuffix(path, "/refund/domestic/refunds"):
			body = `{"refund_id":"wx-refund-1","out_refund_no":"refund-1","transaction_id":"wx-trade-1","out_trade_no":"attempt-1","status":"SUCCESS","amount":{"refund":2500,"total":10001,"currency":"CNY"}}`
		case strings.Contains(path, "/refund/domestic/refunds/refund-1") && request.Method == http.MethodGet:
			body = `{"refund_id":"wx-refund-1","out_refund_no":"refund-1","transaction_id":"wx-trade-1","out_trade_no":"attempt-1","status":"SUCCESS","amount":{"refund":2500,"total":10001,"currency":"CNY"}}`
		case strings.HasSuffix(path, "/bill/tradebill"):
			body = `{"hash_type":"SHA1","hash_value":"abc","download_url":"https://api.mch.weixin.qq.com/v3/billdownload/file"}`
		default:
			return nil, errors.New("unexpected WeChat Pay request: " + request.Method + " " + request.URL.String())
		}
		return signedResponse(t, status, body, "PUB_KEY_ID_0114237487", platformPrivate), nil
	})}
	adapter := New(client)
	base := paymentcap.CreateRequest{AttemptNo: "attempt-1", InvoiceNo: "invoice-1", Currency: "CNY", AmountMinor: 10001, Subject: "服务器", NotifyURL: "https://merchant.example/payment/callback", ReturnURL: "https://merchant.example/payment/return", ClientIP: "203.0.113.10", OpenID: "openid-1", ExpiresAt: time.Now().Add(time.Hour)}
	for _, expected := range []struct {
		mode   paymentcap.Mode
		method string
	}{{paymentcap.ModeWechatNative, "QR"}, {paymentcap.ModeWechatH5, "GET"}, {paymentcap.ModeWechatJSAPI, "JSAPI"}} {
		input := base
		input.Mode = expected.mode
		redirect, err := adapter.Create(context.Background(), credentials, input)
		if err != nil || redirect.Method != expected.method {
			t.Fatalf("mode=%s redirect=%+v err=%v", expected.mode, redirect, err)
		}
	}
	query, err := adapter.Query(context.Background(), credentials, paymentcap.QueryRequest{AttemptNo: "attempt-1"})
	if err != nil || query.Status != "succeeded" || query.AmountMinor != 10001 {
		t.Fatalf("query=%+v err=%v", query, err)
	}
	if err = adapter.Close(context.Background(), credentials, paymentcap.CloseRequest{AttemptNo: "attempt-1"}); err != nil {
		t.Fatalf("close: %v", err)
	}
	refund, err := adapter.Refund(context.Background(), credentials, paymentcap.RefundRequest{RefundAttemptNo: "refund-1", ExternalTradeReference: "wx-trade-1", Currency: "CNY", AmountMinor: 2500, TotalAmountMinor: 10001, NotifyURL: "https://merchant.example/refund/callback"})
	if err != nil || refund.Status != "succeeded" || refund.AmountMinor != 2500 {
		t.Fatalf("refund=%+v err=%v", refund, err)
	}
	refundQuery, err := adapter.QueryRefund(context.Background(), credentials, paymentcap.RefundQueryRequest{RefundAttemptNo: "refund-1"})
	if err != nil || refundQuery.Status != "succeeded" || refundQuery.ExternalTradeReference != "wx-refund-1" {
		t.Fatalf("refund query=%+v err=%v", refundQuery, err)
	}
	statement, err := adapter.Statement(context.Background(), credentials, paymentcap.StatementRequest{BusinessDate: time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)})
	if err != nil || statement.HashType != "SHA1" || statement.DownloadURL == "" {
		t.Fatalf("statement=%+v err=%v", statement, err)
	}

	body := encryptedNotification(t, apiKey)
	headers := signedHeaders(t, body, "PUB_KEY_ID_0114237487", platformPrivate)
	callbackHeaders := map[string]string{}
	for key, values := range headers {
		if len(values) > 0 {
			callbackHeaders[key] = values[0]
		}
	}
	callback, err := adapter.VerifyCallback(context.Background(), credentials, paymentcap.CallbackRequest{Body: []byte(body), Headers: callbackHeaders})
	if err != nil || callback.AttemptNo != "attempt-1" || callback.AmountMinor != 10001 {
		t.Fatalf("callback=%+v err=%v", callback, err)
	}
	_, rotatedPublic := rsaKeyPair(t)
	rotated := credentials
	rotated.PublicKey = rotatedPublic
	if _, err = adapter.VerifyCallback(context.Background(), rotated, paymentcap.CallbackRequest{Body: []byte(body), Headers: callbackHeaders}); err == nil {
		t.Fatal("callback signed by retired platform key was accepted after rotation")
	}
	refundBody := encryptedRefundNotification(t, apiKey)
	refundHeaders := signedHeaders(t, refundBody, "PUB_KEY_ID_0114237487", platformPrivate)
	refundCallbackHeaders := map[string]string{}
	for key, values := range refundHeaders {
		if len(values) > 0 {
			refundCallbackHeaders[key] = values[0]
		}
	}
	refundCallback, err := adapter.VerifyRefundCallback(context.Background(), credentials, paymentcap.CallbackRequest{Body: []byte(refundBody), Headers: refundCallbackHeaders})
	if err != nil || refundCallback.AttemptNo != "refund-1" || refundCallback.Status != "succeeded" || refundCallback.AmountMinor != 2500 {
		t.Fatalf("refund callback=%+v err=%v", refundCallback, err)
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

func signedResponse(t *testing.T, status int, body, serial string, privatePEM []byte) *http.Response {
	t.Helper()
	headers := signedHeaders(t, body, serial, privatePEM)
	return &http.Response{StatusCode: status, Header: headers, Body: io.NopCloser(strings.NewReader(body))}
}

func signedHeaders(t *testing.T, body, serial string, privatePEM []byte) http.Header {
	t.Helper()
	timestamp := time.Now().Unix()
	nonce := "payment-contract-nonce"
	source := strings.Join([]string{strconv.FormatInt(timestamp, 10), nonce, body, ""}, "\n")
	key, err := utils.LoadPrivateKey(string(privatePEM))
	if err != nil {
		t.Fatal(err)
	}
	signature, err := utils.SignSHA256WithRSA(source, key)
	if err != nil {
		t.Fatal(err)
	}
	return http.Header{"Wechatpay-Timestamp": []string{strconv.FormatInt(timestamp, 10)}, "Wechatpay-Nonce": []string{nonce}, "Wechatpay-Serial": []string{serial}, "Wechatpay-Signature": []string{signature}}
}

func encryptedNotification(t *testing.T, apiKey []byte) string {
	t.Helper()
	plaintext := []byte(`{"appid":"wx-app-1","mchid":"1900000109","out_trade_no":"attempt-1","transaction_id":"wx-trade-1","trade_state":"SUCCESS","amount":{"total":10001,"payer_total":10001,"currency":"CNY","payer_currency":"CNY"}}`)
	block, err := aes.NewCipher(apiKey)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := []byte("notify-nonce")
	associated := []byte("transaction")
	ciphertext := gcm.Seal(nil, nonce, plaintext, associated)
	resource := map[string]string{"algorithm": "AEAD_AES_256_GCM", "ciphertext": base64.StdEncoding.EncodeToString(ciphertext), "associated_data": string(associated), "nonce": string(nonce)}
	encoded, err := json.Marshal(map[string]any{"id": "event-1", "create_time": "2026-08-19T12:00:00+08:00", "event_type": "TRANSACTION.SUCCESS", "resource_type": "encrypt-resource", "resource": resource, "summary": "success"})
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func encryptedRefundNotification(t *testing.T, apiKey []byte) string {
	t.Helper()
	plaintext := []byte(`{"refund_id":"wx-refund-1","out_refund_no":"refund-1","transaction_id":"wx-trade-1","out_trade_no":"attempt-1","channel":"ORIGINAL","user_received_account":"零钱","create_time":"2026-08-19T12:00:00+08:00","success_time":"2026-08-19T12:01:00+08:00","status":"SUCCESS","amount":{"refund":2500,"total":10001,"currency":"CNY"}}`)
	block, err := aes.NewCipher(apiKey)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := []byte("refund-nonce")
	associated := []byte("refund")
	ciphertext := gcm.Seal(nil, nonce, plaintext, associated)
	resource := map[string]string{"algorithm": "AEAD_AES_256_GCM", "ciphertext": base64.StdEncoding.EncodeToString(ciphertext), "associated_data": string(associated), "nonce": string(nonce)}
	encoded, err := json.Marshal(map[string]any{"id": "refund-event-1", "create_time": "2026-08-19T12:01:00+08:00", "event_type": "REFUND.SUCCESS", "resource_type": "encrypt-resource", "resource": resource, "summary": "success"})
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
