// This file verifies that all provider delivery classes map to supported audit
// results without claiming that a failed or unknown delivery was successful.

package notification

import (
	"lina-plugin-idc-notify/backend/cap/channelcap"
	"testing"
)

func TestChannelTestAuditResult(t *testing.T) {
	for _, tc := range []struct {
		class channelcap.ResultClass
		want  string
	}{
		{channelcap.ResultAccepted, "succeeded"}, {channelcap.ResultDelivered, "succeeded"},
		{channelcap.ResultUnknown, "result_unknown"},
		{channelcap.ResultDefinitiveNotReceivedTemporary, "failed"},
		{channelcap.ResultDefinitiveNotReceivedPermanent, "failed"},
		{channelcap.ResultInvalidRecipient, "failed"}, {channelcap.ResultInsufficientBalance, "failed"},
		{channelcap.ResultConfigurationError, "failed"},
	} {
		if got := channelTestAuditResult(tc.class); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.class, got, tc.want)
		}
	}
}
