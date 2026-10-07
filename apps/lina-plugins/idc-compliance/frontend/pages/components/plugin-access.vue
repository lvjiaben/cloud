<script lang="ts">
import { useAccess as useHostAccess } from "@vben/access";

export function useAccess() {
  const access = useHostAccess();
  return {
    ...access,
    hasAccessByCodes(codes: string[]) {
      return access.hasAccessByCodes(["*:*:*"]) || access.hasAccessByCodes(codes);
    },
  };
}

export default { name: "IdcCompliancePluginAccessAdapter" };
</script>

<template><span hidden /></template>
