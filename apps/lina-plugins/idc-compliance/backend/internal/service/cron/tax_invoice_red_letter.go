// This file registers bounded red-letter intake with the plugin job host.

package cron

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-compliance/backend/internal/service/taxinvoiceredletter"
	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

const (
	taxInvoiceRedLetterPattern     = "@every 1m"
	taxInvoiceRedLetterName        = "idc-compliance-tax-invoice-red-letter"
	taxInvoiceRedLetterDisplayName = "IDC Tax Invoice Red-Letter Intake"
	taxInvoiceRedLetterDescription = "Creates installation-local red-letter tasks after posted customer refunds without delaying the refund."
)

// RegisterTaxInvoiceRedLetter registers the bounded primary-node refund intake worker.
func RegisterTaxInvoiceRedLetter(
	ctx context.Context,
	registrar pluginhost.JobsRegistrar,
	worker taxinvoiceredletter.IntakeWorker,
) error {
	if ctx == nil || ctx.Err() != nil || nilDependency(registrar) || nilDependency(worker) {
		return gerror.New("tax invoice red-letter job registrar is unavailable")
	}
	if !registrar.IsPrimaryNode() {
		return nil
	}
	return registrar.AddWithMetadata(
		ctx, taxInvoiceRedLetterPattern, taxInvoiceRedLetterName,
		taxInvoiceRedLetterDisplayName, taxInvoiceRedLetterDescription,
		func(jobCtx context.Context) error {
			_, err := worker.Run(jobCtx, taxinvoicecap.MaximumRefundOperations)
			return err
		},
	)
}
