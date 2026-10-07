// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerAddress is the golang structure for table customer_address.
type CustomerAddress struct {
	Id              int64      `json:"id"              orm:"id"                description:""`
	TenantId        int        `json:"tenantId"        orm:"tenant_id"         description:""`
	AddressNo       string     `json:"addressNo"       orm:"address_no"        description:""`
	CustomerNo      string     `json:"customerNo"      orm:"customer_no"       description:""`
	AddressTypeCode string     `json:"addressTypeCode" orm:"address_type_code" description:""`
	Label           string     `json:"label"           orm:"label"             description:""`
	RecipientName   string     `json:"recipientName"   orm:"recipient_name"    description:""`
	RecipientPhone  string     `json:"recipientPhone"  orm:"recipient_phone"   description:""`
	CountryCode     string     `json:"countryCode"     orm:"country_code"      description:""`
	Province        string     `json:"province"        orm:"province"          description:""`
	City            string     `json:"city"            orm:"city"              description:""`
	District        string     `json:"district"        orm:"district"          description:""`
	AddressLine1    string     `json:"addressLine1"    orm:"address_line1"     description:""`
	AddressLine2    string     `json:"addressLine2"    orm:"address_line2"     description:""`
	PostalCode      string     `json:"postalCode"      orm:"postal_code"       description:""`
	IsDefault       bool       `json:"isDefault"       orm:"is_default"        description:""`
	StatusCode      string     `json:"statusCode"      orm:"status_code"       description:""`
	Version         int64      `json:"version"         orm:"version"           description:""`
	CreatedBy       int        `json:"createdBy"       orm:"created_by"        description:""`
	UpdatedBy       int        `json:"updatedBy"       orm:"updated_by"        description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"        description:""`
	UpdatedAt       *time.Time `json:"updatedAt"       orm:"updated_at"        description:""`
}
