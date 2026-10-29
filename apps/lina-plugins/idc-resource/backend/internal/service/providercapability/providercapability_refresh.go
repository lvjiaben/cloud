// This file refreshes due connector matrices and commits headers plus all items
// atomically. Database time and channel version checks fence concurrent edits.
package providercapability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/model/do"
	"lina-plugin-idc-resource/backend/internal/model/entity"
	"time"
)

func (s *serviceImpl) RunOne(ctx context.Context) (bool, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return false, gerror.New("provider capability collector is unavailable")
	}
	value, err := s.database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return false, err
	}
	now := value.Time().UTC()
	var target *channelTarget
	err = s.database.GetScan(ctx, &target, `SELECT channel.channel_no,channel.version
 FROM plugin_idc_resource_channel channel
 LEFT JOIN LATERAL (
  SELECT snapshot.observed_at,snapshot.status_code FROM plugin_idc_resource_capability_snapshot snapshot
  WHERE snapshot.tenant_id=channel.tenant_id AND snapshot.channel_no=channel.channel_no
  ORDER BY snapshot.observed_at DESC,snapshot.id DESC LIMIT 1
 ) latest ON TRUE
 WHERE channel.tenant_id=0 AND channel.status_code='enabled' AND channel.deleted_at IS NULL
  AND (latest.observed_at IS NULL OR latest.observed_at<=CASE WHEN latest.status_code=? THEN ?::timestamptz ELSE ?::timestamptz END)
 ORDER BY latest.observed_at ASC NULLS FIRST,channel.id LIMIT 1`, snapshotComplete, now.Add(-refreshInterval), now.Add(-failedRefreshInterval))
	if err != nil || target == nil {
		return false, err
	}
	snapshot, callErr := s.discover(ctx, target.ChannelNo)
	if err = s.persist(ctx, target, snapshot); err != nil {
		return true, err
	}
	if callErr != nil {
		return true, ErrDiscoveryFailed
	}
	return true, nil
}

func (s *serviceImpl) discover(ctx context.Context, channelNo string) (*resourcecap.CapabilitySnapshot, error) {
	callCtx, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()
	provider, cleanup, err := s.construct(callCtx, channelNo)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil || provider == nil || cleanup == nil {
		return nil, gerror.New("enabled provider session is unavailable")
	}
	snapshot, err := provider.DiscoverCapabilities(callCtx)
	if err != nil || snapshot == nil || snapshot.ValidateAt(time.Now().UTC()) != nil || len(snapshot.AdapterVersion) > 64 {
		return nil, gerror.New("provider capability declaration is incomplete")
	}
	for _, item := range snapshot.Capabilities {
		if len(item.UnsupportedReason) > 512 {
			return nil, gerror.New("provider capability reason exceeds storage limit")
		}
	}
	return snapshot, nil
}

func (s *serviceImpl) persist(ctx context.Context, target *channelTarget, snapshot *resourcecap.CapabilitySnapshot) error {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	snapshotNo := "CAP_" + hex.EncodeToString(random[:])
	return s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var channel *entity.Channel
		if err := s.database.Model(do.Channel{}).Ctx(ctx).Fields("channel_no,version,status_code").Where(do.Channel{TenantId: 0, ChannelNo: target.ChannelNo}).LockUpdate().Scan(&channel); err != nil {
			return err
		}
		if channel == nil || channel.Version != target.Version || channel.StatusCode != channelEnabled {
			return gerror.New("provider channel changed during capability discovery")
		}
		value, err := s.database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
		if err != nil {
			return err
		}
		now := value.Time().UTC()
		header := do.CapabilitySnapshot{TenantId: 0, SnapshotNo: snapshotNo, ChannelNo: target.ChannelNo, AdapterVersion: unavailableAdapter, StatusCode: snapshotFailed, ItemCount: 0, ObservedAt: &now, CompletedAt: &now}
		items := make([]do.CapabilitySnapshotItem, 0, resourcecap.MaximumCapabilityCount)
		if snapshot != nil {
			header.AdapterVersion = snapshot.AdapterVersion
			header.StatusCode = snapshotComplete
			header.ItemCount = len(snapshot.Capabilities)
			for _, capability := range snapshot.Capabilities {
				constraints := capability.Constraints
				if constraints == nil {
					constraints = []resourcecap.ParameterConstraint{}
				}
				payload, encodeErr := json.Marshal(constraints)
				if encodeErr != nil {
					return encodeErr
				}
				items = append(items, do.CapabilitySnapshotItem{TenantId: 0, SnapshotNo: snapshotNo, CapabilityCode: string(capability.Code), SupportCode: string(capability.Support), Constraints: string(payload), UnsupportedReason: capability.UnsupportedReason})
			}
		}
		if _, err = s.database.Model(do.CapabilitySnapshot{}).Ctx(ctx).Data(header).Insert(); err != nil {
			return err
		}
		if len(items) > 0 {
			if _, err = s.database.Model(do.CapabilitySnapshotItem{}).Ctx(ctx).Data(items).Batch(resourcecap.MaximumCapabilityCount).Insert(); err != nil {
				return err
			}
		}
		return nil
	})
}
