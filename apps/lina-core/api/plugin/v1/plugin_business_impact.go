// This file defines plugin new-business and lifecycle-impact management APIs.

package v1

import "github.com/gogf/gf/v2/frame/g"

// StopNewBusinessReq stops only new admission for one managed plugin.
type StopNewBusinessReq struct {
	g.Meta `path:"/plugins/{id}/new-business/stop" method:"put" tags:"Plugin Management" summary:"Stop plugin new business" permission:"plugin:stop-new-business" dc:"Stop new interface bindings, product-version bindings, purchases, and fulfillment tasks without disabling existing-service menus or operations."`
	Id     string `json:"id" v:"required|length:2,64" dc:"Managed plugin identifier" eg:"ssl-provider"`
}

// StopNewBusinessRes reports the persisted admission state.
type StopNewBusinessRes struct {
	Id               string           `json:"id" dc:"Managed plugin identifier" eg:"ssl-provider"`
	NewBusinessState NewBusinessState `json:"newBusinessState" dc:"Current new-business admission state" eg:"stopped"`
}

// ResumeNewBusinessReq reopens new admission after server-side readiness checks.
type ResumeNewBusinessReq struct {
	g.Meta `path:"/plugins/{id}/new-business/resume" method:"put" tags:"Plugin Management" summary:"Resume plugin new business" permission:"plugin:resume-new-business" dc:"Resume new interface bindings, product-version bindings, purchases, and fulfillment tasks only after the installed enabled release and dependencies pass readiness checks."`
	Id     string `json:"id" v:"required|length:2,64" dc:"Managed plugin identifier" eg:"ssl-provider"`
}

// ResumeNewBusinessRes reports the persisted admission state.
type ResumeNewBusinessRes struct {
	Id               string           `json:"id" dc:"Managed plugin identifier" eg:"ssl-provider"`
	NewBusinessState NewBusinessState `json:"newBusinessState" dc:"Current new-business admission state" eg:"accepting"`
}

// BusinessImpactReq queries trusted owner aggregates before disable or uninstall.
type BusinessImpactReq struct {
	g.Meta `path:"/plugins/{id}/business-impact" method:"get" tags:"Plugin Management" summary:"Check plugin business impact" permission:"plugin:business-impact" dc:"Return bounded aggregate counts from trusted built-in business owners. No object details, secrets, or plugin-private state are exposed."`
	Id     string `json:"id" v:"required|length:2,64" dc:"Managed plugin identifier" eg:"ssl-provider"`
}

// BusinessImpactRes is the complete impact snapshot used for operator review.
type BusinessImpactRes struct {
	Id            string                `json:"id" dc:"Managed plugin identifier" eg:"ssl-provider"`
	BlockingCount int64                 `json:"blockingCount" dc:"Total objects still blocking disable or uninstall" eg:"3"`
	CanDisable    bool                  `json:"canDisable" dc:"Whether the current authoritative impact snapshot permits disable" eg:"false"`
	CanUninstall  bool                  `json:"canUninstall" dc:"Whether the current authoritative impact snapshot permits uninstall before dependency and plugin-owned checks" eg:"false"`
	Items         []*BusinessImpactItem `json:"items" dc:"Bounded aggregate impact items" eg:"[]"`
}

// BusinessImpactItem is one trusted owner's aggregate lifecycle impact.
type BusinessImpactItem struct {
	OwnerPluginId string                 `json:"ownerPluginId" dc:"Built-in business owner that produced the aggregate" eg:"commerce-core"`
	Category      BusinessImpactCategory `json:"category" dc:"Impact category" eg:"active_services"`
	TotalCount    int64                  `json:"totalCount" dc:"Historical and active objects in this category" eg:"10"`
	BlockingCount int64                  `json:"blockingCount" dc:"Objects not yet drained, migrated, or taken over" eg:"3"`
}
