// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContractAutoSignUsage is the golang structure for table contract_auto_sign_usage.
type ContractAutoSignUsage struct {
	Id                       int64      `json:"id"                       orm:"id"                         description:""`
	TenantId                 int        `json:"tenantId"                 orm:"tenant_id"                  description:""`
	UsageNo                  string     `json:"usageNo"                  orm:"usage_no"                   description:""`
	AuthorizationNo          string     `json:"authorizationNo"          orm:"authorization_no"           description:""`
	ContractNo               string     `json:"contractNo"               orm:"contract_no"                description:""`
	CurrencyCode             string     `json:"currencyCode"             orm:"currency_code"              description:""`
	AmountMinor              int64      `json:"amountMinor"              orm:"amount_minor"               description:""`
	CommandIdempotencyDigest string     `json:"commandIdempotencyDigest" orm:"command_idempotency_digest" description:""`
	CommandFingerprint       string     `json:"commandFingerprint"       orm:"command_fingerprint"        description:""`
	ProviderAcceptedAt       *time.Time `json:"providerAcceptedAt"       orm:"provider_accepted_at"       description:""`
	CreatedAt                *time.Time `json:"createdAt"                orm:"created_at"                 description:""`
}
