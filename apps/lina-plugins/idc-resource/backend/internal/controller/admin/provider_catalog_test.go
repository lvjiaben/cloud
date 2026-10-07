// These tests keep the HTTP adapter thin: exact targets and bounded queries
// reach the injected owner source, and source failures never become empty success.
package admin

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/api/admin/v1"
)

type catalogSourceStub struct {
	interfaceNo string
	listQuery   resourceinterfaceprojectioncap.CatalogQuery
	detailQuery resourceinterfaceprojectioncap.CatalogProductQuery
	page        *resourceinterfaceprojectioncap.CatalogPage
	detail      *resourceinterfaceprojectioncap.CatalogDetail
	err         error
}

func (s *catalogSourceStub) ListCatalog(_ context.Context, interfaceNo string, query resourceinterfaceprojectioncap.CatalogQuery) (*resourceinterfaceprojectioncap.CatalogPage, error) {
	s.interfaceNo, s.listQuery = interfaceNo, query
	return s.page, s.err
}

func (s *catalogSourceStub) ReadCatalogProduct(_ context.Context, interfaceNo string, query resourceinterfaceprojectioncap.CatalogProductQuery) (*resourceinterfaceprojectioncap.CatalogDetail, error) {
	s.interfaceNo, s.detailQuery = interfaceNo, query
	return s.detail, s.err
}

func TestProviderCatalogAdaptersPreserveExactTargetAndProjection(t *testing.T) {
	t.Parallel()
	const interfaceNo = "pfi_11111111111111111111111111111111"
	source := &catalogSourceStub{
		page: &resourceinterfaceprojectioncap.CatalogPage{
			Items: []resourceinterfaceprojectioncap.CatalogProduct{{Reference: "101", Name: "Cloud"}}, Total: 21,
		},
		detail: &resourceinterfaceprojectioncap.CatalogDetail{
			Product: resourceinterfaceprojectioncap.CatalogProduct{Reference: "101", Name: "Cloud"},
		},
	}
	controller := &ControllerV1{catalog: source}
	list, err := controller.ProviderCatalogList(t.Context(), &v1.ProviderCatalogListReq{
		InterfaceNo: interfaceNo, Page: 3, PageSize: 10, Keyword: "Cloud",
	})
	if err != nil || list.Total != 21 || len(list.Items) != 1 || list.Items[0].Reference != "101" {
		t.Fatalf("list projection changed: %+v, %v", list, err)
	}
	if source.interfaceNo != interfaceNo || source.listQuery != (resourceinterfaceprojectioncap.CatalogQuery{Page: 3, PageSize: 10, Keyword: "Cloud"}) {
		t.Fatalf("list scope changed: %+v", source)
	}
	detail, err := controller.ProviderCatalogProduct(t.Context(), &v1.ProviderCatalogProductReq{InterfaceNo: interfaceNo, Reference: "101"})
	if err != nil || detail.Item != source.detail || source.interfaceNo != interfaceNo || source.detailQuery.Reference != "101" {
		t.Fatalf("detail target or projection changed: %+v, %v", detail, err)
	}
}

func TestProviderCatalogAdaptersPreserveOwnerFailure(t *testing.T) {
	t.Parallel()
	source := &catalogSourceStub{err: gerror.New("owner denied")}
	controller := &ControllerV1{catalog: source}
	list, err := controller.ProviderCatalogList(t.Context(), &v1.ProviderCatalogListReq{})
	if list != nil || err != source.err {
		t.Fatal("list owner failure became a successful empty response")
	}
	detail, err := controller.ProviderCatalogProduct(t.Context(), &v1.ProviderCatalogProductReq{})
	if detail != nil || err != source.err {
		t.Fatal("detail owner failure became a successful empty response")
	}
}
