package runtimeconfig

import (
	"os"
	"strings"
)

const contractTemplateLibreOfficeEnvironment = "LINAPRO_IDC_CONTRACT_TEMPLATE_LIBREOFFICE_PATH"

// ContractTemplateLibreOfficePath returns the optional installation-owned converter path.
// An empty value deliberately leaves office/HTML conversion unavailable while uploaded PDFs still work.
func ContractTemplateLibreOfficePath() string {
	return strings.TrimSpace(os.Getenv(contractTemplateLibreOfficeEnvironment))
}
