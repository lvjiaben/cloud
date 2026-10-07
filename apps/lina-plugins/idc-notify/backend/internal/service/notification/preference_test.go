package notification

import (
	"context"
	"errors"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
)

type preferenceSourceStub struct {
	snapshot *notificationcap.PreferenceSnapshot
	err      error
}

func (stub preferenceSourceStub) Current(context.Context) (*notificationcap.PreferenceSnapshot, error) {
	return stub.snapshot, stub.err
}

func (preferenceSourceStub) UpdateCurrent(context.Context, notificationcap.PreferenceCommand) (*notificationcap.PreferenceResult, error) {
	return nil, nil
}

func TestGetOwnPreferencesProjectsCoreSnapshot(t *testing.T) {
	service := &serviceImpl{sources: notificationcap.Runtime{Preferences: preferenceSourceStub{snapshot: &notificationcap.PreferenceSnapshot{
		CustomerNo: "cust_1", PreferredLocale: "zh-CN", Version: 7,
		Preferences: []notificationcap.PreferenceProjection{{
			ContactNo: "contact_1", Topic: notificationcap.TopicSupport,
			Channel: notificationcap.ChannelEmail, Enabled: true, Version: 4,
		}},
		MarketingConsents: []notificationcap.MarketingConsentProjection{{
			ContactNo: "contact_1", Channel: notificationcap.ChannelSMS, Status: "opted_out",
			PolicyReference: "privacy/marketing", PolicyVersion: "2026-08", Locale: "zh-CN", Version: 5,
		}},
		QuietHours: &notificationcap.QuietHoursProjection{
			TimezoneName: "Asia/Shanghai", StartMinute: 1320, EndMinute: 480,
			CrossesMidnight: true, Enabled: true, Version: 6,
		},
	}}}}

	result, err := service.GetOwnPreferences(t.Context())
	if err != nil {
		t.Fatalf("get current preference snapshot: %v", err)
	}
	if result.CustomerNo != "cust_1" || result.PreferredLocale != "zh-CN" || result.Version != 7 {
		t.Fatalf("unexpected snapshot head: %+v", result)
	}
	if len(result.Preferences) != 1 || result.Preferences[0].ChannelCode != "email" ||
		result.Preferences[0].Topic != "support" || !result.Preferences[0].Enabled {
		t.Fatalf("unexpected preference projection: %+v", result.Preferences)
	}
	if len(result.MarketingConsents) != 1 || result.MarketingConsents[0].Status != "opted_out" ||
		result.MarketingConsents[0].PolicyReference != "privacy/marketing" ||
		result.MarketingConsents[0].PolicyVersion != "2026-08" || result.MarketingConsents[0].Locale != "zh-CN" {
		t.Fatalf("unexpected consent projection: %+v", result.MarketingConsents)
	}
	if result.QuietHours == nil || !result.QuietHours.CrossesMidnight || result.QuietHours.Version != 6 {
		t.Fatalf("unexpected quiet-hours projection: %+v", result.QuietHours)
	}
}

func TestCustomerPreferenceChannelsRejectEmployeeOnlyWeCom(t *testing.T) {
	if validCustomerChannel(string(notificationcap.ChannelWeChatWork)) {
		t.Fatal("customer preference accepted the employee-only WeCom channel")
	}
	service := &serviceImpl{sources: notificationcap.Runtime{Preferences: preferenceSourceStub{snapshot: &notificationcap.PreferenceSnapshot{
		CustomerNo: "cust_1", Version: 1,
		Preferences: []notificationcap.PreferenceProjection{{ContactNo: "contact_1", Topic: notificationcap.TopicSupport,
			Channel: notificationcap.ChannelWeChatWork, Enabled: true, Version: 1}},
	}}}}
	if _, err := service.GetOwnPreferences(t.Context()); err == nil {
		t.Fatal("employee-only WeCom projection was exposed through the customer endpoint")
	}
}

func TestWithdrawalReusesAuthoritativeMarketingEvidence(t *testing.T) {
	snapshot := &notificationcap.PreferenceSnapshot{MarketingConsents: []notificationcap.MarketingConsentProjection{{
		ContactNo: "contact_1", Channel: notificationcap.ChannelSMS, Status: "opted_in",
		PolicyReference: "privacy/marketing", PolicyVersion: "2026-08", Locale: "en-US", Version: 3,
	}}}
	reference, version, locale := currentMarketingConsentEvidence(snapshot, "contact_1", "sms")
	if reference != "privacy/marketing" || version != "2026-08" || locale != "en-US" {
		t.Fatalf("authoritative evidence was not reused: %q %q %q", reference, version, locale)
	}
	if reference, version, locale = currentMarketingConsentEvidence(snapshot, "contact_2", "sms"); reference != "" || version != "" || locale != "" {
		t.Fatalf("missing current consent invented evidence: %q %q %q", reference, version, locale)
	}
}

func TestSaveWithdrawalUsesCurrentPolicyWithoutClientResubmission(t *testing.T) {
	source := &capturingPreferenceSource{snapshot: &notificationcap.PreferenceSnapshot{
		CustomerNo: "cust_1", Version: 7,
		MarketingConsents: []notificationcap.MarketingConsentProjection{{
			ContactNo: "contact_1", Channel: notificationcap.ChannelSMS, Status: "opted_in",
			PolicyReference: "privacy/marketing", PolicyVersion: "2026-08", Locale: "zh-CN", Version: 3,
		}},
	}}
	service := &serviceImpl{bizctx: preferenceBizCtxStub{}, sources: notificationcap.Runtime{Preferences: source}}
	_, err := service.SaveOwnPreference(t.Context(), PreferenceWriteInput{
		ContactNo: "contact_1", Topic: "marketing", ChannelCode: "sms", Enabled: false,
		MarketingConsentAction: "withdrawn", ExpectedVersion: 7, IdempotencyKey: "withdraw-test-001",
	})
	if !errors.Is(err, errPreferenceUpdateCaptured) {
		t.Fatalf("withdrawal did not reach core command: %v", err)
	}
	if source.command.MarketingConsent == nil || source.command.MarketingConsent.PolicyReference != "privacy/marketing" ||
		source.command.MarketingConsent.PolicyVersion != "2026-08" || source.command.MarketingConsent.Locale != "zh-CN" {
		t.Fatalf("withdrawal did not reuse authoritative evidence: %+v", source.command.MarketingConsent)
	}
}

func TestSaveCustomerPreferenceRejectsWeComDirectRequest(t *testing.T) {
	service := &serviceImpl{bizctx: preferenceBizCtxStub{}}
	_, err := service.SaveOwnPreference(t.Context(), PreferenceWriteInput{
		ContactNo: "contact_1", Topic: "support", ChannelCode: "wechat_work", Enabled: true,
		ExpectedVersion: 1, IdempotencyKey: "wecom-direct-001",
	})
	if err == nil || !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("direct customer WeCom preference was not rejected: %v", err)
	}
}

func TestQuietHoursOnlyCommandDoesNotRequireOrRewriteMarketingConsent(t *testing.T) {
	service := &serviceImpl{}
	command, evidenceDigest, err := service.buildPreferenceCommand(t.Context(), PreferenceWriteInput{
		ExpectedVersion: 7, IdempotencyKey: "quiet-hours-only-001",
		QuietHours: &QuietHoursWriteInput{
			TimezoneName: "Asia/Shanghai", StartMinute: 1320, EndMinute: 480, Enabled: true,
		},
	})
	if err != nil {
		t.Fatalf("build quiet-hours-only command: %v", err)
	}
	if len(command.Changes) != 0 || command.MarketingConsent != nil || evidenceDigest != "" {
		t.Fatalf("quiet-hours-only command rewrote contact or consent state: command=%+v digest=%q", command, evidenceDigest)
	}
	if command.QuietHours == nil || command.QuietHours.TimezoneName != "Asia/Shanghai" ||
		command.QuietHours.StartMinute != 1320 || command.QuietHours.EndMinute != 480 || !command.QuietHours.Enabled {
		t.Fatalf("unexpected quiet-hours command: %+v", command.QuietHours)
	}
}

func TestQuietHoursOnlyCommandRejectsPartialContactShape(t *testing.T) {
	service := &serviceImpl{}
	_, _, err := service.buildPreferenceCommand(t.Context(), PreferenceWriteInput{
		ContactNo: "contact_1", ExpectedVersion: 7, IdempotencyKey: "quiet-hours-partial-001",
		QuietHours: &QuietHoursWriteInput{
			TimezoneName: "Asia/Shanghai", StartMinute: 1320, EndMinute: 480, Enabled: true,
		},
	})
	if err == nil || !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("partial contact preference shape was accepted: %v", err)
	}
}

func TestQuietHoursOnlyCommandRejectsUnrelatedPreferenceFields(t *testing.T) {
	service := &serviceImpl{}
	_, _, err := service.buildPreferenceCommand(t.Context(), PreferenceWriteInput{
		Enabled: true, ExpectedVersion: 7, IdempotencyKey: "quiet-hours-enabled-001",
		QuietHours: &QuietHoursWriteInput{
			TimezoneName: "Asia/Shanghai", StartMinute: 1320, EndMinute: 480, Enabled: true,
		},
	})
	if err == nil || !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("unrelated preference fields were silently ignored: %v", err)
	}
}

func TestWithdrawalRejectsStaleAuthoritativeSnapshot(t *testing.T) {
	service := &serviceImpl{sources: notificationcap.Runtime{Preferences: preferenceSourceStub{snapshot: &notificationcap.PreferenceSnapshot{
		CustomerNo: "cust_1", Version: 8,
	}}}}
	_, _, err := service.buildMarketingConsent(t.Context(), PreferenceWriteInput{
		ContactNo: "contact_1", Topic: "marketing", ChannelCode: "sms", Enabled: false,
		MarketingConsentAction: "withdrawn", ExpectedVersion: 7, IdempotencyKey: "withdraw-stale-001",
	})
	if err == nil || !bizerr.Is(err, CodeConflict) {
		t.Fatalf("stale withdrawal did not return a version conflict: %v", err)
	}
}

var errPreferenceUpdateCaptured = errors.New("preference update captured")

type capturingPreferenceSource struct {
	snapshot *notificationcap.PreferenceSnapshot
	command  notificationcap.PreferenceCommand
}

func (source *capturingPreferenceSource) Current(context.Context) (*notificationcap.PreferenceSnapshot, error) {
	return source.snapshot, nil
}

func (source *capturingPreferenceSource) UpdateCurrent(_ context.Context, command notificationcap.PreferenceCommand) (*notificationcap.PreferenceResult, error) {
	source.command = command
	return nil, errPreferenceUpdateCaptured
}

type preferenceBizCtxStub struct{}

func (preferenceBizCtxStub) Current(context.Context) bizctxcap.CurrentContext {
	return bizctxcap.CurrentContext{UserID: 1001, TenantID: notificationTenantID}
}

func TestGetOwnPreferencesRejectsMissingAuthoritativeSnapshot(t *testing.T) {
	service := &serviceImpl{sources: notificationcap.Runtime{Preferences: preferenceSourceStub{}}}
	if _, err := service.GetOwnPreferences(t.Context()); err == nil {
		t.Fatal("missing authoritative core snapshot was accepted")
	}
}
