// This file exposes customer-bound API management to staff without reading stored secrets.

package channel

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
)

// staffCustomerAccount uses the same installation staff boundary as existing API administration.
func (s *service) staffCustomerAccount(ctx context.Context, customerNo string) (*channelAccountRow, error) {
	if err := s.authorizeStaff(ctx); err != nil {
		return nil, err
	}
	customerNo = strings.TrimSpace(customerNo)
	if customerNo == "" || len(customerNo) > 40 {
		return nil, bizerr.NewCode(codeLifecycleInvalid)
	}
	var row *channelAccountRow
	err := s.db.Model("plugin_idc_channel_account").Ctx(ctx).Fields("channel_id,customer_no,display_name,status_code,new_sales_enabled").Where("customer_no=?", customerNo).Scan(&row)
	if err != nil {
		return nil, bizerr.WrapCode(err, codeLifecycleInvalid)
	}
	return row, nil
}

// StaffPlatform includes revoked history metadata but never returns a stored secret.
func (s *service) StaffPlatform(ctx context.Context, customerNo string) (PlatformOverview, error) {
	account, err := s.staffCustomerAccount(ctx, customerNo)
	if err != nil {
		return PlatformOverview{}, err
	}
	if account == nil {
		return PlatformOverview{Tokens: []TokenSummary{}}, nil
	}
	tokens, err := scanTokenSummaries(s.db.Model("plugin_idc_channel_token").Ctx(ctx).Where("channel_id=?", account.ChannelID).OrderDesc("id").Limit(1))
	return PlatformOverview{Authorized: true, AppID: account.ChannelID, DisplayName: account.DisplayName, Status: account.StatusCode, ResetEnabled: account.NewSalesEnable, Tokens: tokens}, err
}

// StaffCalls returns the exact count under the customer-bound application filter.
func (s *service) StaffCalls(ctx context.Context, customerNo string, query PageQuery) (PageResult[APICall], error) {
	query, err := normalizePage(query)
	if err != nil {
		return PageResult[APICall]{}, err
	}
	account, err := s.staffCustomerAccount(ctx, customerNo)
	if err != nil {
		return PageResult[APICall]{}, err
	}
	if account == nil {
		return PageResult[APICall]{List: []APICall{}, Page: query.Page, PageSize: query.PageSize}, nil
	}
	model := s.db.Model("plugin_idc_channel_api_call").Ctx(ctx).Where("channel_id=?", account.ChannelID).OrderDesc("id")
	return queryPage(model, query, func(page *gdb.Model) ([]APICall, error) {
		rows := []APICall{}
		err := page.Fields("id,method,path,succeeded,duration_millis,created_at").Scan(&rows)
		return rows, err
	})
}

// RotateStaffSecret retains permissions, source filters, expiry and current rate-window usage.
func (s *service) RotateStaffSecret(ctx context.Context, customerNo string, expectedID int64) (TokenResult, error) {
	account, err := s.staffCustomerAccount(ctx, customerNo)
	if err != nil {
		return TokenResult{}, err
	}
	if account == nil || expectedID <= 0 {
		return TokenResult{}, bizerr.NewCode(codeTokenNotAccepted)
	}
	return s.rotateAccountSecret(ctx, account, expectedID)
}
