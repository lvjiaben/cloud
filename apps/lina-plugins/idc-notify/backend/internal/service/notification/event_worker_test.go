package notification

import (
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/notificationcap"
)

func TestMaterializationContactRequestsUseCoreContactsAndFailClosedWithoutOtherResolvers(t *testing.T) {
	base := materializationCandidate{policy: eventPolicyRow{Enabled: true}, channelStatus: string(ChannelEnabled)}
	candidates := []materializationCandidate{
		{policy: eventPolicyRow{Enabled: false}, channel: "email"},
		{policy: base.policy, channel: "in_app", channelStatus: base.channelStatus,
			claim: notificationcap.ClaimedEvent{Request: notificationcap.EventRequest{Recipient: notificationcap.Recipient{Kind: notificationcap.RecipientCustomerContact}}}},
		{policy: base.policy, channel: "email", channelStatus: base.channelStatus,
			claim: notificationcap.ClaimedEvent{Request: notificationcap.EventRequest{Recipient: notificationcap.Recipient{Kind: notificationcap.RecipientStaffUser, StaffUserID: 7}}}},
		{policy: base.policy, channel: "email", channelStatus: base.channelStatus,
			claim: notificationcap.ClaimedEvent{Request: notificationcap.EventRequest{Recipient: notificationcap.Recipient{Kind: notificationcap.RecipientVerifiedDestination, DestinationNo: "verified-1"}}}},
		{policy: base.policy, channel: "signed_webhook", channelStatus: base.channelStatus,
			claim: notificationcap.ClaimedEvent{Request: notificationcap.EventRequest{Recipient: notificationcap.Recipient{Kind: notificationcap.RecipientStaffUser, StaffUserID: 7}}}},
	}
	_, requests := materializationContactRequests(candidates)
	if len(requests) != 1 || requests[0].Channel != notificationcap.ChannelInApp {
		t.Fatalf("customer in-app policy must use the bounded core contact lookup: %#v", requests)
	}
	want := []string{"policy_disabled", "", "staff_resolver_unavailable", "verified_destination_resolver_unavailable", ""}
	for index := range want {
		if candidates[index].outcome.reason != want[index] {
			t.Fatalf("candidate %d reason=%q want=%q", index, candidates[index].outcome.reason, want[index])
		}
	}
}

func TestPrepareMaterializationInputsKeepsSignedWebhookMinimalAndRecipientIndependent(t *testing.T) {
	request := notificationcap.EventRequest{
		EventCode:          "support.ticket_created",
		Recipient:          notificationcap.Recipient{Kind: notificationcap.RecipientStaffUser, StaffUserID: 7},
		BusinessObjectType: notificationcap.BusinessObjectSupportTicket,
		BusinessObjectNo:   "ticket-1", BusinessObjectVersion: 2,
		Variables: map[string]string{"ticket_no": "T-1", "title": "Need help"},
	}
	candidates := []materializationCandidate{{
		claim: notificationcap.ClaimedEvent{EventReceipt: notificationcap.EventReceipt{
			SourcePlugin: "idc-core", EventNo: "event-webhook-1",
		}, Request: request},
		event: eventRow{EventCode: request.EventCode, TopicCode: "support", ObjectType: "support_ticket",
			RequiredVariables: `["ticket_no","title"]`, OptionalVariables: `[]`},
		channel: "signed_webhook", policy: eventPolicyRow{Enabled: true},
	}}
	marketingIndexes, marketingInputs, err := prepareMaterializationInputs(candidates)
	if err != nil || len(marketingIndexes) != 0 || len(marketingInputs) != 0 {
		t.Fatalf("prepare signed webhook: indexes=%v inputs=%v err=%v", marketingIndexes, marketingInputs, err)
	}
	input := candidates[0].outcome.input
	if input == nil || input.ChannelCode != "signed_webhook" || input.TemplateNo != "" ||
		input.Subject != "" || input.Body != "" || len(input.Variables) != 0 ||
		input.CustomerNo != "" || input.ContactNo != "" || input.OwnerUserID != 0 {
		t.Fatalf("signed webhook materialization leaked recipient or template data: %#v", input)
	}
}

func TestPrepareMaterializationInputsFreezesResolvedLocaleAndTemplate(t *testing.T) {
	request := notificationcap.EventRequest{
		EventCode: "support.ticket_created",
		Recipient: notificationcap.Recipient{Kind: notificationcap.RecipientCustomerContact,
			CustomerNo: "customer-1", ContactNo: "contact-1"},
		BusinessObjectType: notificationcap.BusinessObjectSupportTicket,
		BusinessObjectNo:   "ticket-1", Variables: map[string]string{"ticket_no": "T-1", "title": "Need help"},
		IdempotencyKey: "support-event-1",
	}
	candidates := []materializationCandidate{{
		claim: notificationcap.ClaimedEvent{EventReceipt: notificationcap.EventReceipt{SourcePlugin: "idc-core", EventNo: "event-1"}, Request: request},
		event: eventRow{EventCode: request.EventCode, TopicCode: "support", ObjectType: "support_ticket",
			RequiredVariables: `["ticket_no","title"]`, OptionalVariables: `[]`},
		channel: "email", policy: eventPolicyRow{Enabled: true},
		projection: notificationcap.ContactProjection{PreferredLocale: "en-US"},
		templates: map[string]templateRow{"en-US": {TemplateNo: "ntpl-1", Version: 1,
			StatusCode: "active", SubjectTemplate: "Ticket {{ticket_no}}", BodyTemplate: "{{title}}"}},
	}}
	marketingIndexes, marketingInputs, err := prepareMaterializationInputs(candidates)
	if err != nil || len(marketingIndexes) != 0 || len(marketingInputs) != 0 {
		t.Fatalf("prepare inputs: indexes=%v inputs=%v err=%v", marketingIndexes, marketingInputs, err)
	}
	input := candidates[0].outcome.input
	if input == nil || input.Locale != "en-US" || input.TemplateNo != "ntpl-1" || input.Subject != "Ticket T-1" || input.Body != "Need help" {
		t.Fatalf("unexpected frozen delivery: %#v", input)
	}
}

func TestNotificationEventFailureIsRedactedAndClassified(t *testing.T) {
	code, summary, retryable := notificationEventFailure(bizerr.NewCode(CodeInvalid))
	if code != "notification_event_invalid" || summary == "" || retryable {
		t.Fatalf("unexpected permanent classification: %q %q %v", code, summary, retryable)
	}
	code, summary, retryable = notificationEventFailure(bizerr.NewCode(CodeUnavailable))
	if code != "notification_event_unavailable" || summary == "" || !retryable {
		t.Fatalf("unexpected retryable classification: %q %q %v", code, summary, retryable)
	}
}
