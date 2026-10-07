package aliyunsms

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

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

func TestDeliverUsesAliyunSendSMSContract(t *testing.T) {
	credential, _ := json.Marshal(map[string][]byte{
		accessKeyIDField: []byte("LTAItest12345678"), accessKeySecretField: []byte("0123456789abcdef0123456789abcdef"),
	})
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Scheme != "https" || request.URL.Host != host || request.Host != host {
			t.Fatalf("unexpected Aliyun endpoint: %s %s host=%s", request.Method, request.URL, request.Host)
		}
		query := request.URL.Query()
		if query.Get("PhoneNumbers") != "+8613800138000" || query.Get("SignName") != "IDC通知" ||
			query.Get("TemplateCode") != "SMS_123456" || query.Get("OutId") != "ntask_1" ||
			query.Get("TemplateParam") != `{"code":"123456","minutes":"10"}` {
			t.Fatalf("Aliyun request does not preserve the frozen mapping: %v", query)
		}
		if request.Header.Get("X-Acs-Action") != action || request.Header.Get("X-Acs-Version") != apiVersion ||
			request.Header.Get("X-Acs-Signature-Nonce") != "fixed-nonce" ||
			!strings.HasPrefix(request.Header.Get("Authorization"), "ACS3-HMAC-SHA256 Credential=LTAItest12345678,") {
			t.Fatalf("Aliyun authentication headers are incomplete: %+v", request.Header)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{"Code":"OK","BizId":"biz-1","RequestId":"req-1"}`))}, nil
	})}
	adapter, err := newAdapter(accessorStub{payload: credential}, client,
		func() time.Time { return time.Date(2026, time.August, 16, 8, 0, 0, 0, time.UTC) },
		func() (string, error) { return "fixed-nonce", nil })
	if err != nil {
		t.Fatalf("construct Aliyun SMS adapter: %v", err)
	}
	handle, _ := secretcap.NewHandle("opaque-aliyun-sms-test-handle")
	result, err := adapter.Deliver(t.Context(), channelcap.Delivery{
		TaskNo: "ntask_1", Credential: handle, Destination: []byte("+8613800138000"),
		NonSecretConfig: map[string]string{signNameField: "IDC通知"}, ProviderTemplateID: "SMS_123456",
		ProviderData: map[string]string{"code": "123456", "minutes": "10"},
	})
	if err != nil || result.Class != channelcap.ResultAccepted || result.ProviderMessageRef != "biz-1" || !result.CrossedExternalBorder {
		t.Fatalf("unexpected Aliyun SMS result: result=%+v err=%v", result, err)
	}
}

func TestAliyunFailureClassificationIsFailClosed(t *testing.T) {
	tests := map[string]channelcap.ResultClass{
		"isv.MOBILE_NUMBER_ILLEGAL":       channelcap.ResultInvalidRecipient,
		"isv.AMOUNT_NOT_ENOUGH":           channelcap.ResultInsufficientBalance,
		"isv.BUSINESS_LIMIT_CONTROL":      channelcap.ResultDefinitiveNotReceivedTemporary,
		"isv.TEMPLATE_MISSING_PARAMETERS": channelcap.ResultConfigurationError,
		"isp.SYSTEM_ERROR":                channelcap.ResultUnknown,
	}
	for code, expected := range tests {
		if result := classifyCode("ntask_1", code, "req-1"); result.Class != expected || !result.CrossedExternalBorder || len(result.EvidenceDigest) != 64 {
			t.Fatalf("unexpected classification for %s: %+v", code, result)
		}
	}
}
