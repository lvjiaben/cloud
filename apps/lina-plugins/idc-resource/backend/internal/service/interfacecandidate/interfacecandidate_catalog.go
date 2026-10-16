// This file adds a staff-only sales projection to the resource owner's existing
// confidential read path. It cannot create drafts or bypass interface scope.
package interfacecandidate

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"

	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/interfaceprojection"
)

const catalogReadPermission = "idc:admin:resource:configuration:list"

type catalogSource struct {
	reader     *LazySource
	authorizer managementcap.Authorizer
	projection *interfaceprojection.Worker
}

// NewCatalogSource binds the direct-staff authorizer supplied at startup. The
// shared runtime may be bound later; all reads fail closed until it is ready.
func NewCatalogSource(reader *LazySource, authorizer managementcap.Authorizer, projection *interfaceprojection.Worker) (resourceinterfaceprojectioncap.CatalogSource, error) {
	if reader == nil || authorizer == nil || projection == nil {
		return nil, gerror.New("upstream catalog source dependencies are required")
	}
	return &catalogSource{reader: reader, authorizer: authorizer, projection: projection}, nil
}

func (s *catalogSource) ListCatalog(ctx context.Context, interfaceNo string, query resourceinterfaceprojectioncap.CatalogQuery) (*resourceinterfaceprojectioncap.CatalogPage, error) {
	if query.Validate() != nil {
		return nil, bizerr.NewCode(CodeCatalogInvalid)
	}
	var result resourceinterfaceprojectioncap.CatalogPage
	if _, err := s.read(ctx, interfaceNo, resourcecap.DynamicProviderMethodListCatalog, query, &result); err != nil {
		return nil, err
	}
	if err := result.Validate(query); err != nil {
		return nil, bizerr.WrapCode(err, CodeCatalogUnavailable)
	}
	return &result, nil
}

func (s *catalogSource) ReadCatalogProduct(ctx context.Context, interfaceNo string, query resourceinterfaceprojectioncap.CatalogProductQuery) (*resourceinterfaceprojectioncap.CatalogDetail, error) {
	if query.Validate() != nil {
		return nil, bizerr.NewCode(CodeCatalogInvalid)
	}
	var result resourceinterfaceprojectioncap.CatalogDetail
	evidence, err := s.read(ctx, interfaceNo, resourcecap.DynamicProviderMethodReadCatalogProduct, query, &result)
	if err != nil {
		return nil, err
	}
	result.ReadEvidence = evidence
	if err := result.Validate(query); err != nil {
		return nil, bizerr.WrapCode(err, CodeCatalogUnavailable)
	}
	return &result, nil
}

func (s *catalogSource) read(ctx context.Context, interfaceNo, method string, query, target any) (*resourceinterfaceprojectioncap.CatalogReadEvidence, error) {
	if ctx == nil || ctx.Err() != nil || s == nil || s.authorizer == nil {
		return nil, bizerr.NewCode(CodeCatalogNotAccessible)
	}
	payload, err := json.Marshal(query)
	if err != nil {
		return nil, bizerr.NewCode(CodeCatalogInvalid)
	}
	request := resourceinterfaceprojectioncap.CandidateRequest{InterfaceInstanceNo: interfaceNo, Method: method, Values: payload}
	if resourceinterfaceprojectioncap.ValidateCandidateRequest(request) != nil {
		return nil, bizerr.NewCode(CodeCatalogInvalid)
	}
	actor, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{Permission: catalogReadPermission, TargetNo: interfaceNo})
	if err != nil || actor <= 0 {
		return nil, bizerr.NewCode(CodeCatalogNotAccessible)
	}
	// This allowlist belongs to the confidential HTTP envelope, not the guest's
	// final sales projection; that projection is independently decoded below.
	response, evidence, err := s.reader.invokeRead(ctx, request, []string{"StatusCode", "Data", "Authenticated"})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeCatalogUnavailable)
	}
	if err = decodeCatalogResponse(response, target); err != nil {
		return nil, bizerr.WrapCode(err, CodeCatalogUnavailable)
	}
	if evidence == nil || evidence.Validate() != nil || evidence.InterfaceInstanceNo != interfaceNo {
		return nil, bizerr.NewCode(CodeCatalogUnavailable)
	}
	if err = s.projection.Ensure(ctx, interfaceNo, evidence.InterfaceConfigurationRevision); err != nil {
		return nil, bizerr.WrapCode(err, CodeCatalogProjectionPending)
	}
	return evidence, nil
}

func decodeCatalogResponse(response []byte, target any) error {
	if len(response) == 0 || len(response) > 1<<20 {
		return gerror.New("upstream catalog response is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(response))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil || decoder.Decode(new(any)) != io.EOF {
		return gerror.New("upstream catalog response is invalid")
	}
	return nil
}
