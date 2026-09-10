"""Activate a pre-verified build/config; rollback both on failed runtime check.

Root only. JSON input: api_key, sha256, artifact. No credentials in argv/logs.
Artifact must already exist below /opt/deep-seeing/ops/.
"""
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.request

assert os.geteuid() == 0
args=json.load(sys.stdin)
key=args['api_key']
assert key and '\n' not in key
artifact=Path(args['artifact']).resolve()
assert artifact.parent == Path('/opt/deep-seeing/ops') and artifact.is_file()
digest=hashlib.sha256(artifact.read_bytes()).hexdigest()
assert digest==args['sha256'] and re.fullmatch('[a-f0-9]{64}',digest)
release=Path('/opt/deep-seeing/releases')/('4c7ede2-memory-'+digest[:12])
assert not release.exists(), 'Release already exists; inspect instead of overwrite'
config=Path('/etc/deep-seeing/deep-seeing.env')
old_config=config.read_bytes(); config_stat=config.stat()
old_target=os.readlink('/opt/deep-seeing/current')
with urllib.request.urlopen('http://127.0.0.1:8889/health',timeout=5) as response:
    assert response.status==200, 'Hindsight must be healthy before activation'
journal=json.loads(Path('/var/lib/deep-seeing/data/runtime/episode-index-sync.json').read_text())
assert sum(e['status']=='indexed' for e in journal['entries'].values())>=args['expected_indexed']
assert not any(e['status'] in ('inflight','failed_or_uncertain','delete_uncertain') for e in journal['entries'].values()), 'Inspect uncertain writes before activation'
values={}
for line in old_config.decode().splitlines():
    if '=' in line and not line.lstrip().startswith('#'):
        k,v=line.split('=',1)
        parsed=shlex.split(v)
        values[k]=parsed[0] if parsed else ''
login=values.get('TAILSCALE_ALLOWED_USERS','').split(',')[0].strip()
updates={'OPENAI_API_KEY':key,'OPENAI_BASE_URL':'https://api.siliconflow.cn/v1',
    'OPENAI_MODEL':'deepseek-ai/DeepSeek-V4-Pro','MEMORY_RETRIEVAL_BACKEND':'hindsight',
    'MEMORY_INDEX_MODE':'auto','HINDSIGHT_URL':'http://127.0.0.1:8889','HINDSIGHT_API_KEY':''}
lines=[]
for line in old_config.decode().splitlines():
    if line.split('=',1)[0] not in updates: lines.append(line)
lines.extend(k+'='+json.dumps(v) for k,v in updates.items())
new_config=('\n'.join(lines)+'\n').encode()

def write_config(data):
    fd,path=tempfile.mkstemp(prefix='.release-',dir=config.parent)
    try:
        os.fchmod(fd,config_stat.st_mode & 0o777)
        os.fchown(fd,config_stat.st_uid,config_stat.st_gid)
        with os.fdopen(fd,'wb') as f: f.write(data);f.flush();os.fsync(f.fileno())
        os.replace(path,config)
    finally:
        if os.path.exists(path): os.unlink(path)

def switch_link(target):
    fd,temp=tempfile.mkstemp(prefix='.current-memory-',dir='/opt/deep-seeing')
    os.close(fd);os.unlink(temp)
    try:
        os.symlink(str(target),temp)
        os.replace(temp,'/opt/deep-seeing/current')
    finally:
        if os.path.lexists(temp):os.unlink(temp)

def restart():
    subprocess.run(['systemctl','restart','deep-seeing'],check=True,capture_output=True)

backup_dir=Path('/var/backups/deep-seeing/config')
backup_dir.mkdir(mode=0o700,exist_ok=True)
backup=backup_dir/(release.name+'.env')
with os.fdopen(os.open(backup,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600),'wb') as f:f.write(old_config)
release.mkdir(mode=0o755)
shutil.copyfile(artifact,release/'deep-seeing');os.chmod(release/'deep-seeing',0o555)
manifest={'release':str(release),'sha256':digest,'previous':old_target,'config_backup':str(backup),
          'built_from':'4c7ede2 + uncommitted memory integration changes',
          'activated_at':datetime.datetime.now(datetime.timezone.utc).isoformat()}
(release/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
try:
    write_config(new_config);switch_link(release);restart()
    verified=False
    for _ in range(20):
        try:
            req=urllib.request.Request('http://127.0.0.1:3319/api/runtime',headers={'Tailscale-User-Login':login})
            with urllib.request.urlopen(req,timeout=2) as response:runtime=json.load(response)['runtime']
            if runtime.get('model')=='deepseek-ai/DeepSeek-V4-Pro' and runtime.get('stores',{}).get('episode_retrieval')=='hindsight':verified=True;break
        except Exception: pass
        time.sleep(1)
    if not verified:raise RuntimeError('runtime verification failed')
except Exception:
    write_config(old_config);switch_link(old_target);restart()
    print('Activation failed; prior binary and configuration restored.')
    raise SystemExit(1) from None
print(json.dumps({'activated':str(release),'sha256':digest,'previous':old_target,'model':runtime.get('model')}))
