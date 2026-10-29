// This file defines the bounded local runtime-facts query for customer lists.
package v1

import "github.com/gogf/gf/v2/frame/g"

// NativeServerFactsReq uses array query encoding and never queries suppliers.
type NativeServerFactsReq struct {
	g.Meta         `path:"/account/native-server-facts" method:"get" tags:"IDC Account Servers" summary:"Read my servers' last known facts" dc:"Read only local facts for at most one hundred entitlement numbers encoded as entitlementNos[]. Empty input returns an empty list; duplicates are deduplicated; invalid or oversized input is rejected. Missing and foreign records are indistinguishable."`
	EntitlementNos []string `json:"entitlementNos" in:"query" dc:"At most one hundred entitlement numbers; omitted or empty returns no records" eg:"[\"ent_0123456789abcdef0123456789abcdef\"]"`
}

// NativeServerFact is a secret-free historical observation, never action authority.
type NativeServerFact struct {
	EntitlementNo      string   `json:"entitlementNo" dc:"Owned native service entitlement number" eg:"ent_0123456789abcdef0123456789abcdef"`
	ServerNo           string   `json:"serverNo" dc:"Resource-owned public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	Hostname           string   `json:"hostname" dc:"Last validated hostname, or empty when unavailable" eg:"cloud-example"`
	Addresses          []string `json:"addresses" dc:"Last validated IP address list; empty is not proof of no IP unless addressesConfirmed is true" eg:"[]"`
	AddressesConfirmed bool     `json:"addressesConfirmed" dc:"The last observation explicitly confirmed a complete address list" eg:"true"`
	OsName             string   `json:"osName" dc:"Last validated operating system name, or empty when unavailable" eg:"Debian 12"`
	LastObservedState  string   `json:"lastObservedState" dc:"Last confirmed state: pending, running, active, stopped, suspended, failed, terminated or unknown; never used to authorize writes" eg:"active"`
	ObservedAt         *int64   `json:"observedAt,omitempty" dc:"Last successful observation time as Unix timestamp in milliseconds; absent when never observed" eg:"1792382400000"`
	Stale              bool     `json:"stale" dc:"No observation exists or the last observation is more than five minutes old" eg:"false"`
}

// NativeServerFactsRes returns only currently owned active mappings.
type NativeServerFactsRes struct {
	Items []NativeServerFact `json:"items" dc:"At most one hundred current-customer facts; no upstream calls are made" eg:"[]"`
}
