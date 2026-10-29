// This file defines the immutable native commercial intent fingerprint shared
// by checkout and settlement. Parameters are read from the stored quote so both
// sides hash the same PostgreSQL JSON representation rather than browser bytes.

package serveroperation

func nativeAccessoryIntentDigest(operationNo, quoteNo, orderNo, invoiceNo, entitlementNo, action, parameters string, financial bool, version int64) (string, error) {
	return digestValue(struct {
		OperationNo, QuoteNo, OrderNo, InvoiceNo, EntitlementNo, Action, Parameters string
		Financial                                                                   bool
		ExpectedVersion                                                             int64
	}{operationNo, quoteNo, orderNo, invoiceNo, entitlementNo, action, parameters, financial, version})
}
