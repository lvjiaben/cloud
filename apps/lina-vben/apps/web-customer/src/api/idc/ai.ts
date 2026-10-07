import { pluginApiPath, requestClient } from '#/api/request';

export type CustomerAssistantStatus = { enabled: boolean };
export type CustomerAssistantReply = {
  answer: string;
  navigation: string;
  dataAsOf: string;
  uncertainty: string;
};

const statusPath = pluginApiPath('idc-ai', 'customer/ai/status');
const chatPath = pluginApiPath('idc-ai', 'customer/ai/chat');

export function getCustomerAssistantStatus() {
  return requestClient.get<CustomerAssistantStatus>(statusPath);
}

export function askCustomerAssistant(prompt: string) {
  return requestClient.post<CustomerAssistantReply>(chatPath, { prompt });
}
