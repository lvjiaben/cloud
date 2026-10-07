package notification

import (
	"testing"

	"lina-plugin-idc-core/backend/cap/notificationcap"
)

func TestValidEventPolicyChannelKeepsWeComStaffOnly(t *testing.T) {
	if !validEventPolicyChannel(notificationcap.RecipientStaffUser, "wechat_work") {
		t.Fatal("staff WeCom policy must remain configurable")
	}
	for _, recipient := range []notificationcap.RecipientKind{
		notificationcap.RecipientCustomerContact,
		notificationcap.RecipientVerifiedDestination,
	} {
		if validEventPolicyChannel(recipient, "wechat_work") {
			t.Fatalf("recipient %q must not use WeCom", recipient)
		}
	}
	for _, recipient := range []notificationcap.RecipientKind{
		notificationcap.RecipientCustomerContact,
		notificationcap.RecipientStaffUser,
		notificationcap.RecipientVerifiedDestination,
	} {
		if !validEventPolicyChannel(recipient, "signed_webhook") {
			t.Fatalf("recipient %q must allow installation-level signed webhook subscription", recipient)
		}
	}
}
