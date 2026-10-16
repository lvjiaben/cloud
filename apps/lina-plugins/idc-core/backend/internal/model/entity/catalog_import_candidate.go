// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogImportCandidate is the golang structure for table catalog_import_candidate.
type CatalogImportCandidate struct {
	Id             int64      `json:"id"             orm:"id"              description:""`
	TenantId       int        `json:"tenantId"       orm:"tenant_id"       description:""`
	VersionNo      string     `json:"versionNo"      orm:"version_no"      description:""`
	FieldKey       string     `json:"fieldKey"       orm:"field_key"       description:""`
	CandidateValue string     `json:"candidateValue" orm:"candidate_value" description:""`
	OptionToken    string     `json:"optionToken"    orm:"option_token"    description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"      description:""`
}
