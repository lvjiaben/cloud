<script setup lang="ts">
import { computed } from 'vue';
import { Select as ASelect } from 'ant-design-vue';
import type { PurchaseConfigurationOption } from './purchase-configuration-client';
const props=defineProps<{options:PurchaseConfigurationOption[];value?:string;disabled:boolean;label:string;locale:'zh-CN'|'en-US'}>();
const emit=defineEmits<{'update:value':[value:string]}>();
const families=[['windows','Windows'],['centos','CentOS'],['ubuntu','Ubuntu'],['debian','Debian'],['rocky','Rocky Linux'],['alma','AlmaLinux'],['fedora','Fedora'],['alpine','Alpine Linux'],['arch','Arch Linux'],['opensuse','openSUSE'],['oracle','Oracle Linux'],['other','Other']] as const;
const patterns:Record<string,RegExp>={windows:/\bwindows\b/i,centos:/\bcentos(?=\b|\d)/i,ubuntu:/\bubuntu\b/i,debian:/\bdebian\b/i,rocky:/\brocky\b/i,alma:/\balma\s*linux\b/i,fedora:/\bfedora\b/i,alpine:/\balpine\b/i,arch:/\barch(?:\s*linux)?\b/i,opensuse:/\bopen\s*suse\b/i,oracle:/\boracle\s*linux\b/i};
const groups=computed(()=>{
 const result:Array<{id:string;name:string;options:PurchaseConfigurationOption[]}>=[];
 for(const option of props.options){
  const name=option.labelEnUS+' '+option.labelZhCN;
  const [id,title]=families.find(([key])=>patterns[key]?.test(name))||families[families.length-1]!;
  let group=result.find(item=>item.id===id);
  if(!group){group={id,name:id==='other'&&props.locale==='zh-CN'?'其他系统':title,options:[]};result.push(group);}
  group.options.push(option);
 }
 return result;
});
const selected=computed(()=>groups.value.find(group=>group.options.some(option=>option.optionToken===props.value)));
function optionLabel(option:PurchaseConfigurationOption){return props.locale==='zh-CN'?option.labelZhCN:option.labelEnUS;}
</script>
<template>
 <div class="os-selector" data-testid="purchase-os-selector">
  <div class="os-families" role="group" :aria-label="label">
   <button v-for="group in groups" :key="group.id" type="button" class="os-family" :class="{selected:selected?.id===group.id}" :disabled="disabled" :aria-pressed="selected?.id===group.id" :data-testid="'purchase-os-family-'+group.id" @click="selected?.id!==group.id && emit('update:value',group.options[0]!.optionToken)">
    <svg v-if="group.id==='windows'" viewBox="0 0 32 32" class="os-icon windows" aria-hidden="true"><path fill="currentColor" d="M3 6l11-1.5v11H3zm13-1.8L29 2v13.5H16zM3 17h11v11L3 26.5zm13 0h13v13L16 28.2z"/></svg>
    <svg v-else-if="group.id==='ubuntu'" viewBox="0 0 32 32" class="os-icon ubuntu" aria-hidden="true"><circle cx="16" cy="16" r="15" fill="currentColor"/><circle cx="16" cy="16" r="7" fill="none" stroke="white" stroke-width="3"/><g fill="white" stroke="#e95420" stroke-width="2"><circle cx="16" cy="7" r="3"/><circle cx="8.2" cy="20.5" r="3"/><circle cx="23.8" cy="20.5" r="3"/></g></svg>
    <svg v-else-if="group.id==='debian'" viewBox="0 0 32 32" class="os-icon debian" aria-hidden="true"><path d="M17 29C5 24 5 9 15 5c11-4 15 10 8 14-6 4-12-3-8-7 3-3 6 0 5 2" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/></svg>
    <svg v-else-if="group.id==='rocky'" viewBox="0 0 32 32" class="os-icon rocky" aria-hidden="true"><circle cx="16" cy="16" r="15" fill="currentColor"/><path d="M5 24l12-14 10 11-7-5-7 9z" fill="white"/></svg>
    <svg v-else viewBox="0 0 32 32" class="os-icon" :class="group.id" aria-hidden="true"><rect x="2" y="2" width="28" height="28" rx="8" fill="currentColor"/><text x="16" y="22" text-anchor="middle" font-size="17" font-weight="700" fill="white">{{group.id==='other'?'L':group.name.charAt(0)}}</text></svg>
    <span>{{group.name}}</span><span v-if="selected?.id===group.id" class="selected-check" aria-hidden="true">✓</span>
   </button>
  </div>
  <ASelect v-if="selected" class="os-version" :aria-label="locale==='zh-CN'?'系统版本':'System version'" :disabled="disabled" :value="value" :options="selected.options.map(option=>({value:option.optionToken,label:optionLabel(option)}))" @update:value="emit('update:value',String($event))" />
 </div>
</template>
<style scoped>
.os-families{display:grid;grid-template-columns:repeat(auto-fill,minmax(135px,1fr));gap:10px}.os-family{position:relative;display:flex;align-items:center;gap:10px;min-height:64px;padding:12px;border:1px solid #dfe5ed;border-radius:6px;background:#fff;color:#334155;text-align:left;cursor:pointer;transition:border-color .16s,background .16s}.os-family:hover:not(:disabled),.os-family.selected{border-color:var(--ant-primary-color,#1677ff);background:#f0f7ff}.os-family:focus-visible{outline:2px solid #1677ff;outline-offset:2px}.os-family:disabled{cursor:not-allowed;opacity:.5}.os-icon{width:30px;height:30px;flex-shrink:0;color:#64748b}.windows{color:#0078d4}.ubuntu{color:#e95420}.debian{color:#a80030}.rocky{color:#10a878}.centos{color:#932279}.alma{color:#f79825}.fedora{color:#3c6eb4}.alpine{color:#0d597f}.arch{color:#1793d1}.opensuse{color:#73ba25}.oracle{color:#c74634}.selected-check{position:absolute;right:5px;top:2px;color:#1677ff;font-size:12px}.os-version{width:100%;margin-top:12px;max-width:440px}@media(max-width:480px){.os-families{grid-template-columns:repeat(2,minmax(0,1fr))}.os-family{font-size:13px;padding:10px 8px;gap:8px}}
</style>
