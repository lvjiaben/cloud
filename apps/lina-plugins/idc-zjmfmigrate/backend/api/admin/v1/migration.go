// Package v1 contains versioned migration request and response projections.
package v1

import "github.com/gogf/gf/v2/frame/g"

type PreflightReq struct {
	g.Meta        `path:"/admin/migration/preflight" method:"post" tags:"Migration" summary:"Run read-only source preflight" permission:"idc-zjmfmigrate:migration:preflight" pageBridgeCapability:"idc-migration.admin.v1" pageBridgeHost:"admin"`
	SourceSystem  string   `json:"sourceSystem" v:"required"`
	SourceVersion string   `json:"sourceVersion" v:"required"`
	Charset       string   `json:"charset"`
	Timezone      string   `json:"timezone"`
	Currencies    []string `json:"currencies"`
}
type PreflightRes struct {
	SourceSystem  string   `json:"sourceSystem"`
	SourceVersion string   `json:"sourceVersion"`
	Supported     bool     `json:"supported"`
	Blockers      []string `json:"blockers"`
	Warnings      []string `json:"warnings"`
	Capabilities  []string `json:"capabilities"`
}
type CreateBatchReq struct {
	g.Meta        `path:"/admin/migration/batches" method:"post" tags:"Migration" summary:"Create migration batch" permission:"idc-zjmfmigrate:migration:write" pageBridgeCapability:"idc-migration.admin.v1" pageBridgeHost:"admin"`
	BatchNo       string `json:"batchNo" v:"required"`
	SourceSystem  string `json:"sourceSystem" v:"required"`
	SourceVersion string `json:"sourceVersion" v:"required"`
	Mode          string `json:"mode" d:"dry_run"`
}
type CreateBatchRes struct {
	Id      int64  `json:"id"`
	BatchNo string `json:"batchNo"`
	Status  string `json:"status"`
}
type RunReq struct {
	g.Meta    `path:"/admin/migration/batches/{id}/run" method:"post" tags:"Migration" summary:"Run migration dry-run or import" permission:"idc-zjmfmigrate:migration:write" pageBridgeCapability:"idc-migration.admin.v1" pageBridgeHost:"admin"`
	Id        int64  `json:"id" v:"required"`
	Watermark string `json:"watermark"`
}
type RunRes struct {
	Id       int64    `json:"id"`
	Status   string   `json:"status"`
	Mapped   int      `json:"mapped"`
	Blockers []string `json:"blockers"`
}
type ConflictsReq struct {
	g.Meta   `path:"/admin/migration/batches/{id}/conflicts" method:"get" tags:"Migration" summary:"List migration conflicts" dc:"Return one bounded conflict page and the exact authorized total for the selected batch through governed record storage." permission:"idc-zjmfmigrate:migration:query" pageBridgeCapability:"idc-migration.admin.v1" pageBridgeHost:"admin"`
	Id       int64 `json:"id" v:"required" dc:"Selected migration batch identifier" eg:"1"`
	Page     int   `json:"page" d:"1" v:"min:1|max:1000000" dc:"Requested page number" eg:"1"`
	PageSize int   `json:"pageSize" d:"10" v:"min:1|max:100" dc:"Maximum rows per page" eg:"10"`
}
type Conflict struct {
	Id         int64  `json:"id"`
	ObjectType string `json:"objectType"`
	SourceId   string `json:"sourceId"`
	Summary    string `json:"summary"`
	Decision   string `json:"decision"`
}
type ConflictsRes struct {
	List  []*Conflict `json:"list" dc:"Conflicts in the current bounded page" eg:"[]"`
	Total int         `json:"total" dc:"Exact authorized conflict count for the selected batch" eg:"25"`
}
type DecideConflictReq struct {
	g.Meta   `path:"/admin/migration/conflicts/{id}/decision" method:"put" tags:"Migration" summary:"Decide migration conflict" permission:"idc-zjmfmigrate:migration:decide" pageBridgeCapability:"idc-migration.admin.v1" pageBridgeHost:"admin"`
	Id       int64  `json:"id" v:"required"`
	Decision string `json:"decision" v:"required|in:merge,keep_source,skip"`
	Reason   string `json:"reason" v:"required|length:2,500"`
}
type DecideConflictRes struct {
	Id       int64  `json:"id"`
	Decision string `json:"decision"`
}
type ReconcileReq struct {
	g.Meta `path:"/admin/migration/batches/{id}/reconcile" method:"post" tags:"Migration" summary:"Reconcile migration batch" permission:"idc-zjmfmigrate:migration:reconcile" pageBridgeCapability:"idc-migration.admin.v1" pageBridgeHost:"admin"`
	Id     int64 `json:"id" v:"required"`
}
type ReconcileRes struct {
	Id          int64  `json:"id"`
	Status      string `json:"status"`
	Differences int    `json:"differences"`
}
type CutoverReq struct {
	g.Meta `path:"/admin/migration/batches/{id}/cutover" method:"post" tags:"Migration" summary:"Check migration cutover" permission:"idc-zjmfmigrate:migration:cutover" pageBridgeCapability:"idc-migration.admin.v1" pageBridgeHost:"admin"`
	Id     int64 `json:"id" v:"required"`
}
type CutoverRes struct {
	Id      int64    `json:"id"`
	Allowed bool     `json:"allowed"`
	Checks  []string `json:"checks"`
}
type RollbackReq struct {
	g.Meta `path:"/admin/migration/batches/{id}/rollback" method:"post" tags:"Migration" summary:"Rollback pre-cutover migration batch" permission:"idc-zjmfmigrate:migration:rollback" pageBridgeCapability:"idc-migration.admin.v1" pageBridgeHost:"admin"`
	Id     int64 `json:"id" v:"required"`
}
type RollbackRes struct {
	Id     int64  `json:"id"`
	Status string `json:"status"`
}
type ReportReq struct {
	g.Meta `path:"/admin/migration/batches/{id}/report" method:"get" tags:"Migration" summary:"Get migration evidence report" permission:"idc-zjmfmigrate:migration:query" pageBridgeCapability:"idc-migration.admin.v1" pageBridgeHost:"admin"`
	Id     int64 `json:"id" v:"required"`
}
type ReportRes struct {
	Id       int64             `json:"id"`
	Status   string            `json:"status"`
	ByType   map[string]string `json:"byType"`
	Blockers []string          `json:"blockers"`
	Warnings []string          `json:"warnings"`
}
