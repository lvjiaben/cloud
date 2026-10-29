// Package catalogdraft coordinates explicit staff imports using catalog-owned
// writes and product-automation admission. Supplier reads never run in its owner
// transaction, and successful request retries need no upstream availability.
package catalogdraft

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

// Service imports only unpublished drafts under exact direct-staff permissions.
type Service interface {
	// Import returns the original identity for an unchanged actor-scoped retry,
	// rejects changed-key content, and rolls back every local write on failure.
	Import(context.Context, catalog.ImportInput) (*catalog.ImportResult, error)
	// ImportExisting prepares remote data outside the local transaction, then atomically replaces and applies the exact editable version. Failed or incomplete supplier contracts leave the product untouched.
	ImportExisting(context.Context, catalog.ImportInput, catalog.PublicationService, productautomation.Service, string) (*catalog.ImportResult, error)
}

type serviceImpl struct {
	authorizer managementcap.Authorizer
	source     resourceinterfaceprojectioncap.CatalogSource
	catalog    catalog.ImportWriter
	automation productautomation.ImportBindingService
}

var _ Service = (*serviceImpl)(nil)

// New accepts an optional source for installations without the resource owner;
// imports fail closed there, while other catalog functions remain available.
func New(authorizer managementcap.Authorizer, source resourceinterfaceprojectioncap.CatalogSource, writer catalog.ImportWriter, automation productautomation.ImportBindingService) (Service, error) {
	if authorizer == nil || writer == nil || automation == nil {
		return nil, gerror.New("catalog import runtime dependencies are required")
	}
	return &serviceImpl{authorizer: authorizer, source: source, catalog: writer, automation: automation}, nil
}
