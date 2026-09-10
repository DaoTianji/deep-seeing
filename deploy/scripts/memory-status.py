"""Read-only deployment status; redact config credentials before showing logs."""
import json
from pathlib import Path
import re
import shlex
import subprocess
import urllib.request

values={}
secrets=[]
for path in ['/etc/deep-seeing/deep-seeing.env','/etc/deep-seeing/memory/hindsight.env']:
    for line in Path(path).read_text().splitlines():
        if '=' not in line or line.lstrip().startswith('#'):continue
        k,v=line.split('=',1)
        try:v=shlex.split(v)[0]
        except (ValueError,IndexError):continue
        values[k]=v
        if any(x in k for x in ['KEY','PASSWORD','DATABASE_URL']) and v:secrets.append(v)
def clean(text):
    for secret in secrets:text=text.replace(secret,'[REDACTED]')
    return re.sub(r'postgres(?:ql)?://[^\s]+','[REDACTED_DSN]',text)
for unit in ['deep-seeing','deep-seeing-hindsight','deep-seeing-memory-backup.timer']:
    result=subprocess.run(['systemctl','show',unit,'-p','ActiveState','-p','SubState','-p','MemoryCurrent','-p','MainPID'],capture_output=True,text=True)
    print(unit,clean(result.stdout.strip()))
for path in ['/api/runtime']:
    try:
        request=urllib.request.Request('http://127.0.0.1:3319'+path,headers={'Tailscale-User-Login':values.get('TAILSCALE_ALLOWED_USERS','').split(',')[0]})
        with urllib.request.urlopen(request,timeout=5) as response:print(clean(json.dumps(json.load(response),ensure_ascii=False)))
    except Exception as e:print('Runtime status:',type(e).__name__)
try:
    with urllib.request.urlopen('http://127.0.0.1:8889/health',timeout=5) as response:print('Hindsight health HTTP',response.status,clean(response.read(1000).decode()))
except Exception as e: print('Hindsight health:',type(e).__name__)
result=subprocess.run(['journalctl','-u','deep-seeing-hindsight','-n','35','--no-pager'],capture_output=True,text=True)
print(clean(result.stdout))
