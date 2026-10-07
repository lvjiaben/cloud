// Package poolcandidate projects resource-owner pool eligibility to the catalog requester.
package poolcandidate

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/poolsourcecap"
)

const eligiblePoolProjectionSQL = `
SELECT p.pool_no, p.channel_no, p.display_name, p.resource_kind,
       p.version AS pool_version,
       c.display_name AS channel_name, c.provider_id,
       c.provider_plugin_id, c.version AS channel_version
FROM plugin_idc_resource_pool p
JOIN plugin_idc_resource_channel c
  ON c.tenant_id = p.tenant_id AND c.channel_no = p.channel_no
WHERE p.tenant_id = ? AND p.resource_kind = ?
  AND p.status_code = 'enabled' AND p.deleted_at IS NULL
  AND c.status_code = 'enabled' AND c.deleted_at IS NULL
ORDER BY p.display_name, p.pool_no
LIMIT ?`

const exactEligiblePoolProjectionSQL = `
SELECT p.pool_no, p.channel_no, p.display_name, p.resource_kind,
       p.version AS pool_version,
       c.display_name AS channel_name, c.provider_id,
       c.provider_plugin_id, c.version AS channel_version
FROM plugin_idc_resource_pool p
JOIN plugin_idc_resource_channel c
  ON c.tenant_id = p.tenant_id AND c.channel_no = p.channel_no
WHERE p.tenant_id = ? AND p.pool_no = ? AND p.resource_kind = ?
  AND p.status_code = 'enabled' AND p.deleted_at IS NULL
  AND c.status_code = 'enabled' AND c.deleted_at IS NULL
LIMIT 1`

type projection struct {
	PoolNo           string
	ChannelNo        string
	DisplayName      string
	ResourceKind     string
	PoolVersion      int64
	ChannelName      string
	ProviderID       string
	ProviderPluginID string
	ChannelVersion   int64
}

// Service performs one bounded owner query for listing and one exact query for commit-time verification.
type Service struct{ database gdb.DB }

var _ poolsourcecap.Source = (*Service)(nil)

// New creates the resource-owner candidate source.
func New(database gdb.DB) (*Service, error) {
	if database == nil {
		return nil, gerror.New("resource pool candidate database is required")
	}
	return &Service{database: database}, nil
}

// ListEligible returns enabled pools under enabled channels for one exact server family.
func (s *Service) ListEligible(
	ctx context.Context,
	request poolsourcecap.ListRequest,
) ([]poolsourcecap.Candidate, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil ||
		poolsourcecap.ValidateListRequest(request) != nil {
		return nil, poolsourcecap.ErrSourceUnavailable
	}
	var rows []projection
	if err := s.database.GetScan(ctx, &rows, eligiblePoolProjectionSQL,
		idccap.DefaultTenantID, request.ResourceKind, poolsourcecap.MaximumCandidates+1); err != nil {
		return nil, gerror.Wrap(err, "list eligible resource pool candidates")
	}
	if len(rows) > poolsourcecap.MaximumCandidates {
		return nil, gerror.New("eligible resource pool candidate collection exceeds limit")
	}
	candidates := make([]poolsourcecap.Candidate, len(rows))
	for index, row := range rows {
		candidate, err := projectCandidate(row)
		if err != nil {
			return nil, err
		}
		candidates[index] = candidate
	}
	if err := poolsourcecap.ValidateCandidates(candidates); err != nil {
		return nil, gerror.Wrap(err, "validate eligible resource pool candidates")
	}
	return candidates, nil
}

// VerifyEligible re-reads the exact current owner row and compares its revision in constant time.
func (s *Service) VerifyEligible(
	ctx context.Context,
	request poolsourcecap.VerifyRequest,
) (*poolsourcecap.Candidate, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil {
		return nil, poolsourcecap.ErrSourceUnavailable
	}
	if err := poolsourcecap.ValidateVerifyRequest(request); err != nil {
		return nil, poolsourcecap.ErrCandidateUnavailable
	}
	var row projection
	if err := s.database.GetScan(ctx, &row, exactEligiblePoolProjectionSQL,
		idccap.DefaultTenantID, request.PoolNo, request.ResourceKind); err != nil {
		return nil, gerror.Wrap(err, "verify eligible resource pool candidate")
	}
	if row.PoolNo == "" {
		return nil, poolsourcecap.ErrCandidateUnavailable
	}
	candidate, err := projectCandidate(row)
	if err != nil {
		return nil, poolsourcecap.ErrCandidateUnavailable
	}
	if subtle.ConstantTimeCompare([]byte(candidate.Revision), []byte(request.Revision)) != 1 {
		return nil, poolsourcecap.ErrCandidateUnavailable
	}
	return &candidate, nil
}

func projectCandidate(row projection) (poolsourcecap.Candidate, error) {
	candidate := poolsourcecap.Candidate{
		PoolNo: row.PoolNo, DisplayName: row.DisplayName, ChannelName: row.ChannelName,
		ProviderID: row.ProviderID, ProviderPluginID: row.ProviderPluginID,
		ResourceKind: poolsourcecap.ResourceKind(row.ResourceKind),
		Revision:     candidateRevision(row),
	}
	if err := poolsourcecap.ValidateCandidate(candidate); err != nil {
		return poolsourcecap.Candidate{}, gerror.Wrap(err, "project eligible resource pool candidate")
	}
	return candidate, nil
}

func candidateRevision(row projection) string {
	digest := sha256.New()
	for _, value := range []string{
		"idc-resource/pool-candidate/v1", row.PoolNo, row.ChannelNo, row.ResourceKind,
		row.ProviderPluginID, row.ProviderID,
	} {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		_, _ = digest.Write(length[:])
		_, _ = digest.Write([]byte(value))
	}
	var versions [16]byte
	binary.BigEndian.PutUint64(versions[:8], uint64(row.PoolVersion))
	binary.BigEndian.PutUint64(versions[8:], uint64(row.ChannelVersion))
	_, _ = digest.Write(versions[:])
	return hex.EncodeToString(digest.Sum(nil))
}
