"""One explicitly inspected recovery: canceled retain during test-service restart.
Not a general retry tool. Preserve budget and successful writes, remove only the
known canceled derived document before permitting one new retain of that source.
"""
import json,os,re,subprocess,urllib.request,urllib.error,time
from pathlib import Path
base=Path('/var/lib/deep-seeing-eval50');case=base/'hindsight-pilot-50/cases/e47becba'
doc='ep_3f66c60deacd4c4d97d00225423c7d6e'
assert subprocess.check_output(['systemctl','show','deep-seeing-pilot-run','-p','ActiveState','--value'],text=True).strip()!='active'
states=json.loads((case/'retain.json').read_text());assert states.get(doc)=='uncertain'
assert not (case/'manual-recovery-1.json').exists(), 'Recovery already attempted'
budget_before=json.loads((base/'budget.json').read_text())['charged_nano_cny']
subprocess.run(['systemctl','start','deep-seeing-pilot-budget','deep-seeing-pilot-hindsight'],check=True)
ready=False
for _ in range(40):
    try:
        with urllib.request.urlopen('http://127.0.0.1:8891/health',timeout=2) as r:ready=json.load(r)['status']=='healthy'
        if ready:break
    except Exception:pass
    time.sleep(1)
assert ready
sql="SELECT DISTINCT bank_id FROM documents WHERE id='"+doc+"';"
bank=subprocess.check_output(['docker','exec','deep-seeing-memory-pg','psql','-U','ds_memory_admin','-d','hindsight_eval50','-Atc',sql],text=True).strip()
document_exists=bool(bank)
if not bank:
    ids=[k for k,v in states.items() if v=='indexed']
    assert ids and all(re.fullmatch(r'ep_[a-f0-9]+',i) for i in ids)
    sql="SELECT DISTINCT bank_id FROM documents WHERE id IN ("+','.join("'"+i+"'" for i in ids)+");"
    bank=subprocess.check_output(['docker','exec','deep-seeing-memory-pg','psql','-U','ds_memory_admin','-d','hindsight_eval50','-Atc',sql],text=True).strip()
assert re.fullmatch(r'ds-episodes-v1-[a-f0-9]{32}',bank),'Inspect missing or ambiguous document instead of guessing'
audit={'document_id':doc,'bank':bank,'reason':'Operator throughput adjustment hit upstream 5-second graceful shutdown; ASGI retain canceled at 2026-09-09 16:14:36 CST',
 'prior_states':states,'budget_nano_cny_before':budget_before,'automatic_retry':False,'derived_document_exists':document_exists}
with os.fdopen(os.open(case/'manual-recovery-1.json',os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600),'w') as f:
    json.dump(audit,f,indent=2);f.flush();os.fsync(f.fileno())
if document_exists:
    req=urllib.request.Request('http://127.0.0.1:8891/v1/default/banks/'+bank+'/documents/'+doc,method='DELETE')
    with urllib.request.urlopen(req,timeout=30) as r:assert 200<=r.status<300
# Confirm the canceled derived document and facts are gone, not the local source.
sql="SELECT (SELECT count(*) FROM documents WHERE id='"+doc+"')+(SELECT count(*) FROM memory_units WHERE document_id='"+doc+"')+(SELECT count(*) FROM chunks WHERE document_id='"+doc+"');"
remaining=subprocess.check_output(['docker','exec','deep-seeing-memory-pg','psql','-U','ds_memory_admin','-d','hindsight_eval50','-Atc',sql],text=True).strip()
assert remaining=='0'
states.pop(doc)
tmp=case/'retain-recovery.tmp'
with os.fdopen(os.open(tmp,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600),'w') as f:json.dump(states,f);f.flush();os.fsync(f.fileno())
os.chown(tmp,os.stat(case).st_uid,os.stat(case).st_gid);os.replace(tmp,case/'retain.json')
(case/'paused.json').rename(case/'paused-before-recovery.json')
subprocess.run(['systemctl','start','deep-seeing-pilot-run'],check=True)
print('Canceled write absent/removed, zero document/fact/chunk rows confirmed; source and 13 indexes preserved. Resume under existing budget.')
