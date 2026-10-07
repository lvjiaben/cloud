// This file maps bounded customer custom-field and related-entitlement projections without exposing internal settings.
package supportaccount

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/supportaccount/v1"
	"lina-plugin-idc-core/backend/internal/service/supportdesk"
)

// SupportCustomFieldList returns active customer-visible definitions for the current trusted customer.
func (c *ControllerV1) SupportCustomFieldList(ctx context.Context, _ *v1.SupportCustomFieldListReq) (*v1.SupportCustomFieldListRes, error) {
	items, err := c.service.AccountCustomFields(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.SupportCustomFieldListRes{Items: ToSafeCustomFieldDefinitions(items)}, nil
}

// SupportEntitlementCandidateList returns one bounded page of non-terminal long-term entitlements owned by the current customer.
func (c *ControllerV1) SupportEntitlementCandidateList(ctx context.Context, req *v1.SupportEntitlementCandidateListReq) (*v1.SupportEntitlementCandidateListRes, error) {
	page, err := c.service.SearchAccountEntitlementCandidates(ctx, req.Keyword, req.AssociationReference, req.Cursor, req.Limit)
	if err != nil {
		return nil, err
	}
	items := make([]v1.SupportEntitlementCandidate, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, v1.SupportEntitlementCandidate{AssociationReference: item.AssociationReference, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS, Status: item.Status})
	}
	return &v1.SupportEntitlementCandidateListRes{Items: items, NextCursor: page.NextCursor}, nil
}

// ToSafeCustomFieldDefinitions removes configuration-only and internal fields from an already authorized definition set.
func ToSafeCustomFieldDefinitions(items []supportdesk.CustomFieldDefinition) []v1.SupportCustomFieldDefinition {
	result := make([]v1.SupportCustomFieldDefinition, 0, len(items))
	for _, item := range items {
		options := make([]v1.SupportCustomFieldOption, 0, len(item.Options))
		for _, option := range item.Options {
			options = append(options, v1.SupportCustomFieldOption{OptionNo: option.OptionNo, NameZhCN: option.NameZhCN, NameEnUS: option.NameEnUS, SortOrder: option.SortOrder})
		}
		result = append(result, v1.SupportCustomFieldDefinition{FieldNo: item.FieldNo, FieldType: item.FieldType, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS, DescriptionZhCN: item.DescriptionZhCN, DescriptionEnUS: item.DescriptionEnUS, Required: item.Required, SortOrder: item.SortOrder, Options: options})
	}
	return result
}

func fromAccountCustomFieldSubmissions(items []v1.TicketCustomFieldSubmission) []supportdesk.CustomFieldSubmission {
	result := make([]supportdesk.CustomFieldSubmission, 0, len(items))
	for _, item := range items {
		result = append(result, supportdesk.CustomFieldSubmission{FieldNo: item.FieldNo, TextValue: item.TextValue, NumberValue: item.NumberValue, DateValue: item.DateValue, BooleanValue: item.BooleanValue, OptionNos: append([]string(nil), item.OptionNos...)})
	}
	return result
}

func toAccountCustomFieldValues(items []supportdesk.CustomFieldValue) []v1.TicketCustomFieldValue {
	result := make([]v1.TicketCustomFieldValue, 0, len(items))
	for _, item := range items {
		if item.Visibility != "customer_visible" {
			continue
		}
		options := make([]v1.SupportCustomFieldOption, 0, len(item.SelectedOptions))
		for _, option := range item.SelectedOptions {
			options = append(options, v1.SupportCustomFieldOption{OptionNo: option.OptionNo, NameZhCN: option.NameZhCN, NameEnUS: option.NameEnUS, SortOrder: option.SortOrder})
		}
		result = append(result, v1.TicketCustomFieldValue{FieldNo: item.FieldNo, FieldType: item.FieldType, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS, DescriptionZhCN: item.DescriptionZhCN, DescriptionEnUS: item.DescriptionEnUS, Required: item.Required, SortOrder: item.SortOrder, NormalizedValue: item.NormalizedValue, SelectedOptions: options})
	}
	return result
}

func toAccountRelatedEntitlement(item *supportdesk.Ticket) *v1.TicketRelatedEntitlement {
	if item == nil || item.RelatedEntitlementNo == "" {
		return nil
	}
	return &v1.TicketRelatedEntitlement{AssociationReference: item.EntitlementAssociationRef, SnapshotNameZhCN: item.RelatedEntitlementNameZhCN, SnapshotNameEnUS: item.RelatedEntitlementNameEnUS, SnapshotStatus: item.RelatedEntitlementStatus, SnapshotVersion: item.RelatedEntitlementVersion, CurrentNameZhCN: item.CurrentEntitlementNameZhCN, CurrentNameEnUS: item.CurrentEntitlementNameEnUS, CurrentStatus: item.CurrentEntitlementStatus, CurrentVersion: item.CurrentEntitlementVersion, CurrentAvailable: item.CurrentEntitlementAvailable}
}
