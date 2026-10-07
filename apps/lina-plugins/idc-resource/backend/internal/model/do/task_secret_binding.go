// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// TaskSecretBinding is the golang structure of table plugin_idc_resource_task_secret_binding for DAO operations like Where/Data.
type TaskSecretBinding struct {
	g.Meta           `orm:"table:plugin_idc_resource_task_secret_binding, do:true"`
	Id               any        //
	TenantId         any        //
	BindingNo        any        //
	TaskNo           any        //
	BindingName      any        //
	BindingDirection any        //
	SecretRef        any        // Sensitive secretcap routing reference excluded from task JSON, logs, audit, and API responses
	OutputPolicy     any        // One-time credential or short-lived ticket delivery policy for output bindings
	ExpiresAt        *time.Time //
	StatusCode       any        //
	RevealedAt       *time.Time //
	RevokedAt        *time.Time //
	Version          any        //
	CreatedAt        *time.Time //
	UpdatedAt        *time.Time //
}
