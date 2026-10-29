// Only known validation labels enter operational logs; provider payloads,
// query parameters, personal data and secrets remain redacted.
package productfulfillment

func frozenConfigurationFailureReason(err error) string {
	if err == nil {
		return "evidence_count_mismatch"
	}
	switch err.Error() {
	case "frozen configuration batch evidence is unavailable":
		return "batch_missing"
	case "frozen configuration binding evidence does not match":
		return "binding_mismatch"
	case "frozen configuration provider evidence does not match":
		return "provider_mismatch"
	case "frozen configuration provider evidence is unavailable":
		return "provider_missing"
	case "frozen customer candidate snapshot does not match":
		return "candidate_digest_mismatch"
	case "frozen customer configuration batch is invalid":
		return "batch_invalid"
	case "frozen customer configuration evidence is invalid":
		return "input_invalid"
	case "frozen customer controls are invalid":
		return "controls_invalid"
	case "frozen customer declaration digest does not match":
		return "customer_declaration_digest_mismatch"
	case "frozen customer declaration is unavailable":
		return "customer_declaration_missing"
	case "frozen customer files are unavailable":
		return "files_unavailable"
	case "frozen customer values are invalid":
		return "values_invalid"
	case "frozen interface configuration evidence does not match":
		return "interface_configuration_mismatch"
	case "frozen product declaration is unavailable":
		return "product_declaration_missing"
	case "frozen product version declaration evidence does not match":
		return "version_declaration_mismatch"
	default:
		return "unclassified"
	}
}
