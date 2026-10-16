// This file verifies permission-first import ordering, local replay before any
// remote read, failure propagation and one atomic owner callback.

package catalogdraft

import (
	"context"
	"errors"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
	"testing"
)

type importTestAuthorizer struct {
	calls                int
	denied               string
	denyAfterRead        bool
	changeActorAfterRead bool
}

func (a *importTestAuthorizer) Authorize(_ context.Context, input managementcap.AuthorizationRequest) (int, error) {
	a.calls++
	if input.Permission == a.denied || (a.denyAfterRead && a.calls > 4) {
		return 0, errors.New("denied")
	}
	if a.changeActorAfterRead && a.calls > 4 {
		return 8, nil
	}
	return 7, nil
}

func TestImportReauthorizesBeforeCommitAfterRemotePreparation(t *testing.T) {
	t.Parallel()
	for _, authorizer := range []*importTestAuthorizer{{denyAfterRead: true}, {changeActorAfterRead: true}} {
		writer := &importTestWriter{}
		source := &importTestSource{}
		automation := &importTestAutomation{}
		service, err := New(authorizer, source, writer, automation)
		if err != nil {
			t.Fatal(err)
		}
		_, err = service.Import(context.Background(), catalog.ImportInput{Reference: "101"})
		if !bizerr.Is(err, catalog.CodeManagementNotAccessible) || source.calls != 1 || automation.prepareCalls != 1 || writer.commitCalls != 0 {
			t.Fatalf("changed permission or actor crossed write boundary: %v", err)
		}
	}
}

type importTestWriter struct {
	lookupCalls, commitCalls int
	previous                 *catalog.ImportResult
	err                      error
}

func (w *importTestWriter) LookupImport(context.Context, catalog.ImportInput) (*catalog.ImportResult, error) {
	w.lookupCalls++
	return w.previous, w.err
}
func (w *importTestWriter) CommitImport(context.Context, catalog.ImportInput, *resourceinterfaceprojectioncap.CatalogDetail, []catalog.ImportMissingConfiguration) (*catalog.ImportResult, bool, error) {
	w.commitCalls++
	return &catalog.ImportResult{Item: catalog.ProductDraft{VersionNo: "created"}}, true, w.err
}

type importTestSource struct {
	calls int
	err   error
}

func (s *importTestSource) ListCatalog(context.Context, string, resourceinterfaceprojectioncap.CatalogQuery) (*resourceinterfaceprojectioncap.CatalogPage, error) {
	return nil, errors.New("unexpected list")
}
func (s *importTestSource) ReadCatalogProduct(context.Context, string, resourceinterfaceprojectioncap.CatalogProductQuery) (*resourceinterfaceprojectioncap.CatalogDetail, error) {
	s.calls++
	return &resourceinterfaceprojectioncap.CatalogDetail{Product: resourceinterfaceprojectioncap.CatalogProduct{Reference: "101", Name: "Supplier"}}, s.err
}

type importTestAutomation struct {
	prepareCalls, commitCalls int
	err                       error
}

func (a *importTestAutomation) PrepareImportBinding(context.Context, string, *resourceinterfaceprojectioncap.CatalogReadEvidence, []resourceinterfaceprojectioncap.CatalogConfigurationValue, *channelcommercecap.ImportSnapshot) (*productautomation.ImportBinding, error) {
	a.prepareCalls++
	return &productautomation.ImportBinding{}, a.err
}
func (a *importTestAutomation) WithImportBinding(ctx context.Context, _ *productautomation.ImportBinding, action func(context.Context) (string, error)) error {
	a.commitCalls++
	_, err := action(ctx)
	return err
}

func TestImportAuthorizesBeforeAnyReadAndReplaysLocally(t *testing.T) {
	t.Parallel()
	for _, permission := range []string{importProductCreatePermission, productautomation.PermissionBindingWrite, productautomation.PermissionInterfaceView, importResourceInterfacePermission} {
		authorizer := &importTestAuthorizer{denied: permission}
		writer := &importTestWriter{}
		source := &importTestSource{}
		automation := &importTestAutomation{}
		service, err := New(authorizer, source, writer, automation)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = service.Import(context.Background(), catalog.ImportInput{}); !bizerr.Is(err, catalog.CodeManagementNotAccessible) || writer.lookupCalls != 0 || source.calls != 0 || automation.prepareCalls != 0 {
			t.Fatalf("permission %s crossed boundary: %v", permission, err)
		}
	}
	previous := &catalog.ImportResult{Item: catalog.ProductDraft{ProductNo: "original"}}
	writer := &importTestWriter{previous: previous}
	source := &importTestSource{err: errors.New("offline")}
	automation := &importTestAutomation{}
	service, err := New(&importTestAuthorizer{}, source, writer, automation)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Import(context.Background(), catalog.ImportInput{Reference: "101"})
	if err != nil || result != previous || source.calls != 0 || automation.prepareCalls != 0 || writer.commitCalls != 0 {
		t.Fatalf("replay required remote: result=%+v err=%v", result, err)
	}
}

func TestImportRemoteFailureAndSuccessfulAtomicCallback(t *testing.T) {
	t.Parallel()
	for _, fail := range []bool{true, false} {
		writer := &importTestWriter{}
		source := &importTestSource{}
		automation := &importTestAutomation{}
		if fail {
			source.err = errors.New("upstream timeout")
		}
		service, err := New(&importTestAuthorizer{}, source, writer, automation)
		if err != nil {
			t.Fatal(err)
		}
		result, err := service.Import(context.Background(), catalog.ImportInput{Reference: "101"})
		if fail {
			if err == nil || writer.commitCalls != 0 || automation.prepareCalls != 0 {
				t.Fatal("remote failure produced local writes")
			}
			continue
		}
		if err != nil || result == nil || writer.commitCalls != 1 || automation.commitCalls != 1 || source.calls != 1 {
			t.Fatalf("non-atomic workflow result=%+v err=%v", result, err)
		}
	}
}
