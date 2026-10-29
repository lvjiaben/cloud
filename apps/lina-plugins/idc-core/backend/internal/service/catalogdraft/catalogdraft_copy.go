// This file coordinates a catalog draft copy through the already-created
// catalog and automation owners. No upstream purchase or fulfillment runs.

package catalogdraft

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

// CopyService exposes only local draft copying under exact staff permissions.
type CopyService interface {
	// Copy atomically copies commercial and automation settings, returning a
	// new unsaleable draft with zero stock; any owner failure rolls back all rows.
	Copy(context.Context, catalog.ProductCopyInput) (*catalog.ProductDraft, error)
}

type copyService struct {
	catalog    catalog.ProductCopyWriter
	automation productautomation.ProductCopyService
	prices     salesmanagement.ProductPriceCopyService
}

// NewCopy reuses startup instances; it neither creates service graphs nor
// weakens permission checks owned by catalog and product automation.
func NewCopy(writer catalog.ProductCopyWriter, automation productautomation.ProductCopyService, prices salesmanagement.ProductPriceCopyService) (CopyService, error) {
	if writer == nil || automation == nil || prices == nil {
		return nil, gerror.New("catalog copy runtime dependencies are required")
	}
	return &copyService{catalog: writer, automation: automation, prices: prices}, nil
}

func (s *copyService) Copy(ctx context.Context, input catalog.ProductCopyInput) (*catalog.ProductDraft, error) {
	var result *catalog.ProductDraft
	err := s.automation.WithProductCopy(ctx, input.SourceVersionNo, input.Revision, func(txCtx context.Context) (string, error) {
		var writeErr error
		result, writeErr = s.catalog.CommitProductCopy(txCtx, input)
		if writeErr != nil {
			return "", writeErr
		}
		if result == nil {
			return "", gerror.New("catalog copy owner returned no draft")
		}
		if !input.Revision {
			if writeErr = s.prices.CopyLevelProductPrices(txCtx, input.SourceProductNo, result.ProductNo); writeErr != nil {
				return "", writeErr
			}
		}
		return result.VersionNo, nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
