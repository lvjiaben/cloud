package notificationpolicy

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

type marketingGate struct{ service marketing.EngagementService }

var _ notificationcap.MarketingSendGate = (*marketingGate)(nil)

func NewMarketingGate(service marketing.EngagementService) (notificationcap.MarketingSendGate, error) {
	if service == nil {
		return nil, gerror.New("notification marketing gate requires engagement service")
	}
	return &marketingGate{service: service}, nil
}

func (g *marketingGate) CheckSendBatch(
	ctx context.Context,
	inputs []notificationcap.MarketingSendInput,
) ([]notificationcap.MarketingSendDecision, error) {
	mapped := make([]marketing.MarketingSendGateInput, len(inputs))
	for index, input := range inputs {
		mapped[index] = marketing.MarketingSendGateInput{
			CampaignNo: input.CampaignNo, SnapshotNo: input.SnapshotNo,
			CustomerNo: input.CustomerNo, ContactNo: input.ContactNo,
			Channel: string(input.Channel), ContentDigest: input.ContentDigest,
		}
	}
	decisions, err := g.service.CheckSendBatch(ctx, mapped)
	if err != nil {
		return nil, err
	}
	result := make([]notificationcap.MarketingSendDecision, len(decisions))
	for index, decision := range decisions {
		result[index] = notificationcap.MarketingSendDecision{
			CampaignNo: decision.CampaignNo, SnapshotNo: decision.SnapshotNo,
			CustomerNo: decision.CustomerNo, ContactNo: decision.ContactNo,
			Channel: notificationcap.Channel(decision.Channel), Allowed: decision.Allowed,
			ReasonCode: decision.ReasonCode, ConsentEventNo: decision.ConsentEventNo,
			ConsentVersion: decision.ConsentVersion, PreferenceVersion: decision.PreferenceVersion,
			QuietHoursVersion: decision.QuietHoursVersion, EvaluatedAt: decision.EvaluatedAt,
			DecisionDigest: decision.DecisionDigest,
		}
	}
	return result, nil
}
