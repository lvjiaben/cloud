// This file verifies the typed Tencent Cloud SendSms request and fail-closed result mapping.

package tencentcloudsms

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

func TestDeliverUsesTencentCloudSendSMS20210111Contract(t *testing.T) {
	credential, err := json.Marshal(map[string][]byte{
		secretIDField:  []byte("AKIDEXAMPLE123456"),
		secretKeyField: []byte("0123456789abcdef0123456789abcdef"),
	})
	if err != nil {
		t.Fatalf("encode Tencent SMS credential: %v", err)
	}
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.String() != endpoint || request.Host != host {
			t.Fatalf("unexpected Tencent SMS endpoint: %s %s host=%s", request.Method, request.URL, request.Host)
		}
		if request.Header.Get("X-Tc-Action") != "SendSms" || request.Header.Get("X-Tc-Version") != apiVersion ||
			request.Header.Get("X-Tc-Region") != "ap-guangzhou" || !strings.HasPrefix(request.Header.Get("Authorization"), "TC3-HMAC-SHA256 Credential=AKIDEXAMPLE123456/") {
			t.Fatalf("Tencent SMS authentication headers are incomplete: %+v", request.Header)
		}
		var payload sendRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode Tencent SMS request: %v", err)
		}
		if len(payload.PhoneNumberSet) != 1 || payload.PhoneNumberSet[0] != "+8613800138000" ||
			payload.SmsSdkAppID != "1400000000" || payload.SignName != "IDC通知" || payload.TemplateID != "123456" ||
			len(payload.TemplateParams) != 2 || payload.TemplateParams[0] != "server-1" || payload.TemplateParams[1] != "3" || payload.SessionContext != "ntask_1" {
			t.Fatalf("Tencent SMS request does not preserve the frozen one-recipient template mapping: %+v", payload)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				`{"Response":{"RequestId":"req-1","SendStatusSet":[{"SerialNo":"serial-1","Code":"Ok"}]}}`,
			)),
		}, nil
	})}
	adapter, err := newAdapter(accessorStub{payload: credential}, client, func() time.Time {
		return time.Date(2026, time.August, 15, 8, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatalf("construct Tencent SMS adapter: %v", err)
	}
	handle, _ := secretcap.NewHandle("opaque-tencent-sms-test-handle")
	result, err := adapter.Deliver(t.Context(), channelcap.Delivery{
		TaskNo: "ntask_1", Credential: handle, Destination: []byte("+8613800138000"),
		NonSecretConfig:    map[string]string{regionField: "ap-guangzhou", appIDField: "1400000000", signNameField: "IDC通知"},
		ProviderTemplateID: "123456", ProviderParameters: []string{"server-1", "3"},
	})
	if err != nil || result.Class != channelcap.ResultAccepted || result.ProviderMessageRef != "serial-1" || !result.CrossedExternalBorder {
		t.Fatalf("unexpected Tencent SMS result: result=%+v err=%v", result, err)
	}
}

func TestTencentSMSFailureClassificationIsFailClosed(t *testing.T) {
	tests := map[string]channelcap.ResultClass{
		"FailedOperation.InsufficientBalanceInSmsPackage": channelcap.ResultInsufficientBalance,
		"InvalidParameterValue.IncorrectPhoneNumber":      channelcap.ResultInvalidRecipient,
		"LimitExceeded.PhoneNumberDailyLimit":             channelcap.ResultDefinitiveNotReceivedTemporary,
		"FailedOperation.TemplateIncorrectOrUnapproved":   channelcap.ResultConfigurationError,
		"InternalError.RequestTimeException":              channelcap.ResultUnknown,
		"FailedOperation.Other":                           channelcap.ResultUnknown,
	}
	for code, expected := range tests {
		t.Run(code, func(t *testing.T) {
			result := classifyCode("ntask_1", code, "provider-ref")
			if result.Class != expected || !result.CrossedExternalBorder || len(result.EvidenceDigest) != 64 {
				t.Fatalf("unexpected classification for %s: %+v", code, result)
			}
		})
	}
}
