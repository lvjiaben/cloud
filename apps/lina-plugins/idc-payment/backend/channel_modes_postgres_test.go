// This file verifies mode-only editing retains credentials and connection facts.
package backend

import (
	"context"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
)

func TestChannelModesPreserveCredentialAndRejectStaleEditsPostgreSQL(t *testing.T) {
	db, _, _, channels, _, _ := newSubmissionServices(t)
	ctx := context.Background()
	before, err := channels.Get(ctx, "submission-channel")
	if err != nil {
		t.Fatal(err)
	}
	reference, err := db.GetValue(ctx, `SELECT secret_ref FROM plugin_idc_payment_channel WHERE channel_no='submission-channel'`)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := channels.SaveModes(ctx, before.ChannelNo, []paymentcap.Mode{paymentcap.ModeAlipayPage}, before.Version, 101)
	if err != nil || updated.Version != before.Version+1 || updated.StatusCode != before.StatusCode || !updated.VerifiedAt.Equal(before.VerifiedAt) {
		t.Fatalf("mode edit changed connection facts: updated=%+v err=%v", updated, err)
	}
	current, err := db.GetValue(ctx, `SELECT secret_ref FROM plugin_idc_payment_channel WHERE channel_no='submission-channel'`)
	if err != nil || current.String() != reference.String() {
		t.Fatalf("credential reference changed: %v", err)
	}
	if _, err = channels.SaveModes(ctx, before.ChannelNo, []paymentcap.Mode{paymentcap.ModeAlipayPage}, before.Version, 101); !bizerr.Is(err, channelconfig.CodeChannelModesConflict) {
		t.Fatalf("stale modes accepted: %v", err)
	}
	for _, modes := range [][]paymentcap.Mode{nil, {paymentcap.ModeEPayAlipay}, {paymentcap.ModeAlipayPage, paymentcap.ModeAlipayPage}} {
		if _, err = channels.SaveModes(ctx, before.ChannelNo, modes, updated.Version, 101); !bizerr.Is(err, channelconfig.CodeChannelModesInvalid) {
			t.Fatalf("invalid modes accepted: %v", err)
		}
	}
}
