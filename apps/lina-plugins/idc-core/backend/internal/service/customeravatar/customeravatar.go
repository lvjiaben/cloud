// Package customeravatar uploads validated images for the current active customer through public plugin storage.
package customeravatar

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"io"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

// Service provides customer-owned avatar uploads without granting file-center permissions.
type Service interface {
	// Upload validates a PNG, JPEG or WebP reader (at most 5 MiB and 16 million pixels), stores it for the current customer and returns its URL. Invalid content, inaccessible actors and storage errors fail closed.
	Upload(ctx context.Context, content io.Reader) (string, error)
	// Open returns a public avatar addressed only by its cryptographically random key; invalid or missing keys are inaccessible. The caller must close the returned body.
	Open(ctx context.Context, key string) (*storagecap.GetOutput, error)
}

type serviceImpl struct {
	access  *customer.AccessGuard
	storage storagecap.Service
}

var _ Service = (*serviceImpl)(nil)

// New receives the shared customer guard and plugin-scoped storage capability from plugin startup.
func New(access *customer.AccessGuard, storage storagecap.Service) (Service, error) {
	if access == nil || storage == nil {
		return nil, gerror.New("customer avatar service requires customer access and storage")
	}
	return &serviceImpl{access: access, storage: storage}, nil
}
