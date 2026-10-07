// This file keeps apidoc i18n catalog parsing helpers scoped to tests.

package apidoc

import (
	// Register the host-supported PostgreSQL driver for dynamic apidoc bundle tests.
	_ "lina-core/pkg/dbdriver"
	"lina-core/pkg/i18nresource"
)

// parseOpenAPIMessageCatalogJSON parses one apidoc bundle for focused tests.
func parseOpenAPIMessageCatalogJSON(content []byte) (map[string]string, error) {
	return i18nresource.ParseCatalog(content, i18nresource.ValueModeStringOnly)
}
