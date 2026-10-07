// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContractTemplateVersion is the golang structure for table contract_template_version.
type ContractTemplateVersion struct {
	Id                             int64      `json:"id"                             orm:"id"                                description:""`
	TenantId                       int        `json:"tenantId"                       orm:"tenant_id"                         description:""`
	TemplateNo                     string     `json:"templateNo"                     orm:"template_no"                       description:""`
	VersionNo                      int64      `json:"versionNo"                      orm:"version_no"                        description:""`
	Name                           string     `json:"name"                           orm:"name"                              description:""`
	ScopeCode                      *string    `json:"scopeCode"                      orm:"scope_code"                        description:""`
	SigningOrderCode               *string    `json:"signingOrderCode"               orm:"signing_order_code"                description:""`
	LanguageCode                   string     `json:"languageCode"                   orm:"language_code"                     description:"Explicit contract language; product and service names must use the matching locale"`
	StatusCode                     string     `json:"statusCode"                     orm:"status_code"                       description:""`
	ProviderVersion                *string    `json:"providerVersion"                orm:"provider_version"                  description:""`
	ProviderSnapshotDigest         *string    `json:"providerSnapshotDigest"         orm:"provider_snapshot_digest"          description:""`
	ProviderCustomerSignatureCount int        `json:"providerCustomerSignatureCount" orm:"provider_customer_signature_count" description:""`
	ProviderCustomerSealCount      int        `json:"providerCustomerSealCount"      orm:"provider_customer_seal_count"      description:""`
	ProviderMerchantSignatureCount int        `json:"providerMerchantSignatureCount" orm:"provider_merchant_signature_count" description:""`
	ProviderMerchantSealCount      int        `json:"providerMerchantSealCount"      orm:"provider_merchant_seal_count"      description:""`
	CurrentContentRevisionNo       *string    `json:"currentContentRevisionNo"       orm:"current_content_revision_no"       description:"Opaque pointer to the latest append-only encrypted source and PDF revision"`
	CurrentFieldSetNo              *string    `json:"currentFieldSetNo"              orm:"current_field_set_no"              description:""`
	PreviewConfirmed               bool       `json:"previewConfirmed"               orm:"preview_confirmed"                 description:""`
	UsedCount                      int64      `json:"usedCount"                      orm:"used_count"                        description:""`
	LockVersion                    int64      `json:"lockVersion"                    orm:"lock_version"                      description:""`
	CreatedAt                      *time.Time `json:"createdAt"                      orm:"created_at"                        description:""`
	UpdatedAt                      *time.Time `json:"updatedAt"                      orm:"updated_at"                        description:""`
	CurrentProviderMappingSetNo    string     `json:"currentProviderMappingSetNo"    orm:"current_provider_mapping_set_no"   description:""`
}
