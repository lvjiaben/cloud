<script setup lang="ts">
import { computed } from 'vue';
import { Slider as ASlider, Input as AInput, Button as AButton } from 'ant-design-vue';
const props=defineProps<{minimum:string;maximum:string;step:string;value:string;disabled:boolean;label:string;unit?:string;locale:'zh-CN'|'en-US'}>();
const emit=defineEmits<{'update:value':[value:string]}>();
const min=computed(()=>Number(props.minimum));const step=computed(()=>Number(props.step)||1);
const max=computed(()=>min.value+Math.floor((Number(props.maximum)-min.value)/step.value)*step.value);
const current=computed(()=>Number(props.value||props.minimum));
function move(direction:number){emit('update:value',String(Math.min(max.value,Math.max(min.value,min.value+Math.round((current.value-min.value)/step.value+direction)*step.value))));}
</script>
<template><div class="quantity-slider"><div class="quantity-range"><ASlider :disabled="disabled" :min="min" :max="max" :step="step" :value="current" :aria-label="label" @update:value="emit('update:value',String($event))"/><div class="range-labels" aria-hidden="true"><span>{{minimum}} {{unit}}</span><span>{{max}} {{unit}}</span></div></div><div class="quantity-entry"><AButton :aria-label="(locale==='zh-CN'?'减少':'Decrease')+' '+label" :disabled="disabled||current<=min" @click="move(-1)">−</AButton><AInput type="number" :aria-label="label" :disabled="disabled" :min="minimum" :max="max" :step="step" :value="value" @update:value="emit('update:value',String($event))"/><span v-if="unit" class="quantity-unit">{{unit}}</span><AButton :aria-label="(locale==='zh-CN'?'增加':'Increase')+' '+label" :disabled="disabled||current>=max" @click="move(1)">+</AButton></div></div></template>
<style scoped>
.quantity-slider{display:flex;align-items:center;gap:24px}.quantity-range{min-width:0;flex:1}.quantity-range :deep(.ant-slider-track){background:#1677ff}.range-labels{display:flex;justify-content:space-between;font-size:12px;color:#64748b;gap:12px}.quantity-entry{display:flex;align-items:center;gap:6px;width:218px;flex-shrink:0}.quantity-entry :deep(input){text-align:center;min-width:0}.quantity-entry :deep(input::-webkit-inner-spin-button){appearance:none}.quantity-unit{color:#64748b;white-space:nowrap;font-size:13px}.quantity-entry :deep(button){padding-inline:10px}@media(max-width:600px){.quantity-slider{align-items:stretch;flex-direction:column;gap:12px}.quantity-entry{width:100%;max-width:280px}}
</style>
