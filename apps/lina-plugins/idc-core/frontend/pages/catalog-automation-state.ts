export function canWriteCatalogFulfillmentBinding(
  versionStatus: string | undefined,
  canViewInterfaces: boolean,
  canWriteBinding: boolean,
) {
  return versionStatus === "draft" && canViewInterfaces && canWriteBinding;
}
