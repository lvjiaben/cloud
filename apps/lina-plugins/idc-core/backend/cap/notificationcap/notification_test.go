package notificationcap

import (
	"context"
	"testing"
)

type contactResolverStub struct{}

func (contactResolverStub) ResolveBatch(context.Context, []ContactRequest, ContactConsumer) error {
	return nil
}

type currentAudienceStub struct{}

func (currentAudienceStub) Current(context.Context) (CurrentAudienceProjection, error) {
	return CurrentAudienceProjection{CustomerNo: "CUS_test", CustomerVersion: 1}, nil
}

type eventFeedStub struct{}

func (eventFeedStub) Claim(context.Context, EventClaimInput) ([]ClaimedEvent, error) {
	return []ClaimedEvent{}, nil
}
func (eventFeedStub) Ack(context.Context, EventLease) (EventReceipt, error) {
	return EventReceipt{}, nil
}
func (eventFeedStub) Release(context.Context, EventReleaseInput) (EventReceipt, error) {
	return EventReceipt{}, nil
}

type businessObjectAccessStub struct{}

func (businessObjectAccessStub) AuthorizeCurrent(
	context.Context,
	BusinessObjectRequest,
) (BusinessObjectProjection, error) {
	return BusinessObjectProjection{}, nil
}

type marketingSendGateStub struct{}

func (marketingSendGateStub) CheckSendBatch(
	context.Context,
	[]MarketingSendInput,
) ([]MarketingSendDecision, error) {
	return nil, nil
}

type preferenceCommanderStub struct{}

func (preferenceCommanderStub) Current(context.Context) (*PreferenceSnapshot, error) {
	return &PreferenceSnapshot{}, nil
}

func (preferenceCommanderStub) UpdateCurrent(
	context.Context,
	PreferenceCommand,
) (*PreferenceResult, error) {
	return &PreferenceResult{}, nil
}

func TestValidateRuntimeRejectsIncompleteDependencies(t *testing.T) {
	valid := Runtime{
		Audience: currentAudienceStub{}, Events: eventFeedStub{}, Contacts: contactResolverStub{}, BusinessObjects: businessObjectAccessStub{},
		Marketing: marketingSendGateStub{}, Preferences: preferenceCommanderStub{},
	}
	if err := ValidateRuntime(valid); err != nil {
		t.Fatalf("valid runtime rejected: %v", err)
	}
	invalid := valid
	invalid.Contacts = nil
	if err := ValidateRuntime(invalid); err == nil {
		t.Fatal("incomplete runtime accepted")
	}
}

func TestMarketingConsentProjectionCarriesAuthoritativePolicyEvidence(t *testing.T) {
	projection := MarketingConsentProjection{
		ContactNo:       "CON_test",
		Channel:         ChannelEmail,
		Status:          "opted_out",
		PolicyReference: "privacy/marketing",
		PolicyVersion:   "2026-08",
		Locale:          "zh-CN",
		Version:         2,
	}
	if projection.PolicyReference == "" || projection.PolicyVersion == "" || projection.Locale == "" {
		t.Fatal("marketing consent projection lost authoritative policy evidence")
	}
}
