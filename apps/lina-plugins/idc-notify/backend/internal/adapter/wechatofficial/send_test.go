// This file verifies Official Account template delivery and local mapping validation.

package wechatofficial

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

type sendAccessorStub struct{ payload []byte }

func (stub sendAccessorStub) Access(_ context.Context, request secretcap.AccessRequest, consume secretcap.Consumer) error {
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

func TestDeliverUsesOfficialTemplateMessageContract(t *testing.T) {
	credential, err := json.Marshal(map[string][]byte{appSecretField: []byte("0123456789abcdef0123456789abcdef")})
	if err != nil {
		t.Fatalf("encode WeChat credential: %v", err)
	}
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		switch requests {
		case 1:
			if request.Method != http.MethodGet || request.URL.Scheme != "https" || request.URL.Host != "api.weixin.qq.com" ||
				request.URL.Path != "/cgi-bin/token" || request.URL.Query().Get("grant_type") != "client_credential" || request.URL.Query().Get("appid") != "wxAppId12345" {
				t.Fatalf("unexpected WeChat token request: %s %s", request.Method, request.URL)
			}
			return jsonResponse(`{"access_token":"access-token","expires_in":7200}`), nil
		case 2:
			if request.Method != http.MethodPost || request.URL.String() != sendEndpoint+"?access_token=access-token" || request.Header.Get("Content-Type") != "application/json" {
				t.Fatalf("unexpected WeChat send request: %s %s", request.Method, request.URL)
			}
			var payload struct {
				ToUser     string                   `json:"touser"`
				TemplateID string                   `json:"template_id"`
				Data       map[string]templateValue `json:"data"`
			}
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				t.Fatalf("decode WeChat template request: %v", err)
			}
			if payload.ToUser != "openid_1234567890" || payload.TemplateID != "wechat-template-1" ||
				payload.Data["server_name"].Value != "server-1" || payload.Data["remaining_days"].Value != "3" {
				t.Fatalf("WeChat provider template mapping was not preserved: %+v", payload)
			}
			return jsonResponse(`{"errcode":0,"errmsg":"ok","msgid":987654321}`), nil
		default:
			t.Fatalf("unexpected extra WeChat request: %s", request.URL)
			return nil, nil
		}
	})}
	adapter, err := newAdapter(sendAccessorStub{payload: credential}, client)
	if err != nil {
		t.Fatalf("construct WeChat Official adapter: %v", err)
	}
	handle, _ := secretcap.NewHandle("opaque-wechat-send-test-handle")
	result, err := adapter.Deliver(t.Context(), channelcap.Delivery{
		TaskNo: "ntask_1", Credential: handle, Destination: []byte("openid_1234567890"),
		NonSecretConfig: map[string]string{appIDField: "wxAppId12345"}, ProviderTemplateID: "wechat-template-1",
		ProviderData: map[string]string{"server_name": "server-1", "remaining_days": "3"},
	})
	if err != nil || result.Class != channelcap.ResultAccepted || result.ProviderMessageRef != "987654321" ||
		result.QueryDeadlineSource != "wechat_encrypted_callback" || !result.CrossedExternalBorder || requests != 2 {
		t.Fatalf("unexpected WeChat template delivery result: result=%+v requests=%d err=%v", result, requests, err)
	}
}

func TestInvalidWeChatProviderMappingDoesNotReachNetwork(t *testing.T) {
	credential, _ := json.Marshal(map[string][]byte{appSecretField: []byte("0123456789abcdef0123456789abcdef")})
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("invalid provider mapping reached WeChat")
		return nil, nil
	})}
	adapter, _ := newAdapter(sendAccessorStub{payload: credential}, client)
	handle, _ := secretcap.NewHandle("opaque-wechat-send-test-handle")
	result, err := adapter.Deliver(t.Context(), channelcap.Delivery{
		TaskNo: "ntask_1", Credential: handle, Destination: []byte("openid_1234567890"),
		NonSecretConfig: map[string]string{appIDField: "wxAppId12345"}, ProviderTemplateID: "wechat-template-1",
		ProviderData: map[string]string{"invalid-key!": "value"},
	})
	if err != nil || result.Class != channelcap.ResultConfigurationError || result.CrossedExternalBorder {
		t.Fatalf("invalid mapping did not fail closed: result=%+v err=%v", result, err)
	}
}

func TestUnknownWeChatErrorCodeRemainsUnknown(t *testing.T) {
	result := classify("ntask_1", 49999)
	if result.Class != channelcap.ResultUnknown || !result.CrossedExternalBorder || len(result.EvidenceDigest) != 64 {
		t.Fatalf("unknown WeChat error code must not be treated as definitively undelivered: %+v", result)
	}
}

func jsonResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
