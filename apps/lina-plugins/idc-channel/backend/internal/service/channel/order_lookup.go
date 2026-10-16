// This file implements bounded same-system protocol projections and rejects
// missing ownership or configuration evidence rather than guessing it.

package channel

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
)

// GetOrderByRequest only reads the application-owned request. The same lock as
// submission waits for an in-flight transaction before reporting absence.
func (s *service) GetOrderByRequest(ctx context.Context, appID, requestID string) (*Order, error) {
	if requestID != strings.TrimSpace(requestID) || len(requestID) < 8 || len(requestID) > 120 {
		return nil, bizerr.NewCode(CodePurchaseInvalid)
	}
	if _, err := s.requireChannel(ctx, appID, false); err != nil {
		return nil, err
	}
	var result *Order
	err := s.db.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, err := s.db.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(?,0))`, appID+":"+requestID); err != nil {
			return err
		}
		var row *struct{ Response string }
		if err := s.db.Model("plugin_idc_channel_request").Ctx(ctx).Fields("response").Where("channel_id=? AND request_id=? AND operation='create_order'", appID, requestID).Scan(&row); err != nil {
			return err
		}
		if row == nil {
			return nil
		}
		var reference struct {
			ID int64 `json:"id"`
		}
		if json.Unmarshal([]byte(row.Response), &reference) != nil || reference.ID <= 0 {
			return bizerr.NewCode(codeCatalogUnavailable)
		}
		order, err := s.GetOrder(ctx, reference.ID, appID)
		if err != nil {
			return err
		}
		result = &order
		return nil
	})
	return result, err
}
