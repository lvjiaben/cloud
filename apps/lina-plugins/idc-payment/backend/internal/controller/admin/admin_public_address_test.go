// This file verifies that missing direct staff identity returns a localized
// business error before any payment setting write can occur.
package admin

import (
	"context"
	"testing"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-payment/backend/api/admin/v1"
	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
)

func TestPaymentPublicAddressRejectsMissingDirectStaff(t *testing.T) {
	controller := &ControllerV1{}
	result, err := controller.PaymentPublicAddressSave(context.Background(), &v1.PaymentPublicAddressSaveReq{})
	if result != nil || !bizerr.Is(err, channelconfig.CodePublicAddressForbidden) {
		t.Fatalf("missing staff was not rejected by typed permission error: result=%+v err=%v", result, err)
	}
}

func TestPaymentChannelModesRejectMissingDirectStaff(t *testing.T) {
	controller := &ControllerV1{}
	result, err := controller.PaymentChannelModesSave(context.Background(), &v1.PaymentChannelModesSaveReq{})
	if result != nil || !bizerr.Is(err, channelconfig.CodeChannelModesForbidden) {
		t.Fatalf("mode edit without staff accepted: result=%+v err=%v", result, err)
	}
}
