// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// PurchaseServerLogin is the golang structure for table purchase_server_login.
type PurchaseServerLogin struct {
	Id                    int64      `json:"id"                    orm:"id"                     description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"              description:""`
	ServerLoginToken      string     `json:"serverLoginToken"      orm:"server_login_token"     description:""`
	CustomerNo            string     `json:"customerNo"            orm:"customer_no"            description:""`
	UserId                int64      `json:"userId"                orm:"user_id"                description:""`
	ProductVersionNo      string     `json:"productVersionNo"      orm:"product_version_no"     description:""`
	ConfigurationRevision string     `json:"configurationRevision" orm:"configuration_revision" description:""`
	CartItemNo            string     `json:"cartItemNo"            orm:"cart_item_no"           description:""`
	Hostname              string     `json:"hostname"              orm:"hostname"               description:""`
	PasswordSecretRef     string     `json:"passwordSecretRef"     orm:"password_secret_ref"    description:"Opaque resource-channel-scoped secretcap handle, never plaintext"`
	ChannelNo             string     `json:"channelNo"             orm:"channel_no"             description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"             description:""`
}
