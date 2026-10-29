// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogImportPriceCommand is the golang structure for table catalog_import_price_command.
type CatalogImportPriceCommand struct {
	Id            int64      `json:"id"            orm:"id"             description:""`
	TenantId      int        `json:"tenantId"      orm:"tenant_id"      description:""`
	ActorUserId   int        `json:"actorUserId"   orm:"actor_user_id"  description:""`
	RequestKey    string     `json:"requestKey"    orm:"request_key"    description:""`
	RequestDigest string     `json:"requestDigest" orm:"request_digest" description:""`
	VersionNo     string     `json:"versionNo"     orm:"version_no"     description:""`
	CreatedAt     *time.Time `json:"createdAt"     orm:"created_at"     description:""`
}
