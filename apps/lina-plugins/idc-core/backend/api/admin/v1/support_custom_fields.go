// This file defines the authorized support custom-field configuration and immutable ticket snapshot API contracts.
package v1

import "github.com/gogf/gf/v2/frame/g"

// SupportCustomFieldOption describes one current or frozen bilingual choice.
type SupportCustomFieldOption struct {
	OptionNo  string `json:"optionNo" dc:"Stable custom field option number" eg:"SCO_compute"`
	NameZhCN  string `json:"nameZhCN" dc:"Chinese option name" eg:"\u8ba1\u7b97"`
	NameEnUS  string `json:"nameEnUS" dc:"English option name" eg:"Compute"`
	SortOrder int    `json:"sortOrder" dc:"Ascending display order" eg:"10"`
	Status    string `json:"status" dc:"active or disabled" eg:"active"`
	Version   int64  `json:"version" dc:"Optimistic concurrency version" eg:"1"`
}

// SupportCustomFieldDefinition is the complete authorized field configuration projection.
type SupportCustomFieldDefinition struct {
	FieldNo         string                     `json:"fieldNo" dc:"Stable support custom field number" eg:"SCF_service_area"`
	FieldType       string                     `json:"fieldType" dc:"One of the seven supported field types" eg:"multi_select"`
	NameZhCN        string                     `json:"nameZhCN" dc:"Chinese field name" eg:"\u670d\u52a1\u8303\u56f4"`
	NameEnUS        string                     `json:"nameEnUS" dc:"English field name" eg:"Service area"`
	DescriptionZhCN string                     `json:"descriptionZhCN,omitempty" dc:"Optional Chinese field description" eg:"\u8bf7\u9009\u62e9\u9700\u8981\u652f\u6301\u7684\u8303\u56f4"`
	DescriptionEnUS string                     `json:"descriptionEnUS,omitempty" dc:"Optional English field description" eg:"Select the area that needs support"`
	AppliesPresales bool                       `json:"appliesPresales" dc:"Whether the field applies to presales tickets" eg:"false"`
	AppliesCustomer bool                       `json:"appliesCustomer" dc:"Whether the field applies to customer tickets" eg:"true"`
	AppliesInternal bool                       `json:"appliesInternal" dc:"Whether the field applies to internal tickets" eg:"false"`
	Visibility      string                     `json:"visibility" dc:"customer_visible or internal_only" eg:"customer_visible"`
	Required        bool                       `json:"required" dc:"Whether a value is required when creating the ticket" eg:"true"`
	SortOrder       int                        `json:"sortOrder" dc:"Ascending display order" eg:"10"`
	Status          string                     `json:"status" dc:"active or disabled" eg:"active"`
	Version         int64                      `json:"version" dc:"Optimistic concurrency version" eg:"1"`
	Options         []SupportCustomFieldOption `json:"options" dc:"Bounded select options" eg:"SCO_compute"`
}

// SupportCustomFieldOptionInput carries one current or newly requested field option.
type SupportCustomFieldOptionInput struct {
	OptionNo        string `json:"optionNo" dc:"Existing option number; empty for a new option" eg:"SCO_compute"`
	NameZhCN        string `json:"nameZhCN" dc:"Chinese option name" eg:"\u8ba1\u7b97"`
	NameEnUS        string `json:"nameEnUS" dc:"English option name" eg:"Compute"`
	SortOrder       int    `json:"sortOrder" dc:"Ascending display order" eg:"10"`
	Status          string `json:"status" d:"active" dc:"active or disabled" eg:"active"`
	ExpectedVersion int64  `json:"expectedVersion" dc:"Existing option version; zero for a new option" eg:"1"`
}

// SupportCustomFieldSettingInput carries a complete bounded field configuration.
type SupportCustomFieldSettingInput struct {
	FieldType       string                          `json:"fieldType" dc:"One of the seven supported field types" eg:"multi_select"`
	NameZhCN        string                          `json:"nameZhCN" dc:"Chinese field name" eg:"\u670d\u52a1\u8303\u56f4"`
	NameEnUS        string                          `json:"nameEnUS" dc:"English field name" eg:"Service area"`
	DescriptionZhCN string                          `json:"descriptionZhCN" dc:"Optional Chinese field description" eg:"\u8bf7\u9009\u62e9\u9700\u8981\u652f\u6301\u7684\u8303\u56f4"`
	DescriptionEnUS string                          `json:"descriptionEnUS" dc:"Optional English field description" eg:"Select the area that needs support"`
	AppliesPresales bool                            `json:"appliesPresales" dc:"Whether the field applies to presales tickets" eg:"false"`
	AppliesCustomer bool                            `json:"appliesCustomer" dc:"Whether the field applies to customer tickets" eg:"true"`
	AppliesInternal bool                            `json:"appliesInternal" dc:"Whether the field applies to internal tickets" eg:"false"`
	Visibility      string                          `json:"visibility" dc:"customer_visible or internal_only" eg:"customer_visible"`
	Required        bool                            `json:"required" dc:"Whether a value is required when creating the ticket" eg:"true"`
	SortOrder       int                             `json:"sortOrder" dc:"Ascending display order" eg:"10"`
	Status          string                          `json:"status" d:"active" dc:"active or disabled" eg:"active"`
	Options         []SupportCustomFieldOptionInput `json:"options" dc:"Complete bounded option set for select fields" eg:"SCO_compute"`
}

// SupportCustomFieldSettingListReq requests the authorized complete settings directory.
type SupportCustomFieldSettingListReq struct {
	g.Meta `path:"/admin/support/settings/custom-fields" method:"get" tags:"IDC Support Settings" summary:"List support custom field settings" dc:"Returns the bounded complete configuration directory, including disabled definitions, for authorized IDC support administrators." permission:"idc:admin:support:configuration:manage"`
}

// SupportCustomFieldSettingListRes returns the bounded settings directory.
type SupportCustomFieldSettingListRes struct {
	Items []SupportCustomFieldDefinition `json:"items" dc:"Support custom field settings" eg:"SCF_service_area"`
}

// SupportCustomFieldSettingCreateReq creates one definition and its option set.
type SupportCustomFieldSettingCreateReq struct {
	g.Meta `path:"/admin/support/settings/custom-fields" method:"post" tags:"IDC Support Settings" summary:"Create a support custom field" dc:"Creates one validated bilingual custom field definition and its bounded option set for selected ticket types." permission:"idc:admin:support:configuration:manage"`
	SupportCustomFieldSettingInput
}

// SupportCustomFieldSettingCreateRes returns the created or updated definition.
type SupportCustomFieldSettingCreateRes struct {
	Item *SupportCustomFieldDefinition `json:"item" dc:"Created support custom field" eg:"SCF_service_area"`
}

// SupportCustomFieldSettingUpdateReq replaces one configuration with optimistic concurrency.
type SupportCustomFieldSettingUpdateReq struct {
	g.Meta          `path:"/admin/support/settings/custom-fields/{fieldNo}" method:"put" tags:"IDC Support Settings" summary:"Update a support custom field" dc:"Updates one visible configuration with optimistic concurrency while preserving every previously submitted ticket snapshot." permission:"idc:admin:support:configuration:manage"`
	FieldNo         string `json:"fieldNo" dc:"Support custom field number" eg:"SCF_service_area"`
	ExpectedVersion int64  `json:"expectedVersion" dc:"Version returned by the latest read" eg:"1"`
	SupportCustomFieldSettingInput
}
type SupportCustomFieldSettingUpdateRes = SupportCustomFieldSettingCreateRes

// SupportCustomFieldSettingDeleteReq disables one definition with optimistic concurrency.
type SupportCustomFieldSettingDeleteReq struct {
	g.Meta          `path:"/admin/support/settings/custom-fields/{fieldNo}" method:"delete" tags:"IDC Support Settings" summary:"Disable a support custom field" dc:"Disables one definition with optimistic concurrency without deleting its historical ticket snapshots." permission:"idc:admin:support:configuration:manage"`
	FieldNo         string `json:"fieldNo" dc:"Support custom field number" eg:"SCF_service_area"`
	ExpectedVersion int64  `json:"expectedVersion" dc:"Version returned by the latest read" eg:"1"`
}
type SupportCustomFieldSettingDeleteRes = SupportCustomFieldSettingCreateRes

// SupportInternalCustomFieldListReq requests active internal-only creation definitions.
type SupportInternalCustomFieldListReq struct {
	g.Meta `path:"/admin/support/custom-fields/internal" method:"get" tags:"IDC Support" summary:"List active internal ticket custom fields" dc:"Returns only active internal-only definitions that an authorized employee may submit when creating an internal ticket." permission:"idc:admin:support:ticket:create"`
}

// SupportInternalCustomFieldListRes returns active internal-only creation definitions.
type SupportInternalCustomFieldListRes struct {
	Items []SupportCustomFieldDefinition `json:"items" dc:"Active internal-only custom field definitions" eg:"SCF_internal_reason"`
}

// TicketCustomFieldSubmission carries exactly one typed submitted value.
type TicketCustomFieldSubmission struct {
	FieldNo      string   `json:"fieldNo" dc:"Custom field number" eg:"SCF_internal_reason"`
	TextValue    *string  `json:"textValue" dc:"Text value for a text field" eg:"Customer requested a manual review"`
	NumberValue  *string  `json:"numberValue" dc:"Decimal value for a number field" eg:"12.50"`
	DateValue    *string  `json:"dateValue" dc:"Date value in YYYY-MM-DD format" eg:"2026-08-15"`
	BooleanValue *bool    `json:"booleanValue" dc:"Boolean value for a yes or no field" eg:"true"`
	OptionNos    []string `json:"optionNos" dc:"Selected option numbers for a select field" eg:"SCO_compute"`
}

// TicketCustomFieldSelectedOption is an immutable option-label snapshot without current configuration state.
type TicketCustomFieldSelectedOption struct {
	OptionNo  string `json:"optionNo" dc:"Frozen selected option number" eg:"SCO_compute"`
	NameZhCN  string `json:"nameZhCN" dc:"Frozen selected option Chinese name" eg:"\u8ba1\u7b97"`
	NameEnUS  string `json:"nameEnUS" dc:"Frozen selected option English name" eg:"Compute"`
	SortOrder int    `json:"sortOrder" dc:"Frozen selected option display order" eg:"10"`
}

// TicketCustomFieldValue is an immutable field, value, and selected-option snapshot.
type TicketCustomFieldValue struct {
	FieldNo           string                            `json:"fieldNo" dc:"Custom field number" eg:"SCF_service_area"`
	FieldType         string                            `json:"fieldType" dc:"Frozen field type" eg:"multi_select"`
	NameZhCN          string                            `json:"nameZhCN" dc:"Frozen Chinese field name" eg:"\u670d\u52a1\u8303\u56f4"`
	NameEnUS          string                            `json:"nameEnUS" dc:"Frozen English field name" eg:"Service area"`
	DescriptionZhCN   string                            `json:"descriptionZhCN,omitempty" dc:"Frozen Chinese field description" eg:"\u8bf7\u9009\u62e9\u9700\u8981\u652f\u6301\u7684\u8303\u56f4"`
	DescriptionEnUS   string                            `json:"descriptionEnUS,omitempty" dc:"Frozen English field description" eg:"Select the area that needs support"`
	Visibility        string                            `json:"visibility" dc:"Frozen field visibility" eg:"customer_visible"`
	Required          bool                              `json:"required" dc:"Frozen required setting" eg:"true"`
	SortOrder         int                               `json:"sortOrder" dc:"Frozen display order" eg:"10"`
	DefinitionVersion int64                             `json:"definitionVersion" dc:"Frozen definition version" eg:"1"`
	NormalizedValue   string                            `json:"normalizedValue" dc:"Immutable normalized submitted value" eg:"SCO_compute"`
	SelectedOptions   []TicketCustomFieldSelectedOption `json:"selectedOptions" dc:"Immutable selected option labels" eg:"SCO_compute"`
}

// TicketRelatedEntitlement combines the immutable creation snapshot and current safe projection.
type TicketRelatedEntitlement struct {
	EntitlementNo    string `json:"entitlementNo" dc:"Opaque related entitlement number used only as an action key" eg:"ent_0123456789abcdef0123456789abcdef"`
	SnapshotNameZhCN string `json:"snapshotNameZhCN" dc:"Chinese product name frozen at ticket creation" eg:"\u4e91\u670d\u52a1\u5668"`
	SnapshotNameEnUS string `json:"snapshotNameEnUS" dc:"English product name frozen at ticket creation" eg:"Cloud server"`
	SnapshotStatus   string `json:"snapshotStatus" dc:"Entitlement status frozen at ticket creation" eg:"active"`
	SnapshotVersion  int64  `json:"snapshotVersion" dc:"Entitlement version frozen at ticket creation" eg:"3"`
	CurrentNameZhCN  string `json:"currentNameZhCN,omitempty" dc:"Current Chinese product name" eg:"\u4e91\u670d\u52a1\u5668"`
	CurrentNameEnUS  string `json:"currentNameEnUS,omitempty" dc:"Current English product name" eg:"Cloud server"`
	CurrentStatus    string `json:"currentStatus,omitempty" dc:"Current entitlement status" eg:"active"`
	CurrentVersion   int64  `json:"currentVersion,omitempty" dc:"Current entitlement version" eg:"4"`
	CurrentAvailable bool   `json:"currentAvailable" dc:"Whether the current entitlement projection is available" eg:"true"`
}
