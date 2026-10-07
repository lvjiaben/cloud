// This file appends a package version and updates its sale settings.

package v1

import "github.com/gogf/gf/v2/frame/g"

type TrafficPackageUpdateReq struct {
	g.Meta          `path:"/admin/resource/traffic-packages/{packageNo}" method:"put" tags:"IDC Server Operations" summary:"Update a fixed traffic package" dc:"Append an immutable version and update sale settings without rewriting existing orders or allocations." permission:"idc:admin:resource:accessory-price:write"`
	PackageNo       string `json:"packageNo" in:"path" v:"required|length:1,40" dc:"Traffic package reference" eg:"tpk_00000000000000000000000000000000"`
	ExpectedVersion int64  `json:"expectedVersion" v:"min:1" dc:"Current version to prevent lost edits" eg:"1"`
	TrafficPackageFields
}

type TrafficPackageUpdateRes struct {
	Package *TrafficPackage `json:"package" dc:"Appended package version" eg:"{}"`
}
