// This file defines independently paginated local resource overview tables.

package v1

import "github.com/gogf/gf/v2/frame/g"

type ResourceOverviewReq struct {
	g.Meta          `path:"/admin/resource/overview" method:"get" tags:"IDC Resource Overview" summary:"Get local resource operations overview" dc:"Return bounded local pool, providerCapacity, finance, expiry, and task projections without contacting any Provider." permission:"idc:admin:resource:overview:list"`
	PoolPage        int `json:"poolPage" in:"query" d:"1" v:"between:1,1000000" dc:"One-based resource pool page number" eg:"1"`
	PoolPageSize    int `json:"poolPageSize" in:"query" d:"10" v:"in:10,20,50,100" dc:"Resource pool page size" eg:"10"`
	FinancePage     int `json:"financePage" in:"query" d:"1" v:"between:1,1000000" dc:"One-based financial snapshot page number" eg:"1"`
	FinancePageSize int `json:"financePageSize" in:"query" d:"10" v:"in:10,20,50,100" dc:"Financial snapshot page size" eg:"10"`
}

type ResourceOverviewRes struct {
	Summary *ResourceOverviewSummary `json:"summary"`
}

type ResourceOverviewSummary struct {
	PoolTotal                int                     `json:"poolTotal" dc:"Exact resource pool count within the current staff scope" eg:"120"`
	FinanceTotal             int                     `json:"financeTotal" dc:"Exact latest financial snapshot count within the current staff scope" eg:"120"`
	EnabledChannels          int                     `json:"enabledChannels"`
	EnabledPools             int                     `json:"enabledPools"`
	ActiveMappings           int                     `json:"activeMappings"`
	PendingTasks             int                     `json:"pendingTasks"`
	UnknownTasks             int                     `json:"unknownTasks"`
	LatestProviderCapacityAt *int64                  `json:"latestProviderCapacityAt"`
	Pools                    []*ResourceOverviewPool `json:"pools"`
	Finance                  []*ResourceFinanceFact  `json:"finance"`
}

type ResourceOverviewPool struct {
	PoolNo          string `json:"poolNo"`
	ChannelNo       string `json:"channelNo"`
	ChannelName     string `json:"channelName"`
	DisplayName     string `json:"displayName"`
	ResourceKind    string `json:"resourceKind"`
	Status          string `json:"status"`
	RoutingPriority int    `json:"routingPriority"`
	RoutingWeight   int    `json:"routingWeight"`
	UpdatedAt       int64  `json:"updatedAt"`
}

type ResourceFinanceFact struct {
	SnapshotNo          string `json:"snapshotNo"`
	ChannelNo           string `json:"channelNo"`
	ChannelName         string `json:"channelName"`
	Kind                string `json:"kind"`
	ExternalResourceRef string `json:"externalResourceRef"`
	CurrencyCode        string `json:"currencyCode"`
	Amount              string `json:"amount"`
	ExpiresAt           *int64 `json:"expiresAt"`
	ObservedAt          int64  `json:"observedAt"`
}
