// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogProductVersionConfiguration is the golang structure for table catalog_product_version_configuration.
type CatalogProductVersionConfiguration struct {
	Id                             int64      `json:"id"                             orm:"id"                               description:""`
	TenantId                       int        `json:"tenantId"                       orm:"tenant_id"                        description:""`
	VersionNo                      string     `json:"versionNo"                      orm:"version_no"                       description:""`
	InterfaceInstanceNo            string     `json:"interfaceInstanceNo"            orm:"interface_instance_no"            description:""`
	InterfaceConfigurationRevision int64      `json:"interfaceConfigurationRevision" orm:"interface_configuration_revision" description:""`
	ProductSchemaVersion           string     `json:"productSchemaVersion"           orm:"product_schema_version"           description:""`
	ProductDeclarationSnapshot     string     `json:"productDeclarationSnapshot"     orm:"product_declaration_snapshot"     description:""`
	MerchantValueSnapshot          string     `json:"merchantValueSnapshot"          orm:"merchant_value_snapshot"          description:""`
	CustomerSchemaVersion          string     `json:"customerSchemaVersion"          orm:"customer_schema_version"          description:""`
	CustomerDeclarationSnapshot    string     `json:"customerDeclarationSnapshot"    orm:"customer_declaration_snapshot"    description:""`
	DeclarationDigest              string     `json:"declarationDigest"              orm:"declaration_digest"               description:""`
	MerchantValueDigest            string     `json:"merchantValueDigest"            orm:"merchant_value_digest"            description:""`
	Version                        int64      `json:"version"                        orm:"version"                          description:""`
	ChangedBy                      int        `json:"changedBy"                      orm:"changed_by"                       description:""`
	CreatedAt                      *time.Time `json:"createdAt"                      orm:"created_at"                       description:""`
	ChangedAt                      *time.Time `json:"changedAt"                      orm:"changed_at"                       description:""`
	CustomerPresentationSnapshot   string     `json:"customerPresentationSnapshot"   orm:"customer_presentation_snapshot"   description:""`
}
