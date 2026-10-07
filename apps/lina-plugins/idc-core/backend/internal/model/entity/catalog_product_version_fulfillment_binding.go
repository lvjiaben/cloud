// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogProductVersionFulfillmentBinding is the golang structure for table catalog_product_version_fulfillment_binding.
type CatalogProductVersionFulfillmentBinding struct {
	Id                           int64     `json:"id"                           orm:"id"                              description:""`
	TenantId                     int       `json:"tenantId"                     orm:"tenant_id"                       description:""`
	VersionNo                    string    `json:"versionNo"                    orm:"version_no"                      description:""`
	InterfaceInstanceNo          string    `json:"interfaceInstanceNo"          orm:"interface_instance_no"           description:""`
	InterfaceDisplayNameSnapshot string    `json:"interfaceDisplayNameSnapshot" orm:"interface_display_name_snapshot" description:""`
	PluginId                     string    `json:"pluginId"                     orm:"plugin_id"                       description:""`
	ExtensionPointCode           string    `json:"extensionPointCode"           orm:"extension_point_code"            description:""`
	PluginReleaseVersion         string    `json:"pluginReleaseVersion"         orm:"plugin_release_version"          description:""`
	ContractVersion              string    `json:"contractVersion"              orm:"contract_version"                description:""`
	AuthorizedMethods            []string  `json:"authorizedMethods"            orm:"authorized_methods"              description:""`
	ReleaseDigest                string    `json:"releaseDigest"                orm:"release_digest"                  description:""`
	DeclarationDigest            string    `json:"declarationDigest"            orm:"declaration_digest"              description:""`
	BoundBy                      int       `json:"boundBy"                      orm:"bound_by"                        description:""`
	BoundAt                      time.Time `json:"boundAt"                      orm:"bound_at"                        description:""`
}
