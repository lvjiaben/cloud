// Published quote states; no client-side price calculation or finality inference.
export function quoteStateText(status){
 if(status==='requires_customer_context')return '需登录用户中心确认价格';
 if(status==='pending_usage')return '按实际用量结算';
 return '报价暂不可用';
}
