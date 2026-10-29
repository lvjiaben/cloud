package notification

import (
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
)

// TestEnqueueRejectsRetiredEmployeeChannelBeforeDependencies ensures a stale
// channel row cannot restore a deleted channel through the service contract.
func TestEnqueueRejectsRetiredEmployeeChannelBeforeDependencies(t *testing.T) {
	input := EnqueueInput{
		EventCode: "support.ticket_updated", ChannelCode: "wechat_work", Locale: "zh-CN",
		OwnerUserID: 101, BusinessObjectType: "support_ticket", BusinessObjectNo: "ticket-retired-channel",
		TemplateNo: "retired-template", TemplateVersion: 1, Body: "Retired channel notification",
		IdempotencyKey: "retired-employee-channel-0001",
	}
	service := &serviceImpl{}
	if _, err := service.Enqueue(t.Context(), input); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("retired channel was not rejected before database/provider access: %v", err)
	}
	for _, channel := range supportedChannelCodes {
		input.ChannelCode = channel
		input.CustomerNo, input.ContactNo = "customer-1", "contact-1"
		if err := validateEnqueueInput(input); err != nil {
			t.Fatalf("supported channel %q lost its enqueue contract: %v", channel, err)
		}
	}
}

func TestDeliveryStatusForResultUsesExactlyThreeAutomaticRetries(t *testing.T) {
	for sequence := 0; sequence < 3; sequence++ {
		if status := deliveryStatusForResult(channelcap.Result{Class: channelcap.ResultDefinitiveNotReceivedTemporary}, sequence); status != DeliveryRetryWait {
			t.Fatalf("sequence %d should wait for the next automatic retry, got %s", sequence, status)
		}
	}
	if status := deliveryStatusForResult(channelcap.Result{Class: channelcap.ResultDefinitiveNotReceivedTemporary}, 3); status != DeliveryFailed {
		t.Fatalf("the third retry must end in failed, got %s", status)
	}
	if retryIntervalLiteral(1) != "1 minute" || retryIntervalLiteral(2) != "5 minutes" || retryIntervalLiteral(3) != "30 minutes" {
		t.Fatal("automatic retry intervals must remain 1, 5, and 30 minutes")
	}
}

func TestDeliveryStatusNeverRetriesUnknownOrDefinitiveProblems(t *testing.T) {
	cases := []struct {
		class channelcap.ResultClass
		want  DeliveryStatus
	}{
		{channelcap.ResultUnknown, DeliveryResultUnknown},
		{channelcap.ResultInvalidRecipient, DeliveryFailed},
		{channelcap.ResultInsufficientBalance, DeliveryFailed},
		{channelcap.ResultConfigurationError, DeliveryFailed},
		{channelcap.ResultDefinitiveNotReceivedPermanent, DeliveryFailed},
		{channelcap.ResultAccepted, DeliverySubmitted},
		{channelcap.ResultDelivered, DeliveryDelivered},
	}
	for _, item := range cases {
		if got := deliveryStatusForResult(channelcap.Result{Class: item.class}, 0); got != item.want {
			t.Errorf("class %s: got %s, want %s", item.class, got, item.want)
		}
	}
}

func TestResendEvidenceAndPublicStatusValidation(t *testing.T) {
	if !explicitNotReceived("definitive_not_received_temporary") || !explicitNotReceived("definitive_not_received_permanent") {
		t.Fatal("explicit not-received provider evidence must remain eligible for further gates")
	}
	for _, unsafe := range []string{"result_unknown", "invalid_recipient", "accepted", "delivered"} {
		if explicitNotReceived(unsafe) {
			t.Fatalf("%s must never be treated as explicit not-received evidence", unsafe)
		}
	}
	for _, status := range []string{"queued", "retry_wait", "manual_review", "manual_unknown_closed", "preference_blocked"} {
		if !validDeliveryStatus(status) {
			t.Errorf("public delivery status %s should be accepted", status)
		}
	}
	if validDeliveryStatus("provider_raw_timeout") {
		t.Fatal("raw provider state must not cross the public status boundary")
	}
}

func TestDigestValidationRejectsNonCanonicalEvidence(t *testing.T) {
	valid := digestText("notification", "evidence")
	if !isDigest(valid) {
		t.Fatal("generated evidence digest must be canonical lowercase SHA-256")
	}
	for _, invalid := range []string{"", valid[:63], "G" + valid[1:], valid + "0"} {
		if isDigest(invalid) {
			t.Fatalf("invalid evidence digest was accepted: %q", invalid)
		}
	}
}

func TestManualResendAllowsConfiguredInAppChannelWithoutProviderSecret(t *testing.T) {
	channel := &deliveryChannelRow{StatusCode: string(ChannelEnabled), BalanceState: "sufficient"}
	if !channelAvailableForManualResend(string(notificationcap.ChannelInApp), channel) {
		t.Fatal("in-app manual resend must not require an external provider or secret")
	}
	if channelAvailableForManualResend(string(notificationcap.ChannelEmail), channel) {
		t.Fatal("external-channel manual resend must fail closed without a provider and secret")
	}
}

func TestChannelsWithoutProviderBalanceDoNotBlockManualResend(t *testing.T) {
	for _, channel := range []string{"in_app", "email", "wechat_official", "signed_webhook", "dingtalk", "feishu"} {
		if state := channelBalanceState(channel); state != "sufficient" {
			t.Fatalf("channel %q balance state=%q want sufficient", channel, state)
		}
	}
	if state := channelBalanceState("sms"); state != "unknown" {
		t.Fatalf("SMS must remain fail-closed without an authoritative balance query: %q", state)
	}
}
