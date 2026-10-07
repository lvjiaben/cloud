// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CustomerAddress is the golang structure of table plugin_idc_core_customer_address for DAO operations like Where/Data.
type CustomerAddress struct {
	g.Meta          `orm:"table:plugin_idc_core_customer_address, do:true"`
	Id              any        //
	TenantId        any        //
	AddressNo       any        //
	CustomerNo      any        //
	AddressTypeCode any        //
	Label           any        //
	RecipientName   any        //
	RecipientPhone  any        //
	CountryCode     any        //
	Province        any        //
	City            any        //
	District        any        //
	AddressLine1    any        //
	AddressLine2    any        //
	PostalCode      any        //
	IsDefault       any        //
	StatusCode      any        //
	Version         any        //
	CreatedBy       any        //
	UpdatedBy       any        //
	CreatedAt       *time.Time //
	UpdatedAt       *time.Time //
}
