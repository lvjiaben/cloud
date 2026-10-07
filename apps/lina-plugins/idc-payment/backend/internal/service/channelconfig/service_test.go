package channelconfig

import (
	"testing"

	"lina-plugin-idc-payment/backend/cap/paymentcap"
)

func TestVerifiedCapabilitiesRequireExplicitWechatJSAPIAuthorization(t *testing.T) {
	declared := paymentcap.Capabilities{
		Modes: map[paymentcap.Mode]bool{
			paymentcap.ModeWechatNative: true,
			paymentcap.ModeWechatH5:     true,
			paymentcap.ModeWechatJSAPI:  true,
		},
		Currencies: map[string]bool{"CNY": true},
		Operations: map[paymentcap.Operation]bool{paymentcap.OperationCreate: true},
	}
	defaults, err := verifiedCapabilities(declared, paymentcap.ProviderWechatPay, nil, CredentialEnvelope{})
	if err != nil || defaults.Modes[paymentcap.ModeWechatJSAPI] || !defaults.Modes[paymentcap.ModeWechatNative] {
		t.Fatalf("default capabilities=%+v err=%v", defaults, err)
	}
	if _, err = verifiedCapabilities(declared, paymentcap.ProviderWechatPay, []paymentcap.Mode{paymentcap.ModeWechatJSAPI}, CredentialEnvelope{}); err == nil {
		t.Fatal("JSAPI was enabled without merchant authorization evidence")
	}
	verified, err := verifiedCapabilities(declared, paymentcap.ProviderWechatPay, []paymentcap.Mode{paymentcap.ModeWechatJSAPI}, CredentialEnvelope{Extra: map[string]string{"jsapi_authorized": "true"}})
	if err != nil || !verified.Modes[paymentcap.ModeWechatJSAPI] {
		t.Fatalf("authorized capabilities=%+v err=%v", verified, err)
	}
}

func TestProviderEndpointsStayInsideVerifiedOrigins(t *testing.T) {
	cases := []SaveInput{
		{Provider: paymentcap.ProviderAlipay, EndpointOrigin: "https://evil.example/gateway.do"},
		{Provider: paymentcap.ProviderWechatPay, EndpointOrigin: "https://evil.example"},
		{Provider: paymentcap.ProviderEPay, EndpointOrigin: "https://gateway.example/api.php"},
	}
	for _, input := range cases {
		input.ChannelNo = "channel-1"
		input.DisplayName = "channel"
		input.MerchantReference = "merchant-1"
		input.IdempotencyKey = "save-1"
		input.ActorID = 1
		if err := validateSave(input); err == nil {
			t.Fatalf("unsafe endpoint accepted: %+v", input)
		}
	}
}
