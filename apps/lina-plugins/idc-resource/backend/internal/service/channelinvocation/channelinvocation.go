// Package channelinvocation owns recoverable cross-process channel serialization.
package channelinvocation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const leaseDuration = 2 * time.Minute

type Lease struct {
	database  gdb.DB
	channelNo string
	token     string
}

type Service struct{ database gdb.DB }

func New(database gdb.DB) (*Service, error) {
	if database == nil {
		return nil, gerror.New("resource channel invocation lease database is required")
	}
	return &Service{database: database}, nil
}

func (s *Service) Acquire(ctx context.Context, channelNo string) (*Lease, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil ||
		resourcecap.ValidateChannelRef(channelNo) != nil {
		return nil, gerror.New("resource channel invocation lease is unavailable")
	}
	var randomBytes [16]byte
	if _, err := rand.Read(randomBytes[:]); err != nil {
		return nil, gerror.New("resource channel invocation lease is unavailable")
	}
	token := "ril_" + hex.EncodeToString(randomBytes[:])
	var row struct {
		LeaseToken string `orm:"lease_token"`
	}
	err := s.database.GetScan(ctx, &row, `
INSERT INTO plugin_idc_resource_channel_invocation_lease
    (tenant_id,channel_no,lease_token,lease_until,updated_at)
VALUES (?,?,?,CURRENT_TIMESTAMP + (? * INTERVAL '1 second'),CURRENT_TIMESTAMP)
ON CONFLICT (tenant_id,channel_no) DO UPDATE
SET lease_token=EXCLUDED.lease_token,lease_until=EXCLUDED.lease_until,updated_at=CURRENT_TIMESTAMP
WHERE plugin_idc_resource_channel_invocation_lease.lease_until <= CURRENT_TIMESTAMP
RETURNING lease_token`, idccap.DefaultTenantID, channelNo, token, int(leaseDuration/time.Second))
	if err != nil || row.LeaseToken != token {
		return nil, gerror.New("resource channel invocation is already in progress")
	}
	return &Lease{database: s.database, channelNo: channelNo, token: token}, nil
}

func (l *Lease) Release(ctx context.Context) error {
	if l == nil || l.database == nil || ctx == nil {
		return gerror.New("resource channel invocation lease is unavailable")
	}
	_, err := l.database.Exec(ctx, `
DELETE FROM plugin_idc_resource_channel_invocation_lease
WHERE tenant_id=? AND channel_no=? AND lease_token=?`, idccap.DefaultTenantID, l.channelNo, l.token)
	return err
}
