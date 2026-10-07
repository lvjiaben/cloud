package electroniccontractpreparation

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/contractcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

const (
	bundleVersion       = 3
	bundleSaltBytes     = 32
	maximumBundleBytes  = 1 << 20
	variableDigestScope = "idc/contract-variable-snapshot/v1\x00"
	signerDigestScope   = "idc/contract-signer-snapshot/v1\x00"
	orderDigestScope    = "idc/contract-order-and-provider-snapshot/v2\x00"
)

type snapshotBundle struct {
	Version          int
	Salt             []byte
	ContractNo       string
	CreatedAt        time.Time
	SigningExpiresAt time.Time
	Order            contractcap.OrderSnapshot
	Preview          contractcap.PreviewVariables
	CustomerSigner   customerSignerSnapshot
	ProviderLaunch   providerLaunchSnapshot
}

type signerSnapshot struct {
	Customer       contractcap.VerifiedIdentity
	CustomerSigner customerSignerSnapshot
	Merchant       contractcap.MerchantIdentity
}

type customerSignerSnapshot struct {
	SignerNo                    string
	CustomerNo                  string
	CustomerType                contractcap.CustomerType
	Name                        []byte
	Phone                       []byte
	IdentityVerificationVersion int64
	IdentityVerifiedAt          time.Time
	PhoneVerificationNo         string
	PhoneVerificationVersion    int64
	PhoneVerifiedAt             time.Time
	AuthorizationVersion        int64
}

func (s *customerSignerSnapshot) Clear() {
	if s == nil {
		return
	}
	clear(s.Name)
	s.Name = nil
	clear(s.Phone)
	s.Phone = nil
}

type snapshotDigests struct {
	Variables string
	Signers   string
	Order     string
}

func newSnapshotBundle(
	random io.Reader,
	contractNo string,
	createdAt time.Time,
	order *contractcap.OrderSnapshot,
	preview *contractcap.PreviewVariables,
	signer *customerSignerSnapshot,
	provider *providerLaunchSnapshot,
) (*snapshotBundle, snapshotDigests, []byte, error) {
	if random == nil || order == nil || preview == nil || signer == nil || provider == nil || createdAt.IsZero() {
		return nil, snapshotDigests{}, nil, gerror.New("contract snapshot bundle input is invalid")
	}
	salt := make([]byte, bundleSaltBytes)
	if _, err := io.ReadFull(random, salt); err != nil {
		clear(salt)
		return nil, snapshotDigests{}, nil, gerror.Wrap(err, "read contract snapshot salt")
	}
	bundle := &snapshotBundle{
		Version: bundleVersion, Salt: salt, ContractNo: contractNo,
		CreatedAt: createdAt.UTC(), SigningExpiresAt: createdAt.UTC().Add(SigningValidity),
		Order: cloneFrozenOrderSnapshot(order), Preview: clonePreviewVariables(preview),
		CustomerSigner: cloneCustomerSigner(signer), ProviderLaunch: provider.Clone(),
	}
	if err := validateCustomerSignerSnapshot(&bundle.CustomerSigner, &bundle.Order); err != nil {
		bundle.Clear()
		return nil, snapshotDigests{}, nil, err
	}
	if err := validateProviderLaunchSnapshot(&bundle.ProviderLaunch, &bundle.Order); err != nil {
		bundle.Clear()
		return nil, snapshotDigests{}, nil, err
	}
	digests, err := digestBundle(bundle)
	if err != nil {
		bundle.Clear()
		return nil, snapshotDigests{}, nil, err
	}
	payload, err := json.Marshal(bundle)
	if err != nil || len(payload) == 0 || len(payload) > maximumBundleBytes {
		clear(payload)
		bundle.Clear()
		return nil, snapshotDigests{}, nil, gerror.New("contract snapshot bundle is too large or invalid")
	}
	return bundle, digests, payload, nil
}

func decodeSnapshotBundle(payload []byte, expected snapshotDigests) (*snapshotBundle, error) {
	bundle, err := decodeSnapshotBundlePayload(payload)
	if err != nil {
		return nil, err
	}
	actual, err := digestBundle(bundle)
	if err != nil || !equalSnapshotDigests(actual, expected) {
		bundle.Clear()
		return nil, gerror.New("contract snapshot bundle integrity check failed")
	}
	return bundle, nil
}

func decodeSnapshotBundlePayload(payload []byte) (*snapshotBundle, error) {
	if len(payload) == 0 || len(payload) > maximumBundleBytes {
		return nil, gerror.New("contract snapshot bundle is unavailable")
	}
	var bundle snapshotBundle
	if err := json.Unmarshal(payload, &bundle); err != nil || bundle.Version != bundleVersion ||
		len(bundle.Salt) != bundleSaltBytes || bundle.ContractNo == "" || bundle.CreatedAt.IsZero() ||
		!bundle.SigningExpiresAt.Equal(bundle.CreatedAt.Add(SigningValidity)) {
		bundle.Clear()
		return nil, gerror.New("contract snapshot bundle is invalid")
	}
	if err := validateCustomerSignerSnapshot(&bundle.CustomerSigner, &bundle.Order); err != nil {
		bundle.Clear()
		return nil, err
	}
	if err := validateProviderLaunchSnapshot(&bundle.ProviderLaunch, &bundle.Order); err != nil {
		bundle.Clear()
		return nil, err
	}
	return &bundle, nil
}

func digestBundle(bundle *snapshotBundle) (snapshotDigests, error) {
	if bundle == nil || len(bundle.Salt) != bundleSaltBytes {
		return snapshotDigests{}, gerror.New("contract snapshot digest input is invalid")
	}
	variables, err := digestSnapshotSection(variableDigestScope, bundle.Salt, bundle.Preview)
	if err != nil {
		return snapshotDigests{}, err
	}
	signers, err := digestSnapshotSection(signerDigestScope, bundle.Salt, signerSnapshot{
		Customer: bundle.Order.Customer, CustomerSigner: bundle.CustomerSigner, Merchant: bundle.Order.Merchant,
	})
	if err != nil {
		return snapshotDigests{}, err
	}
	order, err := digestSnapshotSection(orderDigestScope, bundle.Salt, struct {
		Order          contractcap.OrderSnapshot
		ProviderLaunch providerLaunchSnapshot
	}{Order: bundle.Order, ProviderLaunch: bundle.ProviderLaunch})
	if err != nil {
		return snapshotDigests{}, err
	}
	return snapshotDigests{Variables: variables, Signers: signers, Order: order}, nil
}

func digestSnapshotSection(scope string, salt []byte, value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", gerror.Wrap(err, "encode contract snapshot digest section")
	}
	defer clear(encoded)
	hash := sha256.New()
	_, _ = hash.Write([]byte(scope))
	_, _ = hash.Write(salt)
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(encoded)
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func equalSnapshotDigests(left, right snapshotDigests) bool {
	return secureDigestEqual(left.Variables, right.Variables) &&
		secureDigestEqual(left.Signers, right.Signers) &&
		secureDigestEqual(left.Order, right.Order)
}

func secureDigestEqual(left, right string) bool {
	return len(left) == sha256.Size*2 && len(right) == sha256.Size*2 &&
		hmac.Equal([]byte(left), []byte(right))
}

func bundleProducer(payload []byte) secretcap.PlaintextProducer {
	return func(consume secretcap.Consumer) error {
		if consume == nil || len(payload) == 0 {
			return gerror.New("contract snapshot consumer is unavailable")
		}
		return consume(payload)
	}
}

func projectPrepared(bundle *snapshotBundle, reused bool) *PreparedContract {
	if bundle == nil {
		return nil
	}
	preview := bundle.Preview
	preview.CustomerName = append([]byte(nil), bundle.Preview.CustomerName...)
	preview.MerchantName = append([]byte(nil), bundle.Preview.MerchantName...)
	preview.Items = append([]contractcap.PreviewItem(nil), bundle.Preview.Items...)
	return &PreparedContract{
		ContractNo: bundle.ContractNo, OrderNo: bundle.Order.OrderNo,
		CustomerNo: bundle.Order.Customer.CustomerNo, CustomerType: bundle.Order.Customer.CustomerType,
		Template: bundle.Order.Template, SigningOrderCode: bundle.Order.Template.SigningOrderCode,
		AmountMinor: bundle.Order.AmountMinor, CurrencyCode: bundle.Order.CurrencyCode,
		RequiredForFulfillment: bundle.Order.RequiresContract,
		CreatedAt:              bundle.CreatedAt, SigningExpiresAt: bundle.SigningExpiresAt,
		Preview: &preview, Reused: reused,
		SignerNo: bundle.CustomerSigner.SignerNo, SignerName: append([]byte(nil), bundle.CustomerSigner.Name...),
		SignerPhoneMasked: maskPhone(bundle.CustomerSigner.Phone),
		Version:           1,
	}
}

func (b *snapshotBundle) Clear() {
	if b == nil {
		return
	}
	clear(b.Salt)
	b.Salt = nil
	b.Order.Clear()
	b.Preview.Clear()
	b.CustomerSigner.Clear()
	b.ProviderLaunch.Clear()
}

func cloneFrozenOrderSnapshot(source *contractcap.OrderSnapshot) contractcap.OrderSnapshot {
	result := *source
	result.Customer.LegalName = append([]byte(nil), source.Customer.LegalName...)
	result.Customer.VerifiedPhone = append([]byte(nil), source.Customer.VerifiedPhone...)
	result.Customer.RegistrationIdentifier = append([]byte(nil), source.Customer.RegistrationIdentifier...)
	result.Merchant.LegalName = append([]byte(nil), source.Merchant.LegalName...)
	result.Items = append([]contractcap.OrderItem(nil), source.Items...)
	if source.Customer.ExpiresAt != nil {
		expiresAt := *source.Customer.ExpiresAt
		result.Customer.ExpiresAt = &expiresAt
	}
	return result
}

func clonePreviewVariables(source *contractcap.PreviewVariables) contractcap.PreviewVariables {
	result := *source
	result.CustomerName = append([]byte(nil), source.CustomerName...)
	result.MerchantName = append([]byte(nil), source.MerchantName...)
	result.Items = append([]contractcap.PreviewItem(nil), source.Items...)
	return result
}

func cloneCustomerSigner(source *customerSignerSnapshot) customerSignerSnapshot {
	result := *source
	result.Name = append([]byte(nil), source.Name...)
	result.Phone = append([]byte(nil), source.Phone...)
	return result
}
