import type { RouteRecordRaw } from 'vue-router';

const routes: RouteRecordRaw[] = [
  {
    meta: {
      icon: 'lucide:headphones',
      order: 40,
      title: 'menu.customerGroups.support',
    },
    name: 'CustomerServiceGroup',
    path: '/account/customer-service',
    redirect: '/account/support',
    children: [
      {
        component: () => import('#/views/account/support/index.vue'),
        meta: { icon: 'lucide:messages-square', title: 'support.menu' },
        name: 'CustomerSupportTickets',
        path: '/account/support',
      },
      {
        component: () => import('#/views/account/support/create.vue'),
        meta: { hideInMenu: true, title: 'support.createTitle' },
        name: 'CustomerSupportTicketCreate',
        path: '/account/support/new',
      },
      {
        component: () => import('#/views/account/support/detail.vue'),
        meta: { hideInMenu: true, title: 'support.detailTitle' },
        name: 'CustomerSupportTicketDetail',
        path: '/account/support/:ticketNo',
      },
      {
        component: () => import('#/views/account/ai/index.vue'),
        meta: { icon: 'lucide:sparkles', title: 'aiAssistant.menu' },
        name: 'CustomerAIAssistant',
        path: '/account/ai',
      },
    ],
  },
];

export default routes;
