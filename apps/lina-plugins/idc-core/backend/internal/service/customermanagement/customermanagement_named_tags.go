// This file resolves at most twenty entered tag names with stable unique keys,
// one definition query and one bulk insert, reusing active shared definitions.
package customermanagement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// ensureNamedTags runs inside the caller's transaction. Both entry points use
// the same name-derived unique key, so concurrent inserts reuse one definition.
func ensureNamedTags(ctx context.Context, names []string, actor int) ([]string, error) {
	var (
		rows    []*entity.CustomerTag
		inserts []do.CustomerTag
		numbers = make([]string, 0, len(names))
		known   = make(map[string]string)
	)
	cols := dao.CustomerTag.Columns()
	if err := dao.CustomerTag.Ctx(ctx).Fields(cols.TagNo, cols.NameZhCn, cols.NameEnUs).
		Where(do.CustomerTag{TenantId: idccap.DefaultTenantID, StatusCode: segmentActive}).
		Where(dao.CustomerTag.Ctx(ctx).Builder().WhereIn(cols.NameZhCn, names).WhereOrIn(cols.NameEnUs, names)).OrderAsc(cols.Id).Scan(&rows); err != nil {
		return nil, err
	}
	for _, row := range rows {
		if known[row.NameZhCn] == "" {
			known[row.NameZhCn] = row.TagNo
		}
		if known[row.NameEnUs] == "" {
			known[row.NameEnUs] = row.TagNo
		}
	}
	for _, name := range names {
		number := known[name]
		if number == "" {
			sum := sha256.Sum256([]byte("customer-tag-name\x00" + name))
			suffix := hex.EncodeToString(sum[:16])
			number = "TAG_" + suffix
			inserts = append(inserts, do.CustomerTag{TenantId: idccap.DefaultTenantID, TagNo: number, TagCode: "named_" + suffix, NameZhCn: name, NameEnUs: name, ColorToken: "blue", StatusCode: segmentActive, Version: 1, CreatedBy: actor, UpdatedBy: actor})
		}
		numbers = append(numbers, number)
	}
	if len(inserts) > 0 {
		if _, err := dao.CustomerTag.Ctx(ctx).Data(inserts).OnConflict(cols.TagNo).InsertIgnore(); err != nil {
			return nil, err
		}
	}
	return numbers, nil
}
