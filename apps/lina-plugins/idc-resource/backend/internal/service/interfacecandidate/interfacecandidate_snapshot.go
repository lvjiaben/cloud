// This file serves supplier options from a shared, reconstructible projection.
// Page reads never wait for a supplier. Refresh workers retain exact enabled
// interface authority and only perform the existing read-only candidate call.
package interfacecandidate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"sort"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/model/do"
	"lina-plugin-idc-resource/backend/internal/model/entity"
)

const (
	candidateSnapshotRefresh = 5 * time.Minute
	candidateSnapshotRetry   = 30 * time.Second
	candidateSnapshotLease   = 45 * time.Second
	candidateRefreshTimeout  = 30 * time.Second
)

type candidateSnapshotFailure string

const candidateSnapshotReadFailed candidateSnapshotFailure = "supplier_read_failed"

type snapshotQuery struct {
	FieldKey       string   `json:"fieldKey"`
	ParentValues   []string `json:"parentValues,omitempty"`
	SelectedValues []string `json:"selectedValues,omitempty"`
}

type snapshotRequest struct {
	SchemaVersion     string            `json:"schemaVersion"`
	Requests          []snapshotQuery   `json:"requests"`
	CurrentSelections []json.RawMessage `json:"currentSelections,omitempty"`
}

// candidateSnapshotRequest drops selection values that this frozen connector
// does not consume. CPU or price edits cannot create duplicate remote reads.
func candidateSnapshotRequest(payload json.RawMessage) ([]byte, []snapshotQuery, error) {
	var value snapshotRequest
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF || value.SchemaVersion != "v1" || len(value.Requests) == 0 || len(value.Requests) > 3 {
		return nil, nil, gerror.New("supplier option request is invalid")
	}
	seen := make(map[string]bool)
	for index, query := range value.Requests {
		if seen[query.FieldKey] || len(query.ParentValues) > 1 {
			return nil, nil, gerror.New("supplier option request is invalid")
		}
		seen[query.FieldKey] = true
		value.Requests[index].SelectedValues = nil
		switch query.FieldKey {
		case "region", "operating_system":
			if len(query.ParentValues) != 0 {
				return nil, nil, gerror.New("supplier option parent is invalid")
			}
		case "line":
			if len(query.ParentValues) == 1 && !safeText(query.ParentValues[0]) {
				return nil, nil, gerror.New("supplier option parent is invalid")
			}
		default:
			return nil, nil, gerror.New("supplier option field is unavailable")
		}
	}
	value.CurrentSelections = nil
	sort.Slice(value.Requests, func(i, j int) bool { return value.Requests[i].FieldKey < value.Requests[j].FieldKey })
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > 16384 {
		return nil, nil, gerror.New("supplier option request is invalid")
	}
	return raw, value.Requests, nil
}

func (source *LazySource) readCandidateSnapshot(ctx context.Context, deps *dependencies, request resourceinterfaceprojectioncap.CandidateRequest, current binding) (*resourceinterfaceprojectioncap.CandidateResult, error) {
	// loadBinding already checks the current channel and local sale state.
	// Snapshot reads need the stored contract identity, not a parsed executable.
	// Refresh workers and actual provider calls still enforce host admission.
	if !contains(current.AuthorizedMethods, request.Method) ||
		(request.ExpectedConfigurationRevision != "" && request.ExpectedConfigurationRevision != current.ConfigurationRevision) {
		return nil, gerror.New("supplier option source is unavailable")
	}
	payload, queries, err := candidateSnapshotRequest(request.Values)
	if err != nil {
		return nil, err
	}
	key, err := candidateSnapshotKey(request.InterfaceInstanceNo, current.ConfigurationRevision, current.ReleaseDigest, current.DeclarationDigest, payload)
	if err != nil {
		return nil, err
	}
	var row *entity.InterfaceCandidateSnapshot
	if err = deps.database.Model(do.InterfaceCandidateSnapshot{}).Ctx(ctx).
		Where(do.InterfaceCandidateSnapshot{TenantId: 0, RequestKey: key}).Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "read supplier option projection")
	}
	if (row == nil || row.ObservedAt == nil) && current.ContractVersion == resourcecap.ServerFulfillmentContractVersion {
		previous, previousErr := previousCandidateSnapshot(ctx, deps, request.InterfaceInstanceNo, current.ConfigurationRevision, string(payload))
		if previousErr != nil {
			return nil, previousErr
		}
		if previous != nil {
			if err = retainCandidateSnapshot(ctx, deps, key, previous); err != nil {
				return nil, err
			}
			if err = deps.database.Model(do.InterfaceCandidateSnapshot{}).Ctx(ctx).Where(do.InterfaceCandidateSnapshot{TenantId: 0, RequestKey: key}).Scan(&row); err != nil {
				return nil, gerror.Wrap(err, "read retained supplier directory")
			}
		}
	}
	if row == nil {
		if _, err = deps.database.Model(do.InterfaceCandidateSnapshot{}).Ctx(ctx).
			OnConflict("request_key").Data(do.InterfaceCandidateSnapshot{
			RequestKey: key, TenantId: 0, InterfaceInstanceNo: request.InterfaceInstanceNo,
			ConfigurationRevision: current.ConfigurationRevision, RequestPayload: string(payload),
		}).InsertIgnore(); err != nil {
			return nil, gerror.Wrap(err, "queue supplier option projection")
		}
	}
	// These are configuration choices, not a live inventory or availability
	// assertion. A failed refresh must not erase the last successful directory.
	// The exact current binding/key above still fences account and endpoint
	// changes; a successfully validated response replaces it.
	if row != nil && row.ObservedAt != nil && row.ResultPayload != "" {
		result, readErr := projectCandidateResult([]byte(row.ResultPayload))
		if readErr != nil {
			return nil, readErr
		}
		result.SyncState = resourceinterfaceprojectioncap.CandidateSyncReady
		if row.LastErrorCode != "" {
			result.SyncState = resourceinterfaceprojectioncap.CandidateSyncRetained
		}
		return result, nil
	}
	// A missing projection is an explicit empty set, never fabricated choices.
	// The complete local form and its prices can still be rendered immediately.
	result := &resourceinterfaceprojectioncap.CandidateResult{Fields: []resourceinterfaceprojectioncap.CandidateField{}, SyncState: resourceinterfaceprojectioncap.CandidateSyncPending}
	if row != nil && row.LastErrorCode != "" {
		result.SyncState = resourceinterfaceprojectioncap.CandidateSyncFailed
	}
	for _, query := range queries {
		result.Fields = append(result.Fields, resourceinterfaceprojectioncap.CandidateField{FieldKey: query.FieldKey, Items: []resourceinterfaceprojectioncap.Candidate{}})
	}
	return result, nil
}

func candidateSnapshotKey(interfaceNo, revision, releaseDigest, declarationDigest string, payload []byte) (string, error) {
	identity, err := json.Marshal([]string{interfaceNo, revision, releaseDigest, declarationDigest, string(payload)})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(identity)
	return hex.EncodeToString(digest[:]), nil
}

// RunCandidateRefresh advances one shared leased refresh. It never creates
// supplier resources, retries writes, or uses a web customer's credentials.
func (source *LazySource) RunCandidateRefresh(ctx context.Context) (bool, error) {
	if source == nil || ctx == nil || ctx.Err() != nil {
		return false, gerror.New("supplier option refresh is unavailable")
	}
	source.mu.RLock()
	deps := source.deps
	source.mu.RUnlock()
	if deps == nil {
		return false, nil
	}
	var row *entity.InterfaceCandidateSnapshot
	err := deps.database.GetScan(ctx, &row, `UPDATE plugin_idc_resource_interface_candidate_snapshot
SET lease_until=CURRENT_TIMESTAMP + (? * INTERVAL '1 second')
WHERE request_key=(SELECT request_key FROM plugin_idc_resource_interface_candidate_snapshot
WHERE refresh_after<=CURRENT_TIMESTAMP AND (lease_until IS NULL OR lease_until<CURRENT_TIMESTAMP)
ORDER BY refresh_after,request_key FOR UPDATE SKIP LOCKED LIMIT 1)
RETURNING *`, int(candidateSnapshotLease.Seconds()))
	if err != nil || row == nil || row.RequestKey == "" {
		return false, err
	}
	current, bindingErr := loadBinding(ctx, deps.database, row.InterfaceInstanceNo)
	if bindingErr == nil {
		evidence, evidenceErr := deps.extensions.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{
			PluginID: current.PluginID, ReleaseVersion: current.PluginReleaseVersion, Point: resourcecap.DynamicProviderPoint,
			ContractVersion: current.ContractVersion, ReleaseDigest: current.ReleaseDigest, DeclarationDigest: current.DeclarationDigest,
		})
		if evidenceErr == nil && evidence != nil {
			// PostgreSQL JSONB formatting is canonicalized again before comparison.
			canonical, _, canonicalErr := candidateSnapshotRequest([]byte(row.RequestPayload))
			currentKey, keyErr := candidateSnapshotKey(row.InterfaceInstanceNo, current.ConfigurationRevision, current.ReleaseDigest, current.DeclarationDigest, canonical)
			if keyErr == nil && canonicalErr == nil && currentKey != row.RequestKey {
				// This directory belongs to the same effective account and
				// native v1 protocol. Keep its validated labels before removing
				// the obsolete package key, even if this worker wins the race
				// against the first page read after an implementation upgrade.
				if canRetainCandidateDirectory(current, evidence, row) {
					if err = retainCandidateSnapshot(ctx, deps, currentKey, row); err != nil {
						return true, err
					}
				}
				_, err = deps.database.Model(do.InterfaceCandidateSnapshot{}).Ctx(ctx).
					Where(do.InterfaceCandidateSnapshot{RequestKey: row.RequestKey, LeaseUntil: row.LeaseUntil}).Delete()
				return true, err
			}
		}
	}
	readCtx, cancel := context.WithTimeout(ctx, candidateRefreshTimeout)
	defer cancel()
	response, _, readErr := source.invokeRead(readCtx, resourceinterfaceprojectioncap.CandidateRequest{
		InterfaceInstanceNo: row.InterfaceInstanceNo, ExpectedConfigurationRevision: row.ConfigurationRevision,
		Method: resourcecap.DynamicProviderMethodConfigurationCandidates, Values: []byte(row.RequestPayload),
	}, []string{"StatusCode", "Data", "Authenticated", "Fields", "FieldKey", "Items", "Value", "LabelZhCN", "LabelEnUS", "Relationships", "ParentFieldKey", "ChildFieldKey", "Links", "ParentValue", "ChildValue"})
	if readErr == nil {
		var projected *resourceinterfaceprojectioncap.CandidateResult
		projected, readErr = projectCandidateResult(response)
		if readErr == nil {
			// Persist only the validated projection, never arbitrary connector
			// response properties that the public option contract ignores.
			projected.SyncState = ""
			for field := range projected.Fields {
				for item := range projected.Fields[field].Items {
					projected.Fields[field].Items[item].ParentValue = ""
					projected.Fields[field].Items[item].OptionToken = ""
				}
			}
			response, readErr = json.Marshal(projected)
		}
	}
	// Cleanup is exact-lease fenced, even when an HTTP deadline canceled reads.
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer finishCancel()
	model := deps.database.Model(do.InterfaceCandidateSnapshot{}).Ctx(finishCtx).
		Where(do.InterfaceCandidateSnapshot{TenantId: 0, RequestKey: row.RequestKey, LeaseUntil: row.LeaseUntil})
	now := time.Now()
	if readErr != nil {
		next := now.Add(candidateSnapshotRetry)
		_, err = model.Data(do.InterfaceCandidateSnapshot{RefreshAfter: &next, LastErrorCode: candidateSnapshotReadFailed}).
			Fields("refresh_after,last_error_code").Update()
		if err == nil {
			_, err = deps.database.Exec(finishCtx, `UPDATE plugin_idc_resource_interface_candidate_snapshot SET lease_until=NULL WHERE request_key=? AND lease_until=?`, row.RequestKey, row.LeaseUntil)
		}
		return true, err
	}
	next := now.Add(candidateSnapshotRefresh)
	_, err = model.Data(do.InterfaceCandidateSnapshot{ResultPayload: string(response), ObservedAt: &now, RefreshAfter: &next, LastErrorCode: ""}).
		Fields("result_payload,observed_at,refresh_after,last_error_code").Update()
	if err == nil {
		_, err = deps.database.Exec(finishCtx, `UPDATE plugin_idc_resource_interface_candidate_snapshot SET lease_until=NULL WHERE request_key=? AND lease_until=?`, row.RequestKey, row.LeaseUntil)
	}
	return true, err
}

// previousCandidateSnapshot reads one last successful directory for this exact
// connection revision and canonical request. Local channel state remains checked;
// it never crosses credentials or endpoint changes.
func previousCandidateSnapshot(ctx context.Context, deps *dependencies, interfaceNo, revision, payload string) (*entity.InterfaceCandidateSnapshot, error) {
	var row *entity.InterfaceCandidateSnapshot
	err := deps.database.Model(do.InterfaceCandidateSnapshot{}).Ctx(ctx).
		Where(do.InterfaceCandidateSnapshot{TenantId: 0, InterfaceInstanceNo: interfaceNo, ConfigurationRevision: revision, RequestPayload: payload}).
		WhereNotNull("observed_at").OrderDesc("observed_at").OrderAsc("request_key").Limit(1).Scan(&row)
	if err != nil {
		return nil, gerror.Wrap(err, "read previous supplier directory")
	}
	return row, nil
}

// retainCandidateSnapshot migrates only validated display candidates, retaining
// their observation time. Compare-and-set on observed_at prevents an old result
// from replacing a successful refresh produced concurrently by another node.
func retainCandidateSnapshot(ctx context.Context, deps *dependencies, key string, previous *entity.InterfaceCandidateSnapshot) error {
	if previous == nil || previous.ObservedAt == nil || previous.ResultPayload == "" {
		return nil
	}
	if _, err := projectCandidateResult([]byte(previous.ResultPayload)); err != nil {
		return err
	}
	if _, err := deps.database.Model(do.InterfaceCandidateSnapshot{}).Ctx(ctx).OnConflict("request_key").Data(do.InterfaceCandidateSnapshot{RequestKey: key, TenantId: 0, InterfaceInstanceNo: previous.InterfaceInstanceNo, ConfigurationRevision: previous.ConfigurationRevision, RequestPayload: previous.RequestPayload}).InsertIgnore(); err != nil {
		return gerror.Wrap(err, "queue retained supplier directory")
	}
	_, err := deps.database.Model(do.InterfaceCandidateSnapshot{}).Ctx(ctx).Where(do.InterfaceCandidateSnapshot{TenantId: 0, RequestKey: key}).WhereNull("observed_at").Data(do.InterfaceCandidateSnapshot{ResultPayload: previous.ResultPayload, ObservedAt: previous.ObservedAt, LastErrorCode: previous.LastErrorCode}).Update()
	return gerror.Wrap(err, "retain supplier directory across implementation upgrade")
}

func canRetainCandidateDirectory(current binding, evidence *capmodel.BusinessExtensionProviderEvidence, previous *entity.InterfaceCandidateSnapshot) bool {
	return previous != nil && previous.ConfigurationRevision == current.ConfigurationRevision && current.PluginID == "idc-zjmfcloud" && current.ContractVersion == resourcecap.ServerFulfillmentContractVersion && matchesResolvedEvidence(nil, nil, current, evidence, resourcecap.DynamicProviderMethodConfigurationCandidates)
}
