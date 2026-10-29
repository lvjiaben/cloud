// This file guards email/SMS-only marketing delivery after retiring customer inbox channels.
package marketing

import "testing"

// TestMarketingChannelsRejectRetiredTransports prevents unusable campaign channels.
func TestMarketingChannelsRejectRetiredTransports(t *testing.T) {
	for _, channel := range []string{"in_app", "wechat_official", "signed_webhook", "wechat_work", "dingtalk", "feishu"} {
		if validMarketingChannel(channel) || marketingDestinationAvailable(&marketingContactRow{}, channel) {
			t.Fatalf("retired marketing channel %s accepted", channel)
		}
	}
	for _, channel := range []string{"email", "sms"} {
		if !validMarketingChannel(channel) {
			t.Fatalf("supported marketing channel %s rejected", channel)
		}
		if marketingDestinationAvailable(&marketingContactRow{}, channel) {
			t.Fatalf("channel %s accepted missing contact", channel)
		}
	}
}
