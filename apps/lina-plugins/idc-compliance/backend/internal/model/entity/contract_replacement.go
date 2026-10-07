// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContractReplacement is the golang structure for table contract_replacement.
type ContractReplacement struct {
	Id                int64      `json:"id"                orm:"id"                 description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"          description:""`
	ReplacementNo     string     `json:"replacementNo"     orm:"replacement_no"     description:""`
	OldContractNo     string     `json:"oldContractNo"     orm:"old_contract_no"    description:""`
	NewContractNo     string     `json:"newContractNo"     orm:"new_contract_no"    description:""`
	IdempotencySha256 string     `json:"idempotencySha256" orm:"idempotency_sha256" description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"         description:""`
}
