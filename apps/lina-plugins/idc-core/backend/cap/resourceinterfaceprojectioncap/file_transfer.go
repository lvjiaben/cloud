// This file defines the source-only, governed transfer of one immutable local
// configuration file to the upstream customer account bound to an interface.
package resourceinterfaceprojectioncap

import (
	"context"
	"io"
)

type PurchaseFileTransfer struct {
	PublicSlug, ConfigurationRevision, FieldToken string
	FileName, MediaType, SHA256Hex                string
	SizeBytes                                     int64
	Body                                          io.Reader
}

// PurchaseFileTransferSource is optional on an existing-business outbound
// source. File bytes and credentials never pass through the guest JSON wire.
type PurchaseFileTransferSource interface {
	TransferPurchaseFile(context.Context, ExistingBusinessOutboundRequest, PurchaseFileTransfer) (string, error)
}
