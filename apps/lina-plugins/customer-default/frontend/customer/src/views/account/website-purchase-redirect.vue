<script setup lang="ts">
// Legacy customer purchase links hand off to the configured public website.
import {onMounted,ref} from 'vue';
import {useRoute} from 'vue-router';
import {Alert,Button} from 'ant-design-vue';
import {$t} from '#/locales';
const route=useRoute(),failed=ref(false);
async function openWebsite(){failed.value=false;try{
 const response=await fetch('/__website/configuration');if(!response.ok)throw new Error('unavailable');
 const {origin}=await response.json();const path=route.name==='CustomerCart'?'/cart':route.params.publicSlug?'/products/'+encodeURIComponent(String(route.params.publicSlug)):'/products';
 const url=new URL(path,origin);if(!['http:','https:'].includes(url.protocol)||url.username||url.password)throw new Error('invalid');
 window.location.replace(url.toString());
}catch{failed.value=true;}}
onMounted(openWebsite);
</script>
<template><Alert :message="$t(failed?'purchaseConfiguration.messages.websiteUnavailable':'purchaseConfiguration.messages.websiteRedirect')" :type="failed?'error':'info'" show-icon><template v-if="failed" #action><Button @click="openWebsite">{{$t('purchaseConfiguration.catalog.retry')}}</Button></template></Alert></template>
