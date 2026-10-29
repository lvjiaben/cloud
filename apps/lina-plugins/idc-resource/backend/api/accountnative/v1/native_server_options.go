// This file exposes only resource-owned image and accessory display projections.
package v1

import (
	"encoding/json"
	"github.com/gogf/gf/v2/frame/g"
)

// NativeServerOptionsReq loads local complete directories under customer ownership.
type NativeServerOptionsReq struct {
	g.Meta   `path:"/account/native-servers/{serverNo}/management-options" method:"get" tags:"IDC Account Servers" summary:"Read my server management choices" dc:"Read the last complete local image and accessory directories after ownership checks. Missing directories return empty choices and no timestamp; they never imply availability."`
	ServerNo string `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Public owned server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
}

// NativeServerImage is a normalized image choice with explicit selectability.
type NativeServerImage struct {
	Reference  string `json:"reference" dc:"Opaque image selection reference" eg:"101"`
	Name       string `json:"name" dc:"Image display name" eg:"Debian 12"`
	Group      string `json:"group" dc:"Image display group" eg:"Debian"`
	OSType     string `json:"osType" dc:"Declared operating system type; empty means unknown" eg:"linux"`
	Arch       string `json:"arch" dc:"Declared CPU architecture; empty means unknown" eg:"x86_64"`
	BootMode   string `json:"bootMode" dc:"Declared boot mode; empty means unknown" eg:"uefi"`
	Selectable bool   `json:"selectable" dc:"The complete snapshot explicitly confirms image availability" eg:"true"`
}

// NativeServerAccessory contains a local object number and safe display attributes.
type NativeServerAccessory struct {
	ObjectNo   string          `json:"objectNo" dc:"Opaque resource-owner object number; never an upstream ID" eg:"obj_example"`
	Kind       string          `json:"kind" dc:"Object kind: ip, disk, snapshot, backup, security_group, nat, virtual_media or traffic" eg:"disk"`
	Name       string          `json:"name" dc:"Safe object display name" eg:"System disk"`
	State      string          `json:"state" dc:"Object state: available, attached, active, mounted, inactive or unknown" eg:"attached"`
	Projection json.RawMessage `json:"projection" dc:"Resource-owner whitelisted attributes; never a supplier raw response" eg:"{}"`
}

// NativeServerOptionsRes carries explicit directory observation and expiry times.
type NativeServerOptionsRes struct {
	Images                []NativeServerImage     `json:"images" dc:"Complete image choices; an empty list never allows custom image references" eg:"[]"`
	ImagesObservedAt      *int64                  `json:"imagesObservedAt,omitempty" dc:"Image observation as Unix timestamp in milliseconds" eg:"1792382400000"`
	Accessories           []NativeServerAccessory `json:"accessories" dc:"Current resource-owned accessory directory" eg:"[]"`
	AccessoriesObservedAt *int64                  `json:"accessoriesObservedAt,omitempty" dc:"Accessory observation as Unix timestamp in milliseconds" eg:"1792382400000"`
	AccessoriesExpiresAt  *int64                  `json:"accessoriesExpiresAt,omitempty" dc:"Accessory directory expiry as Unix timestamp in milliseconds" eg:"1792382700000"`
}
