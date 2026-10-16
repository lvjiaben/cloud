// This file exposes customer-bound upload operations to the authenticated
// channel adapter. The customer identity is supplied only by the owner caller.
package channelcommercecap

import (
	"context"
	"io"
)

type PurchaseFileInput struct {
	PublicSlug, ConfigurationRevision, FieldToken, FileName string
	SizeBytes                                               int64
}

type PurchaseFileReservation struct {
	UploadToken string `json:"uploadToken"`
	ExpiresAt   int64  `json:"expiresAt"`
}
type PurchaseFile struct {
	FileToken string `json:"fileToken"`
	FileName  string `json:"fileName"`
	MediaType string `json:"mediaType"`
	Status    string `json:"status"`
	SizeBytes int64  `json:"sizeBytes"`
	ExpiresAt int64  `json:"expiresAt"`
}

type PurchaseFileRuntime interface {
	ReservePurchaseFile(context.Context, string, PurchaseFileInput) (*PurchaseFileReservation, error)
	UploadPurchaseFile(context.Context, string, string, string, io.Reader) (*PurchaseFile, error)
}
