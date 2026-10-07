// This file defines the customer-safe server directory host-service contract.

package resourcecap

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"
)

const (
	// MaximumCurrentCustomerServerPageSize bounds one directory page.
	MaximumCurrentCustomerServerPageSize = 50
	// MaximumCurrentCustomerServerCursorLength bounds an opaque page cursor.
	MaximumCurrentCustomerServerCursorLength = 128
)

var (
	publicServerNoPattern = regexp.MustCompile(`^SVR-[0-9A-F]{32}$`)
	serverCursorPattern   = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
)

// ServerActionCode identifies one customer-facing action supported by the
// current resource capability snapshot.
type ServerActionCode string

// Customer-facing server action codes.
const (
	ServerActionStart          ServerActionCode = "start"
	ServerActionStop           ServerActionCode = "stop"
	ServerActionReboot         ServerActionCode = "reboot"
	ServerActionSuspend        ServerActionCode = "suspend"
	ServerActionResume         ServerActionCode = "resume"
	ServerActionRenew          ServerActionCode = "renew"
	ServerActionResetPassword  ServerActionCode = "reset_password"
	ServerActionConsole        ServerActionCode = "console"
	ServerActionReinstall      ServerActionCode = "reinstall"
	ServerActionEnterRescue    ServerActionCode = "enter_rescue"
	ServerActionExitRescue     ServerActionCode = "exit_rescue"
	ServerActionResize         ServerActionCode = "resize"
	ServerActionChangeIP       ServerActionCode = "change_ip"
	ServerActionSnapshot       ServerActionCode = "snapshot"
	ServerActionBackup         ServerActionCode = "backup"
	ServerActionSecurityGroup  ServerActionCode = "security_group"
	ServerActionNAT            ServerActionCode = "network_forwarding"
	ServerActionVirtualMedia   ServerActionCode = "virtual_media"
	ServerActionTrafficPackage ServerActionCode = "traffic_package"
	ServerActionTicket         ServerActionCode = "support_ticket"
	ServerActionTerminate      ServerActionCode = "release"
)

// CurrentCustomerServerListRequest requests one bounded page for the current
// authenticated customer. Customer identity is never accepted from payload.
type CurrentCustomerServerListRequest struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit"`
}

// CurrentCustomerServerGetRequest selects one current-customer-owned server
// by its public business number.
type CurrentCustomerServerGetRequest struct {
	ServerNo string `json:"serverNo"`
}

// CurrentCustomerServerItem is safe for customer applications. It deliberately
// omits action references, entitlement numbers, mappings, tasks and Provider IDs.
type CurrentCustomerServerItem struct {
	ServerNo         string             `json:"serverNo"`
	ProductNameZhCN  string             `json:"productNameZhCn"`
	ProductNameEnUS  string             `json:"productNameEnUs"`
	ChannelName      string             `json:"channelName"`
	PoolName         string             `json:"poolName"`
	Kind             string             `json:"kind"`
	State            string             `json:"state"`
	LatestOperation  string             `json:"latestOperation"`
	AvailableActions []ServerActionCode `json:"availableActions"`
	BoundAt          int64              `json:"boundAt"`
	UpdatedAt        int64              `json:"updatedAt"`
}

// CurrentCustomerServerPage is one bounded customer server page.
type CurrentCustomerServerPage struct {
	Items      []CurrentCustomerServerItem `json:"items"`
	NextCursor string                      `json:"nextCursor,omitempty"`
}

// CurrentCustomerServerResult contains one owned server projection.
type CurrentCustomerServerResult struct {
	Item CurrentCustomerServerItem `json:"item"`
}

// CurrentCustomerServerService reads only servers owned by the current
// authenticated customer derived from trusted host context.
type CurrentCustomerServerService interface {
	// ListCurrentCustomerServers returns at most request.Limit safe projections.
	ListCurrentCustomerServers(context.Context, CurrentCustomerServerListRequest) (*CurrentCustomerServerPage, error)
	// GetCurrentCustomerServer returns one safe projection or the same unavailable
	// error for a missing server and a server owned by another customer.
	GetCurrentCustomerServer(context.Context, CurrentCustomerServerGetRequest) (*CurrentCustomerServerResult, error)
}

// ValidateCurrentCustomerServerListRequest validates the bounded public request.
func ValidateCurrentCustomerServerListRequest(request CurrentCustomerServerListRequest) error {
	if request.Limit < 1 || request.Limit > MaximumCurrentCustomerServerPageSize ||
		len(request.Cursor) > MaximumCurrentCustomerServerCursorLength ||
		(len(request.Cursor) > 0 && !serverCursorPattern.MatchString(request.Cursor)) {
		return gerror.New("resourcecap: current customer server list request is invalid")
	}
	return nil
}

// ValidateCurrentCustomerServerGetRequest validates one public server number.
func ValidateCurrentCustomerServerGetRequest(request CurrentCustomerServerGetRequest) error {
	if !publicServerNoPattern.MatchString(request.ServerNo) {
		return gerror.New("resourcecap: current customer server request is invalid")
	}
	return nil
}

// ValidateCurrentCustomerServerPage validates one customer-safe bounded page.
func ValidateCurrentCustomerServerPage(page *CurrentCustomerServerPage, request CurrentCustomerServerListRequest) error {
	if ValidateCurrentCustomerServerListRequest(request) != nil || page == nil ||
		len(page.Items) > request.Limit || len(page.Items) > MaximumCurrentCustomerServerPageSize ||
		len(page.NextCursor) > MaximumCurrentCustomerServerCursorLength ||
		(len(page.NextCursor) > 0 && !serverCursorPattern.MatchString(page.NextCursor)) {
		return gerror.New("resourcecap: current customer server page is invalid")
	}
	seen := make(map[string]struct{}, len(page.Items))
	for _, item := range page.Items {
		if err := validateCurrentCustomerServerItem(item); err != nil {
			return err
		}
		if _, exists := seen[item.ServerNo]; exists {
			return gerror.New("resourcecap: current customer server page contains a duplicate")
		}
		seen[item.ServerNo] = struct{}{}
	}
	return nil
}

// ValidateCurrentCustomerServerResult validates one customer-safe detail result.
func ValidateCurrentCustomerServerResult(result *CurrentCustomerServerResult, request CurrentCustomerServerGetRequest) error {
	if ValidateCurrentCustomerServerGetRequest(request) != nil || result == nil || result.Item.ServerNo != request.ServerNo {
		return gerror.New("resourcecap: current customer server result is invalid")
	}
	return validateCurrentCustomerServerItem(result.Item)
}

func validateCurrentCustomerServerItem(item CurrentCustomerServerItem) error {
	if !publicServerNoPattern.MatchString(item.ServerNo) ||
		!validServerLabel(item.ProductNameZhCN) || !validServerLabel(item.ProductNameEnUS) ||
		!validServerLabel(item.ChannelName) || !validServerLabel(item.PoolName) ||
		!validServerKind(item.Kind) || !validServerState(item.State) ||
		!validServerOperationState(item.LatestOperation) || item.BoundAt < 1 || item.UpdatedAt < 1 ||
		len(item.AvailableActions) > len(serverActionSet) {
		return gerror.New("resourcecap: current customer server projection is invalid")
	}
	seen := make(map[ServerActionCode]struct{}, len(item.AvailableActions))
	for _, action := range item.AvailableActions {
		if _, valid := serverActionSet[action]; !valid {
			return gerror.New("resourcecap: current customer server action is invalid")
		}
		if _, exists := seen[action]; exists {
			return gerror.New("resourcecap: current customer server action is duplicated")
		}
		seen[action] = struct{}{}
	}
	return nil
}

func validServerLabel(value string) bool {
	count := utf8.RuneCountInString(value)
	return utf8.ValidString(value) && value == strings.TrimSpace(value) && count >= 1 && count <= 256
}

func validServerKind(value string) bool {
	return value == "cloud_server" || value == "bare_metal_server" || value == "other_server"
}

func validServerState(value string) bool {
	switch value {
	case "ready", "preparing", "attention_required", "releasing", "released", "unknown":
		return true
	default:
		return false
	}
}

func validServerOperationState(value string) bool {
	switch value {
	case "idle", "working", "attention_required", "completed", "unsuccessful", "unknown":
		return true
	default:
		return false
	}
}

var serverActionSet = map[ServerActionCode]struct{}{
	ServerActionStart: {}, ServerActionStop: {}, ServerActionReboot: {},
	ServerActionSuspend: {}, ServerActionResume: {}, ServerActionRenew: {},
	ServerActionResetPassword: {}, ServerActionConsole: {}, ServerActionReinstall: {},
	ServerActionEnterRescue: {}, ServerActionExitRescue: {}, ServerActionResize: {},
	ServerActionChangeIP: {}, ServerActionSnapshot: {}, ServerActionBackup: {},
	ServerActionSecurityGroup: {}, ServerActionNAT: {}, ServerActionVirtualMedia: {},
	ServerActionTrafficPackage: {}, ServerActionTicket: {}, ServerActionTerminate: {},
}
