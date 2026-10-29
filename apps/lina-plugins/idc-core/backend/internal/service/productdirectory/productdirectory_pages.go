// This file persists explicit page ownership independently of products and customer entitlements.
package productdirectory

import (
	"context"
	"lina-core/pkg/bizerr"
)

const directoryPageTable = "plugin_idc_core_product_directory_page"

func (s *serviceImpl) attachPagePlugins(ctx context.Context, directories []*Directory) error {
	if len(directories) == 0 {
		return nil
	}
	nos := make([]any, 0, len(directories))
	positions := make(map[string]*Directory, len(directories))
	for _, item := range directories {
		item.PagePluginIDs = []string{}
		nos = append(nos, item.DirectoryNo)
		positions[item.DirectoryNo] = item
	}
	var rows []struct{ DirectoryNo, PluginID string }
	if err := s.database.GetScan(ctx, &rows, `SELECT directory_no,plugin_id FROM `+directoryPageTable+` WHERE tenant_id=0 AND directory_no IN (`+placeholders(len(nos))+`) ORDER BY sort_order,plugin_id`, nos...); err != nil {
		return err
	}
	for _, row := range rows {
		if item := positions[row.DirectoryNo]; item != nil {
			item.PagePluginIDs = append(item.PagePluginIDs, row.PluginID)
		}
	}
	return nil
}

func validatePagePlugins(input SaveInput) error {
	if input.PagePluginIDs == nil {
		return nil
	}
	ids := *input.PagePluginIDs
	if len(ids) > 16 || (input.NavigationType != NavigationProduct && len(ids) > 0) {
		return bizerr.NewCode(CodeInvalid)
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if len(id) > 128 || !providerPattern.MatchString(id) || seen[id] {
			return bizerr.NewCode(CodeInvalid)
		}
		seen[id] = true
	}
	return nil
}

func (s *serviceImpl) savePagePlugins(ctx context.Context, directoryNo string, ids []string) error {
	if _, err := s.database.Exec(ctx, `DELETE FROM `+directoryPageTable+` WHERE tenant_id=0 AND directory_no=?`, directoryNo); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	values, args := "", make([]any, 0, len(ids)*3)
	for index, id := range ids {
		if index > 0 {
			values += ","
		}
		values += "(0,?,?,?)"
		args = append(args, directoryNo, id, index)
	}
	_, err := s.database.Exec(ctx, `INSERT INTO `+directoryPageTable+` (tenant_id,directory_no,plugin_id,sort_order) VALUES `+values, args...)
	return err
}
