"""Public API setup and assertions shared by TC011's owned dual-instance test."""
from urllib.parse import urlencode
from pathlib import Path
import base64
import hashlib
import json
import secrets
import signal
import time
from isolated import PREFIX, APIError

CHANNEL = '/x/idc-channel/api/v1/channel'


def key(): return 'tc011-' + secrets.token_hex(16)


def customer(api):
    stamp=secrets.token_hex(16)
    account=api.call(PREFIX+'/customers/direct-accounts','POST',{
        'customerType':'individual','displayName':'公开协议隔离验收','countryCode':'CN',
        'preferredLocale':'zh-CN','email':f'tc011-{stamp}@example.test',
        'initialPassword':secrets.token_urlsafe(32),'reason':'本地公开协议隔离测试前置',
        'idempotencyKey':key()})
    category=api.call(PREFIX+'/catalog/categories','POST',{
        'nameZhCn':'公开协议隔离测试','nameEnUs':'Isolated public protocol','sortOrder':999})['category']
    return account,category


def credential(api,account):
    return api.call('/x/idc-channel/api/v1/admin/channel/tokens','POST',{
        'customerNo':account['customerNo'],'displayName':'TC011 owned access',
        'scopes':['capability.read','catalog.read','order.create','order.read','service.read',
                  'service.renew','settlement.read'],'sourceCidrs':[]})


def fund(instances,database,account,amount=100000):
    assert isinstance(amount,int) and amount>0
    number=account['customerNo'];stamp=secrets.token_hex(16)
    balance='bac_'+hashlib.sha256(('customer\0'+number+'\0customer_balance\0CNY').encode()).hexdigest()[:32]
    # This is an explicitly balanced recharge fixture, not a fake payment or procurement.
    instances.sql(database,f"""BEGIN;
    UPDATE plugin_idc_core_customer_individual_profile SET legal_name='Local protocol test',
      legal_name_masked='L**',identity_identifier_masked='test-{stamp}',identity_lookup_digest=repeat('a',64),
      identity_status_code='verified',identity_verified_at=CURRENT_TIMESTAMP,identity_case_no='CAS_{stamp}'
      WHERE customer_no='{number}';
    INSERT INTO plugin_idc_core_billing_ledger_account(account_no,owner_type_code,owner_reference,customer_no,account_type_code,normal_balance_code,currency_code)
      VALUES('{balance}','customer','{number}','{number}','customer_balance','credit','CNY'),
      ('cas_{stamp}','merchant','test-{stamp}',NULL,'merchant_cash','debit','CNY');
    INSERT INTO plugin_idc_core_billing_ledger_transaction(transaction_no,transaction_type_code,currency_code,source_type_code,source_reference,source_snapshot_digest,entry_count,debit_total_minor,credit_total_minor,content_digest)
      VALUES('btx_{stamp}','recharge','CNY','test.recharge','{stamp}',repeat('a',64),2,{amount},{amount},repeat('b',64));
    INSERT INTO plugin_idc_core_billing_ledger_entry(entry_no,transaction_no,currency_code,line_number,account_no,direction_code,entry_role_code,amount_minor,reason_code,content_digest)
      VALUES('cre_{stamp}','btx_{stamp}','CNY',1,'{balance}','credit','balance',{amount},'test.recharge',repeat('a',64)),
      ('deb_{stamp}','btx_{stamp}','CNY',2,'cas_{stamp}','debit','cash',{amount},'test.recharge',repeat('b',64));
    INSERT INTO plugin_idc_core_billing_balance_snapshot(snapshot_no,account_no,currency_code,normal_balance_code,transaction_no,sequence_no,opening_balance_minor,debit_amount_minor,credit_amount_minor,closing_balance_minor,content_digest)
      VALUES('bal_{stamp}','{balance}','CNY','credit','btx_{stamp}',1,0,0,{amount},{amount},repeat('a',64)),
      ('bca_{stamp}','cas_{stamp}','CNY','debit','btx_{stamp}',1,0,{amount},0,{amount},repeat('b',64)); COMMIT;""")
    return balance


def balance(instances,database,account):
    return int(instances.sql(database,"SELECT closing_balance_minor FROM plugin_idc_core_billing_balance_snapshot "
        f"WHERE account_no='{account}' ORDER BY sequence_no DESC LIMIT 1"))


def install(api,plugin,package):
    # This run's exact package was staged in the isolated runtime before start.
    # Respect formal lifecycle when the seed has a previous fixture release.
    item=next(p for p in api.call('/api/v1/plugins?pageSize=100')['list'] if p['id']==plugin)
    if not item['installed']:
        api.call(f'/api/v1/plugins/{plugin}/install','POST',{'installMockData':False,'installMode':'global'})
    elif item.get('runtimeState') in ['pending_upgrade','upgrade_failed']:
        api.call(f'/api/v1/plugins/{plugin}/upgrade','POST',{'confirmed':True})
    item=next(p for p in api.call('/api/v1/plugins?pageSize=100')['list'] if p['id']==plugin)
    if not item['enabled']:api.call(f'/api/v1/plugins/{plugin}/enable','PUT',{})
    if item.get('newBusinessState')=='stopped':api.call(f'/api/v1/plugins/{plugin}/new-business/resume','PUT',{})


def product(api,category,interface=None):
    slug='tc011-'+secrets.token_hex(16)
    p=api.call(PREFIX+'/catalog/products','POST',{
        'categoryNo':category['categoryNo'],'publicSlug':slug,'nameZhCn':'公开协议测试商品',
        'nameEnUs':'Public protocol test product','summaryZhCn':'仅用于隔离公开协议验收',
        'summaryEnUs':'Isolated protocol acceptance only','contractPolicyCode':'none',
        'inventoryTrackingEnabled':True,'sellableQuantity':20})
    v=PREFIX+'/catalog/product-versions/'+p['versionNo']
    if interface:
        api.call(v+'/fulfillment-binding','PUT',{'interfaceInstanceNo':interface})
        api.call(v+'/configuration','PUT',{'expectedVersion':0,'merchantValues':[]})
    api.call(v+'/fulfillment-policy','PUT',{'strategyCode':'payment_confirmed_plugin' if interface else 'manual'})
    bill=api.call(v+'/billing-rules','POST',{'ruleCode':'monthly' if interface else 'once',
        'chargeType':'recurring' if interface else 'one_time','periodUnit':'month' if interface else 'once',
        'periodCount':1,'pricePhase':'both' if interface else 'new_purchase','quantityMode':'fixed',
        'minimumPurchaseCycles':1,'status':'active'})
    tier={'minimumQuantity':0,'maximumQuantity':None,'amountMinor':'100','setupFeeMinor':'0'}
    if interface:tier['renewalAmountMinor']='100'
    api.call(v+'/billing-rules/'+bill['billingRuleNo']+'/price-schedules','POST',{
        'currencyCode':'CNY','tiers':[tier],'status':'active'})
    api.call(v+'/tax-policies','POST',{'currencyCode':'CNY','priceIncludesTax':False,
        'taxTreatment':'exempt','taxCategoryCode':'service','roundingScope':'line',
        'roundingMode':'half_up','roundingQuantumMinor':'1'})
    assert api.call(v+'/publication-preview')['ready'], 'Complete official publication contract required'
    api.call(v+'/publish','POST',{})
    return slug,p


def connect(api,endpoint,credential):
    prefix='/x/idc-resource/api/v1/admin/resource'
    channel=api.call(prefix+'/provider-configurations/channels','POST',{
        'displayName':'TC011 上游','endpoint':endpoint,'providerPluginId':'idc-peer','nonSecretSettings':{}})['item']
    number=channel['channelNo']
    for name,value in [('app_id',credential['appId']),('app_secret',credential['token'])]:
        request=key()
        context=api.call(prefix+'/channels/'+number+'/secrets/access-contexts','POST',{
            'operation':'create','bindingName':name,'secretDigest':hashlib.sha256(value.encode()).hexdigest(),
            'idempotencyKey':request})['item']['token']
        api.call(prefix+'/channels/'+number+'/secrets','POST',{
            'bindingName':name,'secret':base64.b64encode(value.encode()).decode(),
            'idempotencyKey':request,'accessContext':context})
    assert api.call(prefix+'/provider-configurations/channels/'+number+'/connection-verifications','POST',{})['verified']
    channel=next(c for c in api.call(prefix+'/provider-configurations')['channels'] if c['channelNo']==number)
    channel=api.call(prefix+'/provider-configurations/channels/'+number,'PUT',{
        'displayName':channel['displayName'],'endpoint':endpoint,'status':'enabled',
        'version':channel['version'],'nonSecretSettings':{}})['item']
    return channel['interfaceInstanceNo']


def imported(api,interface,category,slug):
    prefix=PREFIX+'/catalog'
    result=api.call(prefix+'/product-imports','POST',{'requestKey':key(),
        'interfaceInstanceNo':interface,'reference':slug,'categoryNo':category['categoryNo'],
        'pricingMode':'markup','markupPercent':'20'})
    assert not result['pricingIncomplete']
    v=prefix+'/product-versions/'+result['item']['versionNo']
    assert api.call(v+'/publication-preview')['ready']
    api.call(v+'/publish','POST',{})
    return api.call(prefix+'/products/'+result['item']['productNo'])['product']['publicSlug']


def purchase(api,slug):
    configuration=api.call(CHANNEL+'/catalog/products/'+slug+'/configuration')
    terms=api.call(CHANNEL+'/catalog/products/'+slug+'/billing-terms?'+urlencode({
        'configurationRevision':configuration['configurationRevision'],'currencyCode':'CNY','values':'[]'}))
    quote=api.call(CHANNEL+'/quotes','POST',{'requestId':key(),'purchase':{
        'publicSlug':slug,'configurationRevision':configuration['configurationRevision'],
        'billingTermToken':terms['terms'][0]['billingTermToken'],'currencyCode':'CNY',
        'purchaseCycles':1,'quantity':1,'values':[]}})
    assert quote['totalAmountMinor']==120
    request={'requestId':key(),'quoteNo':quote['quoteNo']}
    order=api.call(CHANNEL+'/orders','POST',request)
    replay=api.call(CHANNEL+'/orders','POST',request)
    assert order['id']==replay['id'], 'Same request must never double debit'
    detail=api.call(CHANNEL+'/orders/'+str(order['id']))
    assert detail['financiallySettled']
    return detail


def wait(predicate,label,timeout=120):
    end=time.monotonic()+timeout
    while time.monotonic()<end:
        value=predicate()
        if value:return value
        time.sleep(.5)
    raise AssertionError('Timed out waiting for '+label)


def upstream_delivery(api,customer_no,once=True):
    prefix=PREFIX+'/product-fulfillment/tasks'
    tasks=api.call(prefix+'?customerNo='+customer_no)['tasks']
    for row in tasks:
        if once and row['statusCode']=='processing' and row['canManualConclude']:
            detail=api.call(prefix+'/'+row['intentNo'])['task']
            if not detail['manualCompletedReasons']:return False
            api.call(prefix+'/'+row['intentNo']+'/manual-conclusions','POST',{
                'outcomeCode':'completed','expectedVersion':detail['version'],
                'reasonCode':detail['manualCompletedReasons'][0]['code'],
                'note':'隔离双实例测试确认已经完成人工交付','idempotencyKey':key()})
            return True
    return False


def delivery(instances,database,order):
    rows=instances.sql(database,"SELECT status_code FROM plugin_idc_core_product_delivery_record "
        f"WHERE order_no='{order['coreOrderNo']}'").splitlines()
    return rows and all(status=='completed' for status in rows)


def complete_fixture_supplier(instances,database,customer_no,version):
    # Only this test-only supplier's upstream order is controlled. The imported
    # downstream intent/task/service mappings and both sides' money remain real.
    state=instances.sql(database,"SELECT status_code FROM plugin_idc_core_product_fulfillment_intent "
        f"WHERE customer_no='{customer_no}' AND version_no='{version}' "
        "AND plugin_id='plugin-test-peer-entitlement'")
    assert state not in ['manual_pending','incomplete','canceled'], 'Controlled supplier entered terminal failure state'
    task_state=instances.sql(database,"SELECT t.status_code FROM plugin_idc_core_product_fulfillment_task t "
        "JOIN plugin_idc_core_product_fulfillment_intent i ON i.intent_no=t.intent_no "
        f"WHERE i.customer_no='{customer_no}' AND i.version_no='{version}' "
        "AND i.plugin_id='plugin-test-peer-entitlement'")
    assert task_state not in ['failed','result_unknown','manual_takeover','cancellation_attention','canceled'], 'Controlled supplier task requires intervention'
    intent=instances.sql(database,"SELECT intent_no FROM plugin_idc_core_product_fulfillment_intent "
        f"WHERE customer_no='{customer_no}' AND version_no='{version}' "
        "AND plugin_id='plugin-test-peer-entitlement' AND status_code='completed'")
    if not intent:return False
    # The real supplier worker completed provision/query. Only its local expiry
    # is a controlled upstream fixture; downstream dates still arrive over API.
    instances.sql(database,f"UPDATE plugin_idc_core_service_entitlement "
        f"SET expires_at=started_at+INTERVAL '1 month',version=version+1 "
        f"WHERE source_intent_no='{intent}' AND plugin_id='plugin-test-peer-entitlement' AND expires_at IS NULL")
    return True


def run():
    from isolated import Instances
    with Instances() as fixture:
        packages=fixture.build_packages();ca,key_file=fixture.certificate()
        upstream,updb=fixture.start('up',packages,ca)
        downstream,downdb=fixture.start('down',packages,ca)
        install(upstream,'plugin-test-peer-entitlement',packages/'plugin-test-peer-entitlement.wasm')
        # The exact peer package built in this run is scanned before startup and
        # installed formally; no old deployment manifest or hand-written auth.
        peer=next(p for p in downstream.call('/api/v1/plugins?pageSize=100')['list'] if p['id']=='idc-peer')
        if not peer['installed']:downstream.call('/api/v1/plugins/idc-peer/install','POST',{'installMockData':False,'installMode':'global'})
        if not peer['enabled']:downstream.call('/api/v1/plugins/idc-peer/enable','PUT',{})
        up_customer,up_category=customer(upstream)
        down_customer,down_category=customer(downstream)
        up_balance=fund(fixture,updb,up_customer);down_balance=fund(fixture,downdb,down_customer,360)
        up_credential=credential(upstream,up_customer);down_credential=credential(downstream,down_customer)
        connection=connect(downstream,fixture.proxy(upstream,ca,key_file),up_credential)
        once_slug,_=product(upstream,up_category)
        upstream.channel(up_credential).call(CHANNEL+'/catalog/products/'+once_slug+'/import-snapshot')
        print('TC011 stage: connected and upstream once snapshot readable',flush=True)
        local_once=imported(downstream,connection,down_category,once_slug)
        once_order=purchase(downstream.channel(down_credential),local_once)
        def manual_ready():
            tasks=upstream.call(PREFIX+'/product-fulfillment/tasks?customerNo='+up_customer['customerNo'])['tasks']
            for item in tasks:
                if item.get('canManualConclude'):
                    detail=upstream.call(PREFIX+'/product-fulfillment/tasks/'+item['intentNo'])['task']
                    if not detail['manualCompletedReasons']:
                        print('TC011 diagnostic: manual reason catalog empty',flush=True)
            return upstream_delivery(upstream,up_customer['customerNo'])
        try:wait(manual_ready, 'upstream manual order')
        except AssertionError:
            for name,db in [('up',updb),('down',downdb)]:
                print('TC011 diagnostic:',name,fixture.sql(db,"SELECT status_code FROM plugin_idc_core_product_fulfillment_intent"),flush=True)
                print('TC011 diagnostic task:',name,fixture.sql(db,"SELECT status_code||':'||last_result_code FROM plugin_idc_core_product_fulfillment_task"),flush=True)
                print('TC011 diagnostic settlement:',name,fixture.sql(db,"SELECT status_code||':'||attempt_count::text FROM plugin_idc_core_outbox_event WHERE event_type='commerce.order.financially_settled'"),flush=True)
            raise
        wait(lambda:delivery(fixture,downdb,once_order),'one-time peer delivery')
        assert fixture.sql(downdb,"SELECT count(*) FROM plugin_idc_core_service_entitlement "
            f"WHERE order_no='{once_order['coreOrderNo']}'")=='0', 'One-time delivery must not invent a service'
        print('TC011 stage: one-time procurement and delivery completed',flush=True)
        assert balance(fixture,updb,up_balance)==99900
        assert balance(fixture,downdb,down_balance)==240
        providers=upstream.call(PREFIX+'/catalog/automation/providers?pageSize=100')['items']
        provider=next(p for p in providers if p['displayName']=='Local entitlement protocol fixture')
        interface=upstream.call(PREFIX+'/catalog/automation/interfaces','POST',{
            'displayName':'TC011 本地持续权益前置','selectionToken':provider['selectionToken'],'configuration':[]})['interface']
        long_slug,long_product=product(upstream,up_category,interface['interfaceInstanceNo'])
        local_long=imported(downstream,connection,down_category,long_slug)
        channel=downstream.channel(down_credential)
        long_order=purchase(channel,local_long)
        wait(lambda:complete_fixture_supplier(fixture,updb,up_customer['customerNo'],long_product['versionNo']),
            # Two instances each observe query/submit/query with the formal
            # 30-second retry interval; budget their combined natural cadence.
            'controlled upstream supplier completion',timeout=240)
        wait(lambda:delivery(fixture,downdb,long_order),'long-term peer delivery')
        mapping=fixture.sql(downdb,"SELECT e.entitlement_no||'|'||e.upstream_service_no||'|'||e.renewal_mode_code "
            "FROM plugin_idc_core_service_entitlement e JOIN plugin_idc_core_product_fulfillment_intent i "
            "ON i.intent_no=e.source_intent_no "
            f"WHERE e.order_no='{long_order['coreOrderNo']}'").split('|')
        assert len(mapping)==3 and mapping[1] and mapping[2]=='provider_confirmed'
        service,upstream_service,_=mapping
        print('TC011 stage: long-term procurement and service mapping completed',flush=True)
        initial=fixture.sql(downdb,f"SELECT extract(epoch from expires_at)::bigint FROM plugin_idc_core_service_entitlement WHERE entitlement_no='{service}'")
        assert balance(fixture,updb,up_balance)==99800
        assert balance(fixture,downdb,down_balance)==120
        renewal={'requestId':key(),'parameters':{'renewalCycles':1,'settlementMethod':'balance'},'confirmed':True}
        result=channel.call(CHANNEL+'/services/'+service+'/operations/renew','POST',renewal)
        replay=channel.call(CHANNEL+'/services/'+service+'/operations/renew','POST',renewal)
        assert result['operationNo']==replay['operationNo'], 'Renewal replay must return the same operation'
        assert result['amountMinor']==120, 'Renewal response must report its authoritative 120 minor units'
        assert replay['amountMinor']==120 and result['currencyCode']=='CNY', 'Renewal replay must preserve the settled currency and amount'
        def renewed():
            current=fixture.sql(downdb,f"SELECT extract(epoch from expires_at)::bigint FROM plugin_idc_core_service_entitlement WHERE entitlement_no='{service}'")
            upstream_current=fixture.sql(updb,f"SELECT extract(epoch from expires_at)::bigint FROM plugin_idc_core_service_entitlement WHERE entitlement_no='{upstream_service}'")
            return current and int(current)>int(initial) and current==upstream_current
        wait(renewed,'real peer renewal and exact expiry convergence')
        print('TC011 stage: real peer renewal converged',flush=True)
        assert balance(fixture,updb,up_balance)==99700
        assert balance(fixture,downdb,down_balance)==0
        before_down_expiry=fixture.sql(downdb,f"SELECT expires_at::text FROM plugin_idc_core_service_entitlement WHERE entitlement_no='{service}'")
        before_up_expiry=fixture.sql(updb,f"SELECT expires_at::text FROM plugin_idc_core_service_entitlement WHERE entitlement_no='{upstream_service}'")
        before_operations=fixture.sql(downdb,"SELECT count(*) FROM plugin_idc_core_business_operation")
        rejected=False
        try:
            channel.call(CHANNEL+'/services/'+service+'/operations/renew','POST',{
                'requestId':key(),'parameters':{'renewalCycles':1,'settlementMethod':'balance'},'confirmed':True})
        except APIError as error:
            assert error.error_code=="IDC_ENTITLEMENT_RENEWAL_BALANCE_INSUFFICIENT"
            rejected=True
        assert rejected, 'Insufficient balance must reject the whole renewal'
        assert fixture.sql(downdb,"SELECT count(*) FROM plugin_idc_core_business_operation")==before_operations
        assert balance(fixture,downdb,down_balance)==0 and balance(fixture,updb,up_balance)==99700
        assert fixture.sql(downdb,f"SELECT expires_at::text FROM plugin_idc_core_service_entitlement WHERE entitlement_no='{service}'")==before_down_expiry
        assert fixture.sql(updb,f"SELECT expires_at::text FROM plugin_idc_core_service_entitlement WHERE entitlement_no='{upstream_service}'")==before_up_expiry
        print('TC011_PUBLIC_DUAL_INSTANCE_OK')


if __name__=='__main__':
    def interrupted(_signal,_frame):
        raise RuntimeError('TC011 interrupted')
    signal.signal(signal.SIGTERM,interrupted)
    signal.signal(signal.SIGINT,interrupted)
    # Never include exception response bodies, DSNs or credentials in test logs.
    try:run()
    except Exception as error:
        print('TC011 isolated public protocol failed:',type(error).__name__)
        if isinstance(error,AssertionError): print(str(error))
        import traceback
        print('Failure location:',[(Path(frame.filename).name,frame.lineno,frame.name) for frame in traceback.extract_tb(error.__traceback__)])
        raise SystemExit(1) from None
