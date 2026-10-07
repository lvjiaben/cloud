// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// Channel is the golang structure for table channel.
type Channel struct {
	Id                  int64      `json:"id"                  orm:"id"                   description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"            description:""`
	ChannelNo           string     `json:"channelNo"           orm:"channel_no"           description:""`
	ProviderCode        string     `json:"providerCode"        orm:"provider_code"        description:""`
	DisplayName         string     `json:"displayName"         orm:"display_name"         description:""`
	EndpointOrigin      string     `json:"endpointOrigin"      orm:"endpoint_origin"      description:""`
	MerchantReference   string     `json:"merchantReference"   orm:"merchant_reference"   description:""`
	SecretRef           string     `json:"secretRef"           orm:"secret_ref"           description:""`
	SupportedModes      string     `json:"supportedModes"      orm:"supported_modes"      description:""`
	SupportedCurrencies string     `json:"supportedCurrencies" orm:"supported_currencies" description:""`
	CapabilitySnapshot  string     `json:"capabilitySnapshot"  orm:"capability_snapshot"  description:""`
	StatusCode          string     `json:"statusCode"          orm:"status_code"          description:""`
	VerifiedAt          *time.Time `json:"verifiedAt"          orm:"verified_at"          description:""`
	Version             int64      `json:"version"             orm:"version"              description:""`
	CreatedBy           int        `json:"createdBy"           orm:"created_by"           description:""`
	UpdatedBy           int        `json:"updatedBy"           orm:"updated_by"           description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"           description:""`
	UpdatedAt           *time.Time `json:"updatedAt"           orm:"updated_at"           description:""`
	DeletedAt           *time.Time `json:"deletedAt"           orm:"deleted_at"           description:""`
}
