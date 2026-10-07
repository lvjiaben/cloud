// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// EnterpriseEncryptionKey is the golang structure for table enterprise_encryption_key.
type EnterpriseEncryptionKey struct {
	Id         int64      `json:"id"         orm:"id"          description:""`
	TenantId   int        `json:"tenantId"   orm:"tenant_id"   description:""`
	KeyNo      string     `json:"keyNo"      orm:"key_no"      description:""`
	SecretRef  string     `json:"secretRef"  orm:"secret_ref"  description:"Sensitive secretcap handle reference; never expose through API, logs, audit, tasks or errors"`
	Generation int64      `json:"generation" orm:"generation"  description:""`
	StatusCode string     `json:"statusCode" orm:"status_code" description:""`
	Version    int64      `json:"version"    orm:"version"     description:""`
	CreatedAt  *time.Time `json:"createdAt"  orm:"created_at"  description:""`
	UpdatedAt  *time.Time `json:"updatedAt"  orm:"updated_at"  description:""`
}
