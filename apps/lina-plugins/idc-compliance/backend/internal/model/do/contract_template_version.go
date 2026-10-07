// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContractTemplateVersion is the golang structure of table plugin_idc_compliance_contract_template_version for DAO operations like Where/Data.
type ContractTemplateVersion struct {
	g.Meta                         `orm:"table:plugin_idc_compliance_contract_template_version, do:true"`
	Id                             any        //
	TenantId                       any        //
	TemplateNo                     any        //
	VersionNo                      any        //
	Name                           any        //
	ScopeCode                      *string    //
	SigningOrderCode               *string    //
	LanguageCode                   any        // Explicit contract language; product and service names must use the matching locale
	StatusCode                     any        //
	ProviderVersion                *string    //
	ProviderSnapshotDigest         *string    //
	ProviderCustomerSignatureCount any        //
	ProviderCustomerSealCount      any        //
	ProviderMerchantSignatureCount any        //
	ProviderMerchantSealCount      any        //
	CurrentContentRevisionNo       *string    // Opaque pointer to the latest append-only encrypted source and PDF revision
	CurrentFieldSetNo              *string    //
	PreviewConfirmed               any        //
	UsedCount                      any        //
	LockVersion                    any        //
	CreatedAt                      *time.Time //
	UpdatedAt                      *time.Time //
	CurrentProviderMappingSetNo    any        //
}
