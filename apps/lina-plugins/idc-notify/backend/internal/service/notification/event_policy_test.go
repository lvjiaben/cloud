package notification

import (
	"testing"

	"lina-plugin-idc-core/backend/cap/notificationcap"
)

func TestValidEventPolicyChannelRejectsRemovedWeCom(t *testing.T) {
	for _, recipient := range []notificationcap.RecipientKind{
		notificationcap.RecipientCustomerContact,
		notificationcap.RecipientStaffUser,
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
		if validEventPolicyChannel(recipient, "signed_webhook") {
			t.Fatalf("recipient %q must reject removed signed webhook", recipient)
		}
	}
}
