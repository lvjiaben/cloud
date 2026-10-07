// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// Channel is the golang structure of table plugin_idc_payment_channel for DAO operations like Where/Data.
type Channel struct {
	g.Meta              `orm:"table:plugin_idc_payment_channel, do:true"`
	Id                  any        //
	TenantId            any        //
	ChannelNo           any        //
	ProviderCode        any        //
	DisplayName         any        //
	EndpointOrigin      any        //
	MerchantReference   any        //
	SecretRef           any        //
	SupportedModes      any        //
	SupportedCurrencies any        //
	CapabilitySnapshot  any        //
	StatusCode          any        //
	VerifiedAt          *time.Time //
	Version             any        //
	CreatedBy           any        //
	UpdatedBy           any        //
	CreatedAt           *time.Time //
	UpdatedAt           *time.Time //
	DeletedAt           *time.Time //
}
