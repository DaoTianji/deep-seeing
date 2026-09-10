"""Change test throughput ONLY while the runner and test server are stopped.

Do not use SIGSTOP/graceful restart as an in-flight write barrier: the upstream
server's five-second drain can cancel retention. Configure before resuming.
"""
import json,os,subprocess,tempfile
from pathlib import Path
for unit in ['deep-seeing-pilot-run','deep-seeing-pilot-hindsight']:
    state=subprocess.check_output(['systemctl','show',unit,'-p','ActiveState','--value'],text=True).strip()
    assert state in ('inactive','failed'), 'Stop at a verified checkpoint before configuring'
p=Path('/etc/deep-seeing/pilot50/hindsight.env');values={}
for line in p.read_text().splitlines():
    k,v=line.split('=',1);values[k]=json.loads(v)
assert values['HINDSIGHT_API_PORT']=='8891'
assert values['HINDSIGHT_API_LLM_BASE_URL']=='http://127.0.0.1:8890/v1'
values['HINDSIGHT_API_LLM_MAX_CONCURRENT']='4'
fd,tmp=tempfile.mkstemp(dir=p.parent,prefix='.tune-')
with os.fdopen(fd,'w') as f:
    f.write(''.join(k+'='+json.dumps(v)+'\n' for k,v in values.items()));f.flush();os.fsync(f.fileno())
os.replace(tmp,p)
print('Inactive test service configured for four LLM slots; not started; budget untouched')
