<script setup lang="ts">
// Public bridge: only the operator-configured website may receive this session.
// Reading a session never triggers login; an explicit login stays in this tab.
import { onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useAccessStore } from '@vben/stores';
import { Alert, Button } from 'ant-design-vue';
import { getUserInfoApi } from '#/api/core/user';
import { $t } from '#/locales';
const route=useRoute(),router=useRouter();
const failed=ref(false);
async function handoff(){
 failed.value=false;
 const query={...route.query};
 try{
  const response=await fetch('/__website/configuration',{credentials:'same-origin'});
  if(!response.ok)return;
  const config=await response.json();
  const allowed=Array.isArray(config.origins)?config.origins:[config.origin];
  const requested=String(query.websiteOrigin||config.origin);
  if(!allowed.includes(requested))return;
  const website=new URL(requested);
  if(!['http:','https:'].includes(website.protocol)||website.username||website.password)return;
  const nonce=String(query.nonce||'');
  const store=useAccessStore();
  if(store.accessToken){
   try{await getUserInfoApi();}catch{
    // The ordinary customer request client refreshes the access token or
    // clears an expired session. Network failures must not discard a session.
    if(store.accessToken){failed.value=true;return;}
   }
  }
  if(window.parent!==window&&/^[a-zA-Z0-9-]{20,80}$/.test(nonce)){
   window.parent.postMessage({type:'lina-website-session',nonce,accessToken:store.accessToken||''},website.origin);
   return;
  }
  if(query.login!=='1'&&query.probe!=='1')return;
  const path=String(query.returnTo||'/cart');
  const target=new URL(path,website.origin);
  if(target.origin!==website.origin||!/^\/(?:en-US\/)?(?:$|cart|checkout|products(?:\/[^/]+)?|pages\/[^/]+)\/?$/.test(target.pathname))return;
  if(query.probe==='1'||store.accessToken){
    // Form POST transfers credentials outside URLs and works when browsers
    // partition cross-site iframe storage. The gateway validates this origin.
    const form=document.createElement('form');form.method='POST';form.action=website.origin+'/__website/session';
    for(const [name,value] of Object.entries({accessToken:store.accessToken||'',returnTo:target.pathname+target.search})){
      const input=document.createElement('input');input.type='hidden';input.name=name;input.value=value;form.append(input);
    }
    document.body.append(form);form.submit();return;
  }
  sessionStorage.setItem('lina.website.return',target.href);
  await router.replace({path:'/auth/login',query:{redirect:encodeURIComponent('/account/website-purchase')}});
 }catch{failed.value=true;}
}
onMounted(handoff);
</script>
<template><Alert v-if="failed" :message="$t('purchaseConfiguration.messages.websiteUnavailable')" type="error" show-icon><template #action><Button @click="handoff">{{$t('purchaseConfiguration.catalog.retry')}}</Button></template></Alert><span v-else aria-hidden="true" /></template>
