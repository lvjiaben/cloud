package managedparameters

import (
	"context"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"testing"
)

type repeatableExtensionRegistrar struct{}

func (repeatableExtensionRegistrar) ListProviders(context.Context, plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	return nil, nil
}
func (repeatableExtensionRegistrar) GetProviderExact(context.Context, plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, error) {
	return nil, nil
}
func (repeatableExtensionRegistrar) GetProvidersExact(context.Context, []plugincap.BusinessExtensionProviderExactInput) ([]*capmodel.BusinessExtensionProviderEvidence, error) {
	return nil, nil
}
func (repeatableExtensionRegistrar) WithNewBusinessAdmission(context.Context, pluginhost.BusinessExtensionAdmissionInput, capmodel.BusinessExtensionAdmissionAction) error {
	return nil
}
func (repeatableExtensionRegistrar) InvokeProviderMethod(context.Context, pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	return nil, nil
}
func (repeatableExtensionRegistrar) WithConfidentialOutboundContext(context.Context, pluginhost.ConfidentialOutboundContextInput, pluginhost.ConfidentialOutboundAction) error {
	return nil
}

func TestLazyBuilderBindIsRepeatableForJobRediscovery(t *testing.T) {
	builder := NewLazy()
	registrar := repeatableExtensionRegistrar{}
	database, err := gdb.New(gdb.ConfigNode{Link: "pgsql:test:test@tcp(127.0.0.1:1)/test"})
	if err != nil {
		t.Fatal(err)
	}
	if err = builder.Bind(database, registrar); err != nil {
		t.Fatal(err)
	}
	if err = builder.Bind(database, registrar); err != nil {
		t.Fatalf("repeat job discovery could not refresh managed parameter dependencies: %v", err)
	}
}
