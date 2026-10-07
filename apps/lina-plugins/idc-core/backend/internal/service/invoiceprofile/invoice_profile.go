// Package invoiceprofile owns current-customer invoice-title management.
package invoiceprofile

import (
	"context"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	TitleTypeIndividual = "individual"
	TitleTypeEnterprise = "enterprise"
	StatusActive        = "active"
	StatusArchived      = "archived"
	BankAccountKeep     = "keep"
	BankAccountReplace  = "replace"
	BankAccountClear    = "clear"
	maximumPageSize     = 50
)

var (
	CodeInvalid = bizerr.MustDefine(
		"IDC_INVOICE_PROFILE_INVALID", "The invoice profile request is invalid", gcode.CodeInvalidParameter,
	)
	CodeNotAccessible = bizerr.MustDefine(
		"IDC_INVOICE_PROFILE_NOT_ACCESSIBLE", "The invoice profile is not accessible", gcode.CodeNotAuthorized,
	)
	CodeConflict = bizerr.MustDefine(
		"IDC_INVOICE_PROFILE_CONFLICT", "The invoice profile changed; reload it and try again", gcode.CodeInvalidOperation,
	)
	CodeUnavailable = bizerr.MustDefine(
		"IDC_INVOICE_PROFILE_UNAVAILABLE", "Invoice profile management is temporarily unavailable", gcode.CodeInternalError,
	)
)

type Profile struct {
	ProfileNo             string
	TitleTypeCode         string
	InvoiceTitle          string
	TaxRegistrationNo     string
	RegisteredAddress     string
	RegisteredPhone       string
	BankName              string
	BankAccountMasked     string
	BankAccountConfigured bool
	IsDefault             bool
	StatusCode            string
	Version               int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type ListInput struct {
	Cursor string
	Limit  int
}

type Page struct {
	Items      []Profile
	NextCursor string
}

type CreateInput struct {
	TitleTypeCode     string
	InvoiceTitle      string
	TaxRegistrationNo string
	RegisteredAddress string
	RegisteredPhone   string
	BankName          string
	BankAccount       []byte
	IsDefault         bool
	IdempotencyKey    string
}

type UpdateInput struct {
	ProfileNo         string
	ExpectedVersion   int64
	TitleTypeCode     string
	InvoiceTitle      string
	TaxRegistrationNo string
	RegisteredAddress string
	RegisteredPhone   string
	BankName          string
	BankAccountAction string
	BankAccount       []byte
	IdempotencyKey    string
}

type VersionInput struct {
	ProfileNo       string
	ExpectedVersion int64
}

type Service interface {
	ListCurrentCustomer(context.Context, ListInput) (*Page, error)
	CreateCurrentCustomer(context.Context, CreateInput) (*Profile, error)
	UpdateCurrentCustomer(context.Context, UpdateInput) (*Profile, error)
	SetDefaultCurrentCustomer(context.Context, VersionInput) (*Profile, error)
	ArchiveCurrentCustomer(context.Context, VersionInput) (*Profile, error)
}

type serviceImpl struct {
	database  gdb.DB
	customers identitycap.CurrentCustomerReader
	secrets   secretcap.Manager
	cursor    *cursorCodec
	numberKey []byte
}

var _ Service = (*serviceImpl)(nil)

func New(
	database gdb.DB,
	customers identitycap.CurrentCustomerReader,
	secrets secretcap.Manager,
	cursorKey []byte,
) (Service, error) {
	if nilDependency(database) || nilDependency(customers) || nilDependency(secrets) || len(cursorKey) != 32 {
		return nil, gerror.New("invoice profile service dependencies are invalid")
	}
	codec, err := newCursorCodec(cursorKey)
	if err != nil {
		return nil, err
	}
	numberKey := deriveKey(cursorKey, "idc-core/invoice-profile-number/v1")
	return &serviceImpl{
		database: database, customers: customers, secrets: secrets,
		cursor: codec, numberKey: numberKey,
	}, nil
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
