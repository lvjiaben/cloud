<script lang="ts">
export const pluginPageMeta = {
  routePath: "/system/notice",
  title: "Notices",
};
</script>

<script setup lang="ts">
import type { Notice } from "./notice-client";

import { computed, onMounted, ref } from "vue";

import { Page, useVbenDrawer, useVbenModal } from "@vben/common-ui";

import { message, Modal, Popconfirm, Space } from "ant-design-vue";

import { useVbenVxeGrid } from "#/adapter/vxe-table";
import { $t } from "#/locales";
import { noticeDelete, noticeList } from "./notice-client";
import { DictTag } from "#/components/dict";
import { useDictStore } from "#/store/dict";

import { buildColumns, buildQuerySchema } from "./data";
import NoticeModal from "./notice-modal.vue";
import NoticePreviewModal from "./notice-preview-modal.vue";

const dictStore = useDictStore();
const noticeTypeDicts = ref<any[]>([]);
const noticeStatusDicts = ref<any[]>([]);

onMounted(async () => {
  [noticeTypeDicts.value, noticeStatusDicts.value] = await Promise.all([
    dictStore.getDictOptionsAsync("sys_notice_type"),
    dictStore.getDictOptionsAsync("sys_notice_status"),
  ]);
  gridApi.formApi.updateSchema([
    {
      fieldName: "type",
      componentProps: {
        options: noticeTypeDicts.value.map((item: any) => ({
          label: item.label,
          value: Number(item.value),
        })),
      },
    },
  ]);
});

const [NoticeModalRef, noticeModalApi] = useVbenDrawer({
  connectedComponent: NoticeModal,
  closeOnClickModal: false,
  modal: false,
});

const [NoticePreviewModalRef, noticePreviewModalApi] = useVbenModal({
  connectedComponent: NoticePreviewModal,
});

const [Grid, gridApi] = useVbenVxeGrid({
  formOptions: {
    schema: buildQuerySchema(),
    commonConfig: {
      labelWidth: 80,
      componentProps: {
        allowClear: true,
      },
    },
    wrapperClass: "grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4",
  },
  gridOptions: {
    checkboxConfig: {
      highlight: true,
      reserve: true,
    },
    columns: buildColumns(),
    height: "100%",
    keepSource: true,
    pagerConfig: {},
    proxyConfig: {
      ajax: {
        query: async (
          { page }: { page: { currentPage: number; pageSize: number } },
          formValues: Record<string, any> = {},
        ) => {
          return await noticeList({
            pageNum: page.currentPage,
            pageSize: page.pageSize,
            ...formValues,
          });
        },
      },
    },
    rowConfig: {
      keyField: "id",
    },
    id: "system-notice-index",
  },
  gridEvents: {
    checkboxChange: () => {
      checkedRows.value = (gridApi.grid?.getCheckboxRecords() ||
        []) as Notice[];
    },
    checkboxAll: () => {
      checkedRows.value = (gridApi.grid?.getCheckboxRecords() ||
        []) as Notice[];
    },
  },
});

const checkedRows = ref<Notice[]>([]);
const hasChecked = computed(() => checkedRows.value.length > 0);

function handleAdd() {
  noticeModalApi.setData({});
  noticeModalApi.open();
}

function handlePreview(row: Notice) {
  noticePreviewModalApi.setData({ id: row.id });
  noticePreviewModalApi.open();
}

function handleEdit(row: Notice) {
  noticeModalApi.setData({ id: row.id });
  noticeModalApi.open();
}

async function handleDelete(row: Notice) {
  await noticeDelete(String(row.id));
  message.success($t("pages.common.deleteSuccess"));
  await gridApi.query();
}

function handleMultiDelete() {
  const rows = gridApi.grid.getCheckboxRecords() as Notice[];
  const ids = rows.map((row) => row.id);
  Modal.confirm({
    title: $t("pages.common.confirmTitle"),
    okType: "danger",
    content: $t("plugin.content-notice.messages.deleteSelectedConfirm", {
      count: ids.length,
    }),
    onOk: async () => {
      await noticeDelete(ids);
      checkedRows.value = [];
      await gridApi.query();
    },
  });
}

function onReload() {
  gridApi.query();
}
</script>

<template>
  <Page :auto-content-height="true" content-class="p-0">
    <section class="notice-page" data-testid="notice-management-page">
      <div class="notice-list-panel">
        <Grid :table-title="$t('plugin.content-notice.tableTitle')">
          <template #toolbar-tools>
            <Space>
              <a-button
                :disabled="!hasChecked"
                danger
                type="primary"
                @click="handleMultiDelete"
              >
                {{ $t("pages.common.delete") }}
              </a-button>
              <a-button
                data-testid="notice-add"
                type="primary"
                @click="handleAdd"
              >
                {{ $t("pages.common.add") }}
              </a-button>
            </Space>
          </template>

          <template #type="{ row }">
            <DictTag :dicts="noticeTypeDicts" :value="String(row.type)" />
          </template>

          <template #status="{ row }">
            <DictTag :dicts="noticeStatusDicts" :value="String(row.status)" />
          </template>

          <template #action="{ row }">
            <Space>
              <ghost-button @click.stop="handlePreview(row)">
                {{ $t("pages.common.preview") }}
              </ghost-button>
              <ghost-button @click.stop="handleEdit(row)">
                {{ $t("pages.common.edit") }}
              </ghost-button>
              <Popconfirm
                placement="left"
                :title="$t('pages.common.deleteConfirm')"
                @confirm="handleDelete(row)"
              >
                <ghost-button danger @click.stop="">
                  {{ $t("pages.common.delete") }}
                </ghost-button>
              </Popconfirm>
            </Space>
          </template>
        </Grid>
      </div>
    </section>

    <NoticeModalRef @reload="onReload" />
    <NoticePreviewModalRef />
  </Page>
</template>

<style scoped>
.notice-page {
  background: hsl(var(--background-deep));
  height: 100%;
  min-height: 0;
  padding: 12px;
}

.notice-list-panel {
  background: hsl(var(--card));
  border: 1px solid hsl(var(--border));
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.notice-list-panel :deep(.vxe-grid) {
  height: 100%;
  min-height: 0;
}
</style>
