const configuredBaseURL =
  process.env.E2E_SITE_BASE_URL?.trim() || 'http://127.0.0.1:6680';
const configuredCustomerURL =
  process.env.E2E_CUSTOMER_BASE_URL?.trim() || 'http://127.0.0.1:6677';

export const testRuntime = {
  baseURL: new URL(configuredBaseURL).toString(),
  customerURL: new URL(configuredCustomerURL).toString(),
};
