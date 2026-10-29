// This file leases bounded read-only observations using the shared database.
// Old leases and changed mappings cannot publish. Failed calls retain last facts.
package serverfacts

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/dao"
	"lina-plugin-idc-resource/backend/internal/model/do"
)

const observationTimeout = 5 * time.Second
const refreshInterval = 30 * time.Second

type refreshClaim struct {
	MappingNo, ChannelNo, ExternalResourceRef string
	LeaseVersion                              int64
	MappingVersion                            int64
}

func (s *serviceImpl) RunOne(ctx context.Context) (bool, error) {
	if ctx == nil || ctx.Err() != nil {
		return false, gerror.New("native facts refresh context unavailable")
	}
	// Seed at most 64 newly bound resources; subsequent invocations continue the
	// indexed mapping scan. This is local work, never a supplier call per list row.
	if _, err := s.database.Exec(ctx, seedQuery); err != nil {
		return false, gerror.Wrap(err, "seed native runtime facts")
	}
	var claim *refreshClaim
	if err := s.database.GetScan(ctx, &claim, claimQuery); err != nil {
		return false, gerror.Wrap(err, "lease native runtime facts")
	}
	if claim == nil {
		return false, nil
	}
	snapshot, err := s.observe(ctx, claim)
	if err != nil {
		return true, s.release(ctx, claim, "provider_read_unavailable")
	}
	fields := project(snapshot)
	payload, err := json.Marshal(fields)
	if err != nil {
		return true, gerror.Wrap(err, "encode native runtime facts")
	}
	next := time.Now().Add(refreshInterval)
	result, err := s.database.Model(dao.ServerRuntimeFact.Table()).Safe().Ctx(ctx).
		Where(do.ServerRuntimeFact{TenantId: 0, MappingNo: claim.MappingNo, LeaseVersion: claim.LeaseVersion}).
		Where("lease_until>CURRENT_TIMESTAMP AND (observed_at IS NULL OR observed_at<=?)", snapshot.ObservedAt).
		Where(`EXISTS(SELECT 1 FROM plugin_idc_resource_mapping mapping WHERE mapping.mapping_no=plugin_idc_resource_server_runtime_fact.mapping_no AND mapping.tenant_id=0 AND mapping.channel_no=? AND mapping.external_resource_ref=? AND mapping.version=? AND mapping.status_code='active')`, claim.ChannelNo, claim.ExternalResourceRef, claim.MappingVersion).
		Data(do.ServerRuntimeFact{Facts: string(payload), ObservedAt: &snapshot.ObservedAt, NextRefreshAt: &next, LastErrorCode: "", MappingVersion: claim.MappingVersion}).Update()
	if err != nil {
		return true, gerror.Wrap(err, "publish native runtime facts")
	}
	if _, err = result.RowsAffected(); err != nil {
		return true, gerror.Wrap(err, "check native runtime publication")
	}
	return true, s.release(ctx, claim, "")
}

func (s *serviceImpl) observe(ctx context.Context, claim *refreshClaim) (*resourcecap.ResourceSnapshot, error) {
	callCtx, cancel := context.WithTimeout(ctx, observationTimeout)
	defer cancel()
	provider, cleanup, err := s.providers.ConstructSession(callCtx, claim.ChannelNo)
	if err != nil {
		return nil, err
	}
	if provider == nil || cleanup == nil {
		return nil, gerror.New("native facts provider unavailable")
	}
	defer cleanup()
	result, err := provider.QueryResource(callCtx, resourcecap.ResourceQuery{ExternalResourceRef: claim.ExternalResourceRef})
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if result == nil || result.Validate() != nil || result.State == resourcecap.ResourceStateUnknown || result.ExternalResourceRef != claim.ExternalResourceRef || result.ObservedAt.IsZero() || now.Sub(result.ObservedAt) > time.Minute || result.ObservedAt.After(now.Add(time.Second)) {
		return nil, gerror.New("native facts observation invalid")
	}
	return result, nil
}

func project(snapshot *resourcecap.ResourceSnapshot) displayFields {
	result := displayFields{State: snapshot.State, Addresses: []string{}}
	var fields struct {
		Hostname  string   `json:"hostname"`
		OSName    string   `json:"os_name"`
		Addresses []string `json:"addresses"`
	}
	if json.Unmarshal(snapshot.Data, &fields) != nil {
		return result
	}
	result.Hostname = safeLabel(fields.Hostname, 253)
	result.OSName = safeLabel(fields.OSName, 128)
	if validAddresses(fields.Addresses) {
		result.Addresses = fields.Addresses
		result.AddressesConfirmed = true
	}
	return result
}

func (s *serviceImpl) release(ctx context.Context, claim *refreshClaim, code string) error {
	now := time.Now()
	next := now.Add(refreshInterval)
	_, err := s.database.Model(dao.ServerRuntimeFact.Table()).Safe().Ctx(ctx).
		Where(do.ServerRuntimeFact{TenantId: 0, MappingNo: claim.MappingNo, LeaseVersion: claim.LeaseVersion}).
		Data(do.ServerRuntimeFact{NextRefreshAt: &next, LastErrorCode: code, LeaseUntil: &now}).Update()
	return gerror.Wrap(err, "release native facts lease")
}

const seedQuery = `INSERT INTO plugin_idc_resource_server_runtime_fact(mapping_no)
SELECT ownership.mapping_no FROM plugin_idc_resource_server_entitlement_projection ownership
JOIN plugin_idc_resource_mapping mapping ON mapping.mapping_no=ownership.mapping_no AND mapping.tenant_id=ownership.tenant_id AND mapping.local_resource_no=ownership.local_resource_no
WHERE ownership.tenant_id=0 AND mapping.status_code='active' AND mapping.external_resource_ref<>''
AND NOT EXISTS(SELECT 1 FROM plugin_idc_resource_server_runtime_fact fact WHERE fact.mapping_no=ownership.mapping_no)
ORDER BY ownership.mapping_no LIMIT 64 ON CONFLICT DO NOTHING`

const claimQuery = `WITH due AS (
 SELECT fact.mapping_no,mapping.channel_no,mapping.external_resource_ref,mapping.version AS mapping_version
 FROM plugin_idc_resource_server_runtime_fact fact
 JOIN plugin_idc_resource_mapping mapping ON mapping.mapping_no=fact.mapping_no AND mapping.tenant_id=fact.tenant_id
 JOIN plugin_idc_resource_channel channel ON channel.channel_no=mapping.channel_no AND channel.tenant_id=mapping.tenant_id
 WHERE fact.tenant_id=0 AND fact.next_refresh_at<=CURRENT_TIMESTAMP AND (fact.lease_until IS NULL OR fact.lease_until<=CURRENT_TIMESTAMP)
 AND mapping.status_code='active' AND mapping.external_resource_ref<>'' AND channel.status_code='enabled' AND channel.deleted_at IS NULL
 AND EXISTS(SELECT 1 FROM plugin_idc_resource_server_entitlement_projection ownership WHERE ownership.tenant_id=0 AND ownership.mapping_no=mapping.mapping_no AND ownership.local_resource_no=mapping.local_resource_no)
 ORDER BY fact.next_refresh_at,fact.mapping_no LIMIT 1 FOR UPDATE OF fact SKIP LOCKED
) UPDATE plugin_idc_resource_server_runtime_fact fact SET lease_until=CURRENT_TIMESTAMP+INTERVAL '30 seconds',lease_version=lease_version+1
FROM due WHERE fact.mapping_no=due.mapping_no RETURNING fact.mapping_no,due.channel_no,due.external_resource_ref,fact.lease_version,due.mapping_version`
