package unionpay

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-payment/backend/cap/paymentcap"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestUnionPayCreateAndCallbackUseSignedACPFields(t *testing.T) {
	credentials := testCredentials(t)
	adapter, err := New(TestEndpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	adapter.now = func() time.Time { return time.Date(2026, 9, 2, 12, 30, 0, 0, time.UTC) }
	redirect, err := adapter.Create(context.Background(), credentials, paymentcap.CreateRequest{
		AttemptNo: "attempt-unionpay-1", InvoiceNo: "invoice-1", Mode: paymentcap.ModeUnionPayWeb,
		Currency: "CNY", AmountMinor: 1200, Subject: "服务器续费",
		NotifyURL: "https://merchant.example/callback", ReturnURL: "https://merchant.example/return",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil || redirect.URL != TestEndpoint || redirect.Method != http.MethodPost || redirect.Fields["channelType"] != "07" || redirect.Fields["txnAmt"] != "1200" {
		t.Fatalf("redirect=%+v err=%v", redirect, err)
	}
	if err = verify(redirect.Fields, credentials); err != nil {
		t.Fatalf("create signature rejected: %v", err)
	}

	callback := map[string]string{
		"version": "5.1.0", "encoding": "UTF-8", "signMethod": "01",
		"merId": credentials.MerchantID, "respCode": "00", "currencyCode": "156",
		"txnAmt": "1200", "queryId": "union-query-1", "reqReserved": "attempt-unionpay-1",
	}
	if err = sign(callback, credentials); err != nil {
		t.Fatal(err)
	}
	verified, err := adapter.VerifyCallback(context.Background(), credentials, paymentcap.CallbackRequest{Values: callback})
	if err != nil || verified.AttemptNo != "attempt-unionpay-1" || verified.AmountMinor != 1200 || verified.Status != "succeeded" {
		t.Fatalf("verified=%+v err=%v", verified, err)
	}
	callback["txnAmt"] = "1201"
	if _, err = adapter.VerifyCallback(context.Background(), credentials, paymentcap.CallbackRequest{Values: callback}); err == nil {
		t.Fatal("tampered callback was accepted")
	}
}

func TestUnionPayConnectionProbeRequiresSignedNoTransactionResponse(t *testing.T) {
	credentials := testCredentials(t)
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		requestValues, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, err
		}
		responseValues := map[string]string{
			"version": "5.1.0", "encoding": "UTF-8", "signMethod": "01",
			"merId": requestValues.Get("merId"), "respCode": "34", "respMsg": "no transaction",
		}
		if err = sign(responseValues, credentials); err != nil {
			return nil, err
		}
		encoded := url.Values{}
		for key, value := range responseValues {
			encoded.Set(key, value)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(encoded.Encode())), Header: make(http.Header)}, nil
	})}
	adapter, err := New(TestEndpoint, client)
	if err != nil {
		t.Fatal(err)
	}
	adapter.now = func() time.Time { return time.Date(2026, 9, 2, 12, 30, 0, 0, time.UTC) }
	if err = adapter.VerifyConnection(context.Background(), credentials); err != nil {
		t.Fatalf("connection probe failed: %v", err)
	}
}

func TestUnionPayEndpointAllowlistIsExact(t *testing.T) {
	for _, endpoint := range []string{DefaultEndpoint, TestEndpoint} {
		if !AllowedEndpoint(endpoint) {
			t.Fatalf("official endpoint rejected: %s", endpoint)
		}
	}
	for _, endpoint := range []string{
		"http://gateway.95516.com/gateway/api/frontTransReq.do",
		"https://gateway.95516.com.evil.example/gateway/api/frontTransReq.do",
		"https://gateway.95516.com/gateway/api/queryTrans.do",
	} {
		if AllowedEndpoint(endpoint) {
			t.Fatalf("unsafe endpoint accepted: %s", endpoint)
		}
	}
}

func testCredentials(t *testing.T) paymentcap.Credentials {
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
	return paymentcap.Credentials{
		MerchantID: "777290058110097",
		PrivateKey: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}),
		PublicKey:  pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER}),
		Extra:      map[string][]byte{extraCertID: []byte("68759663125")},
	}
}
