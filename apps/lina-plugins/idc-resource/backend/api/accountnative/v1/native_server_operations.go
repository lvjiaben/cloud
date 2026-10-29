package v1

import "github.com/gogf/gf/v2/frame/g"

type NativeServerOperationsReq struct {
	g.Meta   `path:"/account/native-servers/{serverNo}/operations" method:"get" tags:"IDC Account Servers" summary:"List my server operations" dc:"Read a bounded page and exact matching total of local native management operations after current ownership checks; no supplier calls or secret output reads occur."`
	ServerNo string `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Owned public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
	Page     int    `json:"page" d:"1" v:"min:1|max:1000000" dc:"One-based page number; defaults to 1" eg:"1"`
	Limit    int    `json:"limit" d:"20" v:"min:1|max:50" dc:"Maximum rows per page from 1 to 50; defaults to 20" eg:"20"`
	Action   string `json:"action" v:"max-length:64" dc:"Optional native action filter: start, stop, reboot, reset_password, console, reinstall, enter_rescue, exit_rescue, security_group_attach, security_group_detach, nat_create, nat_delete, virtual_media_mount, virtual_media_unmount, snapshot_restore, snapshot_delete, backup_restore, backup_delete; omitted means all native management actions" eg:"reinstall"`
}

type NativeServerOperationsRes struct {
	Items []NativeServerOperation `json:"items" dc:"Local management records ordered newest first; confidential outputs are only queried explicitly through operation detail" eg:"[]"`
	Total int                     `json:"total" dc:"Exact number of matching operations on this owned server" eg:"12"`
}
