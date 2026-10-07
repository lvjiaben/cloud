package electroniccontracttemplate

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/contracttemplatecap"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type directorySource struct{ database gdb.DB }

var _ contracttemplatecap.Source = (*directorySource)(nil)

func NewDirectorySource(database gdb.DB) (contracttemplatecap.Source, error) {
	if database == nil {
		return nil, gerror.New("contract template directory requires database")
	}
	return &directorySource{database: database}, nil
}

func (s *directorySource) ListAvailable(
	ctx context.Context,
	selection contracttemplatecap.Selection,
) ([]contracttemplatecap.Template, error) {
	if err := contracttemplatecap.ValidateSelection(selection); err != nil {
		return nil, err
	}
	rows, err := s.query(ctx, selection, nil)
	if err != nil {
		return nil, err
	}
	if err = contracttemplatecap.ValidateTemplates(rows, selection); err != nil {
		return nil, gerror.Wrap(err, "validate contract template directory")
	}
	return rows, nil
}

func (s *directorySource) ResolveAvailable(
	ctx context.Context,
	selection contracttemplatecap.Selection,
	templateNos []string,
) ([]contracttemplatecap.Template, error) {
	if err := contracttemplatecap.ValidateResolveRequest(selection, templateNos); err != nil {
		return nil, err
	}
	rows, err := s.query(ctx, selection, templateNos)
	if err != nil {
		return nil, err
	}
	if err = contracttemplatecap.ValidateTemplates(rows, selection); err != nil {
		return nil, gerror.Wrap(err, "validate resolved contract templates")
	}
	if len(rows) != len(templateNos) {
		return nil, contracttemplatecap.ErrTemplateUnavailable
	}
	return rows, nil
}

func (s *directorySource) OrderContractExists(
	ctx context.Context,
	orderNo string,
) (bool, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return false, contracttemplatecap.ErrSourceUnavailable
	}
	if _, err := contracttemplatecap.OrderContractLockKey(orderNo); err != nil {
		return false, err
	}
	value, err := s.database.GetValue(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM plugin_idc_compliance_contract
    WHERE tenant_id = ? AND order_no = ?
)`, idccap.DefaultTenantID, orderNo)
	if err != nil {
		return false, gerror.Wrap(
			contracttemplatecap.ErrSourceUnavailable,
			"query order contract existence: "+err.Error(),
		)
	}
	return value.Bool(), nil
}

func (s *directorySource) query(
	ctx context.Context,
	selection contracttemplatecap.Selection,
	templateNos []string,
) ([]contracttemplatecap.Template, error) {
	query := `SELECT template_no, version_no, name, scope_code, signing_order_code, language_code
FROM plugin_idc_compliance_contract_template_version
WHERE tenant_id = ? AND status_code = 'enabled'`
	arguments := []any{idccap.DefaultTenantID}
	switch selection.Level {
	case contracttemplatecap.SelectionLevelProduct:
		query += ` AND scope_code = 'all_customers'`
	case contracttemplatecap.SelectionLevelEnterprise:
		query += ` AND scope_code IN ('enterprise', 'all_customers')`
	case contracttemplatecap.SelectionLevelOrder:
		scope := ScopePersonal
		if selection.CustomerType == contracttemplatecap.CustomerTypeEnterprise {
			scope = ScopeEnterprise
		}
		query += ` AND scope_code IN (?, 'all_customers')`
		arguments = append(arguments, scope)
	default:
		return nil, gerror.New("contract template directory selection is invalid")
	}
	if len(templateNos) > 0 {
		query += ` AND template_no IN(?)`
		arguments = append(arguments, templateNos)
	}
	query += ` ORDER BY updated_at DESC, id DESC LIMIT ?`
	arguments = append(arguments, contracttemplatecap.MaximumTemplates+1)
	rows := make([]contracttemplatecap.Template, 0)
	if err := s.database.GetScan(ctx, &rows, query, arguments...); err != nil {
		return nil, gerror.Wrap(
			contracttemplatecap.ErrSourceUnavailable,
			"query enabled contract template directory: "+err.Error(),
		)
	}
	if len(rows) > contracttemplatecap.MaximumTemplates {
		return nil, gerror.New("contract template directory exceeds limit")
	}
	return rows, nil
}
