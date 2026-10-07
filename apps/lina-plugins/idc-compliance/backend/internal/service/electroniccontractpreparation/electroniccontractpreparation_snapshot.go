package electroniccontractpreparation

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/contractcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

func (s *serviceImpl) projectPersisted(
	ctx context.Context,
	row *contractRow,
	reused bool,
) (*PreparedContract, error) {
	if err := validateStoredRow(row); err != nil {
		return nil, err
	}
	handle, err := secretcap.NewHandle(row.SnapshotKeyRef)
	if err != nil {
		return nil, gerror.New("prepared contract snapshot reference is invalid")
	}
	expected := snapshotDigests{
		Variables: row.VariableSnapshotSHA256,
		Signers:   row.SignerSnapshotSHA256,
		Order:     row.OrderSnapshotSHA256,
	}
	var prepared *PreparedContract
	err = s.withSnapshotBundle(
		ctx, handle, "compliance.contract-preview", &expected,
		func(bundle *snapshotBundle) error {
			if err := validateBundleRow(bundle, row); err != nil {
				return err
			}
			prepared = projectPrepared(bundle, reused)
			prepared.Version = row.Version
			return nil
		},
	)
	if err != nil {
		if prepared != nil {
			prepared.Clear()
		}
		return nil, err
	}
	if prepared == nil {
		return nil, gerror.New("prepared contract preview is unavailable")
	}
	return prepared, nil
}

func (s *serviceImpl) MatchCustomerSigner(
	ctx context.Context,
	contractNo string,
	observed *contractcap.ObservedCustomerSigner,
) error {
	if observed != nil {
		defer observed.Clear()
	}
	contractNo = strings.TrimSpace(contractNo)
	if s == nil || ctx == nil || ctx.Err() != nil ||
		!validBusinessNo(contractNo, 64) || observed == nil {
		return gerror.New("contract signer matching input is invalid")
	}
	row, err := s.loadByContractNo(ctx, contractNo)
	if err != nil {
		return err
	}
	if err = validateStoredRow(row); err != nil {
		return err
	}
	currentValue, err := s.database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
	if err != nil {
		return gerror.Wrap(err, "load contract signer validation time")
	}
	now := currentValue.Time().UTC()
	if row.StatusCode != "pending_launch" || !now.Before(row.SigningExpiresAt) {
		return gerror.New("contract is not eligible to launch signing")
	}
	handle, err := secretcap.NewHandle(row.SnapshotKeyRef)
	if err != nil {
		return gerror.New("contract signer snapshot reference is invalid")
	}
	expected := snapshotDigests{
		Variables: row.VariableSnapshotSHA256,
		Signers:   row.SignerSnapshotSHA256,
		Order:     row.OrderSnapshotSHA256,
	}
	return s.withSnapshotBundle(
		ctx, handle, "compliance.contract-signer-match", &expected,
		func(bundle *snapshotBundle) error {
			if err := validateBundleRow(bundle, row); err != nil {
				return err
			}
			return s.runtime.Orders.MatchCustomerSigner(ctx, bundle.Order.Customer, observed)
		},
	)
}

func (s *serviceImpl) withSnapshotBundle(
	ctx context.Context,
	handle secretcap.Handle,
	purposeCode string,
	expected *snapshotDigests,
	consume func(*snapshotBundle) error,
) error {
	if s == nil || ctx == nil || ctx.Err() != nil || consume == nil {
		return gerror.New("contract snapshot access input is invalid")
	}
	purpose, err := secretcap.NewPurpose(purposeCode)
	if err != nil {
		return err
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return err
	}
	return s.accessor.Access(ctx, request, func(payload []byte) error {
		var bundle *snapshotBundle
		var decodeErr error
		if expected == nil {
			bundle, decodeErr = decodeSnapshotBundlePayload(payload)
		} else {
			bundle, decodeErr = decodeSnapshotBundle(payload, *expected)
		}
		if decodeErr != nil {
			return decodeErr
		}
		defer bundle.Clear()
		return consume(bundle)
	})
}

func (s *serviceImpl) loadByContractNo(ctx context.Context, contractNo string) (*contractRow, error) {
	var rows []*contractRow
	if err := s.database.GetScan(ctx, &rows, `SELECT `+contractProjectionFields+`
FROM plugin_idc_compliance_contract
WHERE tenant_id = ? AND contract_no = ?
LIMIT 2`, idccap.DefaultTenantID, contractNo); err != nil {
		return nil, gerror.Wrap(err, "load prepared contract")
	}
	if len(rows) != 1 || rows[0] == nil {
		return nil, gerror.New("prepared contract is unavailable")
	}
	return rows[0], nil
}

func validateStoredRow(row *contractRow) error {
	if row == nil || row.ContractNo == "" || row.CustomerNo == "" || row.OrderNo == "" ||
		(row.SubjectTypeCode != string(contractcap.CustomerTypeIndividual) &&
			row.SubjectTypeCode != string(contractcap.CustomerTypeEnterprise)) ||
		row.TemplateNo == "" || row.TemplateVersion < 1 || row.VariableSnapshotRef == "" ||
		row.VariableSnapshotRef != row.SnapshotKeyRef || row.CurrencyCode == "" ||
		row.AmountMinor < 0 || row.CreatedAt.IsZero() || row.SigningExpiresAt.IsZero() ||
		!row.SigningExpiresAt.Equal(row.CreatedAt.Add(SigningValidity)) ||
		!validHexDigest(row.VariableSnapshotSHA256) ||
		!validHexDigest(row.SignerSnapshotSHA256) || !validHexDigest(row.OrderSnapshotSHA256) {
		return gerror.New("prepared contract metadata is invalid")
	}
	return nil
}

func validateBundleRow(bundle *snapshotBundle, row *contractRow) error {
	if bundle == nil || row == nil || bundle.ContractNo != row.ContractNo ||
		bundle.Order.OrderNo != row.OrderNo || bundle.Order.Customer.CustomerNo != row.CustomerNo ||
		string(bundle.Order.Customer.CustomerType) != row.SubjectTypeCode ||
		bundle.Order.Template.TemplateNo != row.TemplateNo ||
		bundle.Order.Template.VersionNo != row.TemplateVersion ||
		bundle.Order.Template.SigningOrderCode != row.SigningOrderCode ||
		validateCustomerSignerSnapshot(&bundle.CustomerSigner, &bundle.Order) != nil ||
		validateProviderLaunchSnapshot(&bundle.ProviderLaunch, &bundle.Order) != nil ||
		bundle.Order.RequiresContract != row.RequiredForFulfillment ||
		bundle.Order.CurrencyCode != row.CurrencyCode || bundle.Order.AmountMinor != row.AmountMinor ||
		!sameInstant(bundle.CreatedAt, row.CreatedAt) ||
		!sameInstant(bundle.SigningExpiresAt, row.SigningExpiresAt) {
		return gerror.New("prepared contract snapshot does not match persisted metadata")
	}
	return nil
}

func sameInstant(left, right time.Time) bool {
	return !left.IsZero() && !right.IsZero() && left.Equal(right)
}
