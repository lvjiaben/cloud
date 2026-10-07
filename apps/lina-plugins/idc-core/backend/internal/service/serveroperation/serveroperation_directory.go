package serveroperation

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type DirectoryFilter struct {
	Limit  int
	Cursor string
}

type ActiveOperation struct {
	OperationNo string
	Action      serveroperationresourcecap.Action
	Status      string
	CreatedAt   time.Time
}

type Server struct {
	ServiceNo            string
	CustomerAlias        string
	CustomerRemark       string
	AutoRenewEnabled     bool
	RenewalCycles        int
	ActiveRefundNo       string
	ProductNameZhCN      string
	ProductNameEnUS      string
	ProductType          string
	EntitlementStatus    string
	RuntimeStatus        string
	ProviderTaskStatus   string
	PeriodUnit           string
	PeriodCount          int
	EntitlementStartedAt *time.Time
	EntitlementExpiresAt *time.Time
	UpdatedAt            time.Time
	AvailableActions     []serveroperationresourcecap.Action
	CapabilityObservedAt *time.Time
	ActiveOperation      *ActiveOperation
	Version              int64
	channelNo            string
	poolNo               string
}

type ConfigurationItem struct {
	ComponentType   string
	GroupNameZhCN   string
	GroupNameEnUS   string
	OptionNameZhCN  string
	OptionNameEnUS  string
	NormalizedValue json.RawMessage
	Quantity        int
}

type OperationHistory struct {
	OperationNo                 string
	Action                      serveroperationresourcecap.Action
	Status                      string
	FailureCode                 string
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
	CompletedAt                 *time.Time
	ConfidentialOutputAvailable bool
}

type ServerDetail struct {
	Server
	Configuration             []ConfigurationItem
	Operations                []OperationHistory
	ReinstallImages           []serveroperationresourcecap.ReinstallImage
	ReinstallImagesObservedAt *time.Time
	Accessories               []serveroperationresourcecap.AccessoryItem
	AccessoriesObservedAt     *time.Time
	AccessoriesExpiresAt      *time.Time
}

type ServerPage struct {
	Items      []Server
	NextCursor string
}

type Directory interface {
	ListCurrentCustomer(context.Context, DirectoryFilter) (*ServerPage, error)
	GetCurrentCustomer(context.Context, string) (*ServerDetail, error)
}

type directoryImpl struct {
	database     gdb.DB
	customers    identitycap.CurrentCustomerReader
	capabilities serveroperationresourcecap.CapabilityDirectory
	images       serveroperationresourcecap.ReinstallDirectory
	accessories  serveroperationresourcecap.AccessoryDirectory
}

func NewDirectory(database gdb.DB, customers identitycap.CurrentCustomerReader,
	capabilities serveroperationresourcecap.CapabilityDirectory,
	images serveroperationresourcecap.ReinstallDirectory,
	accessories serveroperationresourcecap.AccessoryDirectory) (Directory, error) {
	if database == nil || customers == nil || capabilities == nil || images == nil || accessories == nil {
		return nil, ErrServiceUnavailable
	}
	return &directoryImpl{database: database, customers: customers, capabilities: capabilities,
		images: images, accessories: accessories}, nil
}

type serverDirectoryRow struct {
	InternalID           int64      `orm:"internal_id"`
	ServiceNo            string     `orm:"service_no"`
	CustomerAlias        string     `orm:"customer_alias"`
	CustomerRemark       string     `orm:"customer_remark"`
	AutoRenewEnabled     bool       `orm:"auto_renew_enabled"`
	RenewalCycles        int        `orm:"renewal_cycles"`
	ActiveRefundNo       string     `orm:"active_refund_no"`
	ProductNameZhCN      string     `orm:"product_name_zh_cn"`
	ProductNameEnUS      string     `orm:"product_name_en_us"`
	ProductType          string     `orm:"product_type_code"`
	EntitlementStatus    string     `orm:"entitlement_status_code"`
	RuntimeStatus        string     `orm:"runtime_status_code"`
	TrafficSuspended     bool       `orm:"traffic_suspended"`
	ProviderTaskStatus   string     `orm:"provider_task_status_code"`
	PeriodUnit           string     `orm:"period_unit_code"`
	PeriodCount          int        `orm:"period_count"`
	EntitlementStartedAt *time.Time `orm:"entitlement_started_at"`
	EntitlementExpiresAt *time.Time `orm:"entitlement_expires_at"`
	UpdatedAt            time.Time  `orm:"updated_at"`
	Version              int64      `orm:"version"`
	ChannelNo            string     `orm:"channel_no"`
	PoolNo               string     `orm:"pool_no"`
	OperationNo          string     `orm:"operation_no"`
	OperationAction      string     `orm:"operation_action"`
	OperationStatus      string     `orm:"operation_status"`
	OperationCreatedAt   *time.Time `orm:"operation_created_at"`
}

func (s *directoryImpl) ListCurrentCustomer(ctx context.Context,
	filter DirectoryFilter) (*ServerPage, error) {
	if s == nil || s.database == nil || s.customers == nil || s.capabilities == nil ||
		ctx == nil || ctx.Err() != nil || filter.Limit < 0 || filter.Limit > 50 ||
		(filter.Cursor != "" && !validReference(filter.Cursor, 40)) {
		return nil, ErrInvalidInput
	}
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || !validReference(subject.CustomerNo, 40) {
		return nil, ErrServiceUnavailable
	}
	boundary := int64(1<<63 - 1)
	if filter.Cursor != "" {
		value, boundaryErr := s.database.GetValue(ctx, `SELECT id
FROM plugin_idc_core_service_instance
WHERE tenant_id = 0 AND customer_no = ? AND service_no = ?`, subject.CustomerNo, filter.Cursor)
		if boundaryErr != nil || value.IsNil() {
			return nil, ErrInvalidInput
		}
		boundary = value.Int64()
	}
	var rows []serverDirectoryRow
	if err = s.database.GetScan(ctx, &rows, serverDirectorySQL+` AND service.id < ?
ORDER BY service.id DESC LIMIT ?`, subject.CustomerNo, boundary, filter.Limit+1); err != nil {
		return nil, gerror.Wrap(err, "list customer servers")
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	items, err := s.projectServers(ctx, rows)
	if err != nil {
		return nil, err
	}
	page := &ServerPage{Items: items}
	if hasMore && len(items) > 0 {
		page.NextCursor = items[len(items)-1].ServiceNo
	}
	return page, nil
}

func (s *directoryImpl) GetCurrentCustomer(ctx context.Context,
	serviceNo string) (*ServerDetail, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || !validReference(serviceNo, 40) {
		return nil, ErrInvalidInput
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || !validReference(subject.CustomerNo, 40) {
		return nil, ErrServiceUnavailable
	}
	var row *serverDirectoryRow
	if err = s.database.GetScan(ctx, &row, serverDirectorySQL+` AND service.service_no = ?`,
		subject.CustomerNo, serviceNo); err != nil {
		return nil, gerror.Wrap(err, "read customer server")
	}
	if row == nil {
		return nil, ErrServiceUnavailable
	}
	servers, err := s.projectServers(ctx, []serverDirectoryRow{*row})
	if err != nil {
		return nil, err
	}
	var configuration []ConfigurationItem
	if err = s.database.GetScan(ctx, &configuration, `SELECT component_type_code,group_name_zh_cn,
group_name_en_us,option_name_zh_cn,option_name_en_us,normalized_value,quantity
FROM plugin_idc_core_commerce_configuration_snapshot_item item
JOIN plugin_idc_core_service_instance service
 ON service.tenant_id=item.tenant_id AND service.configuration_snapshot_no=item.configuration_snapshot_no
WHERE service.tenant_id=0 AND service.customer_no=? AND service.service_no=?
ORDER BY item.sort_order,item.id LIMIT 256`, subject.CustomerNo, serviceNo); err != nil {
		return nil, gerror.Wrap(err, "read customer server configuration")
	}
	var operations []OperationHistory
	if err = s.database.GetScan(ctx, &operations, `SELECT operation.operation_no,execution.action_code AS action,
operation.status_code AS status,execution.last_failure_code AS failure_code,
operation.created_at,operation.updated_at,execution.completed_at
,(execution.output_bindings <> '[]'::jsonb) AS confidential_output_available
FROM plugin_idc_core_business_operation operation
JOIN plugin_idc_core_server_operation_execution execution
 ON execution.tenant_id=operation.tenant_id AND execution.operation_no=operation.operation_no
WHERE operation.tenant_id=0 AND operation.customer_no=? AND operation.service_no=?
ORDER BY operation.id DESC LIMIT 50`, subject.CustomerNo, serviceNo); err != nil {
		return nil, gerror.Wrap(err, "read customer server operation history")
	}
	imageProjection, err := s.images.GetReinstallImages(ctx, serveroperationresourcecap.CapabilityRoute{
		ServiceNo: servers[0].ServiceNo, ChannelNo: servers[0].channelNo, PoolNo: servers[0].poolNo,
	})
	if err != nil || imageProjection == nil || imageProjection.ServiceNo != servers[0].ServiceNo {
		return nil, ErrServiceUnavailable
	}
	accessoryProjection, err := s.accessories.GetAccessories(ctx, serveroperationresourcecap.CapabilityRoute{
		ServiceNo: servers[0].ServiceNo, ChannelNo: servers[0].channelNo, PoolNo: servers[0].poolNo,
	})
	if err != nil || accessoryProjection == nil || accessoryProjection.ServiceNo != servers[0].ServiceNo {
		return nil, ErrServiceUnavailable
	}
	detail := &ServerDetail{Server: servers[0], Configuration: configuration, Operations: operations,
		ReinstallImages: append([]serveroperationresourcecap.ReinstallImage(nil), imageProjection.Images...),
		Accessories:     append([]serveroperationresourcecap.AccessoryItem(nil), accessoryProjection.Items...)}
	if !imageProjection.ObservedAt.IsZero() {
		observed := imageProjection.ObservedAt.UTC()
		detail.ReinstallImagesObservedAt = &observed
	}
	if !accessoryProjection.ObservedAt.IsZero() {
		observed := accessoryProjection.ObservedAt.UTC()
		detail.AccessoriesObservedAt = &observed
	}
	if !accessoryProjection.ExpiresAt.IsZero() {
		expires := accessoryProjection.ExpiresAt.UTC()
		detail.AccessoriesExpiresAt = &expires
	}
	return detail, nil
}

func (s *directoryImpl) projectServers(ctx context.Context,
	rows []serverDirectoryRow) ([]Server, error) {
	if len(rows) == 0 {
		return []Server{}, nil
	}
	routes := make([]serveroperationresourcecap.CapabilityRoute, 0, len(rows))
	for _, row := range rows {
		// Historical acceptance data may predate resource-route binding. Such a
		// row is still safe to show, but it must expose no operation instead of
		// making the whole server list unavailable.
		if !validReference(row.ChannelNo, 40) || !validReference(row.PoolNo, 40) {
			continue
		}
		routes = append(routes, serveroperationresourcecap.CapabilityRoute{
			ServiceNo: row.ServiceNo, ChannelNo: row.ChannelNo, PoolNo: row.PoolNo})
	}
	projections := []serveroperationresourcecap.CapabilityProjection{}
	if len(routes) > 0 {
		var err error
		projections, err = s.capabilities.ListCapabilities(ctx, routes)
		if err != nil || len(projections) != len(routes) {
			return nil, ErrServiceUnavailable
		}
	}
	byService := make(map[string]serveroperationresourcecap.CapabilityProjection, len(projections))
	for _, projection := range projections {
		byService[projection.ServiceNo] = projection
	}
	items := make([]Server, 0, len(rows))
	for _, row := range rows {
		item := Server{ServiceNo: row.ServiceNo, CustomerAlias: row.CustomerAlias,
			CustomerRemark: row.CustomerRemark, AutoRenewEnabled: row.AutoRenewEnabled,
			RenewalCycles: row.RenewalCycles, ActiveRefundNo: row.ActiveRefundNo,
			ProductNameZhCN: row.ProductNameZhCN,
			ProductNameEnUS: row.ProductNameEnUS, ProductType: row.ProductType,
			EntitlementStatus: row.EntitlementStatus, RuntimeStatus: row.RuntimeStatus,
			ProviderTaskStatus: row.ProviderTaskStatus, PeriodUnit: row.PeriodUnit,
			PeriodCount: row.PeriodCount, EntitlementStartedAt: row.EntitlementStartedAt,
			EntitlementExpiresAt: row.EntitlementExpiresAt, UpdatedAt: row.UpdatedAt.UTC(),
			Version: row.Version, channelNo: row.ChannelNo, poolNo: row.PoolNo}
		if row.OperationNo != "" && row.OperationCreatedAt != nil {
			item.ActiveOperation = &ActiveOperation{OperationNo: row.OperationNo,
				Action: serveroperationresourcecap.Action(row.OperationAction), Status: row.OperationStatus,
				CreatedAt: row.OperationCreatedAt.UTC()}
		}
		if projection, exists := byService[row.ServiceNo]; exists && item.ActiveOperation == nil &&
			(row.ProviderTaskStatus == "none" || row.ProviderTaskStatus == "succeeded" || row.ProviderTaskStatus == "failed") {
			for _, action := range projection.Actions {
				if action == serveroperationresourcecap.ActionResize {
					continue
				}
				if allowedAction(&serviceRow{EntitlementStatusCode: row.EntitlementStatus,
					RuntimeStatusCode: row.RuntimeStatus, TrafficSuspended: row.TrafficSuspended}, action) {
					item.AvailableActions = append(item.AvailableActions, action)
				}
			}
			if !projection.ObservedAt.IsZero() {
				observed := projection.ObservedAt.UTC()
				item.CapabilityObservedAt = &observed
			}
		}
		if item.AvailableActions == nil {
			item.AvailableActions = []serveroperationresourcecap.Action{}
		}
		items = append(items, item)
	}
	return items, nil
}

const serverDirectorySQL = `SELECT service.id AS internal_id,service.service_no,
service.customer_alias,service.customer_remark,
COALESCE(renewal_policy.auto_renew_enabled,FALSE) AS auto_renew_enabled,
COALESCE(renewal_policy.renewal_cycles,0) AS renewal_cycles,
COALESCE(active_refund.refund_no,'') AS active_refund_no,
COALESCE(order_item.product_name_zh_cn,product.name_zh_cn) AS product_name_zh_cn,
COALESCE(order_item.product_name_en_us,product.name_en_us) AS product_name_en_us,
service.product_type_code,service.entitlement_status_code,service.runtime_status_code,
EXISTS (SELECT 1 FROM plugin_idc_core_service_traffic_suspension suspension
 WHERE suspension.tenant_id = service.tenant_id AND suspension.service_no = service.service_no
 AND suspension.active) AS traffic_suspended,
service.provider_task_status_code,service.period_unit_code,service.period_count,
service.entitlement_started_at,service.entitlement_expires_at,service.updated_at,service.version,
COALESCE(service.channel_no,'') AS channel_no,service.pool_no,
COALESCE(active.operation_no,'') AS operation_no,COALESCE(active.action_code,'') AS operation_action,
COALESCE(active.status_code,'') AS operation_status,active.created_at AS operation_created_at
FROM plugin_idc_core_service_instance service
JOIN plugin_idc_core_catalog_product product
 ON product.tenant_id=service.tenant_id AND product.product_no=service.product_no
LEFT JOIN plugin_idc_core_commerce_order_item order_item
 ON order_item.tenant_id=service.tenant_id AND order_item.order_no=service.order_no
 AND order_item.order_item_no=service.order_item_no
LEFT JOIN plugin_idc_core_service_renewal_policy renewal_policy
 ON renewal_policy.tenant_id=service.tenant_id
 AND renewal_policy.policy_no=service.current_renewal_policy_no
 AND renewal_policy.policy_version=service.current_renewal_policy_version
LEFT JOIN LATERAL (
 SELECT entitlement_lock.refund_no
 FROM plugin_idc_core_commerce_fulfillment_unit fulfillment
 JOIN plugin_idc_core_service_entitlement entitlement
  ON entitlement.tenant_id=fulfillment.tenant_id
 AND entitlement.order_item_no=fulfillment.order_item_no
 AND entitlement.unit_sequence=fulfillment.unit_sequence
 JOIN plugin_idc_core_refund_entitlement_lock entitlement_lock
  ON entitlement_lock.tenant_id=entitlement.tenant_id
 AND entitlement_lock.entitlement_no=entitlement.entitlement_no
 AND entitlement_lock.released_at IS NULL
 WHERE fulfillment.tenant_id=service.tenant_id
  AND fulfillment.fulfillment_no=service.source_fulfillment_no
 ORDER BY entitlement_lock.id DESC LIMIT 1
) active_refund ON TRUE
LEFT JOIN LATERAL (
 SELECT operation.operation_no,execution.action_code,operation.status_code,operation.created_at
 FROM plugin_idc_core_server_operation_execution execution
 JOIN plugin_idc_core_business_operation operation
  ON operation.tenant_id=execution.tenant_id AND operation.operation_no=execution.operation_no
 WHERE execution.tenant_id=service.tenant_id AND execution.service_no=service.service_no
  AND execution.status_code IN ('ready','pending','running','result_unknown')
 ORDER BY execution.id DESC LIMIT 1
) active ON TRUE
WHERE service.tenant_id=0 AND service.customer_no=?`
