package composition

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"lina-plugin-idc-core/backend/cap/secretcap"
	secretspi "lina-plugin-idc-core/backend/cap/secretcap/spi"
)

var (
	sourceIntegrationDeclarationOnce sync.Once
	sourceIntegrationDeclarationErr  error
	sourceIntegrationExpected        atomic.Pointer[captureSource]
)

func TestCoreDeclarationBuildsResourceSecretSource(t *testing.T) {
	expected := &captureSource{value: inertAccessor{}, manager: inertManager{}}
	sourceIntegrationExpected.Store(expected)
	sourceIntegrationDeclarationOnce.Do(func() {
		sourceIntegrationDeclarationErr = secretspi.Declare(func(context.Context) (secretcap.Source, error) {
			return sourceIntegrationExpected.Load(), nil
		})
	})
	if sourceIntegrationDeclarationErr != nil {
		t.Fatalf("declare test core secret source: %v", sourceIntegrationDeclarationErr)
	}
	source, err := LoadSecretSource(context.Background())
	if err != nil || source != expected {
		t.Fatalf("load declared core secret source: source=%T err=%v", source, err)
	}
}
