// This file defines plugin uninstallation API DTOs and the typed policy flags
// used for storage cleanup and lifecycle response state.

package v1

import (
	"lina-core/pkg/statusflag"

	"github.com/gogf/gf/v2/frame/g"
)

// UninstallReq is the request for uninstalling a plugin.
type UninstallReq struct {
	g.Meta           `path:"/plugins/{id}" method:"delete" tags:"Plugin Management" summary:"Uninstall plugin" permission:"plugin:uninstall" dc:"Execute the plugin's uninstall life cycle. Both source plugins and dynamic plugins will deactivate the plugin at this stage, and you can click the confirmation option to decide whether to clean up the plugin's own storage data at the same time; after checking, the host will execute the uninstall SQL under manifest/sql/uninstall, and the dynamic plugin will also clean up the plugin's own storage files according to authorized storage paths."`
	Id               string            `json:"id" v:"required|length:1,64" dc:"Plugin unique identifier" eg:"demo-source"`
	PurgeStorageData *statusflag.YesNo `json:"purgeStorageData" dc:"Whether to clear the plugin's own technical storage data at the same time when uninstalling the plugin: 1=clear explicitly permitted plugin-private data and files 0=keep; data is preserved by default and business evidence cannot be purged" eg:"0"`
	Force            bool              `json:"force" dc:"Whether to bypass only plugin-owned lifecycle callback vetoes when host policy permits it; trusted business-impact, stop-new-business, dependency, and historical-evidence gates cannot be bypassed" eg:"false"`
}

// UninstallRes is the response for uninstalling a plugin.
type UninstallRes struct {
	Id        string                  `json:"id" dc:"Plugin unique identifier" eg:"demo-source"`
	Installed statusflag.Installation `json:"installed" dc:"Installation status: 1=Installed 0=Not installed" eg:"0"`
	Enabled   statusflag.Enabled      `json:"enabled" dc:"Enabled status: 1=enabled 0=disabled" eg:"0"`
}
