// Package provisionsource implements the resource-owner side of paid server provisioning.
package provisionsource

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/logger"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	providerspi "lina-plugin-idc-resource/backend/cap/resourcecap/spi"
	"lina-plugin-idc-resource/backend/internal/composition"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
	"lina-plugin-idc-resource/backend/internal/service/resourcetask"
)

const capabilityEvidenceLifetime = 24 * time.Hour

type Service struct {
	candidates         resourceinterfaceprojectioncap.CandidateSource
	database           gdb.DB
	tasks              *resourcetask.Store
	secrets            secretcap.Source
	random             io.Reader
	principal          PrincipalEnsurer
	managedParameters  ManagedProvisionParameterBuilder
	publicOriginSource func() string
}

func (s *Service) SetPublicOriginSource(source func() string) { s.publicOriginSource = source }

// PrincipalEnsurer creates the resource-owned upstream-account task when a
// confirmed mapping does not yet exist.
type PrincipalEnsurer func(context.Context, string, string) (bool, error)

var _ provisionresourcecap.Source = (*Service)(nil)

// New constructs the resource-owned provisioning source from startup-shared dependencies.
func New(
	database gdb.DB,
	secrets secretcap.Source,
	principal PrincipalEnsurer,
	candidates resourceinterfaceprojectioncap.CandidateSource,
	_ []providerspi.Declaration,
	managed ...ManagedProvisionParameterBuilder,
) (*Service, error) {
	service, err := newWithRandomAndPrincipal(database, secrets, rand.Reader, principal)
	if err == nil {
		service.candidates = candidates
	}
	if err == nil && len(managed) > 0 {
		service.managedParameters = managed[0]
	}
	return service, err
}

func newWithRandomAndPrincipal(
	database gdb.DB,
	secrets secretcap.Source,
	random io.Reader,
	principal PrincipalEnsurer,
) (*Service, error) {
	if database == nil || secrets == nil || random == nil || principal == nil {
		return nil, gerror.New("resource provisioning dependencies are required")
	}
	tasks, err := resourcetask.NewStore(database)
	if err != nil {
		return nil, err
	}
	return &Service{
		database: database, tasks: tasks, secrets: secrets, random: random,
		principal: principal,
	}, nil
}

type routeEvidence struct {
	PoolNo                  string    `orm:"pool_no"`
	ChannelNo               string    `orm:"channel_no"`
	ProviderID              string    `orm:"provider_id"`
	ProviderPluginID        string    `orm:"provider_plugin_id"`
	ProviderRuntimeType     string    `orm:"provider_runtime_type"`
	Selector                string    `orm:"selector"`
	PoolVersion             int64     `orm:"pool_version"`
	ChannelVersion          int64     `orm:"channel_version"`
	PrincipalMappingNo      string    `orm:"principal_mapping_no"`
	ExternalPrincipalRef    string    `orm:"external_principal_ref"`
	PrincipalEvidenceDigest string    `orm:"principal_evidence_digest"`
	PrincipalVersion        int64     `orm:"principal_version"`
	SnapshotNo              string    `orm:"snapshot_no"`
	AdapterVersion          string    `orm:"adapter_version"`
	ItemCount               int       `orm:"item_count"`
	ActualItemCount         int       `orm:"actual_item_count"`
	ObservedAt              time.Time `orm:"observed_at"`
	SupportCode             string    `orm:"support_code"`
	Constraints             string    `orm:"constraints"`
}

func (s *Service) resolveRoute(
	ctx context.Context,
	poolNo string,
	customerNo string,
) (*routeEvidence, error) {
	var row *routeEvidence
	err := s.database.GetScan(ctx, &row, `
WITH route AS (
    SELECT pool.pool_no, pool.channel_no, pool.selector, pool.version AS pool_version,
           channel.provider_id, channel.provider_plugin_id, channel.provider_runtime_type,
           channel.version AS channel_version
    FROM plugin_idc_resource_pool pool
    INNER JOIN plugin_idc_resource_channel channel
      ON channel.tenant_id = pool.tenant_id AND channel.channel_no = pool.channel_no
     AND channel.status_code = 'enabled' AND channel.deleted_at IS NULL
    WHERE pool.tenant_id = ? AND pool.pool_no = ?
      AND pool.status_code = 'enabled' AND pool.deleted_at IS NULL
), latest AS (
    SELECT snapshot.snapshot_no, snapshot.channel_no, snapshot.adapter_version,
           snapshot.item_count, snapshot.observed_at
    FROM plugin_idc_resource_capability_snapshot snapshot
    INNER JOIN route ON route.channel_no = snapshot.channel_no
    WHERE snapshot.tenant_id = ? AND snapshot.status_code = 'complete'
    ORDER BY snapshot.observed_at DESC, snapshot.id DESC
    LIMIT 1
)
SELECT route.pool_no, route.channel_no, route.provider_id, route.provider_plugin_id,
       route.provider_runtime_type,
       route.selector::TEXT AS selector,
       route.pool_version, route.channel_version,
       principal.mapping_no AS principal_mapping_no,
       principal.external_principal_ref,
       principal.evidence_digest AS principal_evidence_digest,
       principal.version AS principal_version,
       latest.snapshot_no, latest.adapter_version, latest.item_count, latest.observed_at,
       counts.actual_item_count, capability.support_code,
       capability.constraints::TEXT AS constraints
FROM route
INNER JOIN latest ON latest.channel_no = route.channel_no
INNER JOIN plugin_idc_resource_principal_mapping principal
  ON principal.tenant_id = ? AND principal.channel_no = route.channel_no
 AND principal.customer_no = ? AND principal.status_code = 'active'
INNER JOIN LATERAL (
    SELECT COUNT(*)::INT AS actual_item_count
    FROM plugin_idc_resource_capability_snapshot_item item
    WHERE item.tenant_id = ? AND item.snapshot_no = latest.snapshot_no
) counts ON TRUE
INNER JOIN plugin_idc_resource_capability_snapshot_item capability
  ON capability.tenant_id = ? AND capability.snapshot_no = latest.snapshot_no
 AND capability.capability_code = ?`,
		idccap.DefaultTenantID, poolNo, idccap.DefaultTenantID,
		idccap.DefaultTenantID, customerNo, idccap.DefaultTenantID,
		idccap.DefaultTenantID, resourcecap.CapabilityProvision,
	)
	if err != nil {
		return nil, gerror.Wrap(err, "resolve resource provisioning route")
	}
	if row == nil || row.PoolNo != poolNo || row.ChannelNo == "" || row.ProviderRuntimeType != "dynamic" {
		return nil, provisionresourcecap.ErrPrincipalUnavailable
	}
	if row.ItemCount != resourcecap.MaximumCapabilityCount ||
		row.ActualItemCount != resourcecap.MaximumCapabilityCount ||
		(row.SupportCode != string(resourcecap.SupportLevelSupported) &&
			row.SupportCode != string(resourcecap.SupportLevelConditional)) {
		return nil, provisionresourcecap.ErrCapabilityUnavailable
	}
	now, err := databaseNow(ctx, s.database)
	if err != nil {
		return nil, err
	}
	if row.ObservedAt.IsZero() || row.ObservedAt.After(now) ||
		!row.ObservedAt.UTC().Add(capabilityEvidenceLifetime).After(now) {
		return nil, provisionresourcecap.ErrCapabilityUnavailable
	}
	return row, nil
}

// CreateTask creates or reuses one exact resource-owned Provider task.
func (s *Service) CreateTask(
	ctx context.Context,
	request provisionresourcecap.TaskRequest,
	corrections ...*provisionresourcecap.RecoveryConfiguration,
) (*provisionresourcecap.Task, error) {
	return s.createTask(ctx, request, taskRequestID(request.OperationNo), "", corrections...)
}

// createTask retains the frozen paid identity while allowing a separately
// authorized rejected attempt to have its own task and credential identity.
func (s *Service) createServerTask(ctx context.Context, request provisionresourcecap.TaskRequest, requestID, credentialKey string, corrections ...*provisionresourcecap.RecoveryConfiguration) (*provisionresourcecap.Task, error) {
	var correction *provisionresourcecap.RecoveryConfiguration
	if len(corrections) > 1 {
		return nil, provisionresourcecap.ErrTaskConflict
	}
	if len(corrections) == 1 {
		correction = corrections[0]
	}
	if !s.canCreateTask(ctx, request) {
		return nil, provisionresourcecap.ErrSourceUnavailable
	}
	existing, err := s.loadTaskByRequest(ctx, requestID)
	if err != nil {
		return nil, provisionresourcecap.WrapPreparationFailure(provisionresourcecap.PreparationFailureLookup, err)
	}
	if existing != nil {
		if !sameTaskRequest(existing, request) || !sameTaskCorrection(existing, correction) {
			return nil, provisionresourcecap.ErrTaskConflict
		}
		if err = s.healProvisionCredential(ctx, existing); err != nil {
			return nil, err
		}
		result, readErr := s.ReadTask(ctx, provisionresourcecap.TaskReadRequest{
			ProviderTaskNo: existing.TaskNo, Request: request,
		})
		if result != nil {
			result.Reused = true
		}
		return result, readErr
	}
	effective := request
	if len(corrections) == 1 && corrections[0] != nil {
		effective, err = s.correctedRequest(ctx, request, *corrections[0])
		if err != nil {
			return nil, provisionresourcecap.WrapPreparationFailure(provisionresourcecap.PreparationFailureParameters, err)
		}
	}
	route, parameters, localResourceNo, err := s.prepareProvisionParameters(ctx, effective, correction != nil)
	if err != nil {
		return nil, provisionresourcecap.WrapPreparationFailure(provisionresourcecap.PreparationFailureParameters, err)
	}
	if validationErr := validateCapabilityParameters(route.Constraints, parameters); validationErr != nil {
		return nil, provisionresourcecap.WrapPreparationFailure(provisionresourcecap.PreparationFailureParameters, validationErr)
	}
	capabilityDigest, err := capabilityEvidenceDigest(route, parameters)
	if err != nil {
		return nil, provisionresourcecap.WrapPreparationFailure(provisionresourcecap.PreparationFailureParameters, err)
	}
	metadata, err := taskOwnerMetadataWithRevision(request, effective.InterfaceConfigurationRevision, route.SnapshotNo, capabilityDigest, correction)
	if err != nil {
		return nil, provisionresourcecap.WrapPreparationFailure(provisionresourcecap.PreparationFailureParameters, err)
	}
	manager, err := composition.BindProviderSecretManager(s.secrets, route.ChannelNo)
	if err != nil {
		return nil, provisionresourcecap.WrapPreparationFailure(provisionresourcecap.PreparationFailureCredential, errors.Join(provisionresourcecap.ErrSourceUnavailable, err))
	}
	generationPolicy, err := parseServerGenerationPolicy(effective.ProductConfiguration)
	if err != nil {
		return nil, provisionresourcecap.WrapPreparationFailure(provisionresourcecap.PreparationFailureParameters, err)
	}
	secretResult, err := s.createProvisionCredential(ctx, manager, request, generationPolicy, credentialKey)
	if err != nil || secretResult.Handle.IsZero() {
		logger.Warningf(ctx, "server provisioning preparation blocked stage=credential_storage")
		return nil, provisionresourcecap.WrapPreparationFailure(provisionresourcecap.PreparationFailureCredential, provisionresourcecap.ErrSourceUnavailable)
	}
	correlation, err := s.providerCorrelationInput(ctx, manager, route, effective, requestID)
	if err != nil {
		logger.Warningf(ctx, "server provisioning preparation blocked stage=callback_configuration")
		return nil, provisionresourcecap.WrapPreparationFailure(provisionresourcecap.PreparationFailureCallback, err)
	}
	created, err := s.tasks.Create(ctx, resourcetask.CreateInput{
		RequestID: requestID, Capability: resourcecap.CapabilityProvision,
		LocalResourceNo: localResourceNo, ChannelNo: route.ChannelNo, PoolNo: route.PoolNo,
		Parameters: parameters, OwnerMetadata: metadata, Priority: 100, MaxAttempts: 8,
		SecretBindings:      []resourcecap.SecretBinding{{Name: "password", Handle: secretResult.Handle}},
		ProviderCorrelation: correlation,
	})
	if err != nil {
		logger.Warningf(ctx, "server provisioning preparation blocked stage=task_storage validation_failed=%t", strings.Contains(gerror.Current(err).Error(), "invalid"))
		recovered, recoveryErr := s.loadTaskByRequest(ctx, requestID)
		if recoveryErr == nil && recovered != nil && sameTaskRequest(recovered, request) && sameTaskCorrection(recovered, correction) {
			recovered.SecretRef = secretResult.Handle.OpaqueRef()
			if recoveryErr = s.healProvisionCredential(ctx, recovered); recoveryErr == nil {
				result, readErr := s.ReadTask(ctx, provisionresourcecap.TaskReadRequest{
					ProviderTaskNo: recovered.TaskNo, Request: request,
				})
				if result != nil {
					result.Reused = true
				}
				return result, readErr
			}
		}
		// No durable task means the supplier write executor cannot consume the
		// credential. Keep this encrypted staging value for the exact retry key.

		failure := provisionresourcecap.PreparationFailureTaskStorage
		if errors.Is(err, resourcetask.ErrInvalidInput) {
			failure = provisionresourcecap.PreparationFailureTaskInput
		}
		return nil, provisionresourcecap.WrapPreparationFailure(failure, errors.Join(
			provisionresourcecap.ErrSourceUnavailable,
			gerror.Wrap(errors.Join(err, recoveryErr), "create Provider provisioning task"),
		))
	}
	row, err := s.loadTaskByNumber(ctx, created.TaskNo)
	if err != nil || row == nil || !sameTaskRequest(row, request) {
		return nil, provisionresourcecap.ErrTaskConflict
	}
	row.SecretRef = secretResult.Handle.OpaqueRef()
	if err = s.healProvisionCredential(ctx, row); err != nil {
		return nil, err
	}
	result, err := s.ReadTask(ctx, provisionresourcecap.TaskReadRequest{
		ProviderTaskNo: created.TaskNo, Request: request,
	})
	if result != nil {
		result.Reused = !created.Created
	}
	return result, err
}

func (s *Service) canCreateTask(ctx context.Context, request provisionresourcecap.TaskRequest) bool {
	return s != nil && s.database != nil && s.tasks != nil && s.secrets != nil &&
		ctx != nil && ctx.Err() == nil && request.InterfaceInstanceNo != "" &&
		provisionresourcecap.ValidateTaskRequest(request) == nil
}

func (s *Service) providerCorrelationInput(ctx context.Context, manager secretcap.Manager, route *routeEvidence, request provisionresourcecap.TaskRequest, requestID string) (*resourcetask.ProviderCorrelationInput, error) {
	if route == nil || route.ProviderRuntimeType != "dynamic" {
		return nil, nil
	}
	reader, err := channelconfig.NewReader(s.database)
	if err != nil {
		return nil, provisionresourcecap.ErrSourceUnavailable
	}
	configuration, err := reader.ReadRevision(ctx, route.ChannelNo, request.InterfaceConfigurationRevision)
	if err != nil {
		return nil, provisionresourcecap.ErrSourceUnavailable
	}
	profile, err := resourcecap.DecodeHostAuthenticationSettings(configuration.Config.Settings)
	if err != nil || !profile.CallbackRequired {
		return nil, nil
	}
	publicOrigin := ""
	if s.publicOriginSource != nil {
		publicOrigin = s.publicOriginSource()
	}
	downstreamURL, err := governedCallbackURL(publicOrigin)
	if err != nil {
		return nil, provisionresourcecap.ErrSourceUnavailable
	}
	return &resourcetask.ProviderCorrelationInput{
		PluginID: configuration.PluginID, ConfigurationRevision: request.InterfaceConfigurationRevision,
		DownstreamURL: downstreamURL,
		CreateToken: func(tokenCtx context.Context, _ int64, _ string) (secretcap.CreateResult, error) {
			return manager.Create(tokenCtx, "provider-correlation:"+requestID, providerCorrelationTokenProducer(s.random))
		},
	}, nil
}

func providerCorrelationTokenProducer(random io.Reader) secretcap.PlaintextProducer {
	return func(consume secretcap.Consumer) error {
		buffer := make([]byte, 24)
		if _, err := io.ReadFull(random, buffer); err != nil {
			return err
		}
		token := []byte(base64.RawURLEncoding.EncodeToString(buffer))
		clear(buffer)
		defer clear(token)
		return consume(token)
	}
}

func governedCallbackURL(origin string) (string, error) {
	target, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || target.Hostname() == "" || target.User != nil || target.RawQuery != "" || target.Fragment != "" || target.Path != "" {
		return "", gerror.New("resource public origin is invalid")
	}
	callback := strings.TrimRight(origin, "/") + "/api/plugins/idc-resource/provider-callback"
	if resourcecap.ValidateProviderCorrelation(&resourcecap.ProviderCorrelation{DownstreamID: 1, DownstreamURL: callback}) != nil || target.Hostname() == "localhost.invalid" {
		return "", gerror.New("resource public origin is unavailable")
	}
	return callback, nil
}

type taskOwnerFacts struct {
	SaleImportProtocol productfulfillmentcap.SaleImportProtocol `json:"saleImportProtocol,omitempty"`
	// PurchasedInterfaceInstanceNo retains the original paid identity when an
	// independently authorized correction selects another execution channel.
	PurchasedInterfaceInstanceNo   string                                      `json:"purchasedInterfaceInstanceNo,omitempty"`
	RecoveryConfiguration          *provisionresourcecap.RecoveryConfiguration `json:"recoveryConfiguration,omitempty"`
	ContractVersion                string                                      `json:"contractVersion"`
	InterfaceConfigurationRevision string                                      `json:"interfaceConfigurationRevision"`
	OperationNo                    string                                      `json:"operationNo"`
	CustomerNo                     string                                      `json:"customerNo"`
	ConfigurationSnapshotNo        string                                      `json:"configurationSnapshotNo"`
	RequestDigest                  [sha256.Size]byte                           `json:"requestDigest"`
	CapabilityEvidenceNo           string                                      `json:"capabilityEvidenceNo"`
	CapabilityEvidenceDigest       [sha256.Size]byte                           `json:"capabilityEvidenceDigest"`
}

type taskRow struct {
	TaskNo              string     `orm:"task_no"`
	RequestDigest       string     `orm:"request_id_digest"`
	CapabilityCode      string     `orm:"capability_code"`
	LocalResourceNo     string     `orm:"local_resource_no"`
	ChannelNo           string     `orm:"channel_no"`
	PoolNo              string     `orm:"pool_no"`
	StatusCode          string     `orm:"status_code"`
	SanitizedParameters string     `orm:"sanitized_parameters"`
	OwnerMetadata       string     `orm:"owner_metadata"`
	FailureCode         string     `orm:"failure_code"`
	ResultUnknown       bool       `orm:"result_unknown"`
	Version             int64      `orm:"version"`
	FinishedAt          *time.Time `orm:"finished_at"`
	UpdatedAt           time.Time  `orm:"updated_at"`
	SecretRef           string     `orm:"secret_ref"`
}

// ReadTask returns a secret-free projection bound to the original request.
func (s *Service) ReadTask(
	ctx context.Context,
	request provisionresourcecap.TaskReadRequest,
) (*provisionresourcecap.Task, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil ||
		provisionresourcecap.ValidateTaskReadRequest(request) != nil {
		return nil, provisionresourcecap.ErrSourceUnavailable
	}
	row, err := s.loadTaskByNumber(ctx, request.ProviderTaskNo)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, provisionresourcecap.ErrTaskUnavailable
	}
	if !sameTaskRequest(row, request.Request) || (request.Request.NativePurchase == nil && row.SecretRef == "") {
		return nil, provisionresourcecap.ErrTaskConflict
	}
	facts, err := decodeTaskOwnerFacts(row.OwnerMetadata)
	if err != nil {
		return nil, provisionresourcecap.ErrTaskConflict
	}
	status, failureCode := projectTaskStatus(row)
	if status == provisionresourcecap.TaskStatusFailed && failureCode == provisionresourcecap.FailureRequestNotSent {
		if err = s.validateNonSubmissionProof(ctx, row); err != nil {
			return nil, err
		}
	}
	evidenceDigest, err := taskEvidenceDigest(row)
	if err != nil {
		return nil, err
	}
	result := &provisionresourcecap.Task{
		ProviderTaskNo: row.TaskNo, ChannelNo: row.ChannelNo, Status: status,
		FailureCode: failureCode, CapabilityEvidenceNo: facts.CapabilityEvidenceNo,
		CapabilityEvidenceDigest: hex.EncodeToString(facts.CapabilityEvidenceDigest[:]),
		EvidenceDigest:           evidenceDigest, ObservedAt: row.UpdatedAt.UTC(),
	}
	response, err := s.database.GetValue(ctx, `SELECT summary FROM plugin_idc_resource_task_evidence WHERE tenant_id=0 AND task_no=? AND evidence_kind IN (?,?) ORDER BY CASE WHEN evidence_kind=? THEN 0 ELSE 1 END,observed_at DESC,id DESC LIMIT 1`, row.TaskNo, resourcecap.EvidenceKindHTTPResponse, resourcecap.EvidenceKindCallDiagnostic, resourcecap.EvidenceKindHTTPResponse)
	if err != nil {
		return nil, provisionresourcecap.ErrSourceUnavailable
	}
	if excerpt := response.String(); excerpt != "" {
		if (resourcecap.Evidence{Kind: resourcecap.EvidenceKindHTTPResponse, Summary: excerpt, ObservedAt: row.UpdatedAt}).Validate() != nil {
			return nil, provisionresourcecap.ErrSourceUnavailable
		}
		result.ResponseExcerpt = excerpt
	}
	if row.FinishedAt != nil {
		result.CompletedAt = row.FinishedAt.UTC()
		// A terminal task is observed at the exact persisted completion fact.
		// Database-maintained updated_at may be a few microseconds later, which
		// must not make a valid completed projection contradict itself.
		result.ObservedAt = result.CompletedAt
	}
	if err = s.applyNonAcceptanceVerification(ctx, row, result); err != nil {
		return nil, err
	}
	if provisionresourcecap.ValidateTask(*result) != nil {
		return nil, provisionresourcecap.ErrTaskConflict
	}
	return result, nil
}

func (s *Service) loadTaskByRequest(ctx context.Context, requestID string) (*taskRow, error) {
	digest := sha256.Sum256([]byte(requestID))
	var rows []taskRow
	if err := s.database.GetScan(ctx, &rows, taskProjectionSQL+`
WHERE task.tenant_id = ? AND task.request_id_digest = ?
  AND task.capability_code = ?
ORDER BY task.id
LIMIT 2`, idccap.DefaultTenantID, hex.EncodeToString(digest[:]), resourcecap.CapabilityProvision); err != nil {
		return nil, gerror.Wrap(err, "load idempotent Provider provisioning task")
	}
	if len(rows) > 1 {
		return nil, provisionresourcecap.ErrTaskConflict
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

func (s *Service) loadTaskByNumber(ctx context.Context, taskNo string) (*taskRow, error) {
	var row *taskRow
	if err := s.database.GetScan(ctx, &row, taskProjectionSQL+`
WHERE task.tenant_id = ? AND task.task_no = ?`, idccap.DefaultTenantID, taskNo); err != nil {
		return nil, gerror.Wrap(err, "load Provider provisioning task")
	}
	return row, nil
}

const taskProjectionSQL = `
SELECT task.task_no, task.request_id_digest, task.capability_code,
       task.local_resource_no, task.channel_no, COALESCE(task.pool_no, '') AS pool_no,
       task.status_code, task.sanitized_parameters::TEXT AS sanitized_parameters,
       task.owner_metadata::TEXT AS owner_metadata, task.failure_code,
       task.result_unknown, task.version, task.finished_at, task.updated_at,
       COALESCE(credential.secret_ref, binding.secret_ref, '') AS secret_ref
FROM plugin_idc_resource_task task
LEFT JOIN plugin_idc_resource_provision_credential credential
  ON credential.tenant_id = task.tenant_id AND credential.task_no = task.task_no
LEFT JOIN LATERAL (
    SELECT candidate.secret_ref
    FROM plugin_idc_resource_task_secret_binding candidate
    WHERE candidate.tenant_id = task.tenant_id AND candidate.task_no = task.task_no
      AND candidate.binding_direction = 'input' AND candidate.binding_name = 'password'
    ORDER BY candidate.id DESC
    LIMIT 1
) binding ON TRUE
`

func sameTaskRequest(row *taskRow, request provisionresourcecap.TaskRequest) bool {
	localResourceNo, _ := frozenServerIdentity(request)
	poolMatches := row != nil && row.PoolNo != ""
	if request.NativePurchase != nil {
		localResourceNo = frozenNativeIdentity(request)
		poolMatches = row != nil && row.PoolNo == ""
	}
	if row == nil || row.CapabilityCode != string(resourcecap.CapabilityProvision) ||
		row.LocalResourceNo != localResourceNo || !poolMatches {
		return false
	}
	facts, err := decodeTaskOwnerFacts(row.OwnerMetadata)
	return err == nil && facts.SaleImportProtocol == request.NativePurchaseProtocol && facts.ContractVersion == provisionresourcecap.ContractVersion &&
		facts.OperationNo == request.OperationNo && facts.CustomerNo == request.CustomerNo &&
		facts.ConfigurationSnapshotNo == request.ConfigurationSnapshotNo &&
		(facts.PurchasedInterfaceInstanceNo == "" || facts.PurchasedInterfaceInstanceNo == request.InterfaceInstanceNo) &&
		subtle.ConstantTimeCompare(
			[]byte(hex.EncodeToString(facts.RequestDigest[:])), []byte(request.RequestDigest),
		) == 1
}

func taskOwnerMetadata(
	request provisionresourcecap.TaskRequest,
	capabilityEvidenceNo string,
	capabilityEvidenceDigest string,
) (json.RawMessage, error) {
	requestDigest, err := decodeDigest(request.RequestDigest)
	if err != nil {
		return nil, provisionresourcecap.ErrTaskConflict
	}
	capabilityDigest, err := decodeDigest(capabilityEvidenceDigest)
	if err != nil {
		return nil, provisionresourcecap.ErrCapabilityUnavailable
	}
	protocol := productfulfillmentcap.SaleImportProtocol("")
	if request.NativePurchase != nil {
		protocol = request.NativePurchaseProtocol
	}
	return json.Marshal(taskOwnerFacts{
		SaleImportProtocol: protocol,
		ContractVersion:    provisionresourcecap.ContractVersion, OperationNo: request.OperationNo,
		InterfaceConfigurationRevision: request.InterfaceConfigurationRevision,
		PurchasedInterfaceInstanceNo:   request.InterfaceInstanceNo,
		CustomerNo:                     request.CustomerNo, ConfigurationSnapshotNo: request.ConfigurationSnapshotNo,
		RequestDigest: requestDigest, CapabilityEvidenceNo: capabilityEvidenceNo,
		CapabilityEvidenceDigest: capabilityDigest,
	})
}

func decodeTaskOwnerFacts(payload string) (taskOwnerFacts, error) {
	decoder := json.NewDecoder(bytes.NewBufferString(payload))
	decoder.DisallowUnknownFields()
	var facts taskOwnerFacts
	if err := decoder.Decode(&facts); err != nil {
		return taskOwnerFacts{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF ||
		facts.ContractVersion != provisionresourcecap.ContractVersion ||
		(facts.SaleImportProtocol != "" && !facts.SaleImportProtocol.NativePurchase()) ||
		(facts.PurchasedInterfaceInstanceNo != "" && !recoveryInterfacePattern.MatchString(facts.PurchasedInterfaceInstanceNo)) ||
		!configurationRevisionPattern.MatchString(facts.InterfaceConfigurationRevision) ||
		facts.CapabilityEvidenceNo == "" || allZeroDigest(facts.RequestDigest) ||
		allZeroDigest(facts.CapabilityEvidenceDigest) {
		return taskOwnerFacts{}, provisionresourcecap.ErrTaskConflict
	}
	return facts, nil
}

func decodeDigest(value string) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != sha256.Size {
		return digest, provisionresourcecap.ErrTaskConflict
	}
	copy(digest[:], decoded)
	return digest, nil
}

func allZeroDigest(value [sha256.Size]byte) bool {
	var zero [sha256.Size]byte
	return subtle.ConstantTimeCompare(value[:], zero[:]) == 1
}

func capabilityEvidenceDigest(row *routeEvidence, parameters json.RawMessage) (string, error) {
	var canonicalParameters any
	if json.Unmarshal(parameters, &canonicalParameters) != nil {
		return "", provisionresourcecap.ErrParametersUnavailable
	}
	payload, err := json.Marshal(struct {
		Contract, PoolNo, ChannelNo, ProviderID, Selector string
		PoolVersion, ChannelVersion                       int64
		PrincipalMappingNo, PrincipalEvidenceDigest       string
		PrincipalVersion                                  int64
		SnapshotNo, AdapterVersion, Support, Constraints  string
		ObservedUnixMicro                                 int64
		Parameters                                        any
	}{
		"idc-resource/provision-capability/v1", row.PoolNo, row.ChannelNo,
		row.ProviderID, row.Selector, row.PoolVersion, row.ChannelVersion,
		row.PrincipalMappingNo, row.PrincipalEvidenceDigest, row.PrincipalVersion,
		row.SnapshotNo, row.AdapterVersion, row.SupportCode, row.Constraints,
		row.ObservedAt.UTC().UnixMicro(), canonicalParameters,
	})
	if err != nil {
		return "", gerror.Wrap(err, "encode resource provisioning capability evidence")
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func taskEvidenceDigest(row *taskRow) (string, error) {
	payload, err := json.Marshal(struct {
		Contract, TaskNo, RequestDigest, Status, FailureCode, OwnerMetadata string
		ResultUnknown                                                       bool
		Version                                                             int64
		FinishedUnixMicro, UpdatedUnixMicro                                 int64
	}{
		"idc-resource/provision-task/v1", row.TaskNo, row.RequestDigest,
		row.StatusCode, row.FailureCode, row.OwnerMetadata, row.ResultUnknown,
		row.Version, timePointerUnixMicro(row.FinishedAt), row.UpdatedAt.UTC().UnixMicro(),
	})
	if err != nil {
		return "", gerror.Wrap(err, "encode Provider provisioning task evidence")
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func projectTaskStatus(row *taskRow) (provisionresourcecap.TaskStatus, string) {
	switch resourcetask.Status(row.StatusCode) {
	case resourcetask.StatusPending, resourcetask.StatusRetryWait:
		return provisionresourcecap.TaskStatusPending, ""
	case resourcetask.StatusSubmitting, resourcetask.StatusAccepted, resourcetask.StatusRunning:
		return provisionresourcecap.TaskStatusRunning, ""
	case resourcetask.StatusSucceeded:
		return provisionresourcecap.TaskStatusSucceeded, ""
	case resourcetask.StatusFailed, resourcetask.StatusCanceled:
		code := strings.ToLower(row.FailureCode)
		if code == "" {
			code = "resource.task_failed"
		}
		return provisionresourcecap.TaskStatusFailed, code
	case resourcetask.StatusResultUnknown:
		code := strings.ToLower(row.FailureCode)
		if code == "" {
			code = "resource.result_unknown"
		}
		return provisionresourcecap.TaskStatusResultUnknown, code
	case resourcetask.StatusManualTakeover:
		code := strings.ToLower(row.FailureCode)
		if code == "" {
			code = "resource.manual_review"
		}
		return provisionresourcecap.TaskStatusManualReview, code
	default:
		return provisionresourcecap.TaskStatusManualReview, "resource.task_invalid"
	}
}

func taskRequestID(operationNo string) string { return "core.provision." + operationNo }

func provisionSecretIdempotencyKey(request provisionresourcecap.TaskRequest) string {
	return "resource-provision-password-" + request.RequestDigest
}

// passwordProducer keeps the default inside the same 6..20-character contract
// as merchant-configured cloud passwords and guarantees every character class.
func (s *Service) passwordProducer() secretcap.PlaintextProducer {
	length := 20
	return s.generatedPasswordProducer(resourcecap.ServerGenerationPolicy{PasswordLength: &length, PasswordCharacters: []string{"lowercase", "uppercase", "digits", "symbols"}})
}

func (s *Service) healProvisionCredential(ctx context.Context, row *taskRow) error {
	if row == nil || row.TaskNo == "" || row.SecretRef == "" {
		return provisionresourcecap.ErrTaskConflict
	}
	digest := sha256.Sum256([]byte("provision-credential/v1\x00" + row.TaskNo))
	credentialNo := "cred_" + hex.EncodeToString(digest[:16])
	result, err := s.database.Exec(ctx, `
INSERT INTO plugin_idc_resource_provision_credential
    (tenant_id, credential_no, service_no, task_no, channel_no, secret_ref, status_code)
VALUES (?, ?, ?, ?, ?, ?, 'pending')
ON CONFLICT DO NOTHING`, idccap.DefaultTenantID, credentialNo,
		row.LocalResourceNo, row.TaskNo, row.ChannelNo, row.SecretRef)
	if err != nil {
		return gerror.Wrap(err, "persist provisioning credential handle")
	}
	if _, err = result.RowsAffected(); err != nil {
		return gerror.Wrap(err, "read provisioning credential insert result")
	}
	var persisted struct {
		ServiceNo, TaskNo, ChannelNo, SecretRef string
	}
	if err = s.database.GetScan(ctx, &persisted, `
SELECT service_no, task_no, channel_no, secret_ref
FROM plugin_idc_resource_provision_credential
WHERE tenant_id = ? AND task_no = ?`, idccap.DefaultTenantID, row.TaskNo); err != nil {
		return gerror.Wrap(err, "verify provisioning credential handle")
	}
	if persisted.ServiceNo != row.LocalResourceNo || persisted.TaskNo != row.TaskNo ||
		persisted.ChannelNo != row.ChannelNo ||
		subtle.ConstantTimeCompare([]byte(persisted.SecretRef), []byte(row.SecretRef)) != 1 {
		return provisionresourcecap.ErrTaskConflict
	}
	return nil
}

func databaseNow(ctx context.Context, database gdb.DB) (time.Time, error) {
	value, err := database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return time.Time{}, gerror.Wrap(err, "read resource provisioning database time")
	}
	now := value.Time()
	if now.IsZero() {
		return time.Time{}, provisionresourcecap.ErrSourceUnavailable
	}
	return now.UTC(), nil
}

func timePointerUnixMicro(value *time.Time) int64 {
	if value == nil {
		return 0
	}
	return value.UTC().UnixMicro()
}
