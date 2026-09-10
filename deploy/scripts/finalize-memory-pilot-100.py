"""Finish the already-backed-up, already-cleaned budget recovery after auditing
Hindsight's one startup connectivity probe. Not a general retry utility.
"""
import hashlib,json,os,subprocess,time
from pathlib import Path
base=Path('/var/lib/deep-seeing-eval50');root=base/'hindsight-pilot-50'
case=root/'cases/7161e7e2';archive=base/'recovery-budget-100'
audit=json.loads((archive/'authorization.json').read_text())
assert not (archive/'recovery-verified.json').exists()
assert (archive/'hindsight_eval50-before.dump').stat().st_size>0
old=json.loads((archive/'budget-before.json').read_text());b=json.loads((base/'budget.json').read_text())
assert b['cap_nano_cny']==100_000_000_000
assert b['calls'][:1530]==old['calls'] and len(b['calls'])==1531
probe=b['calls'][-1]
assert probe['model']=='deepseek-ai/DeepSeek-V4-Pro' and probe['status']=='metered'
assert probe['input_tokens']==8 and probe['output_tokens']==1 and probe['charged']==120000
assert b['charged_nano_cny']==old['charged_nano_cny']+120000
assert subprocess.check_output(['systemctl','show','deep-seeing-pilot-run','-p','ActiveState','--value'],text=True).strip() in ('inactive','failed')
doc=audit['document_id'];bank=audit['bank']
assert doc=='ep_30f26da6f6a6418891e11c7de772ac82' and bank=='ds-episodes-v1-259390307385570b61ff0c44986f01c1'
sql="SELECT (SELECT count(*) FROM documents WHERE bank_id='%s' AND id='%s')+(SELECT count(*) FROM memory_units WHERE bank_id='%s' AND document_id='%s')+(SELECT count(*) FROM chunks WHERE bank_id='%s' AND document_id='%s');"%(bank,doc,bank,doc,bank,doc)
assert subprocess.check_output(['docker','exec','deep-seeing-memory-pg','psql','-U','ds_memory_admin','-d','hindsight_eval50','-Atc',sql],text=True).strip()=='0'
assert hashlib.sha256((case/'episodes/by_id'/(doc+'.md')).read_bytes()).hexdigest()==audit['source_sha256']
assert {p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in root.glob('*.result.json')}==audit['completed_result_sha256']
states=json.loads((case/'retain.json').read_text());assert states.get(doc)=='uncertain' and list(states.values()).count('indexed')==10
states.pop(doc);tmp=case/'retain-finalize100.tmp'
with os.fdopen(os.open(tmp,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600),'w') as f:
    json.dump(states,f);f.flush();os.fsync(f.fileno())
st=(case/'retain.json').stat();os.chown(tmp,st.st_uid,st.st_gid);os.replace(tmp,case/'retain.json')
(case/'paused.json').rename(case/'paused-before-budget-100.json')
with os.fdopen(os.open(archive/'recovery-verified.json',os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600),'w') as f:
    json.dump({'at':time.time(),'remaining_derived_rows':[0,0,0],'source_preserved':True,'five_results_preserved':True,
      'startup_probe_cny':0.00012,'probe_source':'Hindsight openai_compatible_llm.verify_connection, fixed Say ok message',
      'charged_nano_cny_at_resume':b['charged_nano_cny']},f);f.flush();os.fsync(f.fileno())
subprocess.run(['systemctl','start','deep-seeing-pilot-run'],check=True)
subprocess.run(['systemctl','reset-failed','deep-seeing-pilot-finish'],check=False)
subprocess.run(['systemd-run','--unit=deep-seeing-pilot-finish','--property=Type=exec','/usr/bin/python3','/opt/deep-seeing/ops/finish-memory-pilot.py'],check=True)
print('Last case resumed under total 100 CNY cap; prior spend and results preserved, startup probe included. Automatic final scoring and shutdown attached.')
