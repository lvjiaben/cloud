// This file reads only the buyer portion of an immutable request snapshot for
// an authorized staff review. Secret routing references and bank-account
// secret handles never leave this package.
package taxinvoicemanagement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

const buyerReviewPurposeCode = "compliance.tax-invoice-review"

type snapshotReferenceRow struct {
	BuyerSnapshotRef    string `json:"buyer_snapshot_ref"`
	BuyerSnapshotSHA256 string `json:"buyer_snapshot_sha256"`
}

type storedSnapshotBundle struct {
	TaxInvoiceNo string                        `json:"taxInvoiceNo"`
	Request      taxinvoicecap.RequestSnapshot `json:"request"`
}

func (s *serviceImpl) loadBuyerProfile(ctx context.Context, taxInvoiceNo string) (*BuyerProfile, error) {
	var row *snapshotReferenceRow
	if err := s.database.GetScan(ctx, &row, `
SELECT buyer_snapshot_ref, buyer_snapshot_sha256
FROM plugin_idc_compliance_tax_invoice
WHERE tenant_id = ? AND tax_invoice_no = ?`, idccap.DefaultTenantID, taxInvoiceNo); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if row == nil || row.BuyerSnapshotRef == "" || len(row.BuyerSnapshotSHA256) != 64 {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	handle, err := secretcap.NewHandle(row.BuyerSnapshotRef)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	purpose, err := secretcap.NewPurpose(buyerReviewPurposeCode)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	var buyer *BuyerProfile
	err = s.snapshotAccessor.Access(ctx, request, func(plaintext []byte) error {
		var bundle storedSnapshotBundle
		if decodeErr := json.Unmarshal(plaintext, &bundle); decodeErr != nil ||
			bundle.TaxInvoiceNo != taxInvoiceNo {
			return bizerr.NewCode(CodeUnavailable)
		}
		encodedBuyer, encodeErr := json.Marshal(bundle.Request.Buyer)
		if encodeErr != nil {
			return bizerr.NewCode(CodeUnavailable)
		}
		defer clear(encodedBuyer)
		if buyerSnapshotDigest(encodedBuyer) != row.BuyerSnapshotSHA256 {
			return bizerr.NewCode(CodeUnavailable)
		}
		value := bundle.Request.Buyer
		if !businessNoPattern.MatchString(value.ProfileNo) || value.InvoiceTitle == "" || value.Version <= 0 {
			return bizerr.NewCode(CodeUnavailable)
		}
		buyer = &BuyerProfile{
			ProfileNo: value.ProfileNo, TitleTypeCode: value.TitleTypeCode,
			InvoiceTitle: value.InvoiceTitle, TaxRegistrationNo: value.TaxRegistrationNo,
			RegisteredAddress: value.RegisteredAddress, RegisteredPhone: value.RegisteredPhone,
			BankName: value.BankName, BankAccountMasked: value.BankAccountMasked,
			Version: value.Version,
		}
		return nil
	})
	if err != nil || buyer == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return buyer, nil
}

func buyerSnapshotDigest(encodedBuyer []byte) string {
	digest := sha256.New()
	_, _ = digest.Write([]byte("idc/tax-invoice-buyer/v1\x00"))
	_, _ = digest.Write(encodedBuyer)
	return hex.EncodeToString(digest.Sum(nil))
}
