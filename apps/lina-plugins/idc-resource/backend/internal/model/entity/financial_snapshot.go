// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// FinancialSnapshot is the golang structure for table financial_snapshot.
type FinancialSnapshot struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	SnapshotNo          string     `json:"snapshotNo"          orm:"snapshot_no"           description:""`
	ChannelNo           string     `json:"channelNo"           orm:"channel_no"            description:""`
	SnapshotKind        string     `json:"snapshotKind"        orm:"snapshot_kind"         description:""`
	ExternalResourceRef string     `json:"externalResourceRef" orm:"external_resource_ref" description:""`
	CurrencyCode        string     `json:"currencyCode"        orm:"currency_code"         description:""`
	Amount              string     `json:"amount"              orm:"amount"                description:"Non-negative Provider amount in the exact ISO currency; never converted implicitly"`
	ExpiresAt           *time.Time `json:"expiresAt"           orm:"expires_at"            description:""`
	Projection          string     `json:"projection"          orm:"projection"            description:""`
	ObservedAt          *time.Time `json:"observedAt"          orm:"observed_at"           description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
}
