// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogProductVersion is the golang structure for table catalog_product_version.
type CatalogProductVersion struct {
	Id                    int64      `json:"id"                    orm:"id"                      description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"               description:""`
	VersionNo             string     `json:"versionNo"             orm:"version_no"              description:""`
	ProductNo             string     `json:"productNo"             orm:"product_no"              description:""`
	VersionNumber         int        `json:"versionNumber"         orm:"version_number"          description:""`
	StatusCode            string     `json:"statusCode"            orm:"status_code"             description:""`
	SourceCode            string     `json:"sourceCode"            orm:"source_code"             description:""`
	SourceVersion         string     `json:"sourceVersion"         orm:"source_version"          description:""`
	SourceFingerprint     string     `json:"sourceFingerprint"     orm:"source_fingerprint"      description:""`
	ChangeSummary         string     `json:"changeSummary"         orm:"change_summary"          description:""`
	RequiresContract      bool       `json:"requiresContract"      orm:"requires_contract"       description:""`
	PublishedAt           *time.Time `json:"publishedAt"           orm:"published_at"            description:""`
	RetiredAt             *time.Time `json:"retiredAt"             orm:"retired_at"              description:""`
	CreatedBy             int        `json:"createdBy"             orm:"created_by"              description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"              description:""`
	ContractPolicyCode    string     `json:"contractPolicyCode"    orm:"contract_policy_code"    description:"Product choice: inherit enterprise, explicit none, or required"`
	ContractTemplateNo    string     `json:"contractTemplateNo"    orm:"contract_template_no"    description:""`
	ContractPolicyVersion int64      `json:"contractPolicyVersion" orm:"contract_policy_version" description:""`
}
