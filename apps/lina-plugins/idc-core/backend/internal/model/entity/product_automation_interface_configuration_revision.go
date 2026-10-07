// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ProductAutomationInterfaceConfigurationRevision is the golang structure for table product_automation_interface_configuration_revision.
type ProductAutomationInterfaceConfigurationRevision struct {
	Id                                 int64      `json:"id"                                 orm:"id"                                    description:""`
	TenantId                           int        `json:"tenantId"                           orm:"tenant_id"                             description:""`
	InterfaceInstanceNo                string     `json:"interfaceInstanceNo"                orm:"interface_instance_no"                 description:""`
	Revision                           int64      `json:"revision"                           orm:"revision"                              description:""`
	SchemaVersion                      string     `json:"schemaVersion"                      orm:"schema_version"                        description:""`
	DeclarationSnapshot                string     `json:"declarationSnapshot"                orm:"declaration_snapshot"                  description:""`
	PublicValueSnapshot                string     `json:"publicValueSnapshot"                orm:"public_value_snapshot"                 description:""`
	DeclarationDigest                  string     `json:"declarationDigest"                  orm:"declaration_digest"                    description:""`
	ValueDigest                        string     `json:"valueDigest"                        orm:"value_digest"                          description:""`
	SecretFieldCount                   int        `json:"secretFieldCount"                   orm:"secret_field_count"                    description:""`
	CreatedBy                          int        `json:"createdBy"                          orm:"created_by"                            description:""`
	CreatedAt                          *time.Time `json:"createdAt"                          orm:"created_at"                            description:""`
	ResourceOwnerConfigurationRevision string     `json:"resourceOwnerConfigurationRevision" orm:"resource_owner_configuration_revision" description:"Immutable resource-owner endpoint/account/secret-reference revision; empty for non-resource interfaces / 资源拥有方不可变接口配置修订，非资源接口为空"`
}
