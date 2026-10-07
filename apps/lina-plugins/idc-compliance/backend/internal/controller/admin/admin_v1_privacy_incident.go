// Privacy incident controllers create, list, and advance local response records. They never trigger external notification without explicit service approval.

package admin

import (
	"context"

	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/privacygovernance"
)

func (c *ControllerV1) PrivacyIncidentList(ctx context.Context, req *v1.PrivacyIncidentListReq) (*v1.PrivacyIncidentListRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(privacygovernance.CodeInvalid)
	}
	items, err := c.privacyGovernance.ListIncidents(ctx, req.Limit)
	if err != nil {
		return nil, err
	}
	result := &v1.PrivacyIncidentListRes{Items: make([]v1.PrivacyIncidentItem, 0, len(items))}
	for index := range items {
		result.Items = append(result.Items, privacyIncidentItem(&items[index]))
	}
	return result, nil
}

func (c *ControllerV1) PrivacyIncidentCreate(ctx context.Context, req *v1.PrivacyIncidentCreateReq) (*v1.PrivacyIncidentCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(privacygovernance.CodeInvalid)
	}
	item, err := c.privacyGovernance.CreateIncident(ctx, privacygovernance.IncidentCreateInput{SeverityCode: req.SeverityCode, SourceCode: req.SourceCode, SafeSummary: req.SafeSummary, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.PrivacyIncidentCreateRes{Item: privacyIncidentItem(item)}, nil
}

func (c *ControllerV1) PrivacyIncidentAdvanceCreate(ctx context.Context, req *v1.PrivacyIncidentAdvanceCreateReq) (*v1.PrivacyIncidentAdvanceCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(privacygovernance.CodeInvalid)
	}
	item, err := c.privacyGovernance.AdvanceIncident(ctx, privacygovernance.IncidentAdvanceInput{IncidentNo: req.IncidentNo, ExpectedVersion: req.ExpectedVersion, NextStageCode: req.NextStageCode, SafeNote: req.SafeNote, ConfirmExternalNotification: req.ConfirmExternalNotification, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.PrivacyIncidentAdvanceCreateRes{Item: privacyIncidentItem(item)}, nil
}

func (c *ControllerV1) PrivacySupportExportCreate(ctx context.Context, req *v1.PrivacySupportExportCreateReq) (*v1.PrivacySupportExportCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(privacygovernance.CodeInvalid)
	}
	item, err := c.privacyGovernance.GenerateSupportExport(ctx, privacygovernance.SupportExportInput{SupportAgreementConfirmed: req.SupportAgreementConfirmed, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.PrivacySupportExportCreateRes{ExportNo: item.ExportNo, FileName: item.FileName, Content: string(item.Content), ContentSHA256: item.ContentSHA256, CreatedAt: item.CreatedAt.UnixMilli()}, nil
}

func privacyIncidentItem(item *privacygovernance.Incident) v1.PrivacyIncidentItem {
	if item == nil {
		return v1.PrivacyIncidentItem{}
	}
	result := v1.PrivacyIncidentItem{IncidentNo: item.IncidentNo, StatusCode: item.StatusCode, SeverityCode: item.SeverityCode, SourceCode: item.SourceCode, SafeSummary: item.SafeSummary, TicketGeneration: item.TicketGeneration, ExportBlocked: item.ExportBlocked, ExternalNotificationConfirmed: item.ExternalNotificationConfirmed, CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli(), Version: item.Version, Timeline: make([]v1.PrivacyIncidentEventItem, 0, len(item.Timeline))}
	if item.ResolvedAt != nil {
		result.ResolvedAt = item.ResolvedAt.UnixMilli()
	}
	for _, event := range item.Timeline {
		result.Timeline = append(result.Timeline, v1.PrivacyIncidentEventItem{EventNo: event.EventNo, StageCode: event.StageCode, SafeNote: event.SafeNote, ActorUserID: event.ActorUserID, OccurredAt: event.OccurredAt.UnixMilli()})
	}
	return result
}
