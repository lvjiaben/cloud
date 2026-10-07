// Package contractautosign owns installation-local enterprise auto-sign authorizations.
package contractautosign

import (
	"context"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/sessioncap"
	"lina-plugin-idc-compliance/backend/internal/localgatewayclient"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

const (
	PermissionManage = "idc:admin:compliance:contract-auto-sign:manage"
	ProviderTencent  = "tencent_essbasic"
	SourceRequired   = "required_order"
	SourceOptional   = "manual_optional"
	StatusActive     = "active"
	StatusExpired    = "expired"
	StatusRevoked    = "revoked"
	PageSize         = 20
	MaxTemplates     = 50
)

type TemplateRef struct {
	TemplateNo string `json:"template_no"`
	VersionNo  int64  `json:"version_no"`
	Name       string `json:"name"`
}

type Item struct {
	AuthorizationNo     string
	CurrencyCode        string
	AllowRequiredOrder  bool
	AllowManualOptional bool
	MaxAmountMinor      int64
	SuccessLimit        int64
	SuccessCount        int64
	StatusCode          string
	ExpiresAt           time.Time
	CreatedByUserID     int
	RevokedByUserID     *int
	RevokedAt           *time.Time
	Version             int64
	CreatedAt           time.Time
	Templates           []TemplateRef
}

type List struct {
	Items      []Item
	Candidates []TemplateRef
	Page       int
	PageSize   int
	Total      int64
}

type CreateInput struct {
	CurrencyCode        string
	AllowRequiredOrder  bool
	AllowManualOptional bool
	MaxAmountMinor      int64
	SuccessLimit        int64
	ExpiresAt           time.Time
	Templates           []TemplateRef
	Confirmed           bool
	IdempotencyKey      string
}

type RevokeInput struct {
	AuthorizationNo string
	ExpectedVersion int64
	Confirmed       bool
	IdempotencyKey  string
}

type DecisionInput struct {
	SourceCode   string
	TemplateNo   string
	VersionNo    int64
	CurrencyCode string
	AmountMinor  int64
}

type Decision struct {
	AuthorizationNo string
	Version         int64
}

type AcceptedInput struct {
	AuthorizationNo    string
	ContractNo         string
	CurrencyCode       string
	AmountMinor        int64
	ProviderAcceptedAt time.Time
	IdempotencyKey     string
}

type Service interface {
	List(context.Context, int, ...int) (*List, error)
	Create(context.Context, CreateInput) (*Item, error)
	Revoke(context.Context, RevokeInput) (*Item, error)
	Evaluate(context.Context, DecisionInput) (*Decision, error)
	RecordAccepted(context.Context, AcceptedInput) error
}

type gateway interface {
	TencentOnboardingStatus(context.Context) (localgatewayclient.TencentOnboardingResult, error)
}

type serviceImpl struct {
	database gdb.DB
	runtime  managementcap.Runtime
	sessions sessioncap.Service
	gateway  gateway
	now      func() time.Time
}

var _ Service = (*serviceImpl)(nil)

func New(database gdb.DB, runtime managementcap.Runtime, sessions sessioncap.Service, gateway gateway) (Service, error) {
	if nilDependency(database) || managementcap.ValidateRuntime(runtime) != nil ||
		nilDependency(sessions) || nilDependency(gateway) {
		return nil, gerror.New("contract auto-sign dependencies are invalid")
	}
	return &serviceImpl{database: database, runtime: runtime, sessions: sessions, gateway: gateway, now: time.Now}, nil
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
