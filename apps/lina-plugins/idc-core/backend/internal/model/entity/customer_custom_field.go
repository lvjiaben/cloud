// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerCustomField is the golang structure for table customer_custom_field.
type CustomerCustomField struct {
	Id                int64      `json:"id"                orm:"id"                 description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"          description:""`
	FieldNo           string     `json:"fieldNo"           orm:"field_no"           description:""`
	FieldKey          string     `json:"fieldKey"          orm:"field_key"          description:""`
	LabelZhCn         string     `json:"labelZhCn"         orm:"label_zh_cn"        description:""`
	LabelEnUs         string     `json:"labelEnUs"         orm:"label_en_us"        description:""`
	HelpZhCn          string     `json:"helpZhCn"          orm:"help_zh_cn"         description:""`
	HelpEnUs          string     `json:"helpEnUs"          orm:"help_en_us"         description:""`
	DataTypeCode      string     `json:"dataTypeCode"      orm:"data_type_code"     description:""`
	CustomerTypeCode  string     `json:"customerTypeCode"  orm:"customer_type_code" description:""`
	VisibilityCode    string     `json:"visibilityCode"    orm:"visibility_code"    description:""`
	SensitivityCode   string     `json:"sensitivityCode"   orm:"sensitivity_code"   description:"Controls masking, export, audit, and AI exclusion policy"`
	Required          bool       `json:"required"          orm:"required"           description:""`
	CustomerEditable  bool       `json:"customerEditable"  orm:"customer_editable"  description:""`
	MinimumLength     *int       `json:"minimumLength"     orm:"minimum_length"     description:""`
	MaximumLength     *int       `json:"maximumLength"     orm:"maximum_length"     description:""`
	MinimumNumber     *string    `json:"minimumNumber"     orm:"minimum_number"     description:""`
	MaximumNumber     *string    `json:"maximumNumber"     orm:"maximum_number"     description:""`
	ValidationPattern string     `json:"validationPattern" orm:"validation_pattern" description:"Bounded service-validated pattern; never evaluated by an unbounded backtracking engine"`
	Priority          int        `json:"priority"          orm:"priority"           description:""`
	StatusCode        string     `json:"statusCode"        orm:"status_code"        description:""`
	Version           int64      `json:"version"           orm:"version"            description:""`
	CreatedBy         int        `json:"createdBy"         orm:"created_by"         description:""`
	UpdatedBy         int        `json:"updatedBy"         orm:"updated_by"         description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"         description:""`
	UpdatedAt         *time.Time `json:"updatedAt"         orm:"updated_at"         description:""`
}
