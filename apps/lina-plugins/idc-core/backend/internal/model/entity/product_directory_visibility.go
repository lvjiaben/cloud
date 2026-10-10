// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ProductDirectoryVisibility is the golang structure for table product_directory_visibility.
type ProductDirectoryVisibility struct {
	TenantId    int        `json:"tenantId"    orm:"tenant_id"    description:""`
	CustomerNo  string     `json:"customerNo"  orm:"customer_no"  description:""`
	DirectoryNo string     `json:"directoryNo" orm:"directory_no" description:""`
	Hidden      bool       `json:"hidden"      orm:"hidden"       description:""`
	UpdatedAt   *time.Time `json:"updatedAt"   orm:"updated_at"   description:""`
}
