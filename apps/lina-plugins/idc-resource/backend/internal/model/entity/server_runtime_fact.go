// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServerRuntimeFact is the golang structure for table server_runtime_fact.
type ServerRuntimeFact struct {
	MappingNo      string     `json:"mappingNo"      orm:"mapping_no"      description:""`
	TenantId       int        `json:"tenantId"       orm:"tenant_id"       description:""`
	Facts          string     `json:"facts"          orm:"facts"           description:""`
	ObservedAt     *time.Time `json:"observedAt"     orm:"observed_at"     description:""`
	NextRefreshAt  *time.Time `json:"nextRefreshAt"  orm:"next_refresh_at" description:""`
	LeaseUntil     *time.Time `json:"leaseUntil"     orm:"lease_until"     description:""`
	LeaseVersion   int64      `json:"leaseVersion"   orm:"lease_version"   description:""`
	LastErrorCode  string     `json:"lastErrorCode"  orm:"last_error_code" description:""`
	MappingVersion int64      `json:"mappingVersion" orm:"mapping_version" description:""`
}
