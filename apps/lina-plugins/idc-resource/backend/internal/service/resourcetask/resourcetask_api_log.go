// This file projects append-only invocation metadata with the same merchant scope as tasks.
package resourcetask

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-resource/backend/internal/model/do"
	"lina-plugin-idc-resource/backend/internal/model/entity"
	"strings"
)

// APILogListInput bounds the merchant-local invocation history query.
type APILogListInput struct {
	Page, Limit                                int
	ChannelNo, ChannelName, MethodCode, Result string
}

// APILogItem excludes request/response bodies and all credentials.
type APILogItem struct {
	entity.ProviderApiLog
	ChannelName string `orm:"channel_name"`
}

// APILogPage contains one database page and its exact filtered count.
type APILogPage struct {
	Items []APILogItem
	Total int
}

// ListAPILogs uses two fixed local queries, regardless of page size.
func (s *managementService) ListAPILogs(ctx context.Context, input APILogListInput) (*APILogPage, error) {
	if _, err := s.authorize(ctx, "idc:admin:resource:api-log:list", "provider-api-log-list"); err != nil {
		return nil, err
	}
	input.ChannelNo = strings.TrimSpace(input.ChannelNo)
	input.MethodCode = strings.TrimSpace(input.MethodCode)
	if input.Page < 1 || input.Page > 1000000 || input.Limit < 1 || input.Limit > 100 || len(input.ChannelName) > 128 || len(input.ChannelNo) > 64 || len(input.MethodCode) > 96 || (input.Result != "" && input.Result != "succeeded" && input.Result != "failed") {
		return nil, bizerr.NewCode(CodeTaskManagementInvalid)
	}
	model := s.store.database.Model(do.ProviderApiLog{}).Ctx(ctx).As("log").LeftJoin("plugin_idc_resource_channel channel", "channel.tenant_id=log.tenant_id AND channel.channel_no=log.channel_no").Where("log.tenant_id", 0)
	if input.ChannelName != "" {
		model = model.Where("channel.display_name", strings.TrimSpace(input.ChannelName))
	}
	if input.ChannelNo != "" {
		model = model.Where("log.channel_no", input.ChannelNo)
	}
	if input.MethodCode != "" {
		model = model.Where("log.method_code", input.MethodCode)
	}
	if input.Result != "" {
		model = model.Where("log.succeeded", input.Result == "succeeded")
	}
	total, err := model.Clone().Count()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeTaskManagementUnavailable)
	}
	items := make([]APILogItem, 0, input.Limit)
	err = model.Fields("log.id,log.channel_no,channel.display_name AS channel_name,log.provider_plugin_id,log.method_code,log.succeeded,log.error_code,log.duration_ms,log.started_at,log.created_at").OrderDesc("log.id").Page(input.Page, input.Limit).Scan(&items)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeTaskManagementUnavailable)
	}
	return &APILogPage{Items: items, Total: total}, nil
}
