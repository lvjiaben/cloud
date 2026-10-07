// This file defines plugin dependency-check API DTOs.
package v1

import "github.com/gogf/gf/v2/frame/g"

// DependencyCheckReq is the request for checking plugin dependencies.
type DependencyCheckReq struct {
	g.Meta `path:"/plugins/{id}/dependencies" method:"get" tags:"Plugin Management" summary:"Check plugin dependencies" permission:"plugin:query" dc:"Return server-side plugin dependency check results, including framework compatibility, hard plugin dependency states, blockers, dependency cycles, and uninstall reverse dependents. The endpoint is read-only and does not install dependency plugins automatically."`
	Id     string `json:"id" v:"required|length:1,64" dc:"Plugin unique identifier" eg:"demo-source"`
}

// DependencyCheckRes is the response for checking plugin dependencies.
type DependencyCheckRes = PluginDependencyCheckResult
