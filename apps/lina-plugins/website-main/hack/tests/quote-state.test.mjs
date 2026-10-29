import {test} from 'node:test';
import assert from 'node:assert/strict';
import {quoteStateText} from '../../frontend/public/lina/quote-state.js';

test('metered prices do not promise that login yields a final amount',()=>{
 assert.equal(quoteStateText('pending_usage'),'按实际用量结算');
 assert.equal(quoteStateText('requires_customer_context'),'需登录用户中心确认价格');
 assert.equal(quoteStateText('unknown'),'报价暂不可用');
});
