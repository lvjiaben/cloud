// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// PrincipalMapping is the golang structure for table principal_mapping.
type PrincipalMapping struct {
	Id                   int64      `json:"id"                   orm:"id"                     description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"              description:""`
	MappingNo            string     `json:"mappingNo"            orm:"mapping_no"             description:""`
	CustomerNo           string     `json:"customerNo"           orm:"customer_no"            description:"Opaque core-owned customer reference with no cross-plugin database foreign key"`
	ChannelNo            string     `json:"channelNo"            orm:"channel_no"             description:""`
	ExternalPrincipalRef string     `json:"externalPrincipalRef" orm:"external_principal_ref" description:"Channel-local upstream client identifier; never inferred from the local customer number"`
	StatusCode           string     `json:"statusCode"           orm:"status_code"            description:""`
	EvidenceDigest       string     `json:"evidenceDigest"       orm:"evidence_digest"        description:"Secret-free digest of the authoritative mapping evidence"`
	ObservedAt           *time.Time `json:"observedAt"           orm:"observed_at"            description:""`
	Version              int64      `json:"version"              orm:"version"                description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"             description:""`
	UpdatedAt            *time.Time `json:"updatedAt"            orm:"updated_at"             description:""`
}
