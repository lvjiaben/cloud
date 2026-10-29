// This file persists current merchant payment addresses without publication or
// process-local caches. Route permissions and direct staff identity gate writes.
package channelconfig

import (
	"context"
	"net"
	"net/url"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-payment/backend/internal/model/do"
	"lina-plugin-idc-payment/backend/internal/model/entity"
)

// PublicAddress contains merchant-wide server notification and browser return origins.
type PublicAddress struct {
	NotificationOrigin, CustomerReturnOrigin string
	Version                                  int64
}

// ReadPublicAddress returns current settings or version zero before first save.
// It reads one singleton row, so all nodes immediately observe saved values.
func (s *Service) ReadPublicAddress(ctx context.Context) (*PublicAddress, error) {
	var row *entity.PublicAddress
	if err := s.database.Model(do.PublicAddress{}).Ctx(ctx).Where(do.PublicAddress{TenantId: 0}).Scan(&row); err != nil {
		return nil, bizerr.WrapCode(err, CodePublicAddressUnavailable)
	}
	if row == nil {
		return &PublicAddress{}, nil
	}
	return &PublicAddress{NotificationOrigin: row.NotificationOrigin, CustomerReturnOrigin: row.CustomerReturnOrigin, Version: row.Version}, nil
}

// SavePublicAddress replaces current addresses under optimistic concurrency.
// Empty origins explicitly disable new payment submissions; no upstream write occurs.
func (s *Service) SavePublicAddress(ctx context.Context, input PublicAddress, expectedVersion int64, actorID int) (*PublicAddress, error) {
	notification, err := normalizePublicOrigin(input.NotificationOrigin, false)
	if err != nil {
		return nil, err
	}
	customer, err := normalizePublicOrigin(input.CustomerReturnOrigin, true)
	if err != nil || expectedVersion < 0 || actorID <= 0 {
		return nil, bizerr.NewCode(CodePublicAddressInvalid)
	}
	data := do.PublicAddress{TenantId: 0, NotificationOrigin: notification, CustomerReturnOrigin: customer, Version: expectedVersion + 1, UpdatedBy: actorID}
	var affected int64
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if expectedVersion == 0 {
			result, writeErr := s.database.Model(do.PublicAddress{}).Ctx(ctx).Data(data).OnConflict("tenant_id").InsertIgnore()
			if writeErr != nil {
				return writeErr
			}
			affected, writeErr = result.RowsAffected()
			return writeErr
		}
		result, writeErr := s.database.Model(do.PublicAddress{}).Ctx(ctx).Where(do.PublicAddress{TenantId: 0, Version: expectedVersion}).Data(data).Update()
		if writeErr != nil {
			return writeErr
		}
		affected, writeErr = result.RowsAffected()
		return writeErr
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicAddressUnavailable)
	}
	if affected != 1 {
		return nil, bizerr.NewCode(CodePublicAddressConflict)
	}
	return &PublicAddress{NotificationOrigin: notification, CustomerReturnOrigin: customer, Version: expectedVersion + 1}, nil
}

// NormalizePublicAddress validates source origins without probing external URLs.
// Notifications require HTTPS; local browser returns may use loopback HTTP.
func NormalizePublicAddress(input PublicAddress) (*PublicAddress, error) {
	notification, err := normalizePublicOrigin(input.NotificationOrigin, false)
	if err != nil {
		return nil, err
	}
	customer, err := normalizePublicOrigin(input.CustomerReturnOrigin, true)
	if err != nil {
		return nil, err
	}
	if notification == "" || customer == "" {
		return nil, bizerr.NewCode(CodePublicAddressUnconfigured)
	}
	return &PublicAddress{NotificationOrigin: notification, CustomerReturnOrigin: customer, Version: input.Version}, nil
}

func normalizePublicOrigin(value string, allowLocal bool) (string, error) {
	if value == "" {
		return "", nil
	}
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(value), "/"))
	if err != nil || parsed.Host == "" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.Path != "" || parsed.RawPath != "" || parsed.Opaque != "" || strings.HasSuffix(strings.ToLower(parsed.Hostname()), ".invalid") {
		return "", bizerr.NewCode(CodePublicAddressInvalid)
	}
	if parsed.Scheme != "https" {
		address := net.ParseIP(parsed.Hostname())
		if !allowLocal || parsed.Scheme != "http" || parsed.Hostname() != "localhost" && (address == nil || !address.IsLoopback()) {
			return "", bizerr.NewCode(CodePublicAddressInvalid)
		}
	}
	return parsed.String(), nil
}
