package smsbao

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
)

type accessorStub struct{ payload []byte }

func (stub accessorStub) Access(_ context.Context, request secretcap.AccessRequest, consume secretcap.Consumer) error {
	if err := secretcap.ValidateAccess(request, consume); err != nil {
		return err
	}
	value := append([]byte(nil), stub.payload...)
	defer clear(value)
	return consume(value)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestDeliverUsesSMSBaoSecureDomesticContract(t *testing.T) {
	credential, _ := json.Marshal(map[string][]byte{usernameField: []byte("merchant"), apiKeyField: []byte("safe-api-key")})
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.URL.Scheme != "https" || request.URL.Host != "api.smsbao.com" || request.URL.Path != "/sms" {
			t.Fatalf("unexpected SMSBao endpoint: %s %s", request.Method, request.URL)
		}
		query := request.URL.Query()
		if query.Get("u") != "merchant" || query.Get("p") != "safe-api-key" || query.Get("m") != "13800138000" ||
			query.Get("g") != "10001" || query.Get("c") != "【IDC通知】验证码 123456，10 分钟内有效。" {
			t.Fatalf("unexpected SMSBao request: %v", query)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("0"))}, nil
	})}
	adapter, err := newAdapter(accessorStub{payload: credential}, client)
	if err != nil {
		t.Fatalf("construct SMSBao adapter: %v", err)
	}
	handle, _ := secretcap.NewHandle("opaque-smsbao-test-handle")
	result, err := adapter.Deliver(t.Context(), channelcap.Delivery{
		TaskNo: "ntask_1", Credential: handle, Destination: []byte("+8613800138000"),
		NonSecretConfig: map[string]string{signNameField: "IDC通知", productIDField: "10001"},
		Body:            "验证码 123456，10 分钟内有效。",
	})
	if err != nil || result.Class != channelcap.ResultAccepted || !result.CrossedExternalBorder {
		t.Fatalf("unexpected SMSBao result: result=%+v err=%v", result, err)
	}
}

func TestSMSBaoFailureClassificationIsFailClosed(t *testing.T) {
	tests := map[string]channelcap.ResultClass{
		"41": channelcap.ResultInsufficientBalance,
		"51": channelcap.ResultInvalidRecipient,
		"30": channelcap.ResultConfigurationError,
		"99": channelcap.ResultUnknown,
	}
	for code, expected := range tests {
		client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(code))}, nil
		})}
		credential, _ := json.Marshal(map[string][]byte{usernameField: []byte("merchant"), apiKeyField: []byte("safe-api-key")})
		adapter, _ := newAdapter(accessorStub{payload: credential}, client)
		handle, _ := secretcap.NewHandle("opaque-smsbao-test-handle")
		result, _ := adapter.Deliver(t.Context(), channelcap.Delivery{TaskNo: "ntask_1", Credential: handle,
			Destination: []byte("+8613800138000"), NonSecretConfig: map[string]string{signNameField: "IDC通知"}, Body: "内容"})
		if result.Class != expected || !result.CrossedExternalBorder || len(result.EvidenceDigest) != 64 {
			t.Fatalf("unexpected classification for %s: %+v", code, result)
		}
	}
}
