package providercapacity

import (
	"context"
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type typedNilProviderConstructor struct{}

func (*typedNilProviderConstructor) Construct(
	context.Context,
	string,
) (resourcecap.Provider, error) {
	return nil, nil
}

func TestNilProviderConstructorValidation(t *testing.T) {
	t.Parallel()
	var constructor *typedNilProviderConstructor
	if !isNilProviderConstructor(constructor) {
		t.Fatal("typed-nil Provider constructor was accepted")
	}
	if isNilProviderConstructor(&typedNilProviderConstructor{}) {
		t.Fatal("non-nil Provider constructor was rejected")
	}
}
