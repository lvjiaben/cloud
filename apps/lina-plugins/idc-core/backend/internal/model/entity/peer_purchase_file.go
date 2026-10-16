// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// PeerPurchaseFile is the golang structure for table peer_purchase_file.
type PeerPurchaseFile struct {
	Id                int64      `json:"id"                orm:"id"                  description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"           description:""`
	IntentNo          string     `json:"intentNo"          orm:"intent_no"           description:""`
	FieldKey          string     `json:"fieldKey"          orm:"field_key"           description:""`
	FileSequence      int        `json:"fileSequence"      orm:"file_sequence"       description:""`
	Sha256Hex         string     `json:"sha256Hex"         orm:"sha256_hex"          description:""`
	UpstreamFileToken string     `json:"upstreamFileToken" orm:"upstream_file_token" description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"          description:""`
}
