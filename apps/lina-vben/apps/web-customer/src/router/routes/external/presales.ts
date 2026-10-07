import type { RouteRecordRaw } from 'vue-router';

const routes: RouteRecordRaw[] = [
  {
    children: [
      {
        component: () => import('#/views/public/presales/create.vue'),
        meta: {
          hideInBreadcrumb: true,
          hideInMenu: true,
          hideInTab: true,
          title: 'presales.createTitle',
        },
        name: 'PublicPresalesCreate',
        path: '',
      },
      {
        component: () => import('#/views/public/presales/detail.vue'),
        meta: {
          hideInBreadcrumb: true,
          hideInMenu: true,
          hideInTab: true,
          title: 'presales.detailTitle',
        },
        name: 'PublicPresalesAccess',
        path: 'access/:token',
      },
    ],
    component: () => import('#/layouts/auth.vue'),
    meta: {
      hideInBreadcrumb: true,
      hideInMenu: true,
      hideInTab: true,
      ignoreAccess: true,
      title: 'presales.menu',
    },
    name: 'PublicPresales',
    path: '/auth/presales',
  },
];

export default routes;
