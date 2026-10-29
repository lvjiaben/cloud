<script setup lang="ts">
import { computed, ref } from "vue";
import { useVbenModal } from "@vben/common-ui";
import { Alert, Checkbox, Tag, Upload, message } from "ant-design-vue";
import {
  authorization,
  detail,
  installAndStart,
  preview,
  upgrade,
  upload,
  type Dependencies,
  type HostService,
  type PluginDetail,
  type PluginRow,
  type UpgradePreview,
} from "../plugin-client";

type Mode = "upgrade" | "upload";
const emit = defineEmits<{ reload: [] }>();
const mode = ref<Mode>("upgrade");
const row = ref<PluginRow>();
const current = ref<PluginDetail>();
const check = ref<Dependencies>();
const target = ref<UpgradePreview>();
const loading = ref(false);
const loadFailed = ref(false);
const reviewed = ref(false);
const file = ref<File>();
const submitting = ref(false);
const uploaded = ref<PluginRow>();
let generation = 0;
const names: Record<Mode, string> = {
  upgrade: "更新",
  upload: "安装插件",
};
const title = computed(() =>
  mode.value === "upload"
    ? "安装插件"
    : `${names[mode.value]} ${row.value?.name ?? "插件"}`,
);
const requested = computed<HostService[]>(() =>
  mode.value === "upgrade"
    ? (target.value?.toManifest?.requestedHostServices ?? [])
    : (current.value?.requestedHostServices ?? []),
);
const needsReview = computed(
  () =>
    mode.value === "upgrade" &&
    target.value?.hostServicesDiff?.authorizationRequired === true,
);
const blockers = computed(() => {
  const c = check.value;
  if (!c) return [];
  return [
    ...(c.blockers ?? []).map(
      (item) => item.dependencyId || item.pluginId || "依赖检查未通过",
    ),
    ...(c.cycle ?? []),
    ...(c.framework?.status === "unsatisfied"
      ? ["当前框架版本不满足插件要求"]
      : []),
  ];
});
const disabled = computed(
  () =>
    loading.value ||
    loadFailed.value ||
    submitting.value ||
    blockers.value.length > 0 ||
    (needsReview.value && !reviewed.value) ||
    (mode.value === "upload" && !file.value),
);
const [Modal, api] = useVbenModal({
  onOpenChange: async (open) => {
    if (!open) {
      generation++;
      return;
    }
    const data = api.getData<{ mode: Mode; row?: PluginRow }>();
    loading.value = false;
    mode.value = data.mode;
    row.value = data.row;
    current.value = undefined;
    check.value = undefined;
    target.value = undefined;
    reviewed.value = false;
    file.value = undefined;
    loadFailed.value = false;
    uploaded.value = undefined;
    if (mode.value !== "upload") await load();
  },
  onConfirm: confirm,
});
async function load() {
  if (!row.value) return;
  const requestGeneration = ++generation;
  loading.value = true;
  loadFailed.value = false;
  try {
    const plugin = await detail(row.value.id);
    if (requestGeneration !== generation) return;
    if (mode.value === "upgrade") {
      const result = await preview(row.value.id);
      if (requestGeneration !== generation) return;
      target.value = result;
      check.value = result.dependencyCheck;
    }
    current.value = plugin;
  } catch {
    if (requestGeneration === generation) loadFailed.value = true;
  } finally {
    if (requestGeneration === generation) loading.value = false;
  }
}
function chooseFile(selected: File) {
  file.value = selected;
  uploaded.value = undefined;
  return false;
}
async function confirm() {
  if (disabled.value) return;
  submitting.value = true;
  api.lock(true);
  try {
    if (mode.value === "upload") {
      if (!uploaded.value) {
        const result = await upload(file.value!);
        if (result.installed === 1) {
          message.success("插件包已上传，可在列表中更新");
          emit("reload");
          api.close();
          return;
        }
        uploaded.value = result;
      }
      await installAndStart(uploaded.value.id);
      message.success("插件已安装并启用");
      emit("reload");
      api.close();
      return;
    }
    if (!row.value || !current.value) return;
    await upgrade(
      row.value.id,
      needsReview.value ? authorization(requested.value) : {},
    );
    message.success(`插件已${names[mode.value]}`);
    emit("reload");
    api.close();
  } catch {
    emit("reload");
  } finally {
    api.lock(false);
    submitting.value = false;
  }
}
</script>

<template>
  <Modal
    :title="title"
    :confirm-disabled="disabled"
    :confirm-text="mode === 'upload' ? '上传并安装' : names[mode]"
    class="w-[620px]"
  >
    <div data-testid="plugin-action-modal">
      <div v-if="mode === 'upload'" class="space-y-4">
        <p>上传动态插件包，校验通过后自动安装并启用。</p>
        <Upload.Dragger
          accept=".wasm"
          :max-count="1"
          :before-upload="chooseFile"
          @remove="
            () => {
              file = undefined;
              return true;
            }
          "
        >
          <p class="py-6">点击或拖拽 .wasm 插件包到这里</p>
        </Upload.Dragger>
      </div>
      <div v-else class="space-y-4">
        <a-spin v-if="loading" />
        <Alert
          v-if="loadFailed"
          type="error"
          message="插件检查失败，无法继续操作。"
          show-icon
        >
          <template #action
            ><a-button size="small" @click="load">重试</a-button></template
          >
        </Alert>
        <p v-if="row" class="text-muted-foreground">{{ row.id }}</p>
        <p v-if="target">
          版本：{{ target.effectiveVersion }} → {{ target.discoveredVersion }}
        </p>
        <Alert
          v-if="blockers.length"
          type="error"
          message="请先处理以下依赖，再重试。"
          show-icon
        >
          <template #description
            ><ul>
              <li v-for="blocker in blockers" :key="blocker">{{ blocker }}</li>
            </ul></template
          >
        </Alert>
        <template v-if="needsReview">
          <p class="font-medium">该插件需要以下访问权限，请确认后继续。</p>
          <div
            v-for="service in requested"
            :key="`${service.owner || ''}:${service.service}`"
            class="rounded border p-3"
          >
            <p>
              {{ service.owner ? `${service.owner} / ` : ""
              }}{{ service.service }}
              <Tag v-if="service.version">{{ service.version }}</Tag>
            </p>
            <p class="mt-2 text-sm break-all">
              操作：{{ service.methods.join("、") }}
            </p>
            <p v-if="service.paths?.length" class="mt-1 text-sm break-all">
              存储范围：{{ service.paths.join("、") }}
            </p>
            <p v-if="service.tables?.length" class="mt-1 text-sm break-all">
              数据范围：{{ service.tables.join("、") }}
            </p>
            <p v-if="service.resources?.length" class="mt-1 text-sm break-all">
              资源范围：{{
                service.resources.map((item) => item.ref).join("、")
              }}
            </p>
          </div>
          <Checkbox
            v-model:checked="reviewed"
            data-testid="plugin-authorization-confirm"
            >我已确认上述权限范围</Checkbox
          >
        </template>
      </div>
    </div>
  </Modal>
</template>
