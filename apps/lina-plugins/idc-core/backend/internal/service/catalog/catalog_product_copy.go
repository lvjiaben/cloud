// This file defines the catalog owner's transactional product-copy contract.
// Copies are new unpublished products with zero stock; automation copies its
// own frozen binding and configuration inside the same admission transaction.

package catalog

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
)

// ProductCopyInput identifies one exact source version and the new sales name.
// SourceVersionNo must match SourceProductNo; PublicSlug must be unused.
type ProductCopyInput struct {
	// Revision creates an editable version on the same product; no level prices are copied.
	Revision        bool
	SourceProductNo string
	SourceVersionNo string
	PublicSlug      string
	NameZhCN        string
	NameEnUS        string
}

// ProductCopyWriter owns catalog facts only. The caller must hold the source
// automation admission transaction and copy automation facts before committing.
type ProductCopyWriter interface {
	// CommitProductCopy authorizes source visibility and every copied write,
	// locks source facts, and creates a zero-stock draft. Any failure rolls back.
	CommitProductCopy(context.Context, ProductCopyInput) (*ProductDraft, error)
}

// ProductCopyWriterFromManagement exposes the startup-owned writer without
// manufacturing a second service instance or leaking persistence models.
func ProductCopyWriterFromManagement(service ManagementService) (ProductCopyWriter, error) {
	writer, ok := service.(ProductCopyWriter)
	if !ok || writer == nil {
		return nil, gerror.New("catalog product copy writer is unavailable")
	}
	return writer, nil
}
