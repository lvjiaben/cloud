// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ProductAutomationInterfaceSecretReference is the golang structure for table product_automation_interface_secret_reference.
type ProductAutomationInterfaceSecretReference struct {
	Id                      int64      `json:"id"                      orm:"id"                        description:""`
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                 description:""`
	ConfigurationRevisionId int64      `json:"configurationRevisionId" orm:"configuration_revision_id" description:""`
	FieldKey                string     `json:"fieldKey"                orm:"field_key"                 description:""`
	SecretRef               string     `json:"secretRef"               orm:"secret_ref"                description:""`
	Configured              bool       `json:"configured"              orm:"configured"                description:""`
	MaskedTail              string     `json:"maskedTail"              orm:"masked_tail"               description:""`
	SecretUpdatedAt         *time.Time `json:"secretUpdatedAt"         orm:"secret_updated_at"         description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                description:""`
}
