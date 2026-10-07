package notification

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	modeldo "lina-plugin-idc-notify/backend/internal/model/do"
)

func (s *serviceImpl) GetOwnPreferences(ctx context.Context) (*PreferenceSnapshot, error) {
	snapshot, err := s.sources.Preferences.Current(ctx)
	if err != nil || snapshot == nil || snapshot.CustomerNo == "" || snapshot.Version < 0 ||
		len(snapshot.Preferences) > notificationcap.MaximumPreferenceItems ||
		len(snapshot.MarketingConsents) > notificationcap.MaximumPreferenceItems {
		return nil, publicError(errorOrCode(err, CodeUnavailable))
	}
	result := &PreferenceSnapshot{
		CustomerNo: snapshot.CustomerNo, PreferredLocale: snapshot.PreferredLocale,
		Version:           snapshot.Version,
		Preferences:       make([]ContactPreference, len(snapshot.Preferences)),
		MarketingConsents: make([]MarketingConsent, len(snapshot.MarketingConsents)),
	}
	for index, item := range snapshot.Preferences {
		if !validCustomerChannel(string(item.Channel)) {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		result.Preferences[index] = ContactPreference{
			ContactNo: item.ContactNo, Topic: string(item.Topic), ChannelCode: string(item.Channel),
			Enabled: item.Enabled, Version: item.Version,
		}
	}
	for index, item := range snapshot.MarketingConsents {
		if !validCustomerChannel(string(item.Channel)) {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		result.MarketingConsents[index] = MarketingConsent{
			ContactNo: item.ContactNo, ChannelCode: string(item.Channel), Status: item.Status,
			PolicyReference: item.PolicyReference, PolicyVersion: item.PolicyVersion, Locale: item.Locale,
			Version: item.Version,
		}
	}
	if snapshot.QuietHours != nil {
		result.QuietHours = &QuietHoursPreference{
			TimezoneName: snapshot.QuietHours.TimezoneName,
			StartMinute:  snapshot.QuietHours.StartMinute, EndMinute: snapshot.QuietHours.EndMinute,
			CrossesMidnight: snapshot.QuietHours.CrossesMidnight,
			Enabled:         snapshot.QuietHours.Enabled, Version: snapshot.QuietHours.Version,
		}
	}
	return result, nil
}

func (s *serviceImpl) SaveOwnPreference(ctx context.Context, input PreferenceWriteInput) (*PreferenceWriteResult, error) {
	userID, err := s.currentUser(ctx)
	if err != nil {
		return nil, err
	}
	command, evidenceDigest, err := s.buildPreferenceCommand(ctx, input)
	if err != nil {
		return nil, err
	}
	result, err := s.sources.Preferences.UpdateCurrent(ctx, command)
	if err != nil || result == nil || result.Version <= 0 || !isDigest(result.EvidenceDigest) || result.AppliedAt.IsZero() {
		return nil, publicError(errorOrCode(err, CodeUnavailable))
	}
	if err = s.recordPreferenceEvidence(ctx, userID, input, evidenceDigest, result); err != nil {
		return nil, err
	}
	return &PreferenceWriteResult{Version: result.Version, EvidenceDigest: result.EvidenceDigest,
		AppliedAt: result.AppliedAt.UTC(), Reused: result.Reused}, nil
}

func (s *serviceImpl) buildPreferenceCommand(ctx context.Context, input PreferenceWriteInput) (notificationcap.PreferenceCommand, string, error) {
	if input.ExpectedVersion < 0 || !validateIdempotencyKey(input.IdempotencyKey) {
		return notificationcap.PreferenceCommand{}, "", bizerr.NewCode(CodeInvalid)
	}
	command := notificationcap.PreferenceCommand{
		IdempotencyKey: input.IdempotencyKey, ExpectedVersion: input.ExpectedVersion,
	}
	quietHoursOnly := input.QuietHours != nil && input.ContactNo == "" && input.Topic == "" && input.ChannelCode == "" &&
		!input.Enabled && input.MarketingConsentAction == "" && input.PolicyReference == "" && input.PolicyVersion == "" && input.Locale == ""
	if !quietHoursOnly {
		if input.ContactNo == "" || len(input.ContactNo) > 80 || !validTopic(input.Topic) ||
			!validCustomerChannel(input.ChannelCode) {
			return notificationcap.PreferenceCommand{}, "", bizerr.NewCode(CodeInvalid)
		}
		command.Changes = []notificationcap.PreferenceChange{{ContactNo: input.ContactNo,
			Topic: notificationcap.Topic(input.Topic), Channel: notificationcap.Channel(input.ChannelCode), Enabled: input.Enabled}}
	}
	marketingConsent, evidenceDigest, err := s.buildMarketingConsent(ctx, input)
	if err != nil {
		return notificationcap.PreferenceCommand{}, "", err
	}
	quietHours, err := buildQuietHours(input)
	if err != nil {
		return notificationcap.PreferenceCommand{}, "", err
	}
	command.MarketingConsent = marketingConsent
	command.QuietHours = quietHours
	return command, evidenceDigest, nil
}

func (s *serviceImpl) buildMarketingConsent(ctx context.Context, input PreferenceWriteInput) (*notificationcap.MarketingConsentChange, string, error) {
	if input.Topic != string(notificationcap.TopicMarketing) {
		return nil, "", nil
	}
	if input.MarketingConsentAction != "granted" && input.MarketingConsentAction != "withdrawn" ||
		input.MarketingConsentAction == "withdrawn" && input.Enabled ||
		input.MarketingConsentAction == "granted" && !input.Enabled {
		return nil, "", bizerr.NewCode(CodeInvalid)
	}
	if input.MarketingConsentAction == "withdrawn" {
		if input.PolicyReference != "" || input.PolicyVersion != "" || input.Locale != "" {
			return nil, "", bizerr.NewCode(CodeInvalid)
		}
		snapshot, err := s.sources.Preferences.Current(ctx)
		if err != nil || snapshot == nil {
			return nil, "", publicError(errorOrCode(err, CodeUnavailable))
		}
		if snapshot.Version != input.ExpectedVersion {
			return nil, "", bizerr.NewCode(CodeConflict)
		}
		input.PolicyReference, input.PolicyVersion, input.Locale = currentMarketingConsentEvidence(
			snapshot, input.ContactNo, input.ChannelCode,
		)
	}
	if !validLocale(input.Locale) || input.PolicyReference == "" || len(input.PolicyReference) > 128 ||
		input.PolicyVersion == "" || len(input.PolicyVersion) > 64 {
		return nil, "", bizerr.NewCode(CodeInvalid)
	}
	evidenceDigest := digestText("account_preference", input.ContactNo, input.ChannelCode,
		input.MarketingConsentAction, input.PolicyReference, input.PolicyVersion, input.Locale, input.IdempotencyKey)
	return &notificationcap.MarketingConsentChange{
		ContactNo: input.ContactNo, Channel: notificationcap.Channel(input.ChannelCode),
		Action: input.MarketingConsentAction, PolicyReference: input.PolicyReference,
		PolicyVersion: input.PolicyVersion, Locale: input.Locale, EvidenceDigest: evidenceDigest,
	}, evidenceDigest, nil
}

func buildQuietHours(input PreferenceWriteInput) (*notificationcap.QuietHoursChange, error) {
	if input.QuietHours == nil {
		return nil, nil
	}
	if (input.Topic != "" && input.Topic != string(notificationcap.TopicMarketing)) || input.QuietHours.TimezoneName == "" ||
		len(input.QuietHours.TimezoneName) > 64 || input.QuietHours.StartMinute < 0 || input.QuietHours.StartMinute > 1439 ||
		input.QuietHours.EndMinute < 0 || input.QuietHours.EndMinute > 1439 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	return &notificationcap.QuietHoursChange{TimezoneName: input.QuietHours.TimezoneName,
		StartMinute: input.QuietHours.StartMinute, EndMinute: input.QuietHours.EndMinute, Enabled: input.QuietHours.Enabled}, nil
}

func (s *serviceImpl) recordPreferenceEvidence(ctx context.Context, userID int, input PreferenceWriteInput,
	evidenceDigest string, result *notificationcap.PreferenceResult) error {
	if evidenceDigest == "" {
		return nil
	}
	evidenceNo := stableNumber("nue_", input.IdempotencyKey, input.ContactNo, input.ChannelCode)
	_, err := s.database.Model("plugin_idc_notify_unsubscribe_evidence").Ctx(ctx).Data(modeldo.UnsubscribeEvidence{
		TenantId: notificationTenantID, EvidenceNo: evidenceNo,
		OwnerUserId: userID, CustomerNo: result.CustomerNo,
		ContactNo: input.ContactNo, ChannelCode: input.ChannelCode, TopicCode: input.Topic,
		SourceCode: "account_preference", EvidenceDigest: evidenceDigest, OccurredAt: utcTimePointer(&result.AppliedAt),
	}).InsertIgnore()
	return publicError(err)
}

func validTopic(value string) bool {
	switch notificationcap.Topic(value) {
	case notificationcap.TopicAccount, notificationcap.TopicBilling, notificationcap.TopicService,
		notificationcap.TopicSecurity, notificationcap.TopicSupport, notificationcap.TopicMarketing:
		return true
	default:
		return false
	}
}

func validCustomerChannel(value string) bool {
	switch notificationcap.Channel(value) {
	case notificationcap.ChannelInApp, notificationcap.ChannelEmail, notificationcap.ChannelSMS,
		notificationcap.ChannelWeChatOfficial:
		return true
	default:
		return false
	}
}

func currentMarketingConsentEvidence(snapshot *notificationcap.PreferenceSnapshot, contactNo, channelCode string) (string, string, string) {
	if snapshot == nil {
		return "", "", ""
	}
	for _, item := range snapshot.MarketingConsents {
		if item.ContactNo == contactNo && string(item.Channel) == channelCode {
			return item.PolicyReference, item.PolicyVersion, item.Locale
		}
	}
	return "", "", ""
}
