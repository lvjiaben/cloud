<script setup lang="ts">
// A same-origin website popup reuses the complete customer authentication flow.
// Never deliver credentials to another origin or through a URL.
import { onMounted } from 'vue';
import { useRoute } from 'vue-router';
import { useAccessStore } from '@vben/stores';
import { $t } from '#/locales';
const route = useRoute();
onMounted(async () => {
  const returning=sessionStorage.getItem('lina.website.return');
  if(returning&&useAccessStore().accessToken){
    sessionStorage.removeItem('lina.website.return');
    try{
      const response=await fetch('/__website/configuration');
      if(!response.ok)return;
      const configuration=await response.json(),website=new URL(configuration.origin),target=new URL(returning);
      if((Array.isArray(configuration.origins)?configuration.origins:[website.origin]).includes(target.origin)&&/^\/(?:en-US\/)?(?:$|cart|checkout|products(?:\/[^/]+)?|pages\/[^/]+)\/?$/.test(target.pathname))window.location.replace(target.href);
    }catch{/* Refuse invalid return destinations. */}
    return;
  }
  const nonce = String(route.query.nonce ?? window.name.replace(/^lina-website-login-/, ''));
  if (!/^[a-zA-Z0-9-]{20,80}$/.test(nonce) || !window.opener || !useAccessStore().accessToken) return;
  try {
    if (window.opener.location.origin !== window.location.origin) return;
    window.opener.postMessage({type:'lina-website-session',nonce,accessToken:useAccessStore().accessToken},window.location.origin);
    window.close();
  } catch { /* A cross-origin opener is deliberately refused. */ }
});
</script>
<template><p>{{ $t('purchaseConfiguration.messages.loginCompleted') }}</p></template>
