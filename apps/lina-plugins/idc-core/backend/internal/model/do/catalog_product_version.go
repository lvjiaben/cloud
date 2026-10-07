// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogProductVersion is the golang structure of table plugin_idc_core_catalog_product_version for DAO operations like Where/Data.
type CatalogProductVersion struct {
	g.Meta                `orm:"table:plugin_idc_core_catalog_product_version, do:true"`
	Id                    any        //
	TenantId              any        //
	VersionNo             any        //
	ProductNo             any        //
	VersionNumber         any        //
	StatusCode            any        //
	SourceCode            any        //
	SourceVersion         any        //
	SourceFingerprint     any        //
	ChangeSummary         any        //
	RequiresContract      any        //
	PublishedAt           *time.Time //
	RetiredAt             *time.Time //
	CreatedBy             any        //
	CreatedAt             *time.Time //
	ContractPolicyCode    any        // Product choice: inherit enterprise, explicit none, or required
	ContractTemplateNo    any        //
	ContractPolicyVersion any        //
}
