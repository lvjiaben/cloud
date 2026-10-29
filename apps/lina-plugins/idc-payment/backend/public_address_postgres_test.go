// This file verifies immediate routing changes, concurrency and distinct URLs on PostgreSQL.
package backend

import (
	"context"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
	"lina-plugin-idc-payment/backend/internal/service/paymentflow"
)

func TestPaymentPublicAddressesApplyWithoutRestartPostgreSQL(t *testing.T) {
	_, store, _, channels, adapter, _ := newSubmissionServices(t)
	ctx := context.Background()
	flow, err := paymentflow.New(store, channels, paymentflow.Config{PublicOrigin: "https://localhost.invalid"})
	if err != nil {
		t.Fatal(err)
	}
	initial, err := channels.ReadPublicAddress(ctx)
	if err != nil || initial.Version != 0 {
		t.Fatalf("initial addresses: %+v %v", initial, err)
	}
	input := paymentflow.CreateInput{PaymentContextNo: "context-test", ChannelNo: "submission-channel", Mode: paymentcap.ModeAlipayPage, BalanceAmountMinor: 900}
	if _, err = flow.Create(ctx, input); !bizerr.Is(err, channelconfig.CodePublicAddressUnconfigured) {
		t.Fatalf("unconfigured flow started a payment: %v", err)
	}
	saved, err := channels.SavePublicAddress(ctx, channelconfig.PublicAddress{NotificationOrigin: "https://notify.example/", CustomerReturnOrigin: "http://127.0.0.1:6032"}, 0, 101)
	if err != nil || saved.Version != 1 {
		t.Fatalf("save independent origins: %+v %v", saved, err)
	}
	if _, err = channels.SavePublicAddress(ctx, *saved, 0, 101); !bizerr.Is(err, channelconfig.CodePublicAddressConflict) {
		t.Fatalf("first-save race overwrote current settings: %v", err)
	}
	if _, err = channels.SavePublicAddress(ctx, channelconfig.PublicAddress{NotificationOrigin: "https://other.example", CustomerReturnOrigin: "https://other.example"}, 9, 101); !bizerr.Is(err, channelconfig.CodePublicAddressConflict) {
		t.Fatalf("stale settings overwrote current values: %v", err)
	}
	calls := 0
	adapter.create = func(_ context.Context, request paymentcap.CreateRequest) (*paymentcap.Redirect, error) {
		calls++
		if validationErr := paymentcap.ValidateCreateRequest(request); validationErr != nil {
			return nil, validationErr
		}
		if request.NotifyURL != "https://notify.example/x/idc-payment/api/v1/callbacks/payments/submission-channel" || request.ReturnURL != "http://127.0.0.1:6032/payment/return/"+request.AttemptNo {
			t.Fatalf("notification/browser origins mixed: %+v", request)
		}
		return &paymentcap.Redirect{Method: "POST", URL: "https://payment.example.test/pay"}, nil
	}
	if _, err = flow.Create(ctx, input); err != nil || calls != 1 {
		t.Fatalf("existing flow did not consume saved settings immediately: calls=%d err=%v", calls, err)
	}
	cleared, err := channels.SavePublicAddress(ctx, channelconfig.PublicAddress{}, saved.Version, 101)
	if err != nil || cleared.Version != 2 {
		t.Fatalf("clear settings: %+v %v", cleared, err)
	}
	if _, err = flow.Create(ctx, input); !bizerr.Is(err, channelconfig.CodePublicAddressUnconfigured) || calls != 1 {
		t.Fatalf("cleared origins silently restored startup settings: calls=%d err=%v", calls, err)
	}
}
