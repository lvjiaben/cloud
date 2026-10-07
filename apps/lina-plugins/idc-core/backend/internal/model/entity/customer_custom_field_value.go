// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerCustomFieldValue is the golang structure for table customer_custom_field_value.
type CustomerCustomFieldValue struct {
	Id               int64      `json:"id"               orm:"id"                 description:""`
	TenantId         int        `json:"tenantId"         orm:"tenant_id"          description:""`
	ValueNo          string     `json:"valueNo"          orm:"value_no"           description:""`
	CustomerNo       string     `json:"customerNo"       orm:"customer_no"        description:""`
	FieldNo          string     `json:"fieldNo"          orm:"field_no"           description:""`
	TextValue        *string    `json:"textValue"        orm:"text_value"         description:""`
	TextLookupDigest string     `json:"textLookupDigest" orm:"text_lookup_digest" description:"Keyed normalized digest for exact filtering without indexing long plaintext"`
	SecretValueRef   string     `json:"secretValueRef"   orm:"secret_value_ref"   description:"Opaque core secret-envelope reference used instead of plaintext for sensitive custom values"`
	IntegerValue     *int64     `json:"integerValue"     orm:"integer_value"      description:""`
	DecimalValue     *string    `json:"decimalValue"     orm:"decimal_value"      description:""`
	BooleanValue     *bool      `json:"booleanValue"     orm:"boolean_value"      description:""`
	DateValue        *time.Time `json:"dateValue"        orm:"date_value"         description:""`
	DatetimeValue    *time.Time `json:"datetimeValue"    orm:"datetime_value"     description:""`
	StatusCode       string     `json:"statusCode"       orm:"status_code"        description:""`
	Version          int64      `json:"version"          orm:"version"            description:""`
	CreatedBy        int        `json:"createdBy"        orm:"created_by"         description:""`
	UpdatedBy        int        `json:"updatedBy"        orm:"updated_by"         description:""`
	CreatedAt        *time.Time `json:"createdAt"        orm:"created_at"         description:""`
	UpdatedAt        *time.Time `json:"updatedAt"        orm:"updated_at"         description:""`
}
