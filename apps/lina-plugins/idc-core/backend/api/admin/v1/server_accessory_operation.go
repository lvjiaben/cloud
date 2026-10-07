// This file defines staff-only server accessory detail and operation contracts.

package v1

import (
	"encoding/json"

	"github.com/gogf/gf/v2/frame/g"
)

type ManagementServerAccessory struct {
	ObjectNo   string          `json:"objectNo" dc:"Stable local accessory number" eg:"ACO_0123456789abcdef"`
	Kind       string          `json:"kind" dc:"Accessory kind" eg:"snapshot"`
	Name       string          `json:"name" dc:"Safe display name" eg:"Before upgrade"`
	State      string          `json:"state" dc:"Normalized object state" eg:"available"`
	Projection json.RawMessage `json:"projection" dc:"Safe kind-specific projection without Provider identifiers" eg:"{}"`
}

type ManagementServerAccessoryDetail struct {
	ServiceNo             string                       `json:"serviceNo" dc:"Stable server service number" eg:"SVC_SERVER_0001"`
	CustomerNo            string                       `json:"customerNo" dc:"Stable owning customer number" eg:"CUS_0001"`
	ProductNameZhCN       string                       `json:"productNameZhCN" dc:"Frozen Simplified Chinese product name" eg:"Production server"`
	ProductNameEnUS       string                       `json:"productNameEnUS" dc:"Frozen English product name" eg:"Production server"`
	EntitlementStatus     string                       `json:"entitlementStatus" dc:"Current entitlement status" eg:"active"`
	RuntimeStatus         string                       `json:"runtimeStatus" dc:"Current runtime status" eg:"stopped"`
	ProviderTaskStatus    string                       `json:"providerTaskStatus" dc:"Current Provider task status" eg:"succeeded"`
	ActiveOperationNo     string                       `json:"activeOperationNo" dc:"Current mutually exclusive operation number, or empty" eg:""`
	AvailableActions      []string                     `json:"availableActions" dc:"Actions allowed by capability, state, staff permission, and current local policy" eg:"[\"snapshot_restore\"]"`
	Accessories           []*ManagementServerAccessory `json:"accessories" dc:"Current safe accessory directory" eg:"[]"`
	AccessoriesObservedAt *int64                       `json:"accessoriesObservedAt" dc:"Directory observation time in Unix milliseconds, or null" eg:"1787184000000"`
	AccessoriesExpiresAt  *int64                       `json:"accessoriesExpiresAt" dc:"Directory freshness deadline in Unix milliseconds, or null" eg:"1787187600000"`
}

type ServerAccessoryManagementDetailReq struct {
	g.Meta    `path:"/admin/resource/server-accessories/{serviceNo}" method:"get" tags:"IDC Server Accessories" summary:"Get staff-visible server accessories" dc:"Return one server and its bounded safe accessory directory after exact staff permission and broad merchant data-scope checks. This endpoint never calls a Provider." permission:"idc:admin:resource:server-accessory:view"`
	ServiceNo string `json:"serviceNo" in:"path" v:"required|max-length:40" dc:"Stable target server service number" eg:"SVC_SERVER_0001"`
}

type ServerAccessoryManagementDetailRes struct {
	Detail *ManagementServerAccessoryDetail `json:"detail" dc:"Staff-visible server accessory detail" eg:"{}"`
}

type ServerAccessoryManagementOperationInput struct {
	PackageNo      string          `json:"packageNo" v:"max-length:40" dc:"Fixed traffic package selected for traffic purchases only" eg:"tpk_00000000000000000000000000000000"`
	PackageVersion int64           `json:"packageVersion" v:"min:0" dc:"Displayed package version; required for traffic purchases and zero otherwise" eg:"1"`
	ServiceNo      string          `json:"serviceNo" in:"path" v:"required|max-length:40" dc:"Stable target server service number" eg:"SVC_SERVER_0001"`
	RequestNo      string          `json:"requestNo" v:"required|min-length:8|max-length:256" dc:"Client-generated idempotency key reused only for the exact same operation" eg:"staff-accessory-550e8400-e29b-41d4-a716-446655440000"`
	Parameters     json.RawMessage `json:"parameters" v:"required" dc:"Action-specific object containing only safe local accessory numbers and validated values" eg:"{\"objectNo\":\"ACO_1\",\"confirmedServiceNo\":\"SVC_SERVER_0001\"}"`
	Confirmed      bool            `json:"confirmed" v:"required" dc:"Explicit ordinary impact confirmation from the staff page" eg:"true"`
}

type ServerAccessorySnapshotCreateReq struct {
	g.Meta `path:"/admin/resource/server-accessories/{serviceNo}/operations/snapshot-create" method:"post" tags:"IDC Server Accessories" summary:"Create one server snapshot for a customer" dc:"Create one confirmed snapshot request after exact staff permission, data-scope, capability, ownership, current price, service mutex, and unknown-result checks. A charge creates a customer pending invoice and never debits the customer balance." permission:"idc:admin:resource:server-accessory:snapshot:create"`
	ServerAccessoryManagementOperationInput
}

type ServerAccessorySnapshotDeleteReq struct {
	g.Meta `path:"/admin/resource/server-accessories/{serviceNo}/operations/snapshot-delete" method:"post" tags:"IDC Server Accessories" summary:"Delete one server snapshot" dc:"Create one confirmed snapshot deletion after exact staff permission, data-scope, capability, ownership, service mutex, and unknown-result checks." permission:"idc:admin:resource:server-accessory:snapshot:delete"`
	ServerAccessoryManagementOperationInput
}

type ServerAccessorySnapshotRestoreReq struct {
	g.Meta `path:"/admin/resource/server-accessories/{serviceNo}/operations/snapshot-restore" method:"post" tags:"IDC Server Accessories" summary:"Restore one server snapshot" dc:"Create one confirmed snapshot restore after exact staff permission, data-scope, capability, ownership, powered-off state, typed service-number, service mutex, and unknown-result checks." permission:"idc:admin:resource:server-accessory:snapshot:restore"`
	ServerAccessoryManagementOperationInput
}

type ServerAccessoryBackupCreateReq struct {
	g.Meta `path:"/admin/resource/server-accessories/{serviceNo}/operations/backup-create" method:"post" tags:"IDC Server Accessories" summary:"Create one server backup for a customer" dc:"Create one confirmed backup request after exact staff permission, data-scope, capability, ownership, current price, service mutex, and unknown-result checks. A charge creates a customer pending invoice and never debits the customer balance." permission:"idc:admin:resource:server-accessory:backup:create"`
	ServerAccessoryManagementOperationInput
}

type ServerAccessoryBackupDeleteReq struct {
	g.Meta `path:"/admin/resource/server-accessories/{serviceNo}/operations/backup-delete" method:"post" tags:"IDC Server Accessories" summary:"Delete one server backup" dc:"Create one confirmed backup deletion after exact staff permission, data-scope, capability, ownership, service mutex, and unknown-result checks." permission:"idc:admin:resource:server-accessory:backup:delete"`
	ServerAccessoryManagementOperationInput
}

type ServerAccessoryBackupRestoreReq struct {
	g.Meta `path:"/admin/resource/server-accessories/{serviceNo}/operations/backup-restore" method:"post" tags:"IDC Server Accessories" summary:"Restore one server backup" dc:"Create one confirmed backup restore after exact staff permission, data-scope, capability, ownership, powered-off state, typed service-number, service mutex, and unknown-result checks." permission:"idc:admin:resource:server-accessory:backup:restore"`
	ServerAccessoryManagementOperationInput
}

type ServerAccessorySecurityGroupAttachReq struct {
	g.Meta `path:"/admin/resource/server-accessories/{serviceNo}/operations/security-group-attach" method:"post" tags:"IDC Server Accessories" summary:"Attach one server security group" dc:"Create one confirmed security-group attachment after exact staff permission, data-scope, capability, ownership, service mutex, and unknown-result checks." permission:"idc:admin:resource:server-accessory:security-group:attach"`
	ServerAccessoryManagementOperationInput
}

type ServerAccessorySecurityGroupDetachReq struct {
	g.Meta `path:"/admin/resource/server-accessories/{serviceNo}/operations/security-group-detach" method:"post" tags:"IDC Server Accessories" summary:"Detach the server security group" dc:"Create one confirmed security-group detachment after exact staff permission, data-scope, capability, current relation, service mutex, and unknown-result checks." permission:"idc:admin:resource:server-accessory:security-group:detach"`
	ServerAccessoryManagementOperationInput
}

type ServerAccessoryNATCreateReq struct {
	g.Meta `path:"/admin/resource/server-accessories/{serviceNo}/operations/nat-create" method:"post" tags:"IDC Server Accessories" summary:"Create one server NAT rule" dc:"Create one confirmed NAT rule after exact staff permission, data-scope, capability, validated ports, service mutex, and unknown-result checks." permission:"idc:admin:resource:server-accessory:nat:create"`
	ServerAccessoryManagementOperationInput
}

type ServerAccessoryNATDeleteReq struct {
	g.Meta `path:"/admin/resource/server-accessories/{serviceNo}/operations/nat-delete" method:"post" tags:"IDC Server Accessories" summary:"Delete one server NAT rule" dc:"Create one confirmed NAT-rule deletion after exact staff permission, data-scope, capability, ownership, service mutex, and unknown-result checks." permission:"idc:admin:resource:server-accessory:nat:delete"`
	ServerAccessoryManagementOperationInput
}

type ServerAccessoryVirtualMediaMountReq struct {
	g.Meta `path:"/admin/resource/server-accessories/{serviceNo}/operations/virtual-media-mount" method:"post" tags:"IDC Server Accessories" summary:"Mount one server virtual medium" dc:"Create one confirmed virtual-media mount after exact staff permission, data-scope, capability, ownership, service mutex, and unknown-result checks." permission:"idc:admin:resource:server-accessory:virtual-media:mount"`
	ServerAccessoryManagementOperationInput
}

type ServerAccessoryVirtualMediaUnmountReq struct {
	g.Meta `path:"/admin/resource/server-accessories/{serviceNo}/operations/virtual-media-unmount" method:"post" tags:"IDC Server Accessories" summary:"Unmount the server virtual medium" dc:"Create one confirmed virtual-media unmount after exact staff permission, data-scope, capability, current relation, service mutex, and unknown-result checks." permission:"idc:admin:resource:server-accessory:virtual-media:unmount"`
	ServerAccessoryManagementOperationInput
}

type ServerAccessoryTrafficAddReq struct {
	g.Meta `path:"/admin/resource/server-accessories/{serviceNo}/operations/traffic-add" method:"post" tags:"IDC Server Accessories" summary:"Create one traffic package order for a customer" dc:"Create one confirmed traffic-package request after exact staff permission, data-scope, capability, ownership, current price, service mutex, and unknown-result checks. A charge creates a customer pending invoice and never debits the customer balance." permission:"idc:admin:resource:server-accessory:traffic:add"`
	ServerAccessoryManagementOperationInput
}

type ServerAccessoryManagementOperationRes struct {
	OperationNo     string `json:"operationNo" dc:"Stable operation number" eg:"sop_0123456789abcdef0123456789abcdef"`
	OrderNo         string `json:"orderNo" dc:"Customer order number when payment is required, or empty" eg:"aor_0123456789abcdef"`
	InvoiceNo       string `json:"invoiceNo" dc:"Customer pending invoice number when payment is required, or empty" eg:"ain_0123456789abcdef"`
	Action          string `json:"action" dc:"Accepted action code" eg:"snapshot_restore"`
	Status          string `json:"status" dc:"Current local operation status" eg:"ready"`
	AmountMinor     string `json:"amountMinor" dc:"Frozen charge in the currency's smallest unit, returned as a decimal string" eg:"1500"`
	CurrencyCode    string `json:"currencyCode" dc:"ISO 4217 currency code for the frozen charge, or empty for non-priced actions" eg:"CNY"`
	PaymentRequired bool   `json:"paymentRequired" dc:"Whether the customer must pay the pending invoice before execution" eg:"true"`
}

type ServerAccessorySnapshotCreateRes = ServerAccessoryManagementOperationRes
type ServerAccessorySnapshotDeleteRes = ServerAccessoryManagementOperationRes
type ServerAccessorySnapshotRestoreRes = ServerAccessoryManagementOperationRes
type ServerAccessoryBackupCreateRes = ServerAccessoryManagementOperationRes
type ServerAccessoryBackupDeleteRes = ServerAccessoryManagementOperationRes
type ServerAccessoryBackupRestoreRes = ServerAccessoryManagementOperationRes
type ServerAccessorySecurityGroupAttachRes = ServerAccessoryManagementOperationRes
type ServerAccessorySecurityGroupDetachRes = ServerAccessoryManagementOperationRes
type ServerAccessoryNATCreateRes = ServerAccessoryManagementOperationRes
type ServerAccessoryNATDeleteRes = ServerAccessoryManagementOperationRes
type ServerAccessoryVirtualMediaMountRes = ServerAccessoryManagementOperationRes
type ServerAccessoryVirtualMediaUnmountRes = ServerAccessoryManagementOperationRes
type ServerAccessoryTrafficAddRes = ServerAccessoryManagementOperationRes
