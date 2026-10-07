// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

// TrafficPackageProduct is the golang structure for table traffic_package_product.
type TrafficPackageProduct struct {
	Id            int64  `json:"id"            orm:"id"             description:""`
	TenantId      int    `json:"tenantId"      orm:"tenant_id"      description:""`
	PolicyNo      string `json:"policyNo"      orm:"policy_no"      description:""`
	PolicyVersion int64  `json:"policyVersion" orm:"policy_version" description:""`
	ProductNo     string `json:"productNo"     orm:"product_no"     description:""`
}
