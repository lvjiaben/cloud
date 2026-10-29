//go:build legacy_server_catalog_tests

// This file keeps the historical controller fixture aligned with the reader contract.
package admin

import (
	"context"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (s *catalogProductManagementReaderStub) PreviewProductPrice(context.Context, catalog.ProductEditorPricePreviewInput) (*catalog.ProductEditorPricePreview, error) {
	return nil, nil
}
