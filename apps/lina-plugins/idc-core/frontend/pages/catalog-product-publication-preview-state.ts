// This file keeps publication confirmation fail-closed against every server check.

export interface CatalogProductPublicationPreviewState {
  checks: Array<{ code?: string; passed: boolean }>;
  ready: boolean;
}

export function publicationPreviewCanConfirm(
  loading: boolean,
  preview?: CatalogProductPublicationPreviewState,
) {
  return (
    !loading &&
    preview?.ready === true &&
    preview.checks.length > 0 &&
    preview.checks.every((check) => check.passed)
  );
}
