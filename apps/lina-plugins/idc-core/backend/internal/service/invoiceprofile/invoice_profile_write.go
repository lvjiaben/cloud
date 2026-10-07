package invoiceprofile

import (
	"context"
	"strconv"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

func (s *serviceImpl) CreateCurrentCustomer(ctx context.Context, input CreateInput) (*Profile, error) {
	defer clear(input.BankAccount)
	subject, err := currentCustomer(ctx, s.customers)
	if err != nil {
		return nil, err
	}
	fields, err := normalizeFields(normalizedFields{
		TitleTypeCode: input.TitleTypeCode, InvoiceTitle: input.InvoiceTitle,
		TaxRegistrationNo: input.TaxRegistrationNo, RegisteredAddress: input.RegisteredAddress,
		RegisteredPhone: input.RegisteredPhone, BankName: input.BankName,
	})
	if err != nil || !validIdempotencyKey(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	bankAccount, err := normalizeBankAccount(input.BankAccount, false)
	if err != nil {
		return nil, err
	}
	defer clear(bankAccount)
	if fields.TitleTypeCode == TitleTypeIndividual && len(bankAccount) != 0 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	profileNo := profileNumber(s.numberKey, subject.CustomerNo, input.IdempotencyKey)
	var result *Profile
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		masked, secretRef, txErr := s.createBankSecret(ctx, profileNo, 1, bankAccount)
		if txErr != nil {
			return txErr
		}
		var existing *profileRow
		if txErr = tx.GetScan(&existing, `SELECT `+profileProjection+`
FROM plugin_idc_core_customer_invoice_profile
WHERE tenant_id = 0 AND customer_no = ? AND profile_no = ? FOR UPDATE`, subject.CustomerNo, profileNo); txErr != nil {
			return txErr
		}
		if existing != nil {
			if !sameCreate(existing, fields, masked, secretRef, input.IsDefault, subject.ActorUserID) {
				return bizerr.NewCode(CodeConflict)
			}
			result, txErr = projectProfile(existing)
			return txErr
		}
		if input.IsDefault {
			if _, txErr = tx.Exec(`UPDATE plugin_idc_core_customer_invoice_profile
SET is_default = FALSE, version = version + 1, updated_by = ?, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND customer_no = ? AND status_code = 'active' AND is_default = TRUE`, subject.ActorUserID, subject.CustomerNo); txErr != nil {
				return txErr
			}
		}
		var row profileRow
		txErr = tx.GetScan(&row, `INSERT INTO plugin_idc_core_customer_invoice_profile
    (tenant_id, profile_no, customer_no, title_type_code, invoice_title, tax_registration_no,
     registered_address, registered_phone, bank_name, bank_account_masked,
     bank_account_secret_ref, is_default, status_code, version, created_by, updated_by)
VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', 1, ?, ?)
RETURNING `+profileProjection, profileNo, subject.CustomerNo, fields.TitleTypeCode, fields.InvoiceTitle,
			fields.TaxRegistrationNo, fields.RegisteredAddress, fields.RegisteredPhone, fields.BankName,
			masked, secretRef, input.IsDefault, subject.ActorUserID, subject.ActorUserID)
		if txErr != nil {
			return txErr
		}
		result, txErr = projectProfile(&row)
		return txErr
	})
	if err != nil {
		if bizerr.Is(err, CodeInvalid) || bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeNotAccessible) {
			return nil, err
		}
		return nil, gerror.Wrap(bizerr.NewCode(CodeUnavailable), "create current customer invoice profile")
	}
	return result, nil
}

func (s *serviceImpl) UpdateCurrentCustomer(ctx context.Context, input UpdateInput) (*Profile, error) {
	defer clear(input.BankAccount)
	subject, err := currentCustomer(ctx, s.customers)
	if err != nil {
		return nil, err
	}
	fields, err := normalizeFields(normalizedFields{
		TitleTypeCode: input.TitleTypeCode, InvoiceTitle: input.InvoiceTitle,
		TaxRegistrationNo: input.TaxRegistrationNo, RegisteredAddress: input.RegisteredAddress,
		RegisteredPhone: input.RegisteredPhone, BankName: input.BankName,
	})
	if err != nil || !validBusinessNo(input.ProfileNo) || input.ExpectedVersion <= 0 ||
		!validIdempotencyKey(input.IdempotencyKey) ||
		(input.BankAccountAction != BankAccountKeep && input.BankAccountAction != BankAccountReplace && input.BankAccountAction != BankAccountClear) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	requiredBank := input.BankAccountAction == BankAccountReplace
	bankAccount, err := normalizeBankAccount(input.BankAccount, requiredBank)
	if err != nil || (input.BankAccountAction != BankAccountReplace && len(bankAccount) != 0) ||
		(fields.TitleTypeCode == TitleTypeIndividual && input.BankAccountAction == BankAccountReplace) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	defer clear(bankAccount)
	var result *Profile
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row, txErr := loadOwnedProfile(ctx, tx, subject.CustomerNo, input.ProfileNo, true)
		if txErr != nil {
			return txErr
		}
		if row.StatusCode != StatusActive {
			return bizerr.NewCode(CodeConflict)
		}
		if row.Version != input.ExpectedVersion {
			return bizerr.NewCode(CodeConflict)
		}
		if fields.TitleTypeCode == TitleTypeIndividual && row.BankAccountSecretRef != "" &&
			input.BankAccountAction != BankAccountClear {
			return bizerr.NewCode(CodeInvalid)
		}
		masked, secretRef := row.BankAccountMasked, row.BankAccountSecretRef
		switch input.BankAccountAction {
		case BankAccountReplace:
			masked, secretRef, txErr = s.createBankSecret(ctx, input.ProfileNo, input.ExpectedVersion+1, bankAccount)
			if txErr != nil {
				return txErr
			}
		case BankAccountClear:
			masked, secretRef = "", ""
		}
		var updated profileRow
		txErr = tx.GetScan(&updated, `UPDATE plugin_idc_core_customer_invoice_profile
SET title_type_code = ?, invoice_title = ?, tax_registration_no = ?, registered_address = ?,
    registered_phone = ?, bank_name = ?, bank_account_masked = ?, bank_account_secret_ref = ?,
    version = version + 1, updated_by = ?, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND customer_no = ? AND profile_no = ? AND status_code = 'active' AND version = ?
RETURNING `+profileProjection, fields.TitleTypeCode, fields.InvoiceTitle, fields.TaxRegistrationNo,
			fields.RegisteredAddress, fields.RegisteredPhone, fields.BankName, masked, secretRef,
			subject.ActorUserID, subject.CustomerNo, input.ProfileNo, input.ExpectedVersion)
		if txErr != nil {
			return txErr
		}
		if updated.ID == 0 {
			return bizerr.NewCode(CodeConflict)
		}
		result, txErr = projectProfile(&updated)
		return txErr
	})
	if err != nil {
		if bizerr.Is(err, CodeInvalid) || bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeNotAccessible) {
			return nil, err
		}
		return nil, gerror.Wrap(bizerr.NewCode(CodeUnavailable), "update current customer invoice profile")
	}
	return result, nil
}

func (s *serviceImpl) SetDefaultCurrentCustomer(ctx context.Context, input VersionInput) (*Profile, error) {
	return s.changeStatus(ctx, input, true)
}

func (s *serviceImpl) ArchiveCurrentCustomer(ctx context.Context, input VersionInput) (*Profile, error) {
	return s.changeStatus(ctx, input, false)
}

func (s *serviceImpl) changeStatus(ctx context.Context, input VersionInput, setDefault bool) (*Profile, error) {
	subject, err := currentCustomer(ctx, s.customers)
	if err != nil {
		return nil, err
	}
	if !validBusinessNo(input.ProfileNo) || input.ExpectedVersion <= 0 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var result *Profile
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row, txErr := loadOwnedProfile(ctx, tx, subject.CustomerNo, input.ProfileNo, true)
		if txErr != nil {
			return txErr
		}
		if row.StatusCode != StatusActive || row.Version != input.ExpectedVersion {
			return bizerr.NewCode(CodeConflict)
		}
		if setDefault {
			if _, txErr = tx.Exec(`UPDATE plugin_idc_core_customer_invoice_profile
SET is_default = FALSE, version = version + 1, updated_by = ?, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND customer_no = ? AND status_code = 'active' AND is_default = TRUE AND profile_no <> ?`,
				subject.ActorUserID, subject.CustomerNo, input.ProfileNo); txErr != nil {
				return txErr
			}
			if row.IsDefault {
				result, txErr = projectProfile(row)
				return txErr
			}
		} else if row.StatusCode == StatusArchived {
			result, txErr = projectProfile(row)
			return txErr
		}
		status, isDefault := StatusActive, true
		if !setDefault {
			status, isDefault = StatusArchived, false
		}
		var updated profileRow
		txErr = tx.GetScan(&updated, `UPDATE plugin_idc_core_customer_invoice_profile
SET status_code = ?, is_default = ?, version = version + 1, updated_by = ?, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND customer_no = ? AND profile_no = ? AND status_code = 'active' AND version = ?
RETURNING `+profileProjection, status, isDefault, subject.ActorUserID, subject.CustomerNo, input.ProfileNo, input.ExpectedVersion)
		if txErr != nil {
			return txErr
		}
		if updated.ID == 0 {
			return bizerr.NewCode(CodeConflict)
		}
		result, txErr = projectProfile(&updated)
		return txErr
	})
	if err != nil {
		if bizerr.Is(err, CodeInvalid) || bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeNotAccessible) {
			return nil, err
		}
		return nil, gerror.Wrap(bizerr.NewCode(CodeUnavailable), "change current customer invoice profile")
	}
	return result, nil
}

func (s *serviceImpl) createBankSecret(ctx context.Context, profileNo string, version int64, bankAccount []byte) (string, string, error) {
	if len(bankAccount) == 0 {
		return "", "", nil
	}
	key := "invoice-profile-bank:" + profileNo + ":v" + strconv.FormatInt(version, 10)
	result, err := s.secrets.Create(ctx, key, func(consume secretcap.Consumer) error {
		return consume(append([]byte(nil), bankAccount...))
	})
	if err != nil || result.Handle.IsZero() || !result.Metadata.Configured || result.Metadata.MaskedTail == "" {
		return "", "", bizerr.NewCode(CodeUnavailable)
	}
	return result.Metadata.MaskedTail, result.Handle.OpaqueRef(), nil
}

func sameCreate(row *profileRow, fields normalizedFields, masked, secretRef string, isDefault bool, actor int) bool {
	return row.CustomerNo != "" && row.TitleTypeCode == fields.TitleTypeCode && row.InvoiceTitle == fields.InvoiceTitle &&
		row.TaxRegistrationNo == fields.TaxRegistrationNo && row.RegisteredAddress == fields.RegisteredAddress &&
		row.RegisteredPhone == fields.RegisteredPhone && row.BankName == fields.BankName &&
		row.BankAccountMasked == masked && row.BankAccountSecretRef == secretRef && row.IsDefault == isDefault &&
		row.StatusCode == StatusActive && actor > 0
}
