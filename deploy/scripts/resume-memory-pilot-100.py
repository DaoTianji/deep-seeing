"""One user-authorized +50 CNY amendment and recovery of the known partial retain.
Never resets spend, reruns finished answers, or touches the production database.
"""
import hashlib,json,os,subprocess,time,urllib.request
from pathlib import Path

base=Path('/var/lib/deep-seeing-eval50');root=base/'hindsight-pilot-50'
case=root/'cases/7161e7e2';doc='ep_30f26da6f6a6418891e11c7de772ac82'
bank='ds-episodes-v1-259390307385570b61ff0c44986f01c1'
for unit in ['deep-seeing-pilot-run','deep-seeing-pilot-hindsight','deep-seeing-pilot-budget','deep-seeing-pilot-finish']:
    state=subprocess.check_output(['systemctl','show',unit,'-p','ActiveState','--value'],text=True).strip()
    assert state in ('inactive','failed'), (unit,state)
budget_path=base/'budget.json';raw=budget_path.read_bytes();budget=json.loads(raw)
assert hashlib.sha256(raw).hexdigest()=='feb83ad5c4dab6f08808794b801050adfe3abfd7805b6e692942e12572a5ae00'
assert budget['cap_nano_cny']==50_000_000_000 and len(budget['calls'])==1530
states=json.loads((case/'retain.json').read_text())
assert states.get(doc)=='uncertain' and list(states.values()).count('indexed')==10
assert not (root/'7161e7e2.result.json').exists()
results={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in root.glob('*.result.json')}
assert len(results)==5
mapping=json.loads((case/'sources.json').read_text());assert doc in mapping
source=case/'episodes/by_id'/(doc+'.md');assert source.is_file()
source_hash=hashlib.sha256(source.read_bytes()).hexdigest()
def query(sql):
    return subprocess.check_output(['docker','exec','deep-seeing-memory-pg','psql','-U','ds_memory_admin','-d','hindsight_eval50','-Atc',sql],text=True).strip()
counts_sql="SELECT (SELECT count(*) FROM documents WHERE bank_id='%s' AND id='%s'),(SELECT count(*) FROM memory_units WHERE bank_id='%s' AND document_id='%s'),(SELECT count(*) FROM chunks WHERE bank_id='%s' AND document_id='%s');"%(bank,doc,bank,doc,bank,doc)
assert query("SELECT bank_id FROM documents WHERE id='%s';"%doc)==bank
assert query(counts_sql)=='1|9|3'
archive=base/'recovery-budget-100';archive.mkdir(mode=0o700,exist_ok=False)
def write_new(path,data):
    with os.fdopen(os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600),'wb') as f:
        f.write(data);f.flush();os.fsync(f.fileno())
write_new(archive/'budget-before.json',raw)
write_new(archive/'retain-before.json',(case/'retain.json').read_bytes())
write_new(archive/'paused-before.json',(case/'paused.json').read_bytes())
with os.fdopen(os.open(archive/'hindsight_eval50-before.dump',os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600),'wb') as f:
    subprocess.run(['docker','exec','deep-seeing-memory-pg','pg_dump','-U','ds_memory_admin','-d','hindsight_eval50','-Fc'],stdout=f,check=True)
    f.flush();os.fsync(f.fileno())
assert (archive/'hindsight_eval50-before.dump').stat().st_size>0
audit={'authorization':'User explicitly added 50 CNY; cumulative limit is 100 CNY; finish the same six cases only',
    'at':time.time(),'old_cap_cny':50,'new_cap_cny':100,'spent_nano_cny_before':budget['charged_nano_cny'],
    'prior_calls':1530,'document_id':doc,'bank':bank,'derived_counts_before':[1,9,3],
    'source_sha256':source_hash,'completed_result_sha256':results,'reason':'Prior retain hit budget HTTP 402 and left a partial derived document',
    'backup':'hindsight_eval50-before.dump','automatic_retry':False}
write_new(archive/'authorization.json',json.dumps(audit,indent=2).encode())
def replace(path,data,owner=None):
    tmp=path.with_name(path.name+'.amend100.tmp');write_new(tmp,data)
    st=owner or path.stat();os.chown(tmp,st.st_uid,st.st_gid);os.replace(tmp,path)
# Amend both independently checked cap declarations; all prior receipts remain.
budget['cap_nano_cny']=100_000_000_000
budget.setdefault('authorizations',[]).append(audit)
replace(budget_path,json.dumps(budget).encode())
env_path=Path('/etc/deep-seeing/pilot50/proxy.env');lines=env_path.read_text().splitlines()
assert not any(s.startswith('PILOT_BUDGET_CAP_CNY=') for s in lines)
replace(env_path,('\n'.join(lines)+'\nPILOT_BUDGET_CAP_CNY="100"\n').encode())
subprocess.run(['systemctl','start','deep-seeing-pilot-budget','deep-seeing-pilot-hindsight'],check=True)
ready=False
for _ in range(40):
    try:
        with urllib.request.urlopen('http://127.0.0.1:8891/health',timeout=2) as r:ready=json.load(r)['status']=='healthy'
        if ready:break
    except Exception:pass
    time.sleep(1)
assert ready,'Inspect service; runner has not started'
req=urllib.request.Request('http://127.0.0.1:8891/v1/default/banks/'+bank+'/documents/'+doc,method='DELETE')
with urllib.request.urlopen(req,timeout=30) as r:assert 200<=r.status<300
assert query(counts_sql)=='0|0|0','Partial document not fully removed; do not resume'
assert hashlib.sha256(source.read_bytes()).hexdigest()==source_hash
assert {p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in root.glob('*.result.json')}==results
assert json.loads(budget_path.read_text())['charged_nano_cny']==audit['spent_nano_cny_before'],'Unexpected model spend before resume'
states.pop(doc);replace(case/'retain.json',json.dumps(states).encode())
(case/'paused.json').rename(case/'paused-before-budget-100.json')
write_new(archive/'recovery-verified.json',json.dumps({'at':time.time(),'remaining_derived_rows':[0,0,0],'source_preserved':True,'five_results_preserved':True}).encode())
subprocess.run(['systemctl','start','deep-seeing-pilot-run'],check=True)
subprocess.run(['systemctl','reset-failed','deep-seeing-pilot-finish'],check=False)
subprocess.run(['systemd-run','--unit=deep-seeing-pilot-finish','--property=Type=exec','/usr/bin/python3','/opt/deep-seeing/ops/finish-memory-pilot.py'],check=True)
print('Resumed the last case only. Cap 100 CNY; prior cost and 1530 receipts preserved; partial derived document backed up and removed; source and five results unchanged.')
