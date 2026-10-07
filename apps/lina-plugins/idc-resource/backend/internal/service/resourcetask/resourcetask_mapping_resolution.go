// This file resolves one bounded task batch against authoritative active resource mappings.

package resourcetask

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/model/do"
	"lina-plugin-idc-resource/backend/internal/model/entity"
)

// mappingStatus is the owner-internal lifecycle type for resource mappings.
type mappingStatus string

const (
	// mappingStatusProvisioning identifies an owner reservation awaiting an upstream identity.
	mappingStatusProvisioning mappingStatus = "provisioning"
	// mappingStatusActive identifies the only mapping state authoritative for Provider writes.
	mappingStatusActive mappingStatus = "active"
	// mappingStatusReleased identifies historical mappings that no longer own an upstream resource.
	mappingStatusReleased mappingStatus = "released"
)

// ResolveResources implements the executor's ResourceResolver with one
// projection query for the complete claimed batch. Missing, inactive, or
// destination-mismatched mappings are omitted so no Provider write can use an
// unproven upstream identity. Provision intentionally requires no mapping.
func (s *Store) ResolveResources(
	ctx context.Context,
	requests []ResourceResolutionRequest,
) (map[string]string, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil {
		return nil, gerror.New("resource mapping resolver is unavailable")
	}
	localResourceNos, err := validateResourceResolutionRequests(requests)
	if err != nil {
		return nil, err
	}
	resolved := make(map[string]string, len(requests))
	if len(localResourceNos) == 0 {
		return resolved, nil
	}
	columns := dao.Mapping.Columns()
	rows := make([]entity.Mapping, 0, len(localResourceNos))
	err = s.database.Model(dao.Mapping.Table()).Safe().Ctx(ctx).
		Fields(
			columns.LocalResourceNo,
			columns.ChannelNo,
			columns.PoolNo,
			columns.ExternalResourceRef,
			columns.StatusCode,
		).
		Where(do.Mapping{
			TenantId:   idccap.DefaultTenantID,
			StatusCode: mappingStatusActive,
		}).
		WhereIn(columns.LocalResourceNo, localResourceNos).
		Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "resolve active resource mapping batch")
	}
	byLocal := make(map[string]entity.Mapping, len(rows))
	for index := range rows {
		if _, duplicate := byLocal[rows[index].LocalResourceNo]; duplicate {
			return nil, gerror.New("active resource mapping cardinality is invalid")
		}
		byLocal[rows[index].LocalResourceNo] = rows[index]
	}
	for index := range requests {
		if requests[index].Capability == resourcecap.CapabilityProvision {
			continue
		}
		row, exists := byLocal[requests[index].LocalResourceNo]
		if !exists || row.ChannelNo != requests[index].ChannelNo ||
			row.PoolNo != requests[index].PoolNo ||
			(resourcecap.ResourceQuery{ExternalResourceRef: row.ExternalResourceRef}).Validate() != nil {
			continue
		}
		resolved[requests[index].TaskNo] = row.ExternalResourceRef
	}
	return resolved, nil
}

// validateResourceResolutionRequests verifies and deduplicates one bounded resolver batch.
func validateResourceResolutionRequests(requests []ResourceResolutionRequest) ([]string, error) {
	if len(requests) > maximumClaimBatchSize {
		return nil, gerror.New("resource mapping resolution batch is invalid")
	}
	seenTasks := make(map[string]struct{}, len(requests))
	seenResources := make(map[string]struct{}, len(requests))
	localResourceNos := make([]string, 0, len(requests))
	for index := range requests {
		request := resourcecap.OperationRequest{
			RequestID:   requests[index].TaskNo,
			Capability:  requests[index].Capability,
			ResourceRef: requests[index].LocalResourceNo,
			Parameters:  []byte(`{}`),
		}
		if request.Validate() != nil ||
			!workerIDPattern.MatchString(requests[index].LocalResourceNo) ||
			!workerIDPattern.MatchString(requests[index].ChannelNo) ||
			(requests[index].PoolNo != "" && !workerIDPattern.MatchString(requests[index].PoolNo)) {
			return nil, gerror.New("resource mapping resolution request is invalid")
		}
		if _, duplicate := seenTasks[requests[index].TaskNo]; duplicate {
			return nil, gerror.New("resource mapping resolution task is duplicated")
		}
		seenTasks[requests[index].TaskNo] = struct{}{}
		if requests[index].Capability == resourcecap.CapabilityProvision {
			continue
		}
		if _, duplicate := seenResources[requests[index].LocalResourceNo]; duplicate {
			continue
		}
		seenResources[requests[index].LocalResourceNo] = struct{}{}
		localResourceNos = append(localResourceNos, requests[index].LocalResourceNo)
	}
	return localResourceNos, nil
}
