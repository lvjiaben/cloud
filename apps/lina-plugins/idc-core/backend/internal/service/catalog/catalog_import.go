// This file defines the catalog-owned import command and immutable result.
// Imports reuse ordinary unpublished draft defaults and never call publication.

package catalog

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/managementcap"

	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

// ImportPricingMode distinguishes a percentage uplift from an explicit final price.
type ImportPricingMode string

const (
	ImportPricingMarkup ImportPricingMode = "markup"
	ImportPricingManual ImportPricingMode = "manual"
)

// ImportInput is one explicit staff submission. Reuse RequestKey only for retrying
// unchanged content; another explicit import uses a new key and creates a product.
type ImportInput struct {
	FulfillmentStrategyCode string                                        `json:"fulfillmentStrategyCode,omitempty"`
	TargetVersionNo         string                                        `json:"targetVersionNo,omitempty"`
	RequestKey              string                                        `json:"requestKey"`
	InterfaceInstanceNo     string                                        `json:"interfaceInstanceNo"`
	Reference               string                                        `json:"reference"`
	CategoryNo              string                                        `json:"categoryNo"`
	NameZhCN                string                                        `json:"nameZhCn"`
	NameEnUS                string                                        `json:"nameEnUs"`
	PricingMode             ImportPricingMode                             `json:"pricingMode"`
	MarkupPercent           string                                        `json:"markupPercent"`
	Prices                  []resourceinterfaceprojectioncap.CatalogPrice `json:"prices"`
}

// ImportMissingConfiguration is the initial, bilingual unfilled-field projection.
type ImportMissingConfiguration struct {
	FieldKey string `json:"fieldKey"`
	NameZhCN string `json:"nameZhCn"`
	NameEnUS string `json:"nameEnUs"`
}

// ImportResult records the created identity and initial readiness, not permission
// to publish or proof of a complete machine quote.
type ImportResult struct {
	PricingOnly          bool `json:"-"`
	Item                 ProductDraft
	MissingConfiguration []ImportMissingConfiguration
	PricingIncomplete    bool
}

// ImportWriter owns local request replay and atomic product/price persistence.
type ImportWriter interface {
	// LookupImport authorizes product creation and returns the original result
	// for an identical actor-scoped request, nil if absent, or conflict on reuse.
	LookupImport(context.Context, ImportInput) (*ImportResult, error)
	// CommitImport requires an existing owner transaction. It claims the request
	// key, creates only a new draft and returns created=false on a concurrent replay.
	CommitImport(context.Context, ImportInput, *resourceinterfaceprojectioncap.CatalogDetail, []ImportMissingConfiguration) (*ImportResult, bool, error)
}

type importAuthorizer struct{ management *managementAuthorizerImpl }

// NewImportAuthorizer reuses the ordinary direct-staff and supported data-scope
// checks with exact, cataloged cross-owner permissions. It shares startup inputs.
func NewImportAuthorizer(bizCtx bizctxcap.Service, reader principal.Reader, authorization authz.Service) (managementcap.Authorizer, error) {
	if nilManagementDependency(bizCtx) || nilManagementDependency(reader) || nilManagementDependency(authorization) {
		return nil, gerror.New("catalog import authorizer dependencies are required")
	}
	return &importAuthorizer{management: &managementAuthorizerImpl{bizCtxSvc: bizCtx, principal: reader, authorization: authorization}}, nil
}

func (a *importAuthorizer) Authorize(ctx context.Context, request managementcap.AuthorizationRequest) (int, error) {
	if a == nil || a.management == nil || managementcap.ValidateAuthorizationRequest(request) != nil {
		return 0, bizerr.NewCode(CodeManagementNotAccessible)
	}
	return a.management.authorizePermission(ctx, request.Permission, CodeManagementNotAccessible)
}
