// This file defines safe customer-facing custom-field, entitlement-candidate, and immutable ticket snapshot API contracts.
package v1

import "github.com/gogf/gf/v2/frame/g"

// SupportCustomFieldOption is one safe bilingual option projection.
type SupportCustomFieldOption struct {
	OptionNo  string `json:"optionNo" dc:"Stable custom field option number" eg:"SCO_compute"`
	NameZhCN  string `json:"nameZhCN" dc:"Chinese option name" eg:"\u8ba1\u7b97"`
	NameEnUS  string `json:"nameEnUS" dc:"English option name" eg:"Compute"`
	SortOrder int    `json:"sortOrder" dc:"Ascending display order" eg:"10"`
}

// SupportCustomFieldDefinition is one active customer-visible field definition.
type SupportCustomFieldDefinition struct {
	FieldNo         string                     `json:"fieldNo" dc:"Stable support custom field number" eg:"SCF_service_area"`
	FieldType       string                     `json:"fieldType" dc:"One of the seven supported field types" eg:"multi_select"`
	NameZhCN        string                     `json:"nameZhCN" dc:"Chinese field name" eg:"\u670d\u52a1\u8303\u56f4"`
	NameEnUS        string                     `json:"nameEnUS" dc:"English field name" eg:"Service area"`
	DescriptionZhCN string                     `json:"descriptionZhCN,omitempty" dc:"Optional Chinese field description" eg:"\u8bf7\u9009\u62e9\u9700\u8981\u652f\u6301\u7684\u8303\u56f4"`
	DescriptionEnUS string                     `json:"descriptionEnUS,omitempty" dc:"Optional English field description" eg:"Select the area that needs support"`
	Required        bool                       `json:"required" dc:"Whether a value is required" eg:"true"`
	SortOrder       int                        `json:"sortOrder" dc:"Ascending display order" eg:"10"`
	Options         []SupportCustomFieldOption `json:"options" dc:"Bounded active select options" eg:"SCO_compute"`
}

// TicketCustomFieldSubmission carries exactly one typed customer value.
type TicketCustomFieldSubmission struct {
	FieldNo      string   `json:"fieldNo" dc:"Custom field number" eg:"SCF_service_area"`
	TextValue    *string  `json:"textValue" dc:"Text value for a text field" eg:"The service cannot start"`
	NumberValue  *string  `json:"numberValue" dc:"Decimal value for a number field" eg:"12.50"`
	DateValue    *string  `json:"dateValue" dc:"Date value in YYYY-MM-DD format" eg:"2026-08-15"`
	BooleanValue *bool    `json:"booleanValue" dc:"Boolean value for a yes or no field" eg:"true"`
	OptionNos    []string `json:"optionNos" dc:"Selected option numbers for a select field" eg:"SCO_compute"`
}

// TicketCustomFieldValue is an immutable customer-visible submitted snapshot.
type TicketCustomFieldValue struct {
	FieldNo         string                     `json:"fieldNo" dc:"Custom field number" eg:"SCF_service_area"`
	FieldType       string                     `json:"fieldType" dc:"Frozen field type" eg:"multi_select"`
	NameZhCN        string                     `json:"nameZhCN" dc:"Frozen Chinese field name" eg:"\u670d\u52a1\u8303\u56f4"`
	NameEnUS        string                     `json:"nameEnUS" dc:"Frozen English field name" eg:"Service area"`
	DescriptionZhCN string                     `json:"descriptionZhCN,omitempty" dc:"Frozen Chinese field description" eg:"\u8bf7\u9009\u62e9\u9700\u8981\u652f\u6301\u7684\u8303\u56f4"`
	DescriptionEnUS string                     `json:"descriptionEnUS,omitempty" dc:"Frozen English field description" eg:"Select the area that needs support"`
	Required        bool                       `json:"required" dc:"Frozen required setting" eg:"true"`
	SortOrder       int                        `json:"sortOrder" dc:"Frozen display order" eg:"10"`
	NormalizedValue string                     `json:"normalizedValue" dc:"Immutable normalized submitted value" eg:"SCO_compute"`
	SelectedOptions []SupportCustomFieldOption `json:"selectedOptions" dc:"Immutable selected option labels" eg:"SCO_compute"`
}

// TicketRelatedEntitlement combines the immutable creation snapshot and current safe projection.
type TicketRelatedEntitlement struct {
	AssociationReference string `json:"associationReference" dc:"Opaque support-only reference used for permitted actions without exposing an internal entitlement number" eg:"sar_0123456789abcdef0123456789abcdef"`
	SnapshotNameZhCN     string `json:"snapshotNameZhCN" dc:"Chinese product name frozen at ticket creation" eg:"\u4e91\u670d\u52a1\u5668"`
	SnapshotNameEnUS     string `json:"snapshotNameEnUS" dc:"English product name frozen at ticket creation" eg:"Cloud server"`
	SnapshotStatus       string `json:"snapshotStatus" dc:"Entitlement status frozen at ticket creation" eg:"active"`
	SnapshotVersion      int64  `json:"snapshotVersion" dc:"Entitlement version frozen at ticket creation" eg:"3"`
	CurrentNameZhCN      string `json:"currentNameZhCN,omitempty" dc:"Current Chinese product name" eg:"\u4e91\u670d\u52a1\u5668"`
	CurrentNameEnUS      string `json:"currentNameEnUS,omitempty" dc:"Current English product name" eg:"Cloud server"`
	CurrentStatus        string `json:"currentStatus,omitempty" dc:"Current entitlement status" eg:"active"`
	CurrentVersion       int64  `json:"currentVersion,omitempty" dc:"Current entitlement version" eg:"4"`
	CurrentAvailable     bool   `json:"currentAvailable" dc:"Whether the current entitlement projection is available" eg:"true"`
}

// SupportCustomFieldListReq requests active customer-ticket definitions for the current customer.
type SupportCustomFieldListReq struct {
	g.Meta `path:"/account/support/custom-fields" method:"get" tags:"IDC Support Account" summary:"List active customer ticket custom fields" dc:"Returns only active customer-visible definitions for the authenticated customer ticket creation form."`
}

// SupportCustomFieldListRes returns the bounded customer-visible definition set.
type SupportCustomFieldListRes struct {
	Items []SupportCustomFieldDefinition `json:"items" dc:"Active customer-visible custom field definitions" eg:"SCF_service_area"`
}

// SupportEntitlementCandidateListReq requests one bounded entitlement candidate page.
type SupportEntitlementCandidateListReq struct {
	g.Meta               `path:"/account/support/entitlement-candidates" method:"get" tags:"IDC Support Account" summary:"List current customer non-terminal entitlements" dc:"Returns a bounded page of long-term entitlement product labels owned by the current customer and excludes terminated entitlements."`
	Keyword              string `json:"keyword" dc:"Optional case-insensitive product name prefix; empty returns all visible candidates" eg:"cloud"`
	AssociationReference string `json:"associationReference" dc:"Optional opaque support association reference used to resolve one owned candidate without exposing its internal entitlement number" eg:"sar_0123456789abcdef0123456789abcdef"`
	Cursor               string `json:"cursor" dc:"Opaque cursor from the previous page; empty starts from the first page" eg:"next-page-token"`
	Limit                int    `json:"limit" d:"25" v:"min:1|max:50" dc:"Maximum candidates to return from 1 through 50" eg:"25"`
}

// SupportEntitlementCandidate is one safe non-terminal long-term entitlement projection.
type SupportEntitlementCandidate struct {
	AssociationReference string `json:"associationReference" dc:"Opaque support-only selection reference" eg:"sar_0123456789abcdef0123456789abcdef"`
	NameZhCN             string `json:"nameZhCN" dc:"Current Chinese product label" eg:"\u4e91\u670d\u52a1\u5668"`
	NameEnUS             string `json:"nameEnUS" dc:"Current English product label" eg:"Cloud server"`
	Status               string `json:"status" dc:"Current entitlement status: active, suspended, or expired" eg:"active"`
}

// SupportEntitlementCandidateListRes returns one bounded cursor page.
type SupportEntitlementCandidateListRes struct {
	Items      []SupportEntitlementCandidate `json:"items" dc:"Bounded safe entitlement candidates" eg:"sar_0123456789abcdef0123456789abcdef"`
	NextCursor string                        `json:"nextCursor" dc:"Opaque cursor for the next page; empty means there is no next page" eg:"SVC_cloud_01"`
}
