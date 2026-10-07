// Package resourceoverview provides one bounded, local-only staff projection
// for resource pools, providerCapacity, Provider finance facts, expiry differences,
// and task health. It never calls a Provider from an HTTP request.
package resourceoverview

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

const PermissionRead = "idc:admin:resource:overview:list"

var (
	CodeInvalid       = bizerr.MustDefine("IDC_RESOURCE_OVERVIEW_INVALID", "The resource overview request is invalid", gcode.CodeInvalidParameter)
	CodeNotAccessible = bizerr.MustDefine("IDC_RESOURCE_OVERVIEW_NOT_ACCESSIBLE", "The resource overview is not accessible", gcode.CodeNotAuthorized)
	CodeUnavailable   = bizerr.MustDefine("IDC_RESOURCE_OVERVIEW_UNAVAILABLE", "The resource overview is temporarily unavailable", gcode.CodeInternalError)
)

type Pool struct {
	PoolNo, ChannelNo, ChannelName, DisplayName, ResourceKind, Status string
	RoutingPriority, RoutingWeight                                    int
	UpdatedAt                                                         time.Time
}

type Finance struct {
	SnapshotNo, ChannelNo, ChannelName, Kind, ExternalResourceRef, CurrencyCode string
	Amount                                                                      string
	ExpiresAt                                                                   *time.Time
	ObservedAt                                                                  time.Time
}

type Summary struct {
	PoolTotal, FinanceTotal                                                   int
	EnabledChannels, EnabledPools, ActiveMappings, PendingTasks, UnknownTasks int
	LatestProviderCapacityAt                                                  *time.Time
	Pools                                                                     []Pool
	Finance                                                                   []Finance
}

// ListInput independently bounds the two overview tables.
type ListInput struct {
	PoolPage, PoolPageSize, FinancePage, FinancePageSize int
}

type Service interface {
	Get(context.Context, ListInput) (*Summary, error)
}

type serviceImpl struct {
	database gdb.DB
	runtime  managementcap.Runtime
}

func New(database gdb.DB, runtime managementcap.Runtime) (Service, error) {
	if database == nil || managementcap.ValidateRuntime(runtime) != nil {
		return nil, gerror.New("resource overview dependencies are invalid")
	}
	return &serviceImpl{database: database, runtime: runtime}, nil
}

// Get reads local overview facts without contacting upstream providers.
func (s *serviceImpl) Get(ctx context.Context, input ListInput) (*Summary, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	actor, err := s.runtime.Authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: PermissionRead, TargetNo: "resource-overview",
	})
	if err != nil || actor <= 0 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	input, err = normalizeList(input)
	if err != nil {
		return nil, err
	}
	type summaryRow struct {
		EnabledChannels          int        `orm:"enabled_channels"`
		EnabledPools             int        `orm:"enabled_pools"`
		ActiveMappings           int        `orm:"active_mappings"`
		PendingTasks             int        `orm:"pending_tasks"`
		UnknownTasks             int        `orm:"unknown_tasks"`
		LatestProviderCapacityAt *time.Time `orm:"latest_provider_capacity_at"`
	}
	var row *summaryRow
	if err = s.database.GetScan(ctx, &row, `SELECT
 (SELECT count(*) FROM plugin_idc_resource_channel WHERE tenant_id=? AND deleted_at IS NULL AND status_code='enabled') enabled_channels,
 (SELECT count(*) FROM plugin_idc_resource_pool WHERE tenant_id=? AND deleted_at IS NULL AND status_code='enabled') enabled_pools,
 (SELECT count(*) FROM plugin_idc_resource_mapping WHERE tenant_id=? AND status_code='active') active_mappings,
 (SELECT count(*) FROM plugin_idc_resource_task WHERE tenant_id=? AND status_code IN ('pending','submitting','accepted','running','retry_wait')) pending_tasks,
 (SELECT count(*) FROM plugin_idc_resource_task WHERE tenant_id=? AND status_code IN ('result_unknown','manual_takeover')) unknown_tasks,
 (SELECT max(observed_at) FROM plugin_idc_resource_provider_capacity_observation WHERE tenant_id=? AND status_code='complete') latest_provider_capacity_at`,
		idccap.DefaultTenantID, idccap.DefaultTenantID, idccap.DefaultTenantID,
		idccap.DefaultTenantID, idccap.DefaultTenantID, idccap.DefaultTenantID); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	var pools []struct {
		PoolNo, ChannelNo, ChannelName, DisplayName, ResourceKind, Status string
		RoutingPriority, RoutingWeight                                    int
		UpdatedAt                                                         time.Time
	}
	poolTotal, err := s.database.GetValue(ctx, "SELECT count(*) "+poolDirectorySQL, idccap.DefaultTenantID)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if err = s.database.GetScan(ctx, &pools, `SELECT pool.pool_no,pool.channel_no,channel.display_name channel_name,
pool.display_name,pool.resource_kind,pool.status_code status,pool.routing_priority,pool.routing_weight,pool.updated_at `+
		poolDirectorySQL+` ORDER BY pool.routing_priority,pool.display_name,pool.id LIMIT ? OFFSET ?`,
		idccap.DefaultTenantID, input.PoolPageSize, (input.PoolPage-1)*input.PoolPageSize); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	var finance []struct {
		SnapshotNo, ChannelNo, ChannelName, Kind, ExternalResourceRef, CurrencyCode string
		Amount                                                                      string
		ExpiresAt                                                                   *time.Time
		ObservedAt                                                                  time.Time
	}
	financeTotal, err := s.database.GetValue(ctx, "SELECT count(*) "+financeDirectorySQL, idccap.DefaultTenantID, idccap.DefaultTenantID)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if err = s.database.GetScan(ctx, &finance, `SELECT snapshot.snapshot_no,snapshot.channel_no,channel.display_name channel_name,
snapshot.snapshot_kind kind,snapshot.external_resource_ref,COALESCE(snapshot.currency_code,'') currency_code,
COALESCE(snapshot.amount::TEXT,'') amount,snapshot.expires_at,snapshot.observed_at `+financeDirectorySQL+
		` ORDER BY snapshot.snapshot_kind,snapshot.observed_at DESC,snapshot.id DESC LIMIT ? OFFSET ?`,
		idccap.DefaultTenantID, idccap.DefaultTenantID, input.FinancePageSize, (input.FinancePage-1)*input.FinancePageSize); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	result := &Summary{EnabledChannels: row.EnabledChannels, EnabledPools: row.EnabledPools,
		PoolTotal: poolTotal.Int(), FinanceTotal: financeTotal.Int(),
		ActiveMappings: row.ActiveMappings, PendingTasks: row.PendingTasks,
		UnknownTasks: row.UnknownTasks, LatestProviderCapacityAt: utc(row.LatestProviderCapacityAt),
		Pools: make([]Pool, len(pools)), Finance: make([]Finance, len(finance))}
	for index, item := range pools {
		result.Pools[index] = Pool{PoolNo: item.PoolNo, ChannelNo: item.ChannelNo,
			ChannelName: item.ChannelName, DisplayName: item.DisplayName,
			ResourceKind: item.ResourceKind, Status: item.Status,
			RoutingPriority: item.RoutingPriority, RoutingWeight: item.RoutingWeight,
			UpdatedAt: item.UpdatedAt.UTC()}
	}
	for index, item := range finance {
		result.Finance[index] = Finance{SnapshotNo: item.SnapshotNo, ChannelNo: item.ChannelNo,
			ChannelName: item.ChannelName, Kind: item.Kind,
			ExternalResourceRef: strings.TrimSpace(item.ExternalResourceRef),
			CurrencyCode:        item.CurrencyCode, Amount: item.Amount,
			ExpiresAt: utc(item.ExpiresAt), ObservedAt: item.ObservedAt.UTC()}
	}
	return result, nil
}

func utc(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}
