package serviceaction

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

const (
	AudienceCustomer = "customer"
	AudienceAdmin    = "admin"
)

var safeReferencePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,255}$`)

type Actor struct {
	Audience   string
	CustomerNo string
	UserID     int
}

type CreateInput struct {
	EntitlementNo  string
	ActionKey      string
	IdempotencyKey string
	Input          json.RawMessage
	Actor          Actor
}

type Task struct {
	TaskNo, EntitlementNo, ActionKey, Status string
	SafeProjection                           json.RawMessage
	Version                                  int64
}

// DirectoryFilter restricts one plugin-owned customer list without changing ownership.
type DirectoryFilter struct{ DirectoryNo, PluginID string }

type Service interface {
	// ListEntitlements pages authorized customer-paid services or data-scoped staff results; optional directory/plugin filters are applied before paging. Invalid filters fail without broadening access.
	ListEntitlements(context.Context, Actor, int64, int, ...DirectoryFilter) ([]Entitlement, int64, error)
	GetEntitlement(context.Context, string, Actor) (*Entitlement, error)
	Create(context.Context, CreateInput) (*Task, error)
	Get(context.Context, string, Actor) (*Task, error)
	Takeover(context.Context, string, int64, Actor) (*Task, error)
	CreateDownloadTicket(context.Context, DownloadInput) (*DownloadTicket, error)
	RedeemDownload(context.Context, string, Actor) (*DownloadContent, error)
}

type service struct {
	database   gdb.DB
	extensions pluginhost.BusinessExtensionRegistrar
	random     io.Reader
	downloader productautomation.PrivateSupplierDownloader
	contexts   productautomation.ExistingBusinessRuntimeContextIssuer
	storage    storagecap.Service
}

func New(database gdb.DB, extensions pluginhost.BusinessExtensionRegistrar, optional ...any) (Service, error) {
	if database == nil || extensions == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	result := &service{database: database, extensions: extensions, random: rand.Reader}
	for _, dependency := range optional {
		switch typed := dependency.(type) {
		case productautomation.PrivateSupplierDownloader:
			result.downloader = typed
		case productautomation.ExistingBusinessRuntimeContextIssuer:
			result.contexts = typed
		case storagecap.Service:
			result.storage = typed
		}
	}
	return result, nil
}

func NewConfigured(database gdb.DB, extensions pluginhost.BusinessExtensionRegistrar, downloader productautomation.PrivateSupplierDownloader, contexts productautomation.ExistingBusinessRuntimeContextIssuer, storage storagecap.Service) (Service, error) {
	if database == nil || extensions == nil || downloader == nil || contexts == nil || storage == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &service{database: database, extensions: extensions, downloader: downloader, contexts: contexts, storage: storage, random: rand.Reader}, nil
}

type frozenEntitlement struct {
	EntitlementNo, CustomerNo, PluginID, ReleaseVersion, ContractVersion string
	ReleaseDigest, DeclarationDigest, InterfaceInstanceNo                string
	SourceIntentNo, OrderNo, OrderItemNo, ConfigurationSnapshotNo        string
	ExternalReference                                                    string
	InterfaceConfigurationRevision                                       int64
	ProductConfiguration                                                 json.RawMessage
}

func (s *service) Create(ctx context.Context, input CreateInput) (*Task, error) {
	if err := validateActor(input.Actor); err != nil || !safeReferencePattern.MatchString(input.EntitlementNo) ||
		!safeReferencePattern.MatchString(input.ActionKey) || !safeReferencePattern.MatchString(input.IdempotencyKey) ||
		(len(input.Input) > 0 && !validObject(input.Input)) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	frozen, declaration, action, err := s.resolve(ctx, input.EntitlementNo, input.ActionKey)
	if err != nil || action == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	_ = declaration
	if action.Key != input.ActionKey || action.Execution != productfulfillmentcap.ServiceActionExecutionAsynchronous ||
		!contains(action.Audiences, input.Actor.Audience) || (input.Actor.Audience == AudienceCustomer && frozen.CustomerNo != input.Actor.CustomerNo) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	queryAction := actionByMethod(declaration, action.QueryMethod)
	if queryAction == nil || queryAction.Execution != productfulfillmentcap.ServiceActionExecutionProjection {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	rawInput := input.Input
	if len(rawInput) == 0 {
		rawInput = json.RawMessage(`{}`)
	}
	digest := digest(rawInput)
	taskNo, err := randomReference(s.random, "msa_")
	if err != nil {
		return nil, err
	}
	result, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_managed_service_action_task(tenant_id,task_no,entitlement_no,customer_no,action_key,audience_code,actor_user_id,idempotency_key,input_snapshot,input_digest,plugin_id,plugin_release_version,contract_version,release_digest,declaration_digest,interface_instance_no,interface_configuration_revision,source_intent_no,order_no,order_item_no,configuration_snapshot_no,external_reference,action_method,query_method,query_action_key,confirmed_entitlement) VALUES(0,?,?,?,?,?,?,?,?,?::jsonb,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(customer_no,entitlement_no,action_key,idempotency_key) DO NOTHING`,
		taskNo, frozen.EntitlementNo, frozen.CustomerNo, action.Key, input.Actor.Audience, input.Actor.UserID, input.IdempotencyKey, rawInput, digest,
		frozen.PluginID, frozen.ReleaseVersion, frozen.ContractVersion, frozen.ReleaseDigest, frozen.DeclarationDigest, frozen.InterfaceInstanceNo,
		frozen.InterfaceConfigurationRevision, frozen.SourceIntentNo, frozen.OrderNo, frozen.OrderItemNo, frozen.ConfigurationSnapshotNo,
		frozen.ExternalReference, action.Method, action.QueryMethod, queryAction.Key, action.ConfirmedEntitlement)
	if err != nil {
		return nil, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		var evidence *struct {
			InputDigest  string
			AudienceCode string
			ActorUserID  int
		}
		if err = s.database.GetScan(ctx, &evidence, `SELECT input_digest,audience_code,actor_user_id FROM plugin_idc_core_managed_service_action_task WHERE tenant_id=0 AND customer_no=? AND entitlement_no=? AND action_key=? AND idempotency_key=? LIMIT 1`, frozen.CustomerNo, frozen.EntitlementNo, action.Key, input.IdempotencyKey); err != nil || evidence == nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		if evidence.InputDigest != digest || evidence.AudienceCode != input.Actor.Audience || evidence.ActorUserID != input.Actor.UserID {
			return nil, bizerr.NewCode(CodeConflict)
		}
	}
	return s.getForActor(ctx, input.EntitlementNo, action.Key, input.IdempotencyKey, input.Actor)
}

func (s *service) resolve(ctx context.Context, entitlementNo, actionKey string) (*frozenEntitlement, *productfulfillmentcap.ProviderDeclaration, *productfulfillmentcap.ManagedServiceAction, error) {
	var row *frozenEntitlement
	const query = `SELECT entitlement.entitlement_no,entitlement.customer_no,entitlement.plugin_id,
entitlement.plugin_release_version AS release_version,entitlement.contract_version,intent.release_digest,
entitlement.declaration_digest,entitlement.interface_instance_no,intent.interface_configuration_revision,
entitlement.source_intent_no,entitlement.order_no,entitlement.order_item_no,entitlement.configuration_snapshot_no,
task.external_reference,configuration.merchant_value_snapshot AS product_configuration
FROM plugin_idc_core_service_entitlement entitlement
JOIN plugin_idc_core_product_fulfillment_intent intent
  ON intent.tenant_id=entitlement.tenant_id AND intent.intent_no=entitlement.source_intent_no
JOIN plugin_idc_core_product_fulfillment_task task
  ON task.tenant_id=intent.tenant_id AND task.intent_no=intent.intent_no
JOIN plugin_idc_core_catalog_product_version_configuration configuration
  ON configuration.tenant_id=intent.tenant_id AND configuration.version_no=intent.version_no
 AND configuration.interface_instance_no=intent.interface_instance_no
 AND configuration.interface_configuration_revision=intent.interface_configuration_revision
WHERE entitlement.tenant_id=0 AND entitlement.entitlement_no=?
  AND entitlement.status_code IN('pending_activation','active','suspended','expired')
  AND task.status_code IN('accepted','running','succeeded') LIMIT 1`
	err := s.database.GetScan(ctx, &row, query, entitlementNo)
	if err != nil || row == nil || row.ExternalReference == "" || row.InterfaceConfigurationRevision < 1 {
		return nil, nil, nil, bizerr.NewCode(CodeNotAccessible)
	}
	exact, err := s.extensions.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{PluginID: row.PluginID, ReleaseVersion: row.ReleaseVersion, Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: row.ContractVersion, ReleaseDigest: row.ReleaseDigest, DeclarationDigest: row.DeclarationDigest})
	if err != nil || exact == nil {
		return nil, nil, nil, bizerr.NewCode(CodeUnavailable)
	}
	declaration, err := productfulfillmentcap.ParseProviderDeclaration(exact.ProviderMetadata, exact.AuthorizedMethods)
	if err != nil {
		return nil, nil, nil, err
	}
	if !declaration.OngoingManagement || declaration.EntitlementOwnerID != "idc-core" {
		return nil, nil, nil, bizerr.NewCode(CodeUnavailable)
	}
	return row, declaration, actionByKey(declaration, actionKey), nil
}

func (s *service) getForActor(ctx context.Context, entitlementNo, actionKey, idempotencyKey string, actor Actor) (*Task, error) {
	var task *Task
	err := s.database.GetScan(ctx, &task, `SELECT task_no,entitlement_no,action_key,status_code AS status,safe_projection,version FROM plugin_idc_core_managed_service_action_task WHERE tenant_id=0 AND entitlement_no=? AND action_key=? AND idempotency_key=? AND (?='admin' OR customer_no=?) LIMIT 1`, entitlementNo, actionKey, idempotencyKey, actor.Audience, actor.CustomerNo)
	if err != nil || task == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return task, nil
}

func (s *service) Get(ctx context.Context, taskNo string, actor Actor) (*Task, error) {
	if validateActor(actor) != nil || !safeReferencePattern.MatchString(taskNo) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	var task *Task
	err := s.database.GetScan(ctx, &task, `SELECT task_no,entitlement_no,action_key,status_code AS status,safe_projection,version FROM plugin_idc_core_managed_service_action_task WHERE tenant_id=0 AND task_no=? AND (?='admin' OR customer_no=?) LIMIT 1`, taskNo, actor.Audience, actor.CustomerNo)
	if err != nil || task == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return task, nil
}

func (s *service) Takeover(ctx context.Context, taskNo string, expectedVersion int64, actor Actor) (*Task, error) {
	if actor.Audience != AudienceAdmin || actor.UserID < 1 || expectedVersion < 1 || !safeReferencePattern.MatchString(taskNo) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_managed_service_action_task SET status_code='manual_attention',phase_code='final',takeover_requested=TRUE,lease_token='',lease_expires_at=NULL,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND task_no=? AND version=? AND status_code IN('pending_query','pending_submit','retry_wait','result_unknown')`, taskNo, expectedVersion)
	if err != nil {
		return nil, err
	}
	if rows, _ := result.RowsAffected(); rows != 1 {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return s.Get(ctx, taskNo, actor)
}

func validateActor(actor Actor) error {
	if actor.UserID < 1 || actor.Audience != AudienceCustomer && actor.Audience != AudienceAdmin || actor.Audience == AudienceCustomer && !safeReferencePattern.MatchString(actor.CustomerNo) {
		return bizerr.NewCode(CodeInvalid)
	}
	return nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func validObject(raw []byte) bool {
	var value map[string]json.RawMessage
	return len(raw) <= productfulfillmentcap.MaximumEvidenceBytes && json.Unmarshal(raw, &value) == nil && value != nil
}

func digest(raw []byte) string { value := sha256.Sum256(raw); return hex.EncodeToString(value[:]) }

func randomReference(reader io.Reader, prefix string) (string, error) {
	value := make([]byte, 16)
	if _, err := io.ReadFull(reader, value); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(value), nil
}

func actionByKey(declaration *productfulfillmentcap.ProviderDeclaration, key string) *productfulfillmentcap.ManagedServiceAction {
	if declaration == nil {
		return nil
	}
	for index := range declaration.ServiceActions {
		if declaration.ServiceActions[index].Key == strings.TrimSpace(key) {
			return &declaration.ServiceActions[index]
		}
	}
	return nil
}

func actionByMethod(declaration *productfulfillmentcap.ProviderDeclaration, method string) *productfulfillmentcap.ManagedServiceAction {
	if declaration == nil {
		return nil
	}
	for index := range declaration.ServiceActions {
		if declaration.ServiceActions[index].Method == method {
			return &declaration.ServiceActions[index]
		}
	}
	return nil
}
