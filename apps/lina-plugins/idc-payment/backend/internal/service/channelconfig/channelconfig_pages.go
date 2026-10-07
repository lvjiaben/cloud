// This file owns bounded administrative directory pages using generated models.
// Count and safe row projections share one repeatable-read, read-only transaction.

package channelconfig

import (
	"context"
	"database/sql"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-payment/backend/internal/dao"
	"lina-plugin-idc-payment/backend/internal/model/do"
)

// ListManaged returns a bounded merchant-owned projection without secret handles.
// Invalid bounds or storage failures return localized errors without partial totals.
func (s *Service) ListManaged(ctx context.Context, page, pageSize int) (*ChannelPage, error) {
	if page < 1 || page > 1_000_000 || pageSize < 1 || pageSize > 100 {
		return nil, bizerr.NewCode(CodePageInvalid)
	}
	var (
		total int
		rows  gdb.Result
	)
	err := s.database.TransactionWithOptions(ctx, gdb.TxOptions{Propagation: gdb.PropagationRequired, Isolation: sql.LevelRepeatableRead, ReadOnly: true}, func(ctx context.Context, _ gdb.TX) error {
		model := s.database.Model(do.Channel{}).Ctx(ctx).Where(do.Channel{TenantId: 0})
		var err error
		total, err = model.Clone().Count()
		if err != nil {
			return err
		}
		cols := dao.Channel.Columns()
		rows, err = model.Fields(cols.ChannelNo, cols.ProviderCode, cols.DisplayName, cols.EndpointOrigin, cols.MerchantReference, cols.StatusCode, cols.SupportedModes, cols.SupportedCurrencies, cols.Version, cols.VerifiedAt).
			OrderAsc(cols.Id).Page(page, pageSize).All()
		return err
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePageUnavailable)
	}
	result := make([]Channel, 0, len(rows))
	for _, row := range rows {
		channel, decodeErr := decodeChannel(row)
		if decodeErr != nil {
			return nil, bizerr.WrapCode(decodeErr, CodePageUnavailable)
		}
		result = append(result, *channel)
	}
	return &ChannelPage{Items: result, Total: total}, nil
}
