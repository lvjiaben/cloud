// Package customeroperations manages native entitlement-owned servers without
// manufacturing legacy service instances or reading core's private tables.
package customeroperations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"regexp"
	"slices"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/customerauthcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/serverfacts"
)

var (
	serverPattern           = regexp.MustCompile(`^SVR-[0-9A-F]{32}$`)
	keyPattern              = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)
	ErrConfigurationTimeout = gerror.New("native customer configuration read timed out")
	ErrUnavailable          = gerror.New("native customer server is unavailable")
	ErrConflict             = gerror.New("native customer server operation conflicts with its current state")
	ErrInvalid              = gerror.New("native customer server input is invalid")
)

const customerObservationBudget = 1500 * time.Millisecond
const customerCapabilityBudget = 500 * time.Millisecond

type providerRuntime interface {
	ConstructSession(context.Context, string) (resourcecap.Provider, func(), error)
}

type operationSource interface {
	serveroperationresourcecap.Source
	serveroperationresourcecap.ParameterValidator
	serveroperationresourcecap.CapabilityDirectory
	serveroperationresourcecap.ReinstallDirectory
	serveroperationresourcecap.AccessoryDirectory
	serveroperationresourcecap.InputSecretSource
}

type Service struct {
	database     gdb.DB
	customers    customerauthcap.Reader
	entitlements customerauthcap.EntitlementAccess
	providers    providerRuntime
	operations   operationSource
	audit        idccap.BusinessAuditWriter
	secrets      secretcap.Source
	facts        serverfacts.Service
	confirmation customerauthcap.SessionConfirmationAccess
	objects      servercommercialcap.Service
}

func New(database gdb.DB, customers customerauthcap.Reader, providers providerRuntime, operations operationSource, audit idccap.BusinessAuditWriter, secrets secretcap.Source, facts serverfacts.Service, objects servercommercialcap.Service) (*Service, error) {
	entitlements, ok := customers.(customerauthcap.EntitlementAccess)
	confirmation, confirmed := customers.(customerauthcap.SessionConfirmationAccess)
	if database == nil || customers == nil || !ok || !confirmed || providers == nil || operations == nil || audit == nil || secrets == nil || facts == nil || objects == nil {
		return nil, ErrUnavailable
	}
	return &Service{database: database, customers: customers, entitlements: entitlements, providers: providers, operations: operations, audit: audit, secrets: secrets, facts: facts, confirmation: confirmation, objects: objects}, nil
}

type Operation struct {
	FailureCode      string     `json:"failureCode"`
	AvailableOutputs []Output   `json:"availableOutputs"`
	WaitingUntil     *time.Time `json:"waitingUntil,omitempty"`
	OperationNo      string     `json:"operationNo"`
	Action           string     `json:"action"`
	Status           string     `json:"status"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

// Output advertises a consumable output without exposing its secret handle.
type Output struct {
	Name      string
	Policy    string
	ExpiresAt *time.Time
}

type Detail struct {
	Information            Information
	Capabilities           []Capability
	CommercialCapabilities []CommercialCapability
	ServerNo               string
	Entitlement            *customerauthcap.Entitlement
	RuntimeState           resourcecap.ResourceState
	ObservedAt             time.Time
	AvailableActions       []string
	Operations             []Operation
	Addresses              []string
	AddressesConfirmed     bool
	CredentialAvailable    bool
}

type route struct {
	ServerNo, CustomerNo, EntitlementNo, LocalResourceNo string
	ChannelNo, PoolNo, ExternalResourceRef               string
}

// Get performs a single current upstream read only after proving ownership.
// An unavailable observation produces unknown state and never enables writes.
func (s *Service) Get(ctx context.Context, serverNo string) (*Detail, error) {
	owned, err := s.ownedRoute(ctx, serverNo, false)
	if err != nil {
		return nil, err
	}
	entitlement, err := s.entitlements.ReadCurrentEntitlement(ctx, owned.EntitlementNo)
	if err != nil {
		return nil, ErrUnavailable
	}
	result := &Detail{ServerNo: serverNo, Entitlement: entitlement, RuntimeState: resourcecap.ResourceStateUnknown, AvailableActions: []string{}, Operations: []Operation{}, Addresses: []string{}}
	observationCtx, cancelObservation := context.WithTimeout(ctx, customerObservationBudget)
	observation, observationErr := s.observe(observationCtx, owned)
	cancelObservation()
	if observationErr == nil {
		result.Information = observationInformation(observation.Data)
		result.RuntimeState = observation.State
		result.ObservedAt = observation.ObservedAt
		result.Addresses, result.AddressesConfirmed = observationAddresses(observation.Data)
	}
	if err = s.database.GetScan(ctx, &result.Operations, operationHistoryQuery, owned.LocalResourceNo); err != nil {
		return nil, ErrUnavailable
	}
	busy, err := s.busy(ctx, owned.LocalResourceNo)
	if err != nil {
		return nil, ErrUnavailable
	}

	capabilityCtx, cancelCapabilities := context.WithTimeout(ctx, customerCapabilityBudget)
	capabilities, capabilityErr := s.operations.ListCapabilities(capabilityCtx, []serveroperationresourcecap.CapabilityRoute{{ServiceNo: owned.LocalResourceNo, ChannelNo: owned.ChannelNo, PoolNo: owned.PoolNo}})
	cancelCapabilities()
	choices := &Options{}
	if entitlement.CanOperate && !busy && observationErr == nil {
		choiceCtx, cancelChoices := context.WithTimeout(ctx, customerCapabilityBudget)
		choices = s.readOptions(choiceCtx, owned)
		cancelChoices()
	}
	result.Capabilities = make([]Capability, 0, len(nativeActions))
	for _, action := range nativeActions {
		capability := Capability{Action: string(action)}
		capability.Supported = capabilityErr == nil && len(capabilities) == 1 && !capabilities[0].ObservedAt.IsZero() && slices.Contains(capabilities[0].Actions, action)
		switch {
		case capabilityErr != nil || len(capabilities) != 1 || capabilities[0].ObservedAt.IsZero():
			capability.ReasonCode = "capability_unavailable"
		case !capability.Supported:
			capability.ReasonCode = "unsupported"
		case !entitlement.CanOperate:
			capability.ReasonCode = "entitlement_unavailable"
		case busy:
			capability.ReasonCode = "operation_in_progress"
		case observationErr != nil || result.RuntimeState == resourcecap.ResourceStateUnknown:
			capability.ReasonCode = "runtime_unconfirmed"
		case !allowedObservation(observation, string(action)):
			capability.ReasonCode = "runtime_state_conflict"
		case !choicesAvailable(choices, action):
			capability.ReasonCode = "choices_unavailable"
		default:
			capability.Available = true
			result.AvailableActions = append(result.AvailableActions, string(action))
		}
		result.Capabilities = append(result.Capabilities, capability)
	}
	result.CommercialCapabilities = commercialCapabilities(entitlement.CanOperate, busy, observation, observationErr, capabilities, capabilityErr, choices)
	if entitlement.CanOperate && !busy && observationErr == nil && result.RuntimeState != resourcecap.ResourceStateUnknown {
		available, queryErr := s.database.GetValue(ctx, credentialAvailableQuery, owned.LocalResourceNo, owned.CustomerNo)
		if queryErr != nil {
			return nil, ErrUnavailable
		}
		result.CredentialAvailable = available.Bool()
	}

	return result, nil
}

// Resolve accepts an entitlement already known to its owner from order history
// and returns only the public server number after checking both owners.
func (s *Service) Resolve(ctx context.Context, entitlementNo string) (string, error) {
	entitlement, err := s.entitlements.ReadCurrentEntitlement(ctx, entitlementNo)
	if err != nil {
		return "", ErrUnavailable
	}
	var found *struct{ ServerNo string }
	if err = s.database.GetScan(ctx, &found, `SELECT server_no FROM plugin_idc_resource_server_entitlement_projection WHERE tenant_id=0 AND entitlement_no=? AND customer_no=?`, entitlementNo, entitlement.CustomerNo); err != nil || found == nil || !serverPattern.MatchString(found.ServerNo) {
		return "", ErrUnavailable
	}
	return found.ServerNo, nil
}

func (s *Service) ownedRoute(ctx context.Context, serverNo string, lock bool) (*route, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || !serverPattern.MatchString(serverNo) {
		return nil, ErrUnavailable
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || subject.CustomerNo == "" {
		return nil, ErrUnavailable
	}
	query := ownedRouteQuery
	if lock {
		query += ` FOR UPDATE OF ownership,mapping`
	}
	var owned *route
	if err = s.database.GetScan(ctx, &owned, query, subject.CustomerNo, serverNo); err != nil || owned == nil {
		return nil, ErrUnavailable
	}
	return owned, nil
}

func (s *Service) observe(ctx context.Context, owned *route) (*resourcecap.ResourceSnapshot, error) {
	provider, closeProvider, err := s.providers.ConstructSession(ctx, owned.ChannelNo)
	if err != nil || provider == nil || closeProvider == nil {
		return nil, ErrUnavailable
	}
	defer closeProvider()
	result, err := provider.QueryResource(ctx, resourcecap.ResourceQuery{ExternalResourceRef: owned.ExternalResourceRef})
	if err != nil || result == nil || result.Validate() != nil || result.ExternalResourceRef != owned.ExternalResourceRef || result.ObservedAt.IsZero() || time.Since(result.ObservedAt) > time.Minute || result.ObservedAt.After(time.Now().Add(time.Second)) {
		return nil, ErrUnavailable
	}
	return result, nil
}

func (s *Service) busy(ctx context.Context, resourceNo string) (bool, error) {
	value, err := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_resource_task WHERE tenant_id=0 AND local_resource_no=? AND status_code NOT IN ('succeeded','failed','canceled'))`, resourceNo)
	return value.Bool(), err
}

func allowedState(state resourcecap.ResourceState, action string) bool {
	running := state == resourcecap.ResourceStateRunning || state == resourcecap.ResourceStateActive
	stopped := state == resourcecap.ResourceStateStopped
	switch serveroperationresourcecap.Action(action) {
	case serveroperationresourcecap.ActionStart:
		return stopped
	case serveroperationresourcecap.ActionSnapshotRestore, serveroperationresourcecap.ActionBackupRestore:
		return stopped
	case serveroperationresourcecap.ActionStop, serveroperationresourcecap.ActionReboot, serveroperationresourcecap.ActionConsole:
		return running
	case serveroperationresourcecap.ActionResetPassword, serveroperationresourcecap.ActionReinstall, serveroperationresourcecap.ActionEnterRescue, serveroperationresourcecap.ActionExitRescue,
		serveroperationresourcecap.ActionSecurityGroupAttach, serveroperationresourcecap.ActionSecurityGroupDetach,
		serveroperationresourcecap.ActionNATCreate, serveroperationresourcecap.ActionNATDelete,
		serveroperationresourcecap.ActionVirtualMediaMount, serveroperationresourcecap.ActionVirtualMediaUnmount,
		serveroperationresourcecap.ActionSnapshotDelete, serveroperationresourcecap.ActionBackupDelete:
		return running || stopped
	default:
		return false
	}
}

func operationNumber(customer, server, key string) string {
	digest := sha256.Sum256([]byte("native-customer-operation/v1\x00" + customer + "\x00" + server + "\x00" + key))
	return "nso_" + hex.EncodeToString(digest[:16])
}

func parametersDigest(parameters json.RawMessage) string {
	digest := sha256.Sum256(parameters)
	return hex.EncodeToString(digest[:])
}

const ownedRouteQuery = `SELECT ownership.server_no,ownership.customer_no,ownership.entitlement_no,
ownership.local_resource_no,mapping.channel_no,mapping.pool_no,mapping.external_resource_ref
FROM plugin_idc_resource_server_entitlement_projection ownership
JOIN plugin_idc_resource_mapping mapping ON mapping.tenant_id=ownership.tenant_id AND mapping.mapping_no=ownership.mapping_no AND mapping.local_resource_no=ownership.local_resource_no
JOIN plugin_idc_resource_channel channel ON channel.tenant_id=mapping.tenant_id AND channel.channel_no=mapping.channel_no AND channel.deleted_at IS NULL
JOIN plugin_idc_resource_pool pool ON pool.tenant_id=mapping.tenant_id AND pool.channel_no=mapping.channel_no AND pool.pool_no=mapping.pool_no AND pool.deleted_at IS NULL
WHERE ownership.tenant_id=0 AND ownership.customer_no=? AND ownership.server_no=? AND mapping.status_code='active' AND mapping.external_resource_ref<>''`

const operationHistoryQuery = `SELECT task.owner_metadata->>'operationNo' AS operation_no,task.owner_metadata->>'action' AS action,task.status_code AS status,task.failure_code,task.created_at,task.updated_at,
CASE WHEN task.status_code IN ('pending','retry_wait') AND health.circuit_open=TRUE AND health.write_protected_until>CURRENT_TIMESTAMP THEN health.write_protected_until END AS waiting_until
FROM plugin_idc_resource_task task
LEFT JOIN plugin_idc_resource_channel_health_state health ON health.tenant_id=task.tenant_id AND health.channel_no=task.channel_no
WHERE task.tenant_id=0 AND task.local_resource_no=? AND task.owner_metadata->>'operationNo' LIKE 'nso\_%' ESCAPE '\' ORDER BY task.id DESC LIMIT 10`

const credentialAvailableQuery = `SELECT EXISTS(SELECT 1 FROM plugin_idc_resource_provision_credential credential
JOIN plugin_idc_resource_provision_outcome_receipt receipt ON receipt.tenant_id=credential.tenant_id AND receipt.task_no=credential.task_no AND receipt.service_no=credential.service_no
JOIN plugin_idc_resource_task task ON task.tenant_id=credential.tenant_id AND task.task_no=credential.task_no
WHERE credential.tenant_id=0 AND credential.service_no=? AND receipt.customer_no=? AND receipt.status_code='succeeded' AND task.status_code='succeeded' AND credential.status_code='revealable')`

// observationAddresses validates the normalized owner projection again rather
// than allowing guest-controlled fields or labels into the customer response.
func observationAddresses(raw json.RawMessage) ([]string, bool) {
	var document map[string]json.RawMessage
	if json.Unmarshal(raw, &document) != nil {
		return []string{}, false
	}
	addresses, exists := document["addresses"]
	if !exists {
		return []string{}, false
	}
	var values []string
	if json.Unmarshal(addresses, &values) != nil || values == nil || len(values) > 1024 {
		return []string{}, false
	}
	seen := map[string]bool{}
	for _, value := range values {
		ip, err := netip.ParseAddr(value)
		if err != nil || ip.Zone() != "" || ip.Unmap().String() != value || seen[value] {
			return []string{}, false
		}
		seen[value] = true
	}
	return values, true
}

// Rescue transitions require an explicit mode observation; unknown never means off.
func allowedObservation(observation *resourcecap.ResourceSnapshot, action string) bool {
	if observation == nil || !allowedState(observation.State, action) {
		return false
	}
	switch serveroperationresourcecap.Action(action) {
	case serveroperationresourcecap.ActionEnterRescue, serveroperationresourcecap.ActionExitRescue:
		facts := observationInformation(observation.Data)
		return facts.Rescue != nil && *facts.Rescue == (action == string(serveroperationresourcecap.ActionExitRescue))
	default:
		return true
	}
}

// sameRoute detects a concurrent target or ownership change after observation.
func sameRoute(left, right *route) bool { return left != nil && right != nil && *left == *right }
