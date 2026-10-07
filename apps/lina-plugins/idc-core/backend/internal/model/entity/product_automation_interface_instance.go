// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ProductAutomationInterfaceInstance is the golang structure for table product_automation_interface_instance.
type ProductAutomationInterfaceInstance struct {
	Id                   int64      `json:"id"                   orm:"id"                     description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"              description:""`
	InterfaceInstanceNo  string     `json:"interfaceInstanceNo"  orm:"interface_instance_no"  description:""`
	DisplayName          string     `json:"displayName"          orm:"display_name"           description:""`
	PluginId             string     `json:"pluginId"             orm:"plugin_id"              description:""`
	ExtensionPointCode   string     `json:"extensionPointCode"   orm:"extension_point_code"   description:""`
	PluginReleaseVersion string     `json:"pluginReleaseVersion" orm:"plugin_release_version" description:""`
	ContractVersion      string     `json:"contractVersion"      orm:"contract_version"       description:""`
	AuthorizedMethods    []string   `json:"authorizedMethods"    orm:"authorized_methods"     description:""`
	ReleaseDigest        string     `json:"releaseDigest"        orm:"release_digest"         description:""`
	DeclarationDigest    string     `json:"declarationDigest"    orm:"declaration_digest"     description:""`
	NewBusinessState     string     `json:"newBusinessState"     orm:"new_business_state"     description:""`
	EverReferenced       bool       `json:"everReferenced"       orm:"ever_referenced"        description:""`
	Version              int64      `json:"version"              orm:"version"                description:""`
	CreatedBy            int        `json:"createdBy"            orm:"created_by"             description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"             description:""`
	ChangedBy            int        `json:"changedBy"            orm:"changed_by"             description:""`
	ChangedAt            *time.Time `json:"changedAt"            orm:"changed_at"             description:""`
	InterfaceOriginCode  string     `json:"interfaceOriginCode"  orm:"interface_origin_code"  description:""`
	ResourceOwnerId      string     `json:"resourceOwnerId"      orm:"resource_owner_id"      description:""`
	ResourceChannelRef   string     `json:"resourceChannelRef"   orm:"resource_channel_ref"   description:""`
}
