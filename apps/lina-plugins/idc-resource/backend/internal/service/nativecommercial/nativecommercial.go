// Package nativecommercial binds financially admitted native intents to the
// existing resource task executor. It never owns pricing or sends a cloud write.
package nativecommercial

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/model/do"
	"slices"
)

// ResourceSource uses the startup-owned task and candidate implementation.
type ResourceSource interface {
	serveroperationresourcecap.Source
	serveroperationresourcecap.ParameterValidator
	serveroperationresourcecap.CapabilityDirectory
	serveroperationresourcecap.AccessoryDirectory
}

// Service coordinates durable native intent/task correlation and receipts.
type Service struct {
	database   gdb.DB
	commercial servercommercialcap.Service
	resources  ResourceSource
}

// New requires the existing startup services, never constructing a second executor.
func New(database gdb.DB, commercial servercommercialcap.Service, resources ResourceSource) (*Service, error) {
	if database == nil || commercial == nil || resources == nil {
		return nil, gerror.New("native commercial dependencies are unavailable")
	}
	return &Service{database: database, commercial: commercial, resources: resources}, nil
}

// RunReady performs one bounded scan. Each owner transaction binds at most one
// task; concurrent scans and process restarts reuse its exact operation number.
func (s *Service) RunReady(ctx context.Context) error {
	var failures []error
	if _, err := s.commercial.ReconcileReady(ctx, 8); err != nil {
		failures = append(failures, err)
	}
	numbers, err := s.commercial.ListReady(ctx, 8)
	if err != nil {
		return errors.Join(append(failures, err)...)
	}
	for _, number := range numbers {
		var rejected *servercommercialcap.AdmissionFailure
		err = s.commercial.WithReadyOperation(ctx, number, func(txctx context.Context, operation servercommercialcap.Operation) (string, error) {
			task, enqueueErr := s.enqueue(txctx, operation)
			if enqueueErr != nil {
				rejected = permanentAdmissionFailure(operation, enqueueErr)
			}
			return task, enqueueErr
		})
		if err != nil && rejected != nil {
			err = errors.Join(err, s.commercial.RejectUnsent(ctx, *rejected))
		}
		if err != nil {
			failures = append(failures, err)
		}
		if ctx.Err() != nil {
			break
		}
	}
	return errors.Join(failures...)
}

func (s *Service) enqueue(ctx context.Context, operation servercommercialcap.Operation) (string, error) {
	route, err := s.lockRoute(ctx, operation)
	if err != nil {
		return "", err
	}
	var existing *binding
	if err = s.database.GetScan(ctx, &existing, `SELECT task_no,intent_digest FROM plugin_idc_resource_native_commercial_task WHERE tenant_id=0 AND operation_no=?`, operation.OperationNo); err != nil {
		return "", err
	}
	if existing != nil {
		if existing.IntentDigest != operation.ContentDigest {
			return "", gerror.New("native commercial intent changed")
		}
		return existing.TaskNo, nil
	}
	busy, err := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_resource_task WHERE tenant_id=0 AND local_resource_no=? AND status_code NOT IN ('succeeded','failed','canceled'))`, route.LocalResourceNo)
	if err != nil {
		return "", err
	}
	if busy.Bool() {
		return "", gerror.New("native commercial server has an unfinished task")
	}
	parameters, err := canonicalParameters(operation.Parameters)
	if err != nil {
		return "", errTargetChanged
	}
	capabilities, err := s.resources.ListCapabilities(ctx, []serveroperationresourcecap.CapabilityRoute{{ServiceNo: operation.LocalResourceNo, ChannelNo: operation.ChannelNo, PoolNo: operation.PoolNo}})
	if err != nil {
		return "", err
	}
	if len(capabilities) != 1 || capabilities[0].ObservedAt.IsZero() {
		return "", serveroperationresourcecap.ErrSourceUnavailable
	}
	if !slices.Contains(capabilities[0].Actions, operation.Action) {
		return "", errCapabilityRemoved
	}
	request := serveroperationresourcecap.TaskRequest{OperationNo: operation.OperationNo, ServiceNo: operation.LocalResourceNo, ChannelNo: operation.ChannelNo, PoolNo: operation.PoolNo, Action: operation.Action, ExpectedServiceVersion: operation.ExpectedEntitlementVersion, Parameters: parameters}
	request.RequestDigest, err = serveroperationresourcecap.TaskRequestDigest(request)
	if err != nil {
		return "", err
	}
	task, err := s.resources.CreateTask(ctx, request)
	if err != nil {
		return "", err
	}
	if task == nil {
		return "", gerror.New("native commercial task was not created")
	}
	_, err = s.database.Model(dao.NativeCommercialTask.Table()).Safe().Ctx(ctx).Data(do.NativeCommercialTask{TenantId: 0, OperationNo: operation.OperationNo, EntitlementNo: operation.EntitlementNo, CustomerNo: operation.CustomerNo, TaskNo: task.ProviderTaskNo, LocalResourceNo: operation.LocalResourceNo, ChannelNo: operation.ChannelNo, PoolNo: operation.PoolNo, ExternalResourceRef: route.ExternalResourceRef, ActionCode: operation.Action, Parameters: string(parameters), RequestDigest: request.RequestDigest, IntentDigest: operation.ContentDigest, EntitlementVersion: operation.ExpectedEntitlementVersion}).Insert()
	if err != nil {
		return "", err
	}
	return task.ProviderTaskNo, nil
}

type binding struct {
	OperationNo, EntitlementNo, CustomerNo, TaskNo, LocalResourceNo, ChannelNo, PoolNo string
	ExternalResourceRef                                                                string
	ActionCode                                                                         serveroperationresourcecap.Action
	Parameters                                                                         json.RawMessage
	RequestDigest, IntentDigest                                                        string
	EntitlementVersion                                                                 int64
}

type ownedRoute struct{ LocalResourceNo, ChannelNo, PoolNo, ExternalResourceRef string }

func (s *Service) lockRoute(ctx context.Context, operation servercommercialcap.Operation) (*ownedRoute, error) {
	var route *ownedRoute
	err := s.database.GetScan(ctx, &route, `SELECT mapping.local_resource_no,mapping.channel_no,mapping.pool_no,mapping.external_resource_ref
FROM plugin_idc_resource_server_entitlement_projection ownership
JOIN plugin_idc_resource_mapping mapping ON mapping.tenant_id=ownership.tenant_id AND mapping.mapping_no=ownership.mapping_no AND mapping.local_resource_no=ownership.local_resource_no
WHERE ownership.tenant_id=0 AND ownership.entitlement_no=? AND ownership.customer_no=? AND mapping.status_code='active' FOR UPDATE OF ownership,mapping`, operation.EntitlementNo, operation.CustomerNo)
	if err != nil {
		return nil, err
	}
	if route == nil || route.LocalResourceNo != operation.LocalResourceNo || route.ChannelNo != operation.ChannelNo || route.PoolNo != operation.PoolNo || route.ExternalResourceRef == "" {
		return nil, errTargetChanged
	}
	return route, nil
}

func canonicalParameters(raw json.RawMessage) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if len(raw) == 0 || len(raw) > 16384 || json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil, gerror.New("native commercial parameters are invalid")
	}
	return json.Marshal(fields)
}
