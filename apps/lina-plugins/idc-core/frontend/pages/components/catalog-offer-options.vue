<script setup lang="ts">
import type { CatalogProductDetail, CatalogProductVersionConfiguration } from "../catalog-client";
import { ref, computed } from "vue";
import CatalogCustomerOptionsForm from "./catalog-customer-options-form.vue";
defineProps<{
  versionNo: string;
  detail: CatalogProductDetail;
  configuration: CatalogProductVersionConfiguration;
  writable: boolean;
  canPrice: boolean;
  externalPricingDirty?: boolean;
  refresh: () => Promise<void>;
}>();
const emit = defineEmits<{
  saved: [configuration: CatalogProductVersionConfiguration];
  busy: [value: boolean];
}>();
const form = ref<InstanceType<typeof CatalogCustomerOptionsForm>>();
const dirty = computed(() => !!form.value?.dirty);
defineExpose({save: () => form.value?.save(), dirty});
</script>
<template>
  <section data-testid="catalog-offer-options">
    <CatalogCustomerOptionsForm ref="form"
      :version-no="versionNo" :binding-no="detail.fulfillmentBinding?.interfaceInstanceNo || ''"
      :configuration="configuration" :detail="detail" :writable="writable"
      :can-price="canPrice" :external-pricing-dirty="externalPricingDirty" :refresh="refresh"
      @saved="emit('saved', $event)" @busy="emit('busy', $event)"
    />
  </section>
</template>
