// This file maps pending-fund pages without changing money disposition permissions.
package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/plugin/capability/filecap"
	"lina-plugin-idc-payment/backend/api/admin/v1"
	"lina-plugin-idc-payment/backend/internal/service/exceptionfund"
)

func (c *ControllerV1) ExceptionFundList(ctx context.Context, req *v1.ExceptionFundListReq) (*v1.ExceptionFundListRes, error) {
	page, err := c.exceptionFunds.List(ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	result := &v1.ExceptionFundListRes{Funds: make([]*v1.ExceptionFundItem, 0, len(page.Items)), Total: page.Total}
	for _, item := range page.Items {
		result.Funds = append(result.Funds, mapExceptionFund(item))
	}
	return result, nil
}
func (c *ControllerV1) ExceptionFundAllocate(ctx context.Context, req *v1.ExceptionFundAllocateReq) (*v1.ExceptionFundAllocateRes, error) {
	actor, err := c.currentActor(ctx)
	if err != nil {
		return nil, err
	}
	item, err := c.exceptionFunds.Allocate(ctx, req.ExceptionNo, req.TargetOrderNo, actor)
	if err != nil {
		return nil, err
	}
	return &v1.ExceptionFundAllocateRes{Fund: mapExceptionFund(item)}, nil
}
func (c *ControllerV1) ExceptionFundRefund(ctx context.Context, req *v1.ExceptionFundRefundReq) (*v1.ExceptionFundRefundRes, error) {
	actor, err := c.currentActor(ctx)
	if err != nil {
		return nil, err
	}
	item, err := c.exceptionFunds.RequestOriginalRefund(ctx, req.ExceptionNo, actor)
	if err != nil {
		return nil, err
	}
	return &v1.ExceptionFundRefundRes{Fund: mapExceptionFund(item)}, nil
}
func (c *ControllerV1) ExceptionFundManualRefundConfirm(ctx context.Context, req *v1.ExceptionFundManualRefundConfirmReq) (*v1.ExceptionFundManualRefundConfirmRes, error) {
	actor, err := c.currentActor(ctx)
	if err != nil {
		return nil, err
	}
	if c.files == nil || !strings.HasPrefix(req.ProofReference, "host-file:") {
		return nil, gerror.New("refund proof file is unavailable")
	}
	fileID := filecap.FileID(strings.TrimPrefix(req.ProofReference, "host-file:"))
	if err = c.files.EnsureVisible(ctx, []filecap.FileID{fileID}); err != nil {
		return nil, gerror.Wrap(err, "verify refund proof file")
	}
	item, err := c.exceptionFunds.ConfirmManualRefund(ctx, req.ExceptionNo, req.ProofReference, req.ProofDigest, actor, req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	return &v1.ExceptionFundManualRefundConfirmRes{Fund: mapExceptionFund(item)}, nil
}
func (c *ControllerV1) ExceptionFundProofUpload(ctx context.Context, req *v1.ExceptionFundProofUploadReq) (*v1.ExceptionFundProofUploadRes, error) {
	if c.files == nil {
		return nil, gerror.New("refund proof file service is unavailable")
	}
	item, err := c.exceptionFunds.Get(ctx, req.ExceptionNo)
	if err != nil || item == nil || item.ActionCode != "original_refund" || item.ActionStatus != "manual_pending" {
		return nil, gerror.New("manual original refund is not awaiting proof")
	}
	upload := g.RequestFromCtx(ctx).GetUploadFile("file")
	if upload == nil || upload.Filename == "" || upload.Size <= 0 {
		return nil, gerror.New("refund proof file is required")
	}
	opened, err := upload.Open()
	if err != nil {
		return nil, gerror.Wrap(err, "open refund proof upload")
	}
	defer opened.Close()
	hasher := sha256.New()
	stored, err := c.files.Upload(ctx, filecap.UploadInput{Filename: upload.Filename, BusinessScene: "other", Reader: io.TeeReader(opened, hasher), SizeBytes: upload.Size})
	if err != nil {
		return nil, err
	}
	if stored == nil || stored.ID == "" {
		return nil, gerror.New("refund proof upload returned no file")
	}
	return &v1.ExceptionFundProofUploadRes{ProofReference: "host-file:" + string(stored.ID), ProofDigest: hex.EncodeToString(hasher.Sum(nil)), FileName: stored.Name, SizeBytes: strconv.FormatInt(stored.SizeBytes, 10)}, nil
}
func mapExceptionFund(item *exceptionfund.Item) *v1.ExceptionFundItem {
	if item == nil {
		return nil
	}
	return &v1.ExceptionFundItem{ExceptionNo: item.ExceptionNo, AttemptNo: item.AttemptNo, OrderNo: item.OrderNo, ChannelNo: item.ChannelNo, ProviderCode: item.ProviderCode, CurrencyCode: item.CurrencyCode, AmountMinor: strconv.FormatInt(item.AmountMinor, 10), ReasonCode: item.ReasonCode, StatusCode: item.StatusCode, CreatedAt: item.CreatedAt.UnixMilli(), ActionNo: item.ActionNo, ActionCode: item.ActionCode, ActionStatus: item.ActionStatus, TargetOrderNo: item.TargetOrderNo, ProofReference: item.ProofReference, Version: item.Version}
}
