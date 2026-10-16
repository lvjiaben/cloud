"""Owned single-instance acceptance of the public peer recovery lifecycle."""
from pathlib import Path
import signal
import traceback
from isolated import Instances, APIError
from scenario import install, customer, credential, connect, product, imported, fund

PEER='/api/v1/plugins/idc-peer'


def rejected(api,path,method,code):
    try:
        api.call(path,method,{} if method!='GET' else None)
    except APIError as error:
        assert error.error_code==code, 'Lifecycle must reject with its exact business error'
        return
    raise AssertionError('Lifecycle unexpectedly admitted the operation')


def state(api):
    item=next(p for p in api.call('/api/v1/plugins?id=idc-peer&pageSize=100')['list'] if p['id']=='idc-peer')
    assert item['enabled'], 'Recovery must preserve the enabled plugin'
    assert item['newBusinessState']=='stopped', 'Recovery must never reopen stopped admission'
    detail=api.call(PEER)
    assert detail['installed'] and detail['enabled'] and detail['newBusinessState']=='stopped'
    assert detail['effectiveVersion']==item['effectiveVersion'] and detail['runtimeState']=='normal'
    return item


def run():
    with Instances() as fixture:
        packages=fixture.build_packages();ca,key_file=fixture.certificate()
        api,database=fixture.start('lifecycle',packages,ca)
        install(api,'idc-peer',packages/'idc-peer.wasm')
        api.call(PEER+'/enable','PUT',{})
        assert api.call(PEER+'/business-impact')['blockingCount']==0
        # A formal other-plugin lifecycle performs manifest synchronization;
        # the observed host boundary loses peer extension authorization.
        install(api,'plugin-test-peer-entitlement',packages/'plugin-test-peer-entitlement.wasm')
        rejected(api,PEER+'/business-impact','GET','PLUGIN_BUSINESS_IMPACT_CHECK_FAILED')
        api.call(PEER+'/enable','PUT',{})
        impact=api.call(PEER+'/business-impact')
        assert impact['blockingCount']==0 and impact['canDisable']
        assert api.call(PEER+'/new-business/stop','PUT',{})['newBusinessState']=='stopped'
        api.call(PEER+'/enable','PUT',{})
        state(api)
        assert api.call(PEER+'/business-impact')['blockingCount']==0
        assert not api.call(PEER+'/disable','PUT',{})['enabled']
        disabled=api.call(PEER)
        assert not disabled['enabled'] and disabled['newBusinessState']=='stopped'
        api.call(PEER+'/enable','PUT',{})
        state(api)
        # Disabled-to-enabled metadata synchronization also loses extension
        # authorization; the enabled-plugin confirmation is the public repair.
        rejected(api,PEER+'/business-impact','GET','PLUGIN_BUSINESS_IMPACT_CHECK_FAILED')
        api.call(PEER+'/enable','PUT',{})
        state(api)
        assert api.call(PEER+'/business-impact')['blockingCount']==0
        print('TC011 lifecycle: public recovery preserves stopped admission and empty impact',flush=True)
        api.call(PEER+'/new-business/resume','PUT',{})
        account,category=customer(api)
        # Reuse the authorized isolated identity and balanced recharge fixture.
        # No order, customer payment, fulfillment or authorization is fabricated.
        fund(fixture,database,account)
        access=credential(api,account)
        interface=connect(api,fixture.proxy(api,ca,key_file),access)
        slug,_=product(api,category)
        imported(api,interface,category,slug)
        api.call(PEER+'/new-business/stop','PUT',{})
        api.call(PEER+'/enable','PUT',{})
        state(api)
        impact=api.call(PEER+'/business-impact')
        assert impact['blockingCount']>0 and not impact['canDisable'] and not impact['canUninstall']
        assert any(i['category']=='selling_product_versions' and i['blockingCount']>0 for i in impact['items'])
        rejected(api,PEER+'/disable','PUT','PLUGIN_BUSINESS_REFERENCES_BLOCKED')
        state(api)
        assert api.call(PEER+'/business-impact')['blockingCount']==impact['blockingCount']
        print('TC011_PUBLIC_PEER_LIFECYCLE_OK',flush=True)


if __name__=='__main__':
    def interrupted(_signal,_frame):raise RuntimeError('TC011 lifecycle interrupted')
    signal.signal(signal.SIGTERM,interrupted);signal.signal(signal.SIGINT,interrupted)
    try:run()
    except Exception as error:
        print('TC011 lifecycle failed:',type(error).__name__)
        if isinstance(error,AssertionError):print(str(error))
        print('Failure location:',[(Path(f.filename).name,f.lineno,f.name) for f in traceback.extract_tb(error.__traceback__)])
        raise SystemExit(1) from None
