// This file ensures the invoice supplier remains closed until its external evidence is approved.
package supplierconfig
import "testing"
func TestInvoiceSupplierRemainsClosed(t *testing.T) {
 evidence, enabled := subjectAdapterGate(ProviderNuonuoInvoice, OperationSubjectBind, OperationResultSucceeded)
 if enabled || evidence != EvidenceExternalConfirmationRequired { t.Fatalf("evidence=%s enabled=%v", evidence,enabled) }
 if validProvider("tencent_essbasic") { t.Fatal("retired electronic contract provider is still accepted") }
}
