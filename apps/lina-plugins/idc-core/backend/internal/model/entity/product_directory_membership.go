// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

// ProductDirectoryMembership is the golang structure for table product_directory_membership.
type ProductDirectoryMembership struct {
	TenantId    int    `json:"tenantId"    orm:"tenant_id"    description:""`
	ProductNo   string `json:"productNo"   orm:"product_no"   description:""`
	DirectoryNo string `json:"directoryNo" orm:"directory_no" description:""`
}
