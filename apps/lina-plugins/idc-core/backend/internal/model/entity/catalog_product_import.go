// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogProductImport is the golang structure for table catalog_product_import.
type CatalogProductImport struct {
	Id                           int64      `json:"id"                           orm:"id"                             description:""`
	TenantId                     int        `json:"tenantId"                     orm:"tenant_id"                      description:""`
	RequestKey                   string     `json:"requestKey"                   orm:"request_key"                    description:""`
	RequestDigest                string     `json:"requestDigest"                orm:"request_digest"                 description:""`
	ActorUserId                  int        `json:"actorUserId"                  orm:"actor_user_id"                  description:""`
	InterfaceInstanceNo          string     `json:"interfaceInstanceNo"          orm:"interface_instance_no"          description:""`
	UpstreamReference            string     `json:"upstreamReference"            orm:"upstream_reference"             description:""`
	ProductNo                    *string    `json:"productNo"                    orm:"product_no"                     description:""`
	VersionNo                    *string    `json:"versionNo"                    orm:"version_no"                     description:""`
	SourceSnapshot               string     `json:"sourceSnapshot"               orm:"source_snapshot"                description:""`
	MissingConfigurationSnapshot string     `json:"missingConfigurationSnapshot" orm:"missing_configuration_snapshot" description:""`
	PricingIncomplete            bool       `json:"pricingIncomplete"            orm:"pricing_incomplete"             description:""`
	CreatedAt                    *time.Time `json:"createdAt"                    orm:"created_at"                     description:""`
}
