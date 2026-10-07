// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ContractArchiveJob is the golang structure of table plugin_idc_compliance_contract_archive_job for DAO operations like Where/Data.
type ContractArchiveJob struct {
	g.Meta             `orm:"table:plugin_idc_compliance_contract_archive_job, do:true"`
	Id                 any        //
	TenantId           any        //
	JobNo              any        //
	ContractNo         any        //
	FlowNo             any        //
	GatewayRouteNo     any        //
	ArtifactTypeCode   any        //
	StatusCode         any        //
	AttemptCount       any        //
	NextAttemptAt      *time.Time //
	LeaseSha256        any        //
	LeaseExpiresAt     *time.Time //
	LastReasonCode     any        //
	SupplierObservedAt *time.Time //
	CompletedAt        *time.Time //
	CreatedAt          *time.Time //
	UpdatedAt          *time.Time //
	Version            any        //
}
