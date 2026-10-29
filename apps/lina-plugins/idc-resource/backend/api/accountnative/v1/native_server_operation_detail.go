// This file defines local asynchronous operation status and explicit secret delivery.
package v1

import "github.com/gogf/gf/v2/frame/g"

// NativeServerOperationDetailReq never triggers a supplier request or retry.
type NativeServerOperationDetailReq struct {
	g.Meta      `path:"/account/native-servers/{serverNo}/operations/{operationNo}" method:"get" tags:"IDC Account Servers" summary:"Read my server operation" dc:"Read the exact locally recorded operation after proving current server ownership; unknown results never trigger another write."`
	ServerNo    string `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Owned public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	OperationNo string `json:"operationNo" in:"path" v:"required|regex:^nso_[0-9a-f]{32}$" dc:"Operation number returned when submitting this server action" eg:"nso_0123456789abcdef0123456789abcdef"`
}

// NativeServerOperationDetailRes returns local status rather than supplier acceptance as success.
type NativeServerOperationDetailRes struct {
	Operation NativeServerOperation `json:"operation" dc:"Local owner operation including safe failure and output availability" eg:"{}"`
}

// NativeServerOutputRevealReq consumes one exact succeeded operation output.
type NativeServerOutputRevealReq struct {
	g.Meta      `path:"/account/native-servers/{serverNo}/operations/{operationNo}/outputs/{name}/reveal" method:"post" tags:"IDC Account Servers" summary:"Reveal my operation output once" dc:"Consume the output under current ownership and entitlement before a no-store response. Lost responses remain consumed and must never be retried automatically."`
	ServerNo    string `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Owned public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	OperationNo string `json:"operationNo" in:"path" v:"required|regex:^nso_[0-9a-f]{32}$" dc:"Owned succeeded operation number" eg:"nso_0123456789abcdef0123456789abcdef"`
	Name        string `json:"name" in:"path" v:"required|in:generatedCredential,consoleTicket" dc:"generatedCredential for reinstall or consoleTicket for short-lived console access" eg:"generatedCredential"`
	Confirmed   bool   `json:"confirmed" v:"required" dc:"Explicit confirmation to consume this output once" eg:"true"`
}

// NativeServerOutputRevealRes is written only through the confidential no-store callback.
type NativeServerOutputRevealRes struct{}
