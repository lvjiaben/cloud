package contenthub

import "context"

var (
	ordinaryDownloadExtensions = [...]string{
		".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".pdf",
		".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
	}
	inspectionRequiredDownloadExtensions = [...]string{
		".zip", ".7z", ".exe", ".msi", ".iso",
	}
)

func classifyDownloadExtensionName(extension string) (inspectionRequired bool, allowed bool) {
	for _, candidate := range ordinaryDownloadExtensions {
		if extension == candidate {
			return false, true
		}
	}
	for _, candidate := range inspectionRequiredDownloadExtensions {
		if extension == candidate {
			return true, true
		}
	}
	return false, false
}

func copyDownloadExtensions(source []string) []string {
	result := make([]string, len(source))
	copy(result, source)
	return result
}

func (s *serviceImpl) uploadPolicy() *UploadPolicy {
	policy := &UploadPolicy{
		OrdinaryMaxSizeBytes:         maximumDownloadBytes,
		OrdinaryExtensions:           copyDownloadExtensions(ordinaryDownloadExtensions[:]),
		InspectionRequiredExtensions: copyDownloadExtensions(inspectionRequiredDownloadExtensions[:]),
	}
	if !s.scannerReady() {
		return policy
	}
	limit := s.scanner.DangerousLimitBytes()
	if limit < 1 || limit > maximumDangerousDownloadBytes {
		return policy
	}
	policy.InspectionRequiredAvailable = true
	policy.InspectionRequiredMaxSizeBytes = limit
	return policy
}

// GetUploadPolicy returns only the business limits that the current administrator
// needs before selecting a file. Scanner connection and diagnostic details stay private.
func (s *serviceImpl) GetUploadPolicy(ctx context.Context) (*UploadPolicy, error) {
	if _, err := s.staff(ctx); err != nil {
		return nil, err
	}
	return s.uploadPolicy(), nil
}
