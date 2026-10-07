package account

import (
	"context"

	"lina-plugin-idc-notify/backend/api/account/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) PreferenceGet(ctx context.Context, _ *v1.PreferenceGetReq) (*v1.PreferenceGetRes, error) {
	snapshot, err := c.notifications.GetOwnPreferences(ctx)
	if err != nil {
		return nil, err
	}
	return projectPreferenceSnapshot(snapshot), nil
}

func projectPreferenceSnapshot(snapshot *notification.PreferenceSnapshot) *v1.PreferenceGetRes {
	preferences := make([]*v1.ContactPreferenceItem, len(snapshot.Preferences))
	for index, item := range snapshot.Preferences {
		preferences[index] = &v1.ContactPreferenceItem{
			ContactNo: item.ContactNo, Topic: item.Topic, ChannelCode: item.ChannelCode,
			Enabled: item.Enabled, Version: item.Version,
		}
	}
	consents := make([]*v1.MarketingConsentItem, len(snapshot.MarketingConsents))
	for index, item := range snapshot.MarketingConsents {
		consents[index] = &v1.MarketingConsentItem{
			ContactNo: item.ContactNo, ChannelCode: item.ChannelCode, Status: item.Status,
			PolicyReference: item.PolicyReference, PolicyVersion: item.PolicyVersion, Locale: item.Locale,
			Version: item.Version,
		}
	}
	result := &v1.PreferenceGetRes{
		CustomerNo: snapshot.CustomerNo, PreferredLocale: snapshot.PreferredLocale,
		Version: snapshot.Version, Preferences: preferences, MarketingConsents: consents,
	}
	if snapshot.QuietHours != nil {
		result.QuietHours = &v1.QuietHoursItem{
			TimezoneName: snapshot.QuietHours.TimezoneName,
			StartMinute:  snapshot.QuietHours.StartMinute, EndMinute: snapshot.QuietHours.EndMinute,
			CrossesMidnight: snapshot.QuietHours.CrossesMidnight,
			Enabled:         snapshot.QuietHours.Enabled, Version: snapshot.QuietHours.Version,
		}
	}
	return result
}
