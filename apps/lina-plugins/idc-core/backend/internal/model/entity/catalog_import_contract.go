// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogImportContract is the golang structure for table catalog_import_contract.
type CatalogImportContract struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	VersionNo           string     `json:"versionNo"           orm:"version_no"            description:""`
	InterfaceInstanceNo string     `json:"interfaceInstanceNo" orm:"interface_instance_no" description:""`
	Snapshot            string     `json:"snapshot"            orm:"snapshot"              description:""`
	SnapshotDigest      string     `json:"snapshotDigest"      orm:"snapshot_digest"       description:""`
	ReadEvidence        string     `json:"readEvidence"        orm:"read_evidence"         description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
}
