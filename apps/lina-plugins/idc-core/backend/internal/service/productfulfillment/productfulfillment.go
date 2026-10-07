// Package productfulfillment owns generic, provider-neutral fulfillment intents.
package productfulfillment

import (
	"context"
	"errors"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

var CodeManagementUnavailable = bizerr.MustDefine(
	"IDC_PRODUCT_FULFILLMENT_MANAGEMENT_UNAVAILABLE",
	"Product fulfillment management is temporarily unavailable",
	gcode.CodeOperationFailed,
)

var (
	CodeActionInvalid      = bizerr.MustDefine("IDC_PRODUCT_FULFILLMENT_ACTION_INVALID", "The product fulfillment action is invalid", gcode.CodeInvalidParameter)
	CodeActionConflict     = bizerr.MustDefine("IDC_PRODUCT_FULFILLMENT_ACTION_CONFLICT", "The product fulfillment state changed; reload and try again", gcode.CodeOperationFailed)
	CodeNotAccessible      = bizerr.MustDefine("IDC_PRODUCT_FULFILLMENT_NOT_ACCESSIBLE", "The product fulfillment task is not accessible", gcode.CodeNotAuthorized)
	CodeReasonInvalid      = bizerr.MustDefine("IDC_PRODUCT_FULFILLMENT_REASON_INVALID", "The selected fulfillment reason is no longer available", gcode.CodeInvalidParameter)
	CodeOwnerGateBlocked   = bizerr.MustDefine("IDC_PRODUCT_FULFILLMENT_OWNER_GATE_BLOCKED", "The product fulfillment prerequisites are not complete", gcode.CodeOperationFailed)
	CodeNewBusinessStopped = bizerr.MustDefine("IDC_PRODUCT_FULFILLMENT_NEW_BUSINESS_STOPPED", "The fulfillment plugin has stopped accepting new business", gcode.CodeOperationFailed)
)

const (
	StrategyManual                 = "manual"
	StrategyApprovalThenPlugin     = "approval_then_plugin"
	StrategyPaymentConfirmedPlugin = "payment_confirmed_plugin"
	GranularityPerOrderItem        = "per_order_item"
	GranularityPerQuantityUnit     = "per_quantity_unit"

	PermissionTaskList       = "idc:admin:product-fulfillment:task:list"
	PermissionTaskQuery      = "idc:admin:product-fulfillment:task:query"
	PermissionReviewApprove  = "idc:admin:product-fulfillment:review-approve"
	PermissionReviewReject   = "idc:admin:product-fulfillment:review-reject"
	PermissionManualTakeover = "idc:admin:product-fulfillment:takeover"
	PermissionManualConclude = "idc:admin:product-fulfillment:manual-conclude"
)

// Fulfillment retries are host policy, not merchant or provider input. A query
// is always performed first; only an explicit not_accepted response permits a
// later submit retry.
const (
	maximumQueryAttempts  = 10
	maximumSubmitAttempts = 3
	retryIntervalSeconds  = 30
)

type SettlementInput struct {
	SettlementNo string
	OrderNo      string
}

type SettlementResult struct {
	OrderNo     string
	IntentCount int
	TaskCount   int
	ManualCount int
	ReviewCount int
	Reused      bool
}

// SettlementConsumer accepts only an already persisted funding-owner fact.
type SettlementConsumer interface {
	ConsumeFinanciallySettled(context.Context, SettlementInput) (*SettlementResult, error)
}

// OwnerGateReleaseInput is an internal owner command. It is not an HTTP DTO:
// the order/line evidence is reloaded and verified before any task is created.
type OwnerGateReleaseInput struct {
	OrderNo      string
	OrderItemNos []string
}

type OwnerGateReleaser interface {
	ReleaseOwnerGates(context.Context, OwnerGateReleaseInput) (*SettlementResult, error)
}

type OrderCancellationInput struct {
	OrderNo, CancellationNo string
}

type OrderCancellationResult struct {
	OrderNo                          string
	Cancelled, AwaitingQuery, Manual int
	Reused                           bool
}

// OrderCancellationConsumer accepts only an order-owner cancellation fact;
// it never initiates refunds or exposes a paid-order cancellation API.
type OrderCancellationConsumer interface {
	ConsumeOrderCancelled(context.Context, OrderCancellationInput) (*OrderCancellationResult, error)
}

type Authorization struct {
	ActorUserID        int
	RequiredPermission string
	DataScope          string
	AllCustomers       bool
	AllowedCustomerNos []string
}

// ManagementAuthorizer performs the exact permission and data-scope check
// before the service validates input or reveals whether an intent exists.
type ManagementAuthorizer interface {
	AuthorizeList(context.Context) (Authorization, error)
	AuthorizeDetail(context.Context, string) (Authorization, error)
	AuthorizeReviewApprove(context.Context, string) (Authorization, error)
	AuthorizeReviewReject(context.Context, string) (Authorization, error)
	AuthorizeTakeover(context.Context, string) (Authorization, error)
	AuthorizeManualConclude(context.Context, string) (Authorization, error)
}

// OwnerGateValidator is the owning commerce/contract/inventory boundary. It
// must validate all supplied order items as one bounded batch.
type OwnerGateValidator interface {
	EvaluateFulfillmentRelease(context.Context, string, []string) (map[string]bool, error)
}

type ActorDirectory interface {
	DisplayNames(context.Context, []int) (map[int]string, error)
}

type ReasonOption struct {
	Code, LabelZhCN, LabelEnUS string
}

type ActionInput struct {
	IntentNo        string
	ExpectedVersion int64
	IdempotencyKey  string
	ReasonCode      string
	Note            string
}

type ManualConclusionInput struct {
	ActionInput
	Outcome string
}

type IntentDetail struct {
	IntentNo, OrderNo, OrderItemNo, CustomerNo              string
	CustomerCode, CustomerDisplayName, CustomerPrimaryPhone string
	ProductNameZhCN, ProductNameEnUS                        string
	StrategyCode, GranularityCode, StatusCode               string
	Quantity, UnitSequence                                  int
	Version                                                 int64
	FinanciallySettledAt                                    time.Time
	UpdatedAt                                               time.Time
	CanReviewApprove, CanReviewReject                       bool
	CanTakeover, CanManualConclude                          bool
	ReasonOptions                                           map[string][]ReasonOption
	Events                                                  []IntentEvent
	internalStatus                                          string
}

type IntentEvent struct {
	TypeCode, StatusCode, ReasonCode string
	ReasonLabelZhCN, ReasonLabelEnUS string
	OperatorDisplayName, SafeNote    string
	OccurredAt                       time.Time
}

type ListInput struct {
	OrderNo, CustomerNo, Mode, SafeStatus, Cursor string
	Limit                                         int
	Page                                          int
}

type IntentPage struct {
	Total      int
	Items      []IntentDetail
	NextCursor string
}

type ManagementService interface {
	List(context.Context, ListInput) (*IntentPage, error)
	Detail(context.Context, string) (*IntentDetail, error)
	ApproveAndStart(context.Context, ActionInput) (*IntentDetail, error)
	RejectToManual(context.Context, ActionInput) (*IntentDetail, error)
	Takeover(context.Context, ActionInput) (*IntentDetail, error)
	ConcludeManual(context.Context, ManualConclusionInput) (*IntentDetail, error)
}

type Service interface {
	SettlementConsumer
	OwnerGateReleaser
	OrderCancellationConsumer
	ManagementService
}

type WorkerBatch struct {
	Selected, Completed, Scheduled, ManualAttention, Failed int
}

type Worker interface {
	Run(context.Context, int) (*WorkerBatch, error)
}

type serviceImpl struct {
	database      gdb.DB
	frozen        productautomation.FrozenCustomerConfigurationValidator
	gates         OwnerGateValidator
	systemReasons SystemTransitionReasonCatalog
	authorizer    ManagementAuthorizer
	audit         idccap.BusinessAuditWriter
	actors        ActorDirectory
	cursor        *fulfillmentCursorCodec
}

func New(database gdb.DB) (SettlementConsumer, error) {
	if database == nil {
		return nil, errors.New("product fulfillment requires database")
	}
	return &serviceImpl{database: database}, nil
}

func NewConfigured(database gdb.DB, validator productautomation.FrozenCustomerConfigurationValidator) (SettlementConsumer, error) {
	if database == nil || validator == nil {
		return nil, errors.New("product fulfillment requires database and frozen configuration validator")
	}
	return &serviceImpl{database: database, frozen: validator}, nil
}

func NewSettlementConsumer(database gdb.DB, validator productautomation.FrozenCustomerConfigurationValidator, gates OwnerGateValidator) (SettlementConsumer, error) {
	if database == nil || validator == nil || gates == nil {
		return nil, errors.New("product fulfillment settlement routing requires database and validators")
	}
	return &serviceImpl{database: database, frozen: validator, gates: gates}, nil
}

// NewOrderCancellationConsumer builds the narrow projector used by the
// outbox job. It needs only the authoritative database and stable bilingual
// system reasons; it does not expose management commands.
func NewOrderCancellationConsumer(database gdb.DB, systemReasons SystemTransitionReasonCatalog) (OrderCancellationConsumer, error) {
	if database == nil || systemReasons == nil {
		return nil, errors.New("product fulfillment cancellation requires database and system reasons")
	}
	return &serviceImpl{database: database, systemReasons: systemReasons}, nil
}

func NewFullConfigured(database gdb.DB, validator productautomation.FrozenCustomerConfigurationValidator, gates OwnerGateValidator, systemReasons SystemTransitionReasonCatalog, authorizer ManagementAuthorizer, audit idccap.BusinessAuditWriter, actors ActorDirectory, cursorKey []byte) (Service, error) {
	if database == nil || validator == nil || gates == nil || systemReasons == nil || authorizer == nil || audit == nil || actors == nil {
		return nil, errors.New("product fulfillment requires database, validators, authorization, and audit")
	}
	cursor, err := newFulfillmentCursorCodec(cursorKey)
	if err != nil {
		return nil, err
	}
	return &serviceImpl{database: database, frozen: validator, gates: gates, systemReasons: systemReasons, authorizer: authorizer, audit: audit, actors: actors, cursor: cursor}, nil
}

type settlementAuthority struct {
	SettlementNo           string    `orm:"settlement_no"`
	OrderNo                string    `orm:"order_no"`
	PayerCustomerNo        string    `orm:"payer_customer_no"`
	ServiceOwnerCustomerNo string    `orm:"service_owner_customer_no"`
	InvoiceNo              string    `orm:"invoice_no"`
	CurrencyCode           string    `orm:"currency_code"`
	SettledAt              time.Time `orm:"settled_at"`
	OrderStatus            string    `orm:"order_status"`
	RiskDecision           string    `orm:"risk_decision_code"`
	RequiresContract       bool      `orm:"requires_contract"`
	ItemCount              int       `orm:"item_count"`
}

type frozenItem struct {
	OrderItemNo                    string    `orm:"order_item_no"`
	OrderNo                        string    `orm:"order_no"`
	CustomerNo                     string    `orm:"customer_no"`
	ProductNo                      string    `orm:"product_no"`
	VersionNo                      string    `orm:"version_no"`
	ProductNameZhCN                string    `orm:"product_name_zh_cn"`
	ProductNameEnUS                string    `orm:"product_name_en_us"`
	Quantity                       int       `orm:"quantity"`
	QuantityDigest                 string    `orm:"quantity_digest"`
	StrategyCode                   string    `orm:"strategy_code"`
	GranularityCode                string    `orm:"granularity_code"`
	OngoingManagement              bool      `orm:"ongoing_management"`
	RenewalMode                    string    `orm:"renewal_mode_code"`
	RenewalMethod                  string    `orm:"renewal_method"`
	EntitlementOwnerID             string    `orm:"entitlement_owner_id"`
	ProvisionMethod                string    `orm:"provision_method"`
	QueryMethod                    string    `orm:"query_method"`
	MaximumAttempts                int       `orm:"maximum_attempts"`
	RetryBaseSeconds               int       `orm:"retry_base_seconds"`
	PolicyDigest                   string    `orm:"policy_digest"`
	ConfigurationSnapshotNo        string    `orm:"configuration_snapshot_no"`
	ConfigurationRevision          string    `orm:"configuration_revision"`
	InterfaceInstanceNo            string    `orm:"interface_instance_no"`
	InterfaceConfigurationRevision int64     `orm:"interface_configuration_revision"`
	ConfigurationContentDigest     string    `orm:"configuration_content_digest"`
	PluginID                       string    `orm:"plugin_id"`
	PluginReleaseVersion           string    `orm:"plugin_release_version"`
	ContractVersion                string    `orm:"contract_version"`
	ReleaseDigest                  string    `orm:"release_digest"`
	DeclarationDigest              string    `orm:"declaration_digest"`
	AuthorizedMethods              []string  `orm:"authorized_methods"`
	SettledAt                      time.Time `orm:"settled_at"`
	InterfaceNewBusinessState      string    `orm:"interface_new_business_state"`
	UploaderUserID                 int       `orm:"uploader_user_id"`
	CustomerDeclarationDigest      string    `orm:"customer_declaration_digest"`
	InventoryDecisionCode          string    `orm:"inventory_decision_code"`
	CandidateSnapshotDigest        string    `orm:"candidate_snapshot_digest"`
	CustomerValueDigest            string    `orm:"customer_value_digest"`
	CustomerDeclarationSnapshot    string    `orm:"customer_declaration_snapshot"`
	CandidateSnapshot              string    `orm:"candidate_snapshot"`
	TypedValueSnapshot             string    `orm:"typed_value_snapshot"`
}
