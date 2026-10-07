package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func (c *ControllerV1) ServerTransferRequest(ctx context.Context,
	req *v1.ServerTransferRequestReq) (*v1.ServerTransferRequestRes, error) {
	result, err := c.serverOperations.RequestCurrentCustomerTransfer(ctx, serveroperation.TransferRequestCommand{
		ServiceNo: req.ServiceNo, RecipientEmail: req.RecipientEmail, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, serveroperation.PublicError(err)
	}
	return &v1.ServerTransferRequestRes{Accepted: result.Accepted, Reused: result.Reused}, nil
}

func (c *ControllerV1) ServerTransferIncomingList(ctx context.Context,
	_ *v1.ServerTransferIncomingListReq) (*v1.ServerTransferIncomingListRes, error) {
	items, err := c.serverOperations.ListCurrentCustomerIncomingTransfers(ctx)
	if err != nil {
		return nil, serveroperation.PublicError(err)
	}
	result := make([]*v1.IncomingServerTransfer, 0, len(items))
	for _, item := range items {
		result = append(result, &v1.IncomingServerTransfer{TransferNo: item.TransferNo,
			ServiceNo: item.ServiceNo, ExpiresAt: item.ExpiresAt.UTC().UnixMilli()})
	}
	return &v1.ServerTransferIncomingListRes{Items: result}, nil
}

func (c *ControllerV1) ServerTransferConfirmationCreate(ctx context.Context,
	req *v1.ServerTransferConfirmationCreateReq) (*v1.ServerTransferConfirmationCreateRes, error) {
	result, err := c.serverOperations.ConfirmCurrentCustomerTransfer(ctx, req.TransferNo, req.IdempotencyKey)
	if err != nil {
		return nil, serveroperation.PublicError(err)
	}
	return &v1.ServerTransferConfirmationCreateRes{TransferNo: result.TransferNo,
		ServiceNo: result.ServiceNo, Status: result.Status, Reused: result.Reused}, nil
}
