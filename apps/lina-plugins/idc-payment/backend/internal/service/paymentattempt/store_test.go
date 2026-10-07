// This file verifies that payment callback storage accepts only a previously
// verified success and never treats provider reversals as a new payment.

package paymentattempt

import (
	"context"
	"strings"
	"testing"

	"lina-plugin-idc-payment/backend/cap/paymentcap"
)

// TestRecordVerifiedCallbackRejectsNonSuccessEvents ensures that a revocation,
// chargeback, or settlement reversal cannot enter the ordinary posting queue.
func TestRecordVerifiedCallbackRejectsNonSuccessEvents(t *testing.T) {
	t.Parallel()
	store := &Store{}
	for _, status := range []string{"reversal", "chargeback", "settlement_reversal"} {
		status := status
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			result, err := store.RecordVerifiedCallback(context.Background(), CallbackInput{
				ChannelNo:     "channel-1",
				PayloadDigest: strings.Repeat("a", 64),
				Result: paymentcap.VerifiedResult{
					AttemptNo: "attempt-1", Status: status, EvidenceDigest: strings.Repeat("b", 64),
				},
			})
			if err == nil || result != nil {
				t.Fatalf("status %q entered the success callback path: result=%+v err=%v", status, result, err)
			}
		})
	}
}
