package pluginhost

import (
	"fmt"
	"sync"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestBusinessExtensionProviderValidatorsAllowConcurrentRegistrationAndReads(t *testing.T) {
	declarations := NewDeclarations("concurrent-owner")
	definition, ok := declarations.(SourcePluginDefinition)
	if !ok {
		t.Fatal("source declarations do not expose their registered definition")
	}
	cleanup, err := RegisterSourcePluginForTest(declarations)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)

	const validatorCount = 64
	var operations sync.WaitGroup
	operations.Add(validatorCount * 2)
	for index := 0; index < validatorCount; index++ {
		index := index
		go func() {
			defer operations.Done()
			version := fmt.Sprintf("v%d", index+1)
			if err := declarations.BusinessExtensions().RegisterProviderValidator(
				capmodel.BusinessExtensionProductFulfillment,
				version,
				func([]*capmodel.BusinessExtensionDeclaration) error { return nil },
			); err != nil {
				t.Errorf("register %s: %v", version, err)
			}
		}()
	}

	for index := 0; index < validatorCount; index++ {
		index := index
		go func() {
			defer operations.Done()
			version := fmt.Sprintf("v%d", index+1)
			_, _ = definition.GetBusinessExtensionProviderValidator(
				capmodel.BusinessExtensionProductFulfillment,
				version,
			)
		}()
	}
	operations.Wait()

	for index := 0; index < validatorCount; index++ {
		version := fmt.Sprintf("v%d", index+1)
		if validator, exists := definition.GetBusinessExtensionProviderValidator(
			capmodel.BusinessExtensionProductFulfillment,
			version,
		); !exists || validator == nil {
			t.Fatalf("validator %s was lost during concurrent registration", version)
		}
	}
}
