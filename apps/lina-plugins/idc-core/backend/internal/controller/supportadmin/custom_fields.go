// This file maps authorized support custom-field settings and immutable snapshots between HTTP DTOs and the support service.
package supportadmin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/supportdesk"
)

// SupportCustomFieldSettingList returns the bounded complete settings directory to an authorized administrator.
func (c *ControllerV1) SupportCustomFieldSettingList(ctx context.Context, _ *v1.SupportCustomFieldSettingListReq) (*v1.SupportCustomFieldSettingListRes, error) {
	items, err := c.service.CustomFieldSettings(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.SupportCustomFieldSettingListRes{Items: toAdminCustomFieldDefinitions(items)}, nil
}

// SupportCustomFieldSettingCreate validates and creates one bilingual field definition.
func (c *ControllerV1) SupportCustomFieldSettingCreate(ctx context.Context, req *v1.SupportCustomFieldSettingCreateReq) (*v1.SupportCustomFieldSettingCreateRes, error) {
	item, err := c.service.CreateCustomFieldSetting(ctx, fromAdminCustomFieldSetting(req.SupportCustomFieldSettingInput, 0))
	if err != nil {
		return nil, err
	}
	return &v1.SupportCustomFieldSettingCreateRes{Item: toAdminCustomFieldDefinition(*item)}, nil
}

// SupportCustomFieldSettingUpdate applies one optimistic configuration update without rewriting ticket snapshots.
func (c *ControllerV1) SupportCustomFieldSettingUpdate(ctx context.Context, req *v1.SupportCustomFieldSettingUpdateReq) (*v1.SupportCustomFieldSettingUpdateRes, error) {
	item, err := c.service.UpdateCustomFieldSetting(ctx, req.FieldNo, fromAdminCustomFieldSetting(req.SupportCustomFieldSettingInput, req.ExpectedVersion))
	if err != nil {
		return nil, err
	}
	return &v1.SupportCustomFieldSettingUpdateRes{Item: toAdminCustomFieldDefinition(*item)}, nil
}

// SupportCustomFieldSettingDelete disables one field definition and preserves historical references.
func (c *ControllerV1) SupportCustomFieldSettingDelete(ctx context.Context, req *v1.SupportCustomFieldSettingDeleteReq) (*v1.SupportCustomFieldSettingDeleteRes, error) {
	item, err := c.service.DisableCustomFieldSetting(ctx, req.FieldNo, req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	return &v1.SupportCustomFieldSettingDeleteRes{Item: toAdminCustomFieldDefinition(*item)}, nil
}

// SupportInternalCustomFieldList returns only active internal-only definitions to an employee with create permission.
func (c *ControllerV1) SupportInternalCustomFieldList(ctx context.Context, _ *v1.SupportInternalCustomFieldListReq) (*v1.SupportInternalCustomFieldListRes, error) {
	items, err := c.service.StaffInternalCustomFields(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.SupportInternalCustomFieldListRes{Items: toAdminCustomFieldDefinitions(items)}, nil
}

func fromAdminCustomFieldSetting(input v1.SupportCustomFieldSettingInput, expectedVersion int64) supportdesk.CustomFieldSettingInput {
	options := make([]supportdesk.CustomFieldOptionInput, 0, len(input.Options))
	for _, option := range input.Options {
		options = append(options, supportdesk.CustomFieldOptionInput{OptionNo: option.OptionNo, NameZhCN: option.NameZhCN, NameEnUS: option.NameEnUS, SortOrder: option.SortOrder, Status: supportdesk.SettingStatus(option.Status), ExpectedVersion: option.ExpectedVersion})
	}
	return supportdesk.CustomFieldSettingInput{FieldType: input.FieldType, NameZhCN: input.NameZhCN, NameEnUS: input.NameEnUS, DescriptionZhCN: input.DescriptionZhCN, DescriptionEnUS: input.DescriptionEnUS, AppliesPresales: input.AppliesPresales, AppliesCustomer: input.AppliesCustomer, AppliesInternal: input.AppliesInternal, Visibility: input.Visibility, Required: input.Required, SortOrder: input.SortOrder, Status: supportdesk.SettingStatus(input.Status), ExpectedVersion: expectedVersion, Options: options}
}

func fromAdminCustomFieldSubmissions(items []v1.TicketCustomFieldSubmission) []supportdesk.CustomFieldSubmission {
	result := make([]supportdesk.CustomFieldSubmission, 0, len(items))
	for _, item := range items {
		result = append(result, supportdesk.CustomFieldSubmission{FieldNo: item.FieldNo, TextValue: item.TextValue, NumberValue: item.NumberValue, DateValue: item.DateValue, BooleanValue: item.BooleanValue, OptionNos: append([]string(nil), item.OptionNos...)})
	}
	return result
}

func toAdminCustomFieldDefinitions(items []supportdesk.CustomFieldDefinition) []v1.SupportCustomFieldDefinition {
	result := make([]v1.SupportCustomFieldDefinition, 0, len(items))
	for _, item := range items {
		result = append(result, *toAdminCustomFieldDefinition(item))
	}
	return result
}

func toAdminCustomFieldDefinition(item supportdesk.CustomFieldDefinition) *v1.SupportCustomFieldDefinition {
	options := make([]v1.SupportCustomFieldOption, 0, len(item.Options))
	for _, option := range item.Options {
		options = append(options, v1.SupportCustomFieldOption{OptionNo: option.OptionNo, NameZhCN: option.NameZhCN, NameEnUS: option.NameEnUS, SortOrder: option.SortOrder, Status: string(option.Status), Version: option.Version})
	}
	return &v1.SupportCustomFieldDefinition{FieldNo: item.FieldNo, FieldType: item.FieldType, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS, DescriptionZhCN: item.DescriptionZhCN, DescriptionEnUS: item.DescriptionEnUS, AppliesPresales: item.AppliesPresales, AppliesCustomer: item.AppliesCustomer, AppliesInternal: item.AppliesInternal, Visibility: item.Visibility, Required: item.Required, SortOrder: item.SortOrder, Status: string(item.Status), Version: item.Version, Options: options}
}

func toAdminCustomFieldValues(items []supportdesk.CustomFieldValue) []v1.TicketCustomFieldValue {
	result := make([]v1.TicketCustomFieldValue, 0, len(items))
	for _, item := range items {
		options := make([]v1.TicketCustomFieldSelectedOption, 0, len(item.SelectedOptions))
		for _, option := range item.SelectedOptions {
			options = append(options, v1.TicketCustomFieldSelectedOption{OptionNo: option.OptionNo, NameZhCN: option.NameZhCN, NameEnUS: option.NameEnUS, SortOrder: option.SortOrder})
		}
		result = append(result, v1.TicketCustomFieldValue{FieldNo: item.FieldNo, FieldType: item.FieldType, NameZhCN: item.NameZhCN, NameEnUS: item.NameEnUS, DescriptionZhCN: item.DescriptionZhCN, DescriptionEnUS: item.DescriptionEnUS, Visibility: item.Visibility, Required: item.Required, SortOrder: item.SortOrder, DefinitionVersion: item.DefinitionVersion, NormalizedValue: item.NormalizedValue, SelectedOptions: options})
	}
	return result
}

func toAdminRelatedEntitlement(item *supportdesk.Ticket) *v1.TicketRelatedEntitlement {
	if item == nil || item.RelatedEntitlementNo == "" {
		return nil
	}
	return &v1.TicketRelatedEntitlement{EntitlementNo: item.RelatedEntitlementNo, SnapshotNameZhCN: item.RelatedEntitlementNameZhCN, SnapshotNameEnUS: item.RelatedEntitlementNameEnUS, SnapshotStatus: item.RelatedEntitlementStatus, SnapshotVersion: item.RelatedEntitlementVersion, CurrentNameZhCN: item.CurrentEntitlementNameZhCN, CurrentNameEnUS: item.CurrentEntitlementNameEnUS, CurrentStatus: item.CurrentEntitlementStatus, CurrentVersion: item.CurrentEntitlementVersion, CurrentAvailable: item.CurrentEntitlementAvailable}
}
