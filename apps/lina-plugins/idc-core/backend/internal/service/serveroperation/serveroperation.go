// Package serveroperation owns customer-visible server action intent,
// mutual exclusion, and convergence with resource-owned Provider tasks.
package serveroperation

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

const maximumBatch = 64

var (
	ErrInvalidInput       = errors.New("server operation input is invalid")
	ErrServiceUnavailable = errors.New("server operation service is unavailable")
	ErrActionUnavailable  = errors.New("server operation action is unavailable")
	ErrOperationConflict  = errors.New("server operation conflicts with current state")
)

type Command struct {
	CustomerNo         string
	ServiceNo          string
	Action             serveroperationresourcecap.Action
	Parameters         json.RawMessage
	SecretBindings     []serveroperationresourcecap.SecretBinding
	IdempotencyKey     string
	ActorType          string
	ActorReference     string
	RequiredPermission string
}

type Operation struct {
	OperationNo string
	OrderNo     string
	InvoiceNo   string
	ServiceNo   string
	Action      serveroperationresourcecap.Action
	Status      string
	CreatedAt   time.Time
	Reused      bool
}

type AccessoryCheckoutInput struct {
	QuoteNo        string
	IdempotencyKey string
}

type AccessoryCheckout struct {
	OperationNo  string
	OrderNo      string
	InvoiceNo    string
	ServiceNo    string
	Status       string
	AmountMinor  int64
	CurrencyCode string
	PaymentDueAt time.Time
	Reused       bool
}

type Batch struct {
	Selected       int
	Dispatched     int
	Reconciled     int
	Bound          int
	Renewed        int
	PaymentPending int
	ManualReview   int
	Failures       int
}

type Service interface {
	Create(context.Context, Command) (*Operation, error)
	UpdateCustomerMetadata(context.Context, CustomerMetadataCommand) (*CustomerMetadata, error)
}

type CustomerMetadataCommand struct {
	CustomerNo      string
	ServiceNo       string
	Alias           string
	Remark          string
	ExpectedVersion int64
}

type CustomerMetadata struct {
	Alias   string
	Remark  string
	Version int64
}

type AccountCommand struct {
	ServiceNo      string
	Action         serveroperationresourcecap.Action
	Parameters     json.RawMessage
	Password       []byte
	ProofToken     string
	IdempotencyKey string
}

type Authorization struct {
	Required  bool
	Token     string
	ExpiresAt int64
}

type AccountService interface {
	PrepareCurrentCustomer(context.Context, AccountCommand) (*Authorization, error)
	CreateCurrentCustomer(context.Context, AccountCommand) (*Operation, error)
	RedeemCurrentCustomerOutput(context.Context, string, string,
		serveroperationresourcecap.ConfidentialOutputConsumer) error
	UpdateCurrentCustomerMetadata(context.Context, string, string, string, int64) (*CustomerMetadata, error)
	RequestCurrentCustomerTransfer(context.Context, TransferRequestCommand) (*TransferRequestReceipt, error)
	ListCurrentCustomerIncomingTransfers(context.Context) ([]IncomingTransfer, error)
	ConfirmCurrentCustomerTransfer(context.Context, string, string) (*TransferConfirmation, error)
}

type TransferRequestCommand struct {
	ServiceNo      string
	RecipientEmail string
	IdempotencyKey string
}

type TransferRequestReceipt struct {
	Accepted bool
	Reused   bool
}

type IncomingTransfer struct {
	TransferNo string
	ServiceNo  string
	ExpiresAt  time.Time
}

type TransferConfirmation struct {
	TransferNo string
	ServiceNo  string
	Status     string
	Reused     bool
}

type Worker interface {
	Run(context.Context, int) (*Batch, error)
}

type serviceImpl struct {
	database gdb.DB
	audit    idccap.BusinessAuditWriter
}
type accountServiceImpl struct {
	database     gdb.DB
	customers    identitycap.CurrentCustomerReader
	operations   Service
	inputSecrets serveroperationresourcecap.InputSecretSource
	parameters   serveroperationresourcecap.ParameterValidator
	outputs      serveroperationresourcecap.ConfidentialOutputRedeemer
	ownership    serveroperationresourcecap.OwnershipTransfer
	recentAuth   reauth.SessionConfirmationService
}
type workerImpl struct {
	database      gdb.DB
	source        serveroperationresourcecap.Source
	directory     serveroperationresourcecap.AccessoryDirectory
	ipCapacity    serveroperationresourcecap.IPCapacity
	invoices      billing.InvoiceService
	notifications notificationcap.EventSink
}

func NewService(database gdb.DB, audit idccap.BusinessAuditWriter) (Service, error) {
	if database == nil || audit == nil {
		return nil, ErrServiceUnavailable
	}
	return &serviceImpl{database: database, audit: audit}, nil
}

func NewWorker(database gdb.DB, source serveroperationresourcecap.Source,
	directory serveroperationresourcecap.AccessoryDirectory,
	invoices billing.InvoiceService, notifications notificationcap.EventSink) (Worker, error) {
	ipCapacity, ok := source.(serveroperationresourcecap.IPCapacity)
	if database == nil || source == nil || directory == nil || !ok || ipCapacity == nil || invoices == nil || notifications == nil {
		return nil, ErrServiceUnavailable
	}
	return &workerImpl{database: database, source: source, directory: directory,
		ipCapacity: ipCapacity, invoices: invoices, notifications: notifications}, nil
}

func NewAccountService(database gdb.DB, customers identitycap.CurrentCustomerReader,
	operations Service, inputSecrets serveroperationresourcecap.InputSecretSource,
	parameters serveroperationresourcecap.ParameterValidator,
	outputs serveroperationresourcecap.ConfidentialOutputRedeemer,
	ownership serveroperationresourcecap.OwnershipTransfer,
	recentAuth reauth.SessionConfirmationService) (AccountService, error) {
	if database == nil || customers == nil || operations == nil || inputSecrets == nil || parameters == nil || outputs == nil || ownership == nil || recentAuth == nil {
		return nil, ErrServiceUnavailable
	}
	return &accountServiceImpl{database: database, customers: customers,
		operations: operations, inputSecrets: inputSecrets, parameters: parameters, outputs: outputs, ownership: ownership, recentAuth: recentAuth}, nil
}
