// Package serverdirectory provides the resource owner's customer-scoped,
// read-only server directory. It never reads another plugin's tables.
package serverdirectory

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

const MaximumPageSize = 50

var (
	ErrInvalidRequest = errors.New("resource server directory request is invalid")
	ErrNotFound       = errors.New("resource server directory item is unavailable")
	identityPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,39}$`)
	actionPattern     = regexp.MustCompile(`^srv_[0-9a-f]{32}$`)
	serverNoPattern   = regexp.MustCompile(`^SVR-[0-9A-F]{32}$`)
)

// TrustedCustomerScope is created by a trusted caller after authentication;
// a future HTTP handler must never populate it from a request body or query.
type TrustedCustomerScope struct{ CustomerNo string }

type ResourceState string

const (
	ResourceStateReady     ResourceState = "ready"
	ResourceStatePreparing ResourceState = "preparing"
	ResourceStateAttention ResourceState = "attention_required"
	ResourceStateReleasing ResourceState = "releasing"
	ResourceStateReleased  ResourceState = "released"
	ResourceStateUnknown   ResourceState = "unknown"
)

type OperationState string

const (
	OperationStateIdle      OperationState = "idle"
	OperationStateWorking   OperationState = "working"
	OperationStateAttention OperationState = "attention_required"
	OperationStateCompleted OperationState = "completed"
	OperationStateFailed    OperationState = "unsuccessful"
	OperationStateUnknown   OperationState = "unknown"
)

type ServerKind string

const (
	ServerKindCloud     ServerKind = "cloud_server"
	ServerKindBareMetal ServerKind = "bare_metal_server"
	ServerKindOther     ServerKind = "other_server"
)

type Action string

const (
	ActionStart          Action = "start"
	ActionStop           Action = "stop"
	ActionReboot         Action = "reboot"
	ActionSuspend        Action = "suspend"
	ActionResume         Action = "resume"
	ActionRenew          Action = "renew"
	ActionResetPassword  Action = "reset_password"
	ActionConsole        Action = "console"
	ActionReinstall      Action = "reinstall"
	ActionEnterRescue    Action = "enter_rescue"
	ActionExitRescue     Action = "exit_rescue"
	ActionResize         Action = "resize"
	ActionChangeIP       Action = "change_ip"
	ActionSnapshot       Action = "snapshot"
	ActionBackup         Action = "backup"
	ActionSecurityGroup  Action = "security_group"
	ActionNAT            Action = "network_forwarding"
	ActionVirtualMedia   Action = "virtual_media"
	ActionTrafficPackage Action = "traffic_package"
	ActionTicket         Action = "support_ticket"
	ActionTerminate      Action = "release"
)

type Item struct {
	ServerNo         string
	ActionReference  string
	ProductNameZhCN  string
	ProductNameEnUS  string
	ChannelName      string
	PoolName         string
	Kind             ServerKind
	State            ResourceState
	LatestOperation  OperationState
	AvailableActions []Action
	BoundAt          time.Time
	UpdatedAt        time.Time
}

type Page struct {
	Total      int
	Items      []Item
	NextCursor string
}

type ListRequest struct {
	Scope  TrustedCustomerScope
	Cursor string
	Limit  int
}

type DetailRequest struct {
	Scope    TrustedCustomerScope
	ServerNo string
}

type databaseScanner interface {
	GetScan(context.Context, any, string, ...any) error
}

type Service struct{ database databaseScanner }

func New(database gdb.DB) (*Service, error) {
	if database == nil {
		return nil, gerror.New("resource server directory database is required")
	}
	return newWithScanner(database)
}

func newWithScanner(database databaseScanner) (*Service, error) {
	if database == nil {
		return nil, gerror.New("resource server directory database is required")
	}
	return &Service{database: database}, nil
}

type directoryRow struct {
	ServerNo        string    `orm:"server_no"`
	ActionReference string    `orm:"action_reference"`
	ProductNameZhCN string    `orm:"product_name_zh_cn"`
	ProductNameEnUS string    `orm:"product_name_en_us"`
	ChannelName     string    `orm:"channel_name"`
	PoolName        string    `orm:"pool_name"`
	ResourceKind    string    `orm:"resource_kind"`
	MappingStatus   string    `orm:"mapping_status"`
	TaskStatus      string    `orm:"task_status"`
	Capabilities    string    `orm:"capabilities"`
	BoundAt         time.Time `orm:"bound_at"`
	UpdatedAt       time.Time `orm:"updated_at"`
}

func (service *Service) List(ctx context.Context, request ListRequest) (*Page, error) {
	if service == nil || service.database == nil || ctx == nil || ctx.Err() != nil ||
		!validScope(request.Scope) || request.Limit < 1 || request.Limit > MaximumPageSize {
		return nil, ErrInvalidRequest
	}
	cursor, err := decodeCursor(request.Cursor)
	if err != nil {
		return nil, ErrInvalidRequest
	}
	var rows []directoryRow
	if err = service.database.GetScan(ctx, &rows, directoryQuery, request.Scope.CustomerNo,
		cursor, request.Limit+1); err != nil {
		return nil, gerror.Wrap(err, "list resource-owned servers")
	}
	page := &Page{Items: make([]Item, 0, min(len(rows), request.Limit))}
	for index := 0; index < len(rows) && index < request.Limit; index++ {
		item, projectErr := projectRow(rows[index])
		if projectErr != nil {
			return nil, projectErr
		}
		page.Items = append(page.Items, item)
	}
	if len(rows) > request.Limit && len(page.Items) > 0 {
		page.NextCursor = encodeCursor(page.Items[len(page.Items)-1].ServerNo)
	}
	return page, nil
}

func (service *Service) Detail(ctx context.Context, request DetailRequest) (*Item, error) {
	if service == nil || service.database == nil || ctx == nil || ctx.Err() != nil ||
		!validScope(request.Scope) || !serverNoPattern.MatchString(request.ServerNo) {
		return nil, ErrInvalidRequest
	}
	var rows []directoryRow
	if err := service.database.GetScan(ctx, &rows, directoryDetailQuery,
		request.Scope.CustomerNo, request.ServerNo); err != nil {
		return nil, gerror.Wrap(err, "read resource-owned server")
	}
	if len(rows) != 1 {
		return nil, ErrNotFound
	}
	item, err := projectRow(rows[0])
	if err != nil || item.ServerNo != request.ServerNo {
		return nil, ErrNotFound
	}
	return &item, nil
}

func validScope(scope TrustedCustomerScope) bool {
	return scope.CustomerNo == strings.TrimSpace(scope.CustomerNo) &&
		identityPattern.MatchString(scope.CustomerNo)
}

func encodeCursor(serverNo string) string {
	return "v1." + base64.RawURLEncoding.EncodeToString([]byte(serverNo))
}

func decodeCursor(cursor string) (string, error) {
	if cursor == "" {
		return "", nil
	}
	if !strings.HasPrefix(cursor, "v1.") {
		return "", ErrInvalidRequest
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(cursor, "v1."))
	if err != nil || !serverNoPattern.Match(decoded) {
		return "", ErrInvalidRequest
	}
	return string(decoded), nil
}

func projectRow(row directoryRow) (Item, error) {
	if !serverNoPattern.MatchString(row.ServerNo) || !actionPattern.MatchString(row.ActionReference) ||
		strings.TrimSpace(row.ProductNameZhCN) == "" ||
		strings.TrimSpace(row.ProductNameEnUS) == "" ||
		strings.TrimSpace(row.ChannelName) == "" ||
		strings.TrimSpace(row.PoolName) == "" {
		return Item{}, ErrNotFound
	}
	var capabilityCodes []string
	if json.Unmarshal([]byte(row.Capabilities), &capabilityCodes) != nil {
		return Item{}, gerror.New("resource server capabilities are unavailable")
	}
	return Item{
		ServerNo:         row.ServerNo,
		ActionReference:  row.ActionReference,
		ProductNameZhCN:  strings.TrimSpace(row.ProductNameZhCN),
		ProductNameEnUS:  strings.TrimSpace(row.ProductNameEnUS),
		ChannelName:      strings.TrimSpace(row.ChannelName),
		PoolName:         strings.TrimSpace(row.PoolName),
		Kind:             projectKind(row.ResourceKind),
		State:            projectResourceState(row.MappingStatus),
		LatestOperation:  projectOperationState(row.TaskStatus),
		AvailableActions: projectActions(capabilityCodes),
		BoundAt:          row.BoundAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
	}, nil
}

func projectKind(value string) ServerKind {
	switch value {
	case "virtual_server":
		return ServerKindCloud
	case "bare_metal":
		return ServerKindBareMetal
	default:
		return ServerKindOther
	}
}

func projectResourceState(value string) ResourceState {
	switch value {
	case "active":
		return ResourceStateReady
	case "provisioning":
		return ResourceStatePreparing
	case "quarantined":
		return ResourceStateAttention
	case "releasing":
		return ResourceStateReleasing
	case "released":
		return ResourceStateReleased
	default:
		return ResourceStateUnknown
	}
}

func projectOperationState(value string) OperationState {
	switch value {
	case "":
		return OperationStateIdle
	case "accepted", "pending", "retry_wait", "running", "submitting":
		return OperationStateWorking
	case "manual_takeover", "result_unknown":
		return OperationStateAttention
	case "succeeded":
		return OperationStateCompleted
	case "canceled", "failed":
		return OperationStateFailed
	default:
		return OperationStateUnknown
	}
}

var actionByCapability = map[string]Action{
	"server.start": ActionStart, "server.stop": ActionStop,
	"server.reboot": ActionReboot, "server.suspend": ActionSuspend,
	"server.resume": ActionResume, "server.renew": ActionRenew,
	"server.password.reset": ActionResetPassword, "server.console": ActionConsole,
	"server.reinstall": ActionReinstall, "server.rescue.enter": ActionEnterRescue,
	"server.rescue.exit": ActionExitRescue, "server.resize": ActionResize,
	"server.ip.change": ActionChangeIP, "server.snapshot": ActionSnapshot,
	"server.backup": ActionBackup, "server.security-group": ActionSecurityGroup,
	"server.nat": ActionNAT, "server.virtual-media": ActionVirtualMedia,
	"server.traffic-package": ActionTrafficPackage, "ticket.manage": ActionTicket,
	"server.terminate": ActionTerminate,
}

func projectActions(capabilityCodes []string) []Action {
	result := make([]Action, 0, len(capabilityCodes))
	seen := make(map[Action]struct{}, len(capabilityCodes))
	for _, code := range capabilityCodes {
		action, ok := actionByCapability[code]
		if !ok {
			continue
		}
		if _, duplicated := seen[action]; duplicated {
			continue
		}
		seen[action] = struct{}{}
		result = append(result, action)
	}
	return result
}

const directoryQuery = `WITH selected AS (
    SELECT ownership.server_no,ownership.action_reference,ownership.product_name_zh_cn,
           ownership.product_name_en_us,ownership.local_resource_no,
           ownership.bound_at,mapping.channel_no,mapping.pool_no,
           mapping.status_code AS mapping_status,
           GREATEST(ownership.updated_at,mapping.updated_at) AS updated_at,
           channel.display_name AS channel_name,pool.display_name AS pool_name,
           pool.resource_kind
    FROM plugin_idc_resource_server_entitlement_projection ownership
    JOIN plugin_idc_resource_mapping mapping
      ON mapping.tenant_id=ownership.tenant_id
     AND mapping.mapping_no=ownership.mapping_no
     AND mapping.local_resource_no=ownership.local_resource_no
    JOIN plugin_idc_resource_channel channel
      ON channel.tenant_id=mapping.tenant_id AND channel.channel_no=mapping.channel_no
    JOIN plugin_idc_resource_pool pool
      ON pool.tenant_id=mapping.tenant_id AND pool.channel_no=mapping.channel_no
     AND pool.pool_no=mapping.pool_no
    WHERE ownership.tenant_id=0 AND ownership.customer_no=?
      AND ownership.server_no>?
    ORDER BY ownership.server_no
    LIMIT ?
)` + directoryEnrichmentCTE + `
SELECT server_no,action_reference,product_name_zh_cn,product_name_en_us,
       channel_name,pool_name,resource_kind,mapping_status,task_status,
       capabilities,bound_at,updated_at
FROM projected ORDER BY server_no`

// directoryDetailQuery deliberately establishes the trusted customer and
// opaque reference boundary in the first indexed read. Enrichment only sees
// that already-scoped row, so a caller cannot use related tables to probe a
// resource owned by another customer.
const directoryDetailQuery = `WITH selected AS (
    SELECT ownership.server_no,ownership.action_reference,ownership.product_name_zh_cn,
           ownership.product_name_en_us,ownership.local_resource_no,
           ownership.bound_at,mapping.channel_no,mapping.pool_no,
           mapping.status_code AS mapping_status,
           GREATEST(ownership.updated_at,mapping.updated_at) AS updated_at,
           channel.display_name AS channel_name,pool.display_name AS pool_name,
           pool.resource_kind
    FROM plugin_idc_resource_server_entitlement_projection ownership
    JOIN plugin_idc_resource_mapping mapping
      ON mapping.tenant_id=ownership.tenant_id
     AND mapping.mapping_no=ownership.mapping_no
     AND mapping.local_resource_no=ownership.local_resource_no
    JOIN plugin_idc_resource_channel channel
      ON channel.tenant_id=mapping.tenant_id AND channel.channel_no=mapping.channel_no
    JOIN plugin_idc_resource_pool pool
      ON pool.tenant_id=mapping.tenant_id AND pool.channel_no=mapping.channel_no
     AND pool.pool_no=mapping.pool_no
    WHERE ownership.tenant_id=0 AND ownership.customer_no=?
      AND ownership.server_no=?
    LIMIT 2
)` + directoryEnrichmentCTE + `
SELECT server_no,action_reference,product_name_zh_cn,product_name_en_us,
       channel_name,pool_name,resource_kind,mapping_status,task_status,
       capabilities,bound_at,updated_at
FROM projected ORDER BY server_no`

const directoryEnrichmentCTE = `, latest_task AS (
    SELECT DISTINCT ON (task.local_resource_no)
           task.local_resource_no,task.status_code,task.updated_at
    FROM plugin_idc_resource_task task
    JOIN selected ON selected.local_resource_no=task.local_resource_no
    WHERE task.tenant_id=0
    ORDER BY task.local_resource_no,task.created_at DESC,task.id DESC
), latest_snapshot AS (
    SELECT DISTINCT ON (snapshot.channel_no)
           snapshot.channel_no,snapshot.snapshot_no
    FROM plugin_idc_resource_capability_snapshot snapshot
    JOIN (SELECT DISTINCT channel_no FROM selected) channel_set
      ON channel_set.channel_no=snapshot.channel_no
    WHERE snapshot.tenant_id=0 AND snapshot.status_code='complete'
    ORDER BY snapshot.channel_no,snapshot.observed_at DESC,snapshot.id DESC
), capabilities AS (
    SELECT snapshot.channel_no,
           COALESCE(jsonb_agg(item.capability_code ORDER BY item.capability_code)
             FILTER (WHERE item.support_code IN ('conditional','supported')),'[]'::jsonb)::text AS capabilities
    FROM latest_snapshot snapshot
    LEFT JOIN plugin_idc_resource_capability_snapshot_item item
      ON item.tenant_id=0 AND item.snapshot_no=snapshot.snapshot_no
    GROUP BY snapshot.channel_no
), projected AS (
    SELECT selected.server_no,selected.action_reference,selected.product_name_zh_cn,
           selected.product_name_en_us,selected.channel_name,selected.pool_name,
           selected.resource_kind,selected.mapping_status,
           COALESCE(latest_task.status_code,'') AS task_status,
           COALESCE(capabilities.capabilities,'[]') AS capabilities,
           selected.bound_at,
           GREATEST(selected.updated_at,COALESCE(latest_task.updated_at,selected.updated_at)) AS updated_at
    FROM selected
    LEFT JOIN latest_task ON latest_task.local_resource_no=selected.local_resource_no
    LEFT JOIN capabilities ON capabilities.channel_no=selected.channel_no
)`
