// This file defines the bounded public projection of active presales custom-field definitions.
package v1

import "github.com/gogf/gf/v2/frame/g"

// PresalesCustomFieldOption is one safe bilingual public choice.
type PresalesCustomFieldOption struct {
	OptionNo  string `json:"optionNo" dc:"Stable custom field option number" eg:"SCO_compute"`
	NameZhCN  string `json:"nameZhCN" dc:"Chinese option name" eg:"\u8ba1\u7b97"`
	NameEnUS  string `json:"nameEnUS" dc:"English option name" eg:"Compute"`
	SortOrder int    `json:"sortOrder" dc:"Ascending display order" eg:"10"`
}

// PresalesCustomFieldDefinition is one active customer-visible public field definition.
type PresalesCustomFieldDefinition struct {
	FieldNo         string                      `json:"fieldNo" dc:"Stable support custom field number" eg:"SCF_presales_area"`
	FieldType       string                      `json:"fieldType" dc:"One of the seven supported field types" eg:"single_select"`
	NameZhCN        string                      `json:"nameZhCN" dc:"Chinese field name" eg:"\u54a8\u8be2\u65b9\u5411"`
	NameEnUS        string                      `json:"nameEnUS" dc:"English field name" eg:"Question area"`
	DescriptionZhCN string                      `json:"descriptionZhCN,omitempty" dc:"Optional Chinese field description" eg:"\u8bf7\u9009\u62e9\u54a8\u8be2\u65b9\u5411"`
	DescriptionEnUS string                      `json:"descriptionEnUS,omitempty" dc:"Optional English field description" eg:"Select the question area"`
	Required        bool                        `json:"required" dc:"Whether a value is required" eg:"true"`
	SortOrder       int                         `json:"sortOrder" dc:"Ascending display order" eg:"10"`
	Options         []PresalesCustomFieldOption `json:"options" dc:"Bounded active select options" eg:"SCO_compute"`
}

// PresalesCustomFieldListReq requests the bounded public presales definition set.
type PresalesCustomFieldListReq struct {
	g.Meta `path:"/public/support/presales/custom-fields" method:"get" tags:"IDC Public Support" summary:"List active presales custom fields" dc:"Returns the bounded active customer-visible definitions used by the public presales form without exposing internal fields."`
}

// PresalesCustomFieldListRes returns the bounded public presales definition set.
type PresalesCustomFieldListRes struct {
	Items []PresalesCustomFieldDefinition `json:"items" dc:"Active presales custom field definitions" eg:"SCF_presales_area"`
}
