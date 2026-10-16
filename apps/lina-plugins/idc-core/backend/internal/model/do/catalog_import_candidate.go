// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CatalogImportCandidate is the golang structure of table plugin_idc_core_catalog_import_candidate for DAO operations like Where/Data.
type CatalogImportCandidate struct {
	g.Meta         `orm:"table:plugin_idc_core_catalog_import_candidate, do:true"`
	Id             any        //
	TenantId       any        //
	VersionNo      any        //
	FieldKey       any        //
	CandidateValue any        //
	OptionToken    any        //
	CreatedAt      *time.Time //
}
