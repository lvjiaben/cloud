// This file gives catalog setting endpoints one atomic save/apply boundary.
// Permission checks remain in the individual owners and the catalog save owner.

package admin

import (
	"context"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func saveCatalogSettings[T any](ctx context.Context, publication catalog.PublicationService, versionNo string, save func(context.Context) (*T, error)) (*T, error) {
	var result *T
	err := publication.WithSavedSettings(ctx, versionNo, func(ctx context.Context) error {
		var saveErr error
		result, saveErr = save(ctx)
		return saveErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
