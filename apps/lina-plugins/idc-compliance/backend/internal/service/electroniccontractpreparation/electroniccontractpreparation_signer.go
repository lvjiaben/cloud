package electroniccontractpreparation

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-compliance/backend/internal/service/contractsigner"
	"lina-plugin-idc-core/backend/cap/contractcap"
)

func (s *serviceImpl) resolveCustomerSigner(ctx context.Context, order *contractcap.OrderSnapshot,
	signerNo string) (*customerSignerSnapshot, error) {
	if order == nil || order.Customer.CustomerNo == "" {
		return nil, gerror.New("contract customer signer input is invalid")
	}
	switch order.Customer.CustomerType {
	case contractcap.CustomerTypeIndividual:
		if signerNo != "" || len(order.Customer.LegalName) == 0 || len(order.Customer.VerifiedPhone) == 0 ||
			order.Customer.VerificationVersion <= 0 || order.Customer.VerifiedAt.IsZero() ||
			order.Customer.PhoneContactNo == "" || order.Customer.PhoneContactVersion <= 0 || order.Customer.PhoneVerifiedAt.IsZero() {
			return nil, gerror.New("verified personal contract signer is unavailable")
		}
		return &customerSignerSnapshot{SignerNo: "personal:" + order.Customer.VerificationCaseNo,
			CustomerNo: order.Customer.CustomerNo, CustomerType: order.Customer.CustomerType,
			Name: append([]byte(nil), order.Customer.LegalName...), Phone: append([]byte(nil), order.Customer.VerifiedPhone...),
			IdentityVerificationVersion: order.Customer.VerificationVersion, IdentityVerifiedAt: order.Customer.VerifiedAt.UTC(),
			PhoneVerificationNo: order.Customer.PhoneContactNo, PhoneVerificationVersion: order.Customer.PhoneContactVersion,
			PhoneVerifiedAt: order.Customer.PhoneVerifiedAt.UTC(), AuthorizationVersion: 1}, nil
	case contractcap.CustomerTypeEnterprise:
		if !validBusinessNo(signerNo, 64) || s.signers == nil {
			return nil, gerror.New("verified enterprise contract signer is required")
		}
		var result *customerSignerSnapshot
		err := s.signers.WithCandidate(ctx, order.Customer.CustomerNo, signerNo, func(candidate contractsigner.FrozenSigner) error {
			result = &customerSignerSnapshot{SignerNo: candidate.SignerNo, CustomerNo: candidate.CustomerNo,
				CustomerType: contractcap.CustomerTypeEnterprise, Name: append([]byte(nil), candidate.Name...),
				Phone: append([]byte(nil), candidate.Phone...), IdentityVerificationVersion: candidate.IdentityVerificationVersion,
				IdentityVerifiedAt: candidate.IdentityVerifiedAt, PhoneVerificationNo: candidate.PhoneVerificationNo,
				PhoneVerificationVersion: candidate.PhoneVerificationVersion, PhoneVerifiedAt: candidate.PhoneVerifiedAt,
				AuthorizationVersion: candidate.AuthorizationVersion}
			return nil
		})
		if err != nil || result == nil {
			if result != nil {
				result.Clear()
			}
			return nil, gerror.Wrap(err, "resolve verified enterprise contract signer")
		}
		return result, nil
	default:
		return nil, gerror.New("contract customer signer type is unsupported")
	}
}

func maskPhone(value []byte) string {
	text := strings.TrimSpace(string(value))
	runes := []rune(text)
	if len(runes) <= 7 {
		return "***"
	}
	return string(runes[:3]) + "****" + string(runes[len(runes)-4:])
}
