import type { RouteRecordRaw } from 'vue-router';

const routes: RouteRecordRaw[] = [
  {
    component: () => import('#/views/account/finance/payment-return.vue'),
    meta: { hideInMenu: true, title: 'finance.paymentReturn.title' },
    name: 'CustomerPaymentReturn',
    path: '/payment/return/:attemptNo',
  },
  {
    component: () => import('#/views/account/overview/index.vue'),
    meta: {
      affixTab: true,
      icon: 'lucide:layout-dashboard',
      order: -1,
      title: 'page.customer.overview',
    },
    name: 'CustomerAccountOverview',
    path: '/account',
  },
  {
    meta: {
      icon: 'lucide:boxes',
      order: 10,
      title: 'menu.customerGroups.products',
    },
    name: 'CustomerProductsGroup',
    path: '/account/products',
    redirect: '/account/products/catalog',
    children: [
      {
        component: () => import('#/views/account/products/index.vue'),
        meta: {
          icon: 'lucide:shopping-bag',
          title: 'purchaseConfiguration.catalog.menu',
        },
        name: 'CustomerProductCatalog',
        path: '/account/products/catalog',
      },
      {
        component: () => import('#/views/account/products/configure.vue'),
        meta: { hideInMenu: true, title: 'purchaseConfiguration.title' },
        name: 'CustomerProductConfiguration',
        path: '/account/products/:publicSlug/configure',
      },
      {
        component: () => import('#/views/account/servers/index.vue'),
        meta: { icon: 'lucide:server', title: 'servers.menu' },
        name: 'CustomerServers',
        path: '/account/servers',
      },
      {
        component: () => import('#/views/account/servers/index.vue'),
        meta: { hideInMenu: true, title: 'servers.detail.title' },
        name: 'CustomerServerDetail',
        path: '/account/servers/:serviceNo',
      },
      {
        component: () => import('#/views/account/commerce/index.vue'),
        meta: { icon: 'lucide:shopping-cart', title: 'commerce.cart.menu' },
        name: 'CustomerCart',
        path: '/account/cart',
      },
      {
        component: () => import('#/views/account/commerce/index.vue'),
        meta: { icon: 'lucide:clipboard-list', title: 'commerce.orders.menu' },
        name: 'CustomerOrders',
        path: '/account/orders',
      },
      {
        component: () => import('#/views/account/commerce/index.vue'),
        meta: { hideInMenu: true, title: 'commerce.orders.menu' },
        name: 'CustomerOrderDetail',
        path: '/account/orders/:orderNo',
      },
      {
        component: () => import('#/views/account/marketing-coupons/index.vue'),
        meta: { icon: 'lucide:ticket-percent', title: 'marketingCoupons.menu' },
        name: 'CustomerMarketingCoupons',
        path: '/account/coupons',
      },
    ],
  },
  {
    meta: {
      icon: 'lucide:wallet-cards',
      order: 20,
      title: 'menu.customerGroups.finance',
    },
    name: 'CustomerFinanceGroup',
    path: '/account/finance-center',
    redirect: '/account/billing',
    children: [
      {
        component: () => import('#/views/account/finance/index.vue'),
        meta: { icon: 'lucide:wallet-cards', title: 'finance.views.overview' },
        name: 'CustomerFinance',
        path: '/account/billing',
      },
      {
        component: () => import('#/views/account/credit/index.vue'),
        meta: { icon: 'lucide:badge-dollar-sign', title: 'credit.menu' },
        name: 'CustomerCredit',
        path: '/account/billing/credit',
      },
      {
        component: () => import('#/views/account/finance/index.vue'),
        meta: { icon: 'lucide:receipt-text', title: 'finance.views.invoices' },
        name: 'CustomerInvoices',
        path: '/account/billing/invoices',
      },
      {
        component: () => import('#/views/account/finance/index.vue'),
        meta: { hideInMenu: true, title: 'finance.views.invoices' },
        name: 'CustomerInvoiceDetail',
        path: '/account/billing/invoices/:invoiceNo',
      },
      {
        component: () => import('#/views/account/finance/index.vue'),
        meta: {
          icon: 'lucide:arrow-left-right',
          title: 'finance.views.transactions',
        },
        name: 'CustomerTransactions',
        path: '/account/billing/transactions',
      },
      {
        component: () => import('#/views/account/finance/index.vue'),
        meta: { icon: 'lucide:calendar-sync', title: 'finance.views.renewals' },
        name: 'CustomerRenewals',
        path: '/account/billing/renewals',
      },
      {
        component: () => import('#/views/account/refunds/index.vue'),
        meta: { icon: 'lucide:rotate-ccw', title: 'refunds.menu' },
        name: 'CustomerRefunds',
        path: '/account/refunds',
      },
      {
        component: () => import('#/views/account/invoice-profiles/index.vue'),
        meta: { icon: 'lucide:contact-round', title: 'invoiceProfiles.menu' },
        name: 'CustomerInvoiceProfiles',
        path: '/account/invoice-profiles',
      },
      {
        component: () => import('#/views/account/tax-invoices/index.vue'),
        meta: { icon: 'lucide:receipt', title: 'taxInvoices.menu' },
        name: 'CustomerTaxInvoices',
        path: '/account/tax-invoices',
      },
      {
        component: () => import('#/views/account/sales-channel/index.vue'),
        meta: {
          icon: 'lucide:share-2',
          title: 'salesChannel.customer.menu',
        },
        name: 'CustomerReferrals',
        path: '/account/referrals',
      },
      {
        component: () => import('#/views/account/sales-channel/index.vue'),
        meta: {
          hideInMenu: true,
          title: 'salesChannel.customer.commission.menu',
        },
        name: 'CustomerCommissions',
        path: '/account/commissions',
      },
    ],
  },
  {
    meta: {
      hideInMenu: true,
      icon: 'lucide:user-round-cog',
      order: 30,
      title: 'menu.customerGroups.account',
    },
    name: 'CustomerAccountGroup',
    path: '/account/settings',
    redirect: '/account/profile',
    children: [
      {
        component: () => import('#/views/account/customer-profile/index.vue'),
        meta: {
          icon: 'lucide:user-round-cog',
          title: 'customerAccount.profile.menu',
        },
        name: 'CustomerProfile',
        path: '/account/profile',
      },
      {
        component: () => import('#/views/account/addresses/index.vue'),
        meta: { icon: 'lucide:map-pin-house', title: 'customerAddress.menu' },
        name: 'CustomerAddresses',
        path: '/account/addresses',
      },
    ],
  },
  {
    meta: {
      icon: 'lucide:shield-check',
      order: 35,
      title: 'menu.customerGroups.security',
    },
    name: 'CustomerSecurityGroup',
    path: '/account/security',
    redirect: '/account/ssh-keys',
    children: [
      {
        component: () => import('#/views/account/ssh-keys/index.vue'),
        meta: { icon: 'lucide:key-round', title: 'sshKeys.menu' },
        name: 'CustomerSSHPublicKeys',
        path: '/account/ssh-keys',
      },
    ],
  },
  {
    meta: {
      icon: 'lucide:scale',
      order: 37,
      title: 'menu.customerGroups.compliance',
    },
    name: 'CustomerComplianceGroup',
    path: '/account/compliance',
    redirect: '/account/data-rights',
    children: [
      {
        component: () => import('#/views/account/data-rights/index.vue'),
        meta: {
          icon: 'lucide:file-lock-2',
          title: 'customerAccount.data.menu',
        },
        name: 'CustomerDataRights',
        path: '/account/data-rights',
      },
      {
        component: () => import('#/views/account/privacy/index.vue'),
        meta: { icon: 'lucide:shield-check', title: 'privacy.menu' },
        name: 'CustomerPrivacy',
        path: '/account/privacy',
      },
      {
        component: () =>
          import('#/views/account/platform-verification/index.vue'),
        meta: {
          icon: 'lucide:badge-check',
          title: 'identity.platformMenu',
        },
        name: 'CustomerPlatformVerification',
        path: '/account/verification',
        children: [
          {
            component: () => import('#/views/account/identity/index.vue'),
            meta: { hideInMenu: true, title: 'identity.menu' },
            name: 'CustomerPersonalVerification',
            path: '',
            props: { embedded: true },
          },
          {
            component: () =>
              import('#/views/account/enterprise-verification/index.vue'),
            meta: {
              hideInMenu: true,
              title: 'enterpriseVerification.menu',
            },
            name: 'CustomerEnterpriseVerification',
            path: 'enterprise',
            props: { embedded: true },
          },
        ],
      },
      {
        component: () => import('#/views/account/contracts/center.vue'),
        meta: {
          icon: 'lucide:file-signature',
          title: 'contracts.center.menu',
        },
        name: 'CustomerContractCenter',
        path: '/account/contracts',
        children: [
          {
            component: () => import('#/views/account/contracts/index.vue'),
            meta: { hideInMenu: true, title: 'contracts.mine.menu' },
            name: 'CustomerContracts',
            path: '',
            props: { embedded: true },
          },
          {
            component: () => import('#/views/account/contracts/signers.vue'),
            meta: { hideInMenu: true, title: 'contracts.signers.menu' },
            name: 'CustomerContractSigners',
            path: 'signers',
            props: { embedded: true },
          },
        ],
      },
      {
        meta: { hideInMenu: true, title: 'identity.menu' },
        path: '/account/identity',
        redirect: '/account/verification',
      },
      {
        meta: { hideInMenu: true, title: 'enterpriseVerification.menu' },
        path: '/account/enterprise-verification',
        redirect: '/account/verification/enterprise',
      },
    ],
  },
];

export default routes;
