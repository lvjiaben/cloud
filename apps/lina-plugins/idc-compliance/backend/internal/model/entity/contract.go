// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// Contract is the golang structure for table contract.
type Contract struct {
	Id                        int64      `json:"id"                        orm:"id"                          description:""`
	TenantId                  int        `json:"tenantId"                  orm:"tenant_id"                   description:""`
	ContractNo                string     `json:"contractNo"                orm:"contract_no"                 description:""`
	CustomerNo                string     `json:"customerNo"                orm:"customer_no"                 description:""`
	PrincipalNo               string     `json:"principalNo"               orm:"principal_no"                description:""`
	OrderNo                   string     `json:"orderNo"                   orm:"order_no"                    description:""`
	ServiceNo                 *string    `json:"serviceNo"                 orm:"service_no"                  description:""`
	ContractTypeCode          string     `json:"contractTypeCode"          orm:"contract_type_code"          description:""`
	RequiredForFulfillment    bool       `json:"requiredForFulfillment"    orm:"required_for_fulfillment"    description:""`
	TemplateNo                string     `json:"templateNo"                orm:"template_no"                 description:""`
	TemplateVersion           int64      `json:"templateVersion"           orm:"template_version"            description:""`
	VariableSnapshotRef       string     `json:"variableSnapshotRef"       orm:"variable_snapshot_ref"       description:""`
	VariableSnapshotSha256    string     `json:"variableSnapshotSha256"    orm:"variable_snapshot_sha256"    description:""`
	SignerSnapshotSha256      string     `json:"signerSnapshotSha256"      orm:"signer_snapshot_sha256"      description:""`
	CurrencyCode              string     `json:"currencyCode"              orm:"currency_code"               description:""`
	AmountMinor               int64      `json:"amountMinor"               orm:"amount_minor"                description:""`
	StatusCode                string     `json:"statusCode"                orm:"status_code"                 description:""`
	LaunchedAt                *time.Time `json:"launchedAt"                orm:"launched_at"                 description:""`
	CompletedAt               *time.Time `json:"completedAt"               orm:"completed_at"                description:""`
	TerminatedAt              *time.Time `json:"terminatedAt"              orm:"terminated_at"               description:""`
	Version                   int64      `json:"version"                   orm:"version"                     description:""`
	CreatedAt                 *time.Time `json:"createdAt"                 orm:"created_at"                  description:""`
	UpdatedAt                 *time.Time `json:"updatedAt"                 orm:"updated_at"                  description:""`
	SubjectTypeCode           string     `json:"subjectTypeCode"           orm:"subject_type_code"           description:""`
	SigningOrderCode          string     `json:"signingOrderCode"          orm:"signing_order_code"          description:""`
	SnapshotKeyRef            string     `json:"snapshotKeyRef"            orm:"snapshot_key_ref"            description:"Opaque secretcap handle for the encrypted contract preparation bundle"`
	OrderSnapshotSha256       string     `json:"orderSnapshotSha256"       orm:"order_snapshot_sha256"       description:"Digest of the canonical immutable core order, service, identity, and amount snapshot"`
	CreationIdempotencyDigest string     `json:"creationIdempotencyDigest" orm:"creation_idempotency_digest" description:""`
	CreationFingerprint       string     `json:"creationFingerprint"       orm:"creation_fingerprint"        description:""`
	SigningExpiresAt          *time.Time `json:"signingExpiresAt"          orm:"signing_expires_at"          description:"Fixed signing deadline exactly seven days after authoritative contract creation"`
}
