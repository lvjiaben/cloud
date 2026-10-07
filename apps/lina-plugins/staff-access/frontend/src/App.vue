<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue';
import type { PluginHostMountContext } from './host-context';
import { createRequest } from './request';

type Grant = { grantNo:string; userId:number; username:string; nickname:string; capabilityCode:string; scopeType:string; scopeKey:string; status:number; version:number; updatedAt:number; validFrom?:number; validUntil?:number };
type Candidate = { userId:number; username:string; nickname:string };
const props = defineProps<{ context: PluginHostMountContext }>();
const request = createRequest(() => props.context);
const zh = computed(() => props.context.locale.toLowerCase().startsWith('zh'));
const text = (cn:string,en:string) => zh.value ? cn : en;
const grants = ref<Grant[]>([]); const candidates = ref<Candidate[]>([]); const total = ref(0); const loading = ref(false); const error = ref(''); const success = ref(''); const keyword = ref('');
const form = reactive({ userId:0, capabilityCode:'idc.support.handle', scopeType:'department', scopeKey:'', enabled:true, reason:'', expectedVersion:0, validFrom:'', validUntil:'' });
const capabilityOptions = [
  ['idc.support.handle', '工单处理', 'Handle support tickets'],
  ['idc.support.lead', '工单升级负责人', 'Support escalation lead'],
  ['idc.support.internal-notify', '工单内部通知', 'Internal support notifications'],
];
const selected = computed(() => candidates.value.find((item) => item.userId === form.userId));
function query(values: Record<string,string|number>) { const q = new URLSearchParams(); Object.entries(values).forEach(([key,value]) => { if (String(value) !== '') q.set(key,String(value)); }); return q.toString(); }
async function load() { loading.value=true; error.value=''; try { const [grantPage,userPage] = await Promise.all([
  request<{items:Grant[];total:number}>(`/x/staff-access/api/v1/admin/staff-access/grants?${query({keyword:keyword.value,page:1,pageSize:100})}`),
  request<{items:Candidate[];total:number}>(`/x/staff-access/api/v1/admin/staff-access/user-candidates?${query({keyword:keyword.value,page:1,pageSize:100})}`),
]); grants.value=grantPage.items; total.value=grantPage.total; candidates.value=userPage.items; } catch { error.value=text('加载失败，请检查权限后重试。','Loading failed. Check your permission and retry.'); } finally { loading.value=false; } }
function localTime(value?:number) { if (!value) return ''; const date=new Date(value); const offset=date.getTimezoneOffset()*60000; return new Date(value-offset).toISOString().slice(0,16); }
function displayTime(value?:number) { return value ? new Intl.DateTimeFormat(props.context.locale,{dateStyle:'medium',timeStyle:'short'}).format(new Date(value)) : text('不限','No limit'); }
function edit(item:Grant) { Object.assign(form,{userId:item.userId,capabilityCode:item.capabilityCode,scopeType:item.scopeType,scopeKey:item.scopeKey,enabled:item.status===1,reason:'',expectedVersion:item.version,validFrom:localTime(item.validFrom),validUntil:localTime(item.validUntil)}); window.scrollTo({top:0,behavior:'smooth'}); }
function reset() { Object.assign(form,{userId:0,capabilityCode:'idc.support.handle',scopeType:'department',scopeKey:'',enabled:true,reason:'',expectedVersion:0,validFrom:'',validUntil:''}); }
async function save() { error.value=''; success.value=''; if (!form.userId || !form.reason.trim() || (form.scopeType==='department'&&!form.scopeKey.trim())) return; const validFrom=form.validFrom ? new Date(form.validFrom).getTime() : null; const validUntil=form.validUntil ? new Date(form.validUntil).getTime() : null; if (validFrom && validUntil && validUntil<=validFrom) { error.value=text('到期时间必须晚于生效时间。','Expiry must be later than activation.'); return; } loading.value=true; try { await request('/x/staff-access/api/v1/admin/staff-access/grants',{method:'PUT',body:JSON.stringify({...form,validFrom,validUntil})}); success.value=text('已保存，并记录审计证据。','Saved with audit evidence.'); reset(); await load(); } catch { error.value=text('保存失败。请确认最近 5 分钟内重新登录过，并刷新版本后再试。','Save failed. Sign in fully within the last five minutes and refresh the record before retrying.'); } finally { loading.value=false; } }
onMounted(load);
</script>

<template>
  <main class="workspace" data-testid="staff-access-page">
    <p v-if="error" class="notice error" data-testid="staff-access-error">{{ error }}</p><p v-if="success" class="notice success">{{ success }}</p>
    <section class="grid">
      <form class="panel form" @submit.prevent="save">
        <h2>{{ form.expectedVersion ? text('修改资格','Update qualification') : text('新增资格','Grant qualification') }}</h2>
        <label>{{ text('员工','Employee') }}<select v-model.number="form.userId" required><option :value="0" disabled>{{ text('选择可见且已启用的员工','Choose a visible enabled employee') }}</option><option v-for="item in candidates" :key="item.userId" :value="item.userId">{{ item.nickname || item.username }} · {{ item.username }}</option></select></label>
        <label>{{ text('业务职责','Business duty') }}<select v-model="form.capabilityCode"><option v-for="item in capabilityOptions" :key="item[0]" :value="item[0]">{{ zh ? item[1] : item[2] }} · {{ item[0] }}</option></select></label>
        <div class="two"><label>{{ text('范围','Scope') }}<select v-model="form.scopeType" @change="form.scopeType==='global'&&(form.scopeKey='')"><option value="department">{{ text('指定部门','Department') }}</option><option value="global">{{ text('全局','Global') }}</option></select></label><label v-if="form.scopeType==='department'">{{ text('部门编号','Department number') }}<input v-model.trim="form.scopeKey" required maxlength="128" placeholder="SUPPORT_L1" /></label></div>
        <label class="switch"><input v-model="form.enabled" type="checkbox" />{{ form.enabled ? text('启用','Enabled') : text('停用','Disabled') }}</label>
        <div class="two"><label>{{ text('生效时间（可不填）','Activation time (optional)') }}<input v-model="form.validFrom" type="datetime-local" /></label><label>{{ text('到期时间（可不填）','Expiry time (optional)') }}<input v-model="form.validUntil" type="datetime-local" /></label></div>
        <label>{{ text('修改原因','Reason') }}<textarea v-model.trim="form.reason" required minlength="3" maxlength="500" rows="3" /></label>
        <p class="hint">{{ text('敏感操作：必须是本人直接登录，并且完整登录时间在最近 5 分钟内。','Sensitive action: requires a direct session with a full login in the last five minutes.') }}</p>
        <div class="actions"><button class="primary" :disabled="loading">{{ text('保存','Save') }}</button><button v-if="form.expectedVersion" type="button" @click="reset">{{ text('取消','Cancel') }}</button></div>
        <p v-if="selected" class="selection">{{ text('当前选择：','Selected: ') }}{{ selected.nickname || selected.username }}</p>
      </form>
      <section class="panel list" data-testid="staff-access-list"><div class="toolbar"><input v-model.trim="keyword" :placeholder="text('搜索员工、职责或范围','Search employee, duty, or scope')" @keyup.enter="load" /><button :disabled="loading" @click="load">{{ text('查询','Search') }}</button><span class="count" data-testid="staff-access-total">{{ total }}</span></div>
        <article v-for="item in grants" :key="item.grantNo"><div><strong>{{ item.nickname || item.username || `#${item.userId}` }}</strong><code>{{ item.capabilityCode }}</code><p>{{ item.scopeType==='global' ? text('全局','Global') : `${text('部门','Department')} · ${item.scopeKey}` }} · {{ item.status===1 ? text('已启用','Enabled') : text('已停用','Disabled') }}</p><p>{{ text('生效','Active from') }}：{{ displayTime(item.validFrom) }} · {{ text('到期','Expires') }}：{{ displayTime(item.validUntil) }}</p></div><button @click="edit(item)">{{ text('编辑','Edit') }}</button></article>
        <p v-if="!grants.length && !loading" class="empty">{{ text('还没有符合条件的资格记录。','No matching qualifications.') }}</p>
      </section>
    </section>
  </main>
</template>
