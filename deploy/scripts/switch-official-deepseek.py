"""Root-only bounded provider switch. JSON stdin: api_key, sha256.

Upload the binary to /opt/deep-seeing/ops/deep-seeing-official first.
Preserve vector providers and every unrelated setting. Never rebuild an index.
One synthetic non-thinking official request (max 160 output tokens), no retries.
"""
import datetime
import hashlib
import json
import os
from pathlib import Path
import shlex
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.request


def parse_env(raw):
    result = {}
    for line in raw.decode().splitlines():
        if '=' in line and not line.lstrip().startswith('#'):
            key, value = line.split('=', 1)
            parts = shlex.split(value)
            result[key.strip()] = parts[0] if parts else ''
    return result


def update_env(raw, updates):
    lines = [line for line in raw.decode().splitlines() if line.split('=', 1)[0].strip() not in updates]
    lines.extend(key + '=' + json.dumps(value) for key, value in updates.items())
    return ('\n'.join(lines) + '\n').encode()


def atomic_config(path, data, info):
    fd, tmp = tempfile.mkstemp(prefix='.provider-', dir=path.parent)
    try:
        os.fchmod(fd, info.st_mode & 0o777)
        os.fchown(fd, info.st_uid, info.st_gid)
        with os.fdopen(fd, 'wb') as handle:
            handle.write(data); handle.flush(); os.fsync(handle.fileno())
        os.replace(tmp, path)
    finally:
        if os.path.exists(tmp): os.unlink(tmp)


def run():
    assert os.geteuid() == 0
    args = json.load(sys.stdin)
    key = args['api_key']
    assert key and not any(c.isspace() for c in key)
    artifact = Path('/opt/deep-seeing/ops/deep-seeing-official')
    digest = hashlib.sha256(artifact.read_bytes()).hexdigest()
    assert digest == args['sha256']
    main = Path('/etc/deep-seeing/deep-seeing.env')
    hind = Path('/etc/deep-seeing/memory/hindsight.env')
    originals = {p: p.read_bytes() for p in (main, hind)}
    stats = {p: p.stat() for p in originals}
    m, h = (parse_env(originals[p]) for p in (main, hind))
    assert m['OPENAI_BASE_URL'] == 'https://api.siliconflow.cn/v1'
    assert h['HINDSIGHT_API_LLM_BASE_URL'] == 'https://api.siliconflow.cn/v1'
    assert m['OPENAI_API_KEY'] == h['HINDSIGHT_API_EMBEDDINGS_OPENAI_API_KEY'] == h['HINDSIGHT_API_RERANKER_SILICONFLOW_API_KEY']
    assert key != m['OPENAI_API_KEY']
    headers = {'Tailscale-User-Login': m.get('TAILSCALE_ALLOWED_USERS', '').split(',')[0].strip()}
    def get(path, base='http://127.0.0.1:3319'):
        with urllib.request.urlopen(urllib.request.Request(base + path, headers=headers), timeout=5) as response:
            return json.load(response)
    stable_paths = ['/api/history', '/api/mutations', '/api/reflections', '/api/turns?limit=1']
    def snapshots():
        return {p: hashlib.sha256(json.dumps(get(p), sort_keys=True).encode()).hexdigest() for p in stable_paths}
    def source_hash():
        hasher = hashlib.sha256()
        for p in sorted(Path('/var/lib/deep-seeing/data/memory/episodes/by_id').glob('*.md')):
            hasher.update(p.name.encode()); hasher.update(p.read_bytes())
        return hasher.hexdigest()
    def service(action, unit):
        subprocess.run(['systemctl', action, unit], check=True, capture_output=True)
    def wait_for(check):
        for _ in range(45):
            try:
                if check(): return
            except Exception: pass
            time.sleep(1)
        raise RuntimeError('service readiness timeout')
    before = snapshots(); before_source = source_hash(); before_runtime = get('/api/runtime')['runtime']
    journal = Path('/var/lib/deep-seeing/data/runtime/episode-index-sync.json')
    before_journal = journal.read_bytes()
    entries = json.loads(before_journal)['entries']
    assert entries and all(x['status'] == 'indexed' for x in entries.values()), 'index must be settled before switch'

    # Validate the backend's exact non-thinking request form, on this server.
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, *a, **kw): return None
    payload = {'model': 'deepseek-v4-pro', 'max_tokens': 160, 'thinking': {'type': 'disabled'},
               'response_format': {'type': 'json_object'},
               'messages': [{'role': 'user', 'content': 'Synthetic API test. Return exactly this JSON object: {"probe":"ORCHID-42"}'}]}
    req = urllib.request.Request('https://api.deepseek.com/v1/chat/completions', data=json.dumps(payload).encode(),
                                headers={'Authorization': 'Bearer ' + key, 'Content-Type': 'application/json'})
    with urllib.request.build_opener(NoRedirect).open(req, timeout=30) as response:
        result = json.load(response)
    assert json.loads(result['choices'][0]['message']['content']) == {'probe': 'ORCHID-42'}
    print(json.dumps({'synthetic_json_probe': 'passed', 'model': result.get('model'), 'usage': result.get('usage')}), flush=True)

    updates = {
        main: {'OPENAI_API_KEY': key, 'OPENAI_BASE_URL': 'https://api.deepseek.com/v1', 'OPENAI_MODEL': 'deepseek-v4-pro'},
        hind: {'HINDSIGHT_API_LLM_API_KEY': key, 'HINDSIGHT_API_LLM_BASE_URL': 'https://api.deepseek.com/v1',
               'HINDSIGHT_API_LLM_MODEL': 'deepseek-v4-pro', 'HINDSIGHT_API_LLM_EXTRA_BODY': '{"thinking":{"type":"disabled"}}'},
    }
    current = Path('/opt/deep-seeing/current'); previous = os.readlink(current)
    release = Path('/opt/deep-seeing/releases') / ('99b52c4-deepseek-' + digest[:12])
    assert not release.exists(), 'release exists; inspect before retry'
    stamp = datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%SZ')
    backup = Path('/var/backups/deep-seeing') / ('official-deepseek-' + stamp)
    backup.mkdir(mode=0o700)
    for p, raw in originals.items():
        with os.fdopen(os.open(backup / p.name, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), 'wb') as f: f.write(raw)
    release.mkdir(mode=0o755)
    shutil.copyfile(artifact, release / 'deep-seeing'); os.chmod(release / 'deep-seeing', 0o555)
    def switch(target):
        fd, temp = tempfile.mkstemp(prefix='.official-', dir=current.parent)
        os.close(fd); os.unlink(temp)
        try: os.symlink(str(target), temp); os.replace(temp, current)
        finally:
            if os.path.lexists(temp): os.unlink(temp)
    report = {'source_base': '99b52c4 + local official-provider patch', 'sha256': digest,
              'previous': previous, 'release': str(release), 'config_backup': str(backup), 'time_utc': stamp,
              'model': 'deepseek-v4-pro', 'llm_base': 'https://api.deepseek.com/v1',
              'synthetic_probe_usage': result.get('usage'), 'indexed_documents': len(entries)}
    try:
        service('stop', 'deep-seeing')
        assert journal.read_bytes() == before_journal
        for p in originals:
            raw = update_env(originals[p], updates[p])
            parsed = parse_env(raw)
            assert all(parsed[k] == v for k, v in parse_env(originals[p]).items() if k not in updates[p])
            atomic_config(p, raw, stats[p])
        service('restart', 'deep-seeing-hindsight')
        wait_for(lambda: get('/health', 'http://127.0.0.1:8889').get('status') == 'healthy')
        switch(release); service('start', 'deep-seeing')
        wait_for(lambda: get('/api/runtime')['runtime'].get('model') == 'deepseek-v4-pro')
        runtime = get('/api/runtime')['runtime']
        assert runtime['stores']['episode_retrieval'] == 'hindsight' and runtime['stores']['stm'] == 'redis'
        assert runtime['stores']['context_graph'] == 'available'
        for field in ['recall_mode', 'reflection_mode', 'role_mode', 'role_init_mode']:
            assert runtime[field] == before_runtime[field]
        assert source_hash() == before_source and journal.read_bytes() == before_journal
        assert snapshots() == before, 'persistent API snapshot changed; inspect rather than overwrite data'
        report.update({'deployed': True, 'sources_unchanged': True, 'index_unchanged': True,
                       'history_mutations_reflections_unchanged': True, 'auxiliary_config_unchanged': True})
    except Exception:
        service('stop', 'deep-seeing')
        for p in originals: atomic_config(p, originals[p], stats[p])
        switch(previous); service('restart', 'deep-seeing-hindsight'); service('start', 'deep-seeing')
        print(json.dumps({'deployed': False, 'rollback_attempted': True, 'backup': str(backup)}), flush=True)
        raise
    with os.fdopen(os.open(backup / 'report.json', os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), 'w') as f:
        json.dump(report, f, indent=2)
    (release / 'manifest.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report), flush=True)


if __name__ == '__main__':
    try: run()
    except Exception as exc:
        print(json.dumps({'failed': True, 'error_type': type(exc).__name__, 'detail': 'withheld to protect credentials'}))
        raise SystemExit(1) from None
