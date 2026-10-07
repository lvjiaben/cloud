// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// FinancialExportArtifact is the golang structure of table plugin_idc_core_financial_export_artifact for DAO operations like Where/Data.
type FinancialExportArtifact struct {
	g.Meta        `orm:"table:plugin_idc_core_financial_export_artifact, do:true"`
	Id            any        //
	TenantId      any        //
	RequestNo     any        //
	ObjectRef     any        //
	ContentSha256 any        //
	SizeBytes     any        //
	RowCount      any        //
	StatusCode    any        //
	ExpiresAt     *time.Time //
	CreatedAt     *time.Time //
	UpdatedAt     *time.Time //
}
