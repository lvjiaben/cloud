<script setup lang="ts">
import type {
  CatalogProductPublicationPreview,
  CatalogProductSummary,
} from "../catalog-client";

import { computed, ref } from "vue";

import { useVbenModal } from "@vben/common-ui";
import { preferences } from "@vben/preferences";

import { Alert, message, Skeleton, Tag } from "ant-design-vue";

import { $t } from "#/locales";

import {
  catalogProductPublicationPreview,
  catalogProductPublish,
} from "../catalog-client";
import { publicationPreviewCanConfirm } from "../catalog-product-publication-preview-state";

const emit = defineEmits<{ success: [] }>();

const row = ref<CatalogProductSummary>();
const preview = ref<CatalogProductPublicationPreview>();
const previewFailed = ref(false);
const loading = ref(false);
let requestSequence = 0;

const localizedName = computed(() => {
  if (!row.value) return "";
  return preferences.app.locale === "zh-CN"
    ? row.value.nameZhCn || row.value.nameEnUs
    : row.value.nameEnUs || row.value.nameZhCn;
});

function updateConfirmState() {
  modalApi.setState({
    confirmDisabled: !publicationPreviewCanConfirm(
      loading.value,
      preview.value,
    ),
  });
}

async function loadPreview(versionNo: string, sequence: number) {
  loading.value = true;
  previewFailed.value = false;
  preview.value = undefined;
  updateConfirmState();
  try {
    const result = await catalogProductPublicationPreview(versionNo);
    if (sequence !== requestSequence) return;
    preview.value = result;
  } catch {
    if (sequence === requestSequence) previewFailed.value = true;
  } finally {
    if (sequence === requestSequence) {
      loading.value = false;
      updateConfirmState();
    }
  }
}

const [Modal, modalApi] = useVbenModal({
  async onClosed() {
    requestSequence += 1;
    row.value = undefined;
    preview.value = undefined;
    previewFailed.value = false;
    loading.value = false;
    updateConfirmState();
  },
  async onConfirm() {
    if (
      !row.value ||
      !publicationPreviewCanConfirm(loading.value, preview.value)
    )
      return false;
    modalApi.lock(true);
    try {
      await catalogProductPublish(row.value.versionNo);
      message.success($t("plugin.idc-core.catalog.publish.succeeded"));
      emit("success");
      modalApi.close();
    } finally {
      modalApi.lock(false);
    }
  },
  onOpenChange(open) {
    if (!open) return;
    const data = modalApi.getData<{ row?: CatalogProductSummary }>();
    row.value = data?.row;
    const sequence = ++requestSequence;
    if (!row.value || row.value.versionStatus !== "draft") {
      previewFailed.value = true;
      updateConfirmState();
      return;
    }
    void loadPreview(row.value.versionNo, sequence);
  },
});
</script>

<template>
  <Modal
    :confirm-text="$t('plugin.idc-core.catalog.publish.confirmAction')"
    :title="$t('plugin.idc-core.catalog.publish.previewTitle')"
    class="w-[680px]"
  >
    <div class="publication-preview" data-testid="catalog-publication-preview">
      <div class="preview-identity">
        <div>
          <span>{{ $t("plugin.idc-core.catalog.publish.target") }}</span>
          <strong>{{ localizedName }}</strong>
        </div>
        <Tag color="blue">v{{ row?.versionNumber }}</Tag>
      </div>

      <Skeleton v-if="loading" :paragraph="{ rows: 4 }" active />
      <Alert
        v-else-if="previewFailed"
        :message="$t('plugin.idc-core.catalog.publish.previewFailed')"
        show-icon
        type="error"
      />
      <template v-else-if="preview">
        <Alert
          :message="
            $t(
              preview.ready
                ? 'plugin.idc-core.catalog.publish.ready'
                : 'plugin.idc-core.catalog.publish.blocked',
            )
          "
          show-icon
          :type="preview.ready ? 'success' : 'warning'"
        />
        <div class="check-list">
          <div
            v-for="check in preview.checks"
            :key="check.code"
            class="check-row"
            :data-testid="`catalog-publication-check-${check.code}`"
          >
            <div>
              <strong>{{
                $t(`plugin.idc-core.catalog.publish.check.${check.code}.title`)
              }}</strong>
              <span>{{
                $t(
                  `plugin.idc-core.catalog.publish.check.${check.code}.description`,
                )
              }}</span>
            </div>
            <Tag :color="check.passed ? 'success' : 'error'">
              {{
                $t(
                  check.passed
                    ? "plugin.idc-core.catalog.publish.passed"
                    : "plugin.idc-core.catalog.publish.failed",
                )
              }}
            </Tag>
          </div>
        </div>
        <p class="authority-note">
          {{ $t("plugin.idc-core.catalog.publish.recheckNote") }}
        </p>
      </template>
    </div>
  </Modal>
</template>

<style scoped>
.publication-preview {
  display: grid;
  gap: 16px;
}

.preview-identity {
  align-items: center;
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  display: flex;
  justify-content: space-between;
  padding: 14px 16px;
}

.preview-identity div {
  display: grid;
  gap: 2px;
}

.preview-identity span,
.check-row span,
.authority-note {
  color: hsl(var(--muted-foreground));
  font-size: 12px;
}

.preview-identity strong {
  font-size: 16px;
}

.check-list {
  border: 1px solid hsl(var(--border));
  border-radius: 0;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  overflow: hidden;
}

.check-row {
  align-items: center;
  display: flex;
  gap: 16px;
  justify-content: space-between;
  padding: 11px 14px;
}

.check-row:nth-child(n + 3) {
  border-top: 1px solid hsl(var(--border));
}

.check-row:nth-child(odd) {
  border-right: 1px solid hsl(var(--border));
}

.check-row > div {
  display: grid;
  gap: 2px;
}

.authority-note {
  margin: 0;
}

@media (max-width: 720px) {
  .check-list {
    grid-template-columns: 1fr;
  }

  .check-row:nth-child(n + 2) {
    border-top: 1px solid hsl(var(--border));
  }

  .check-row:nth-child(odd) {
    border-right: 0;
  }
}
</style>
