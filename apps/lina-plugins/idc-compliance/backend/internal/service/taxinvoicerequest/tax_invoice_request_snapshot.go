// This file encodes and verifies the encrypted immutable request bundle.

package taxinvoicerequest

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

type snapshotBundle struct {
	TaxInvoiceNo string                        `json:"taxInvoiceNo"`
	Request      taxinvoicecap.RequestSnapshot `json:"request"`
}

type snapshotDigests struct {
	Bundle string
	Seller string
	Buyer  string
	Items  map[string]string
}

// newSnapshotBundle creates one deterministic payload and separate digests
// for the header snapshots and every immutable source item.
func newSnapshotBundle(
	taxInvoiceNo string,
	snapshot *taxinvoicecap.RequestSnapshot,
) (*snapshotBundle, snapshotDigests, []byte, error) {
	if !validBusinessNo(taxInvoiceNo, 64) || snapshot == nil ||
		!validBusinessNo(snapshot.CustomerNo, 64) || len(snapshot.Invoices) == 0 {
		return nil, snapshotDigests{}, nil, gerror.New("tax invoice source snapshot is invalid")
	}
	bundle := &snapshotBundle{TaxInvoiceNo: taxInvoiceNo, Request: *snapshot}
	sellerPayload, err := json.Marshal(bundle.Request.Seller)
	if err != nil {
		return nil, snapshotDigests{}, nil, gerror.Wrap(err, "encode tax invoice seller snapshot")
	}
	defer clear(sellerPayload)
	buyerPayload, err := json.Marshal(bundle.Request.Buyer)
	if err != nil {
		return nil, snapshotDigests{}, nil, gerror.Wrap(err, "encode tax invoice buyer snapshot")
	}
	defer clear(buyerPayload)
	digests := snapshotDigests{
		Seller: digestBytes("idc/tax-invoice-seller/v1\x00", sellerPayload),
		Buyer:  digestBytes("idc/tax-invoice-buyer/v1\x00", buyerPayload),
		Items:  make(map[string]string),
	}
	for _, invoice := range bundle.Request.Invoices {
		for _, item := range invoice.Items {
			if !validBusinessNo(item.BillingInvoiceItemNo, 64) {
				return nil, snapshotDigests{}, nil, gerror.New("tax invoice source item is invalid")
			}
			itemPayload, marshalErr := json.Marshal(item)
			if marshalErr != nil {
				return nil, snapshotDigests{}, nil, gerror.Wrap(marshalErr, "encode tax invoice item snapshot")
			}
			if _, duplicate := digests.Items[item.BillingInvoiceItemNo]; duplicate {
				clear(itemPayload)
				return nil, snapshotDigests{}, nil, gerror.New("tax invoice source item is duplicated")
			}
			digests.Items[item.BillingInvoiceItemNo] = digestBytes(
				"idc/tax-invoice-item-snapshot/v1\x00", itemPayload,
			)
			clear(itemPayload)
		}
	}
	payload, err := json.Marshal(bundle)
	if err != nil {
		return nil, snapshotDigests{}, nil, gerror.Wrap(err, "encode tax invoice snapshot bundle")
	}
	digests.Bundle = digestBytes("idc/tax-invoice-bundle/v1\x00", payload)
	return bundle, digests, payload, nil
}

// sealSnapshot idempotently stores the bundle and proves a recovered secret
// still represents the same source facts before allowing database recovery.
func (s *serviceImpl) sealSnapshot(
	ctx context.Context,
	requestDigest string,
	bundle *snapshotBundle,
	digests snapshotDigests,
	payload []byte,
) (secretcap.CreateResult, error) {
	created, err := s.manager.Create(
		ctx,
		"tax-invoice-snapshot:"+requestDigest,
		func(consume secretcap.Consumer) error { return consume(payload) },
	)
	if err != nil {
		return secretcap.CreateResult{}, gerror.Wrap(err, "seal tax invoice request snapshot")
	}
	if created.Handle.IsZero() || !created.Metadata.Configured || created.Metadata.UpdatedAt.IsZero() {
		return secretcap.CreateResult{}, gerror.New("sealed tax invoice snapshot metadata is invalid")
	}
	if !created.Reused {
		return created, nil
	}
	purpose, err := secretcap.NewPurpose("compliance.tax-invoice-recovery")
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	request, err := secretcap.NewAccessRequest(created.Handle, purpose)
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	err = s.accessor.Access(ctx, request, func(recoveredPayload []byte) error {
		var recovered snapshotBundle
		if decodeErr := json.Unmarshal(recoveredPayload, &recovered); decodeErr != nil {
			return gerror.Wrap(decodeErr, "decode recovered tax invoice snapshot")
		}
		recoveredBytes, encodeErr := json.Marshal(recovered)
		if encodeErr != nil {
			return gerror.Wrap(encodeErr, "verify recovered tax invoice snapshot")
		}
		defer clear(recoveredBytes)
		if recovered.TaxInvoiceNo != bundle.TaxInvoiceNo ||
			digestBytes("idc/tax-invoice-bundle/v1\x00", recoveredBytes) != digests.Bundle {
			return gerror.New("recovered tax invoice snapshot conflicts with current source facts")
		}
		return nil
	})
	if err != nil {
		return secretcap.CreateResult{}, err
	}
	return created, nil
}
