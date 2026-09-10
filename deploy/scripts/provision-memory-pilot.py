"""Create only the isolated 50-CNY evaluation environment; never alter production."""
import json
import os
from pathlib import Path
import secrets
import subprocess

assert os.geteuid()==0
base=Path('/var/lib/deep-seeing-eval50'); config=Path('/etc/deep-seeing/pilot50')
assert not config.exists(), 'Existing pilot: inspect and resume, never reset its budget'
config.mkdir(mode=0o700);base.mkdir(mode=0o700,exist_ok=True)
programs=Path('/opt/deep-seeing/pilot50');programs.mkdir(mode=0o755,exist_ok=True)
for name,mode in [('pilot-budget-proxy.py','444'),('eval-hindsight-pilot','555')]:
    subprocess.run(['install','-m',mode,str(Path('/opt/deep-seeing/ops')/name),str(programs/name)],check=True)
if subprocess.run(['id','deep-seeing-eval50'],capture_output=True).returncode:
    subprocess.run(['useradd','--system','--home-dir',str(base),'--shell','/usr/sbin/nologin','deep-seeing-eval50'],check=True)
subprocess.run(['chown','deep-seeing-eval50:deep-seeing-eval50',str(base)],check=True)
prod={}
for line in Path('/etc/deep-seeing/memory/hindsight.env').read_text().splitlines():
    k,v=line.split('=',1);prod[k]=json.loads(v)
api_key=prod['HINDSIGHT_API_LLM_API_KEY'];token=secrets.token_urlsafe(32);password=secrets.token_hex(24)
sql="CREATE ROLE hindsight_eval50 LOGIN PASSWORD '"+password+"' NOSUPERUSER NOCREATEDB NOCREATEROLE;\nCREATE DATABASE hindsight_eval50 OWNER hindsight_eval50;\n"
r=subprocess.run(['docker','exec','-i','deep-seeing-memory-pg','psql','-v','ON_ERROR_STOP=1','-U','ds_memory_admin','-d','postgres'],input=sql,text=True,capture_output=True)
assert r.returncode==0,'Isolated database creation failed; inspect, do not reset'
r=subprocess.run(['docker','exec','deep-seeing-memory-pg','psql','-v','ON_ERROR_STOP=1','-U','ds_memory_admin','-d','hindsight_eval50','-c','CREATE EXTENSION vector;'],capture_output=True)
assert r.returncode==0
def env(name,values):
    fd=os.open(config/name,os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600)
    with os.fdopen(fd,'w') as f:f.write(''.join(k+'='+json.dumps(v)+'\n' for k,v in values.items()))
env('proxy.env',{'SILICONFLOW_API_KEY':api_key,'PILOT_PROXY_TOKEN':token,'PILOT_BUDGET_LEDGER':str(base/'budget.json')})
prod.update({'HINDSIGHT_API_DATABASE_URL':'postgresql://hindsight_eval50:'+password+'@127.0.0.1:5457/hindsight_eval50',
    'HINDSIGHT_API_PORT':'8891','HINDSIGHT_API_LLM_API_KEY':token,'HINDSIGHT_API_LLM_BASE_URL':'http://127.0.0.1:8890/v1',
    'HINDSIGHT_API_EMBEDDINGS_OPENAI_API_KEY':token,'HINDSIGHT_API_EMBEDDINGS_OPENAI_BASE_URL':'http://127.0.0.1:8890/v1',
    'HINDSIGHT_API_RERANKER_SILICONFLOW_API_KEY':token,'HINDSIGHT_API_RERANKER_SILICONFLOW_BASE_URL':'http://127.0.0.1:8890/v1',
    'HINDSIGHT_API_DB_POOL_MAX_SIZE':'4'})
env('hindsight.env',prod)
env('runner.env',{'OPENAI_API_KEY':token,'OPENAI_BASE_URL':'http://127.0.0.1:8890/v1','OPENAI_MODEL':'deepseek-ai/DeepSeek-V4-Pro','HINDSIGHT_URL':'http://127.0.0.1:8891'})
common='''[Service]
User=deep-seeing-eval50
Group=deep-seeing-eval50
WorkingDirectory=/var/lib/deep-seeing-eval50
UMask=0077
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/var/lib/deep-seeing-eval50
InaccessiblePaths=/var/lib/deep-seeing /opt/deep-seeing/seed /etc/deep-seeing/deep-seeing.env /etc/deep-seeing/memory
Environment=PYTHONDONTWRITEBYTECODE=1
Environment=XDG_CACHE_HOME=/var/lib/deep-seeing-eval50/cache
Restart=no
'''
units={
 'deep-seeing-pilot-budget':common+'EnvironmentFile=/etc/deep-seeing/pilot50/proxy.env\nExecStart=/usr/bin/python3 /opt/deep-seeing/pilot50/pilot-budget-proxy.py\nMemoryMax=192M\n',
 'deep-seeing-pilot-hindsight':common+'EnvironmentFile=/etc/deep-seeing/pilot50/hindsight.env\nExecStart=/opt/deep-seeing/hindsight-0.9.2-locked/bin/hindsight-api\nMemoryMax=1536M\nCPUQuota=100%\n',
 'deep-seeing-pilot-run':common+'EnvironmentFile=/etc/deep-seeing/pilot50/runner.env\nExecStart=/opt/deep-seeing/pilot50/eval-hindsight-pilot -hindsight-pilot -data /var/lib/deep-seeing-eval50/pilot-data.json -out /var/lib/deep-seeing-eval50/hindsight-pilot-50/results.jsonl\nMemoryMax=512M\nCPUQuota=75%\n'
}
for name,body in units.items():
    p=Path('/etc/systemd/system')/(name+'.service');assert not p.exists();p.write_text('[Unit]\nDescription=Deep-Seeing bounded isolated memory pilot\nAfter=network-online.target\n'+body)
subprocess.run(['systemctl','daemon-reload'],check=True)
print('Created isolated database, user and three non-recurring pilot services; production unchanged')
