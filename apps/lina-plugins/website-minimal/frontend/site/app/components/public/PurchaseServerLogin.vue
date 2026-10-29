<script setup lang="ts">
import { computed, ref } from 'vue';
import { Input as AInput, Button as AButton } from 'ant-design-vue';
import loginCopy from './purchase-server-login-copy.json';
import type { ServerLoginPolicy } from './purchase-configuration-client';
const props=defineProps<{policy:ServerLoginPolicy;hostname:string;password:string;locale:'zh-CN'|'en-US';disabled?:boolean;passwordConfigured?:boolean}>();
const emit=defineEmits<{'update:hostname':[value:string];'update:password':[value:string]}>();
const shown=ref(false);
const text=computed(()=>{const copy=loginCopy[props.locale];return {...copy,hint:props.passwordConfigured?copy.hintExisting:copy.hintNew};});
function generate(){
 const alphabets:Record<string,string>={lowercase:'abcdefghijklmnopqrstuvwxyz',uppercase:'ABCDEFGHIJKLMNOPQRSTUVWXYZ',digits:'0123456789',symbols:'!@#$%*-_=+?'};
 const groups=(props.policy.password?.characters||['lowercase','uppercase','digits']).map(key=>alphabets[key]).filter((value):value is string=>Boolean(value));
 if(!groups.length)return;
 const randomIndex=(size:number)=>{const values=new Uint32Array(1);const limit=Math.floor(4294967296/size)*size;do{crypto.getRandomValues(values);}while(values[0]!>=limit);return values[0]!%size;};
 const pick=(alphabet:string)=>alphabet[randomIndex(alphabet.length)]!;
 const length=props.policy.password?.length||12,characters=groups.map(pick),all=groups.join('');
 while(characters.length<length)characters.push(pick(all));
 for(let i=characters.length-1;i>0;i--){const j=randomIndex(i+1);[characters[i],characters[j]]=[characters[j]!,characters[i]!];}
 emit('update:password',characters.join(''));shown.value=true;
}
</script>
<template><section class="server-login" data-testid="purchase-server-login"><h3>{{text.title}}</h3><p>{{text.hint}}</p><label v-if="policy.hostnameEnabled" class="login-field"><span>{{text.hostname}}</span><AInput :aria-label="text.hostname" :value="hostname" :disabled="disabled" :maxlength="63" autocomplete="off" :placeholder="text.hint" @update:value="emit('update:hostname',String($event))"/></label><div v-if="policy.passwordEnabled" class="login-field"><label :for="'purchase-server-password'">{{text.password}}</label><div class="password-controls"><AInput id="purchase-server-password" :aria-label="text.password" :value="password" :disabled="disabled" :type="shown?'text':'password'" autocomplete="new-password" @update:value="emit('update:password',String($event))"/><AButton :disabled="disabled" @click="shown=!shown">{{shown?text.hide:text.show}}</AButton><AButton :disabled="disabled" @click="generate">{{text.generate}}</AButton></div></div></section></template>
<style scoped>
.server-login{margin-top:24px;padding-top:22px;border-top:1px solid #e2e8f0}.server-login h3{font-size:16px;font-weight:600;margin:0 0 8px}.server-login p{font-size:13px;color:#64748b;margin:0 0 18px;line-height:1.7}.login-field{display:flex;flex-direction:column;gap:8px;margin-bottom:18px}.password-controls{display:flex;gap:8px}.password-controls :deep(input){min-width:0}.password-controls :deep(button){flex-shrink:0}@media(max-width:480px){.password-controls{flex-wrap:wrap}.password-controls :deep(input){flex:1 0 100%}}
</style>
