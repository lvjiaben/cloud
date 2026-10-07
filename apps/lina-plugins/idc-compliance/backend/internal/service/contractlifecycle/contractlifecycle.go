// Package contractlifecycle consumes exact Tencent contract-flow observations
// from the installation-local gateway and appends the local legal timeline.
package contractlifecycle

import (
	"context"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const (
	providerTencentESSBasic = "tencent_essbasic"
	MaximumBatchSize        = 50
)

// Gateway is the narrow local process seam used by the primary-node worker.
type Gateway interface {
	ListTencentContractDeliveries(context.Context, int) ([]processauth.TencentContractDeliveryItem, error)
	AcknowledgeTencentContractDeliveries(context.Context, []string, string) error
}

// Observation is one supplier-authoritative exact status.
type Observation struct {
	DeliveryNo          string
	RouteNo             string
	RouteVersion        int64
	ProviderStatus      string
	SourceKind          string
	GatewayDecisionCode string
	SupplierObservedAt  time.Time
	ObservedAt          time.Time
}

// ApplyResult describes the durable local decision for one observation.
type ApplyResult struct {
	ContractNo     string
	FlowNo         string
	ContractStatus string
	FlowStatus     string
	DecisionCode   string
	Replayed       bool
}

// ConsumeResult is one bounded worker pass.
type ConsumeResult struct {
	Received     int
	Acknowledged int
}

// Service owns exact status convergence and the append-only timeline.
type Service interface {
	Consume(context.Context, int) (ConsumeResult, error)
	Apply(context.Context, Observation, idccap.BusinessAuditWriter) (ApplyResult, error)
}

type serviceImpl struct {
	contracts   scopedModel
	flows       scopedModel
	events      scopedModel
	gateway     Gateway
	systemAudit idccap.BusinessAuditWriter
}

var _ Service = (*serviceImpl)(nil)

// New constructs the primary-node lifecycle worker and reusable convergence service.
func New(database gdb.DB, gateway Gateway, systemAudit idccap.BusinessAuditWriter) (Service, error) {
	if nilDependency(database) || nilDependency(gateway) || nilDependency(systemAudit) {
		return nil, gerror.New("contract lifecycle dependencies are invalid")
	}
	contracts := newScopedModel(database, dao.Contract.Table())
	flows := newScopedModel(database, dao.ContractFlow.Table())
	events := newScopedModel(database, dao.ContractStatusEvent.Table())
	return &serviceImpl{
		contracts: contracts, flows: flows, events: events,
		gateway: gateway, systemAudit: systemAudit,
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
