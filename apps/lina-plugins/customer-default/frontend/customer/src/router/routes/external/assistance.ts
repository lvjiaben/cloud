import type { RouteRecordRaw } from 'vue-router';

const routes: RouteRecordRaw[] = [
  {
    component: () => import('#/views/account/assistance/index.vue'),
    meta: {
      hideInBreadcrumb: true,
      hideInMenu: true,
      hideInTab: true,
      ignoreAccess: true,
      title: 'assistance.title',
    },
    name: 'CustomerAssistance',
    path: '/account/assistance',
  },
];

export default routes;
