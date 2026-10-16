<script lang="ts" setup>
import { VbenAvatar } from '@vben-core/shadcn-ui';

interface Props {
  avatar?: string;
  projectLabel?: string;
  projectValue?: number | string;
  teamLabel?: string;
  teamValue?: number | string;
  todoLabel?: string;
  todoValue?: number | string;
}

defineOptions({
  name: 'WorkbenchHeader',
});

withDefaults(defineProps<Props>(), {
  avatar: '',
  projectLabel: 'Projects',
  projectValue: 8,
  teamLabel: 'Team',
  teamValue: 300,
  todoLabel: 'To-dos',
  todoValue: '2/10',
});
</script>
<template>
  <div class="card-box p-4 py-6 lg:flex">
    <VbenAvatar :src="avatar" class="size-20" />
    <div
      v-if="$slots.title || $slots.description"
      class="flex flex-col justify-center md:mt-0 md:ml-6"
    >
      <h1 v-if="$slots.title" class="text-md font-semibold md:text-xl">
        <slot name="title"></slot>
      </h1>
      <span v-if="$slots.description" class="mt-1 text-foreground/80">
        <slot name="description"></slot>
      </span>
    </div>
    <div class="mt-4 flex flex-1 justify-end md:mt-0">
      <div class="flex flex-col justify-center text-right">
        <span class="text-foreground/80">{{ todoLabel }}</span>
        <span class="text-2xl">{{ todoValue }}</span>
      </div>

      <div class="mx-12 flex flex-col justify-center text-right md:mx-16">
        <span class="text-foreground/80">{{ projectLabel }}</span>
        <span class="text-2xl">{{ projectValue }}</span>
      </div>
      <div class="mr-4 flex flex-col justify-center text-right md:mr-10">
        <span class="text-foreground/80">{{ teamLabel }}</span>
        <span class="text-2xl">{{ teamValue }}</span>
      </div>
    </div>
  </div>
</template>
