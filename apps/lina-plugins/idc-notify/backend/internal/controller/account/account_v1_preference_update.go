package account

import (
	"context"

	"lina-plugin-idc-notify/backend/api/account/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) PreferenceUpdate(ctx context.Context, req *v1.PreferenceUpdateReq) (res *v1.PreferenceUpdateRes, err error) {
	input := notification.PreferenceWriteInput{ContactNo: req.ContactNo, Topic: req.Topic,
		ChannelCode: req.ChannelCode, Enabled: req.Enabled,
		MarketingConsentAction: req.MarketingConsentAction, PolicyReference: req.PolicyReference,
		PolicyVersion: req.PolicyVersion, Locale: req.Locale,
		ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey}
	if req.QuietHours != nil {
		input.QuietHours = &notification.QuietHoursWriteInput{TimezoneName: req.QuietHours.TimezoneName,
			StartMinute: req.QuietHours.StartMinute, EndMinute: req.QuietHours.EndMinute, Enabled: req.QuietHours.Enabled}
	}
	result, err := c.notifications.SaveOwnPreference(ctx, input)
	if err != nil {
		return nil, err
	}
	return &v1.PreferenceUpdateRes{Version: result.Version, EvidenceDigest: result.EvidenceDigest,
		AppliedAt: result.AppliedAt.UnixMilli(), Reused: result.Reused}, nil
}
