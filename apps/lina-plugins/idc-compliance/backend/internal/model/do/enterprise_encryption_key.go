// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// EnterpriseEncryptionKey is the golang structure of table plugin_idc_compliance_enterprise_encryption_key for DAO operations like Where/Data.
type EnterpriseEncryptionKey struct {
	g.Meta     `orm:"table:plugin_idc_compliance_enterprise_encryption_key, do:true"`
	Id         any        //
	TenantId   any        //
	KeyNo      any        //
	SecretRef  any        // Sensitive secretcap handle reference; never expose through API, logs, audit, tasks or errors
	Generation any        //
	StatusCode any        //
	Version    any        //
	CreatedAt  *time.Time //
	UpdatedAt  *time.Time //
}
