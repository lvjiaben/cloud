package admin

import "strconv"

func parseCatalogAmount(raw string) (int64, error) {
	return strconv.ParseInt(raw, 10, 64)
}

func parseOptionalCatalogAmount(raw *string) (*int64, error) {
	if raw == nil {
		return nil, nil
	}
	value, err := parseCatalogAmount(*raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func formatCatalogAmount(value int64) string {
	return strconv.FormatInt(value, 10)
}

func formatOptionalCatalogAmount(value *int64) *string {
	if value == nil {
		return nil
	}
	formatted := formatCatalogAmount(*value)
	return &formatted
}
