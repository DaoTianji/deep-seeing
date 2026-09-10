"""Small post-release probes. Prints counters, never source or answer bodies.

--chat adds one ordinary deployment-check message to the real Room transcript.
--recall performs one bounded query against the production memory bank.
--restart checks persistence across a controlled application/service restart.
Neither option creates or rewrites an Episode directly.
"""
import collections
import hashlib
import json
from pathlib import Path
import shlex
import subprocess
import sys
import time
import urllib.request

values = {}
for line in Path('/etc/deep-seeing/deep-seeing.env').read_text().splitlines():
    if '=' in line and not line.lstrip().startswith('#'):
        k, v = line.split('=', 1)
        parsed = shlex.split(v)
        values[k] = parsed[0] if parsed else ''
headers = {'Tailscale-User-Login': values.get('TAILSCALE_ALLOWED_USERS', '').split(',')[0].strip()}

def request(path, data=None, base='http://127.0.0.1:3319', timeout=15):
    payload = None if data is None else json.dumps(data).encode()
    return urllib.request.urlopen(urllib.request.Request(base + path, data=payload,
        headers=dict(headers, **{'Content-Type': 'application/json'})), timeout=timeout)

with request('/api/runtime') as response:
    runtime = json.load(response)['runtime']
assert values.get('OPENAI_MODEL'), 'Selected model missing from production config'
assert runtime['model'] == values['OPENAI_MODEL']
assert runtime['stores']['episode_retrieval'] == 'hindsight'
assert runtime['stores']['stm'] == 'redis'
assert runtime['stores']['context_graph'] == 'available'
print(json.dumps({'runtime_verified': True, 'model': runtime['model'], 'stores': runtime['stores']}))

p = json.loads(Path('/var/lib/deep-seeing/data/runtime/episode-index-sync.json').read_text())
print(json.dumps({'index_states': dict(collections.Counter(x['status'] for x in p['entries'].values())),
    'attempts_today': p['attempts_today']}))
h = hashlib.sha256()
files = sorted(Path('/var/lib/deep-seeing/data/memory/episodes/by_id').glob('*.md'))
for f in files:
    h.update(f.name.encode()); h.update(f.read_bytes())
print(json.dumps({'source_files': len(files), 'source_sha256': h.hexdigest()}))

for path in ['/', '/roles', '/memory', '/mind', '/theater']:
    with request(path) as response:
        assert response.status == 200 and b'<html' in response.read(4096).lower()
    print(json.dumps({'page': path, 'http': 200}))

if '--recall' in sys.argv:
    start = time.monotonic()
    with request('/v1/default/banks/' + p['bank'] + '/memories/recall',
            {'query': '用户的沟通方式、偏好与我们过去的约定', 'types': ['world', 'experience'],
             'budget': 'low', 'max_tokens': 2048}, base='http://127.0.0.1:8889') as response:
        facts = json.load(response)['results']
    valid = set()
    for fact in facts:
        doc = fact.get('document_id')
        meta = fact.get('metadata', {})
        entry = p['entries'].get(doc, {})
        if entry.get('status') == 'indexed' and meta.get('episode_id') == doc and meta.get('revision') == entry.get('revision'):
            valid.add(doc)
    assert valid, 'Production recall returned no current indexed source'
    print(json.dumps({'production_recall_valid_documents': len(valid), 'seconds': round(time.monotonic()-start, 2)}))

if '--chat' in sys.argv:
    with request('/api/role/active') as response:
        role_state = json.load(response)
    # A deployment probe must go to An, not an active Actor. Backstage itself
    # is unavailable when no role is active, so use ordinary Stage in that case.
    channel = 'backstage' if role_state.get('active') else 'stage'
    start = time.monotonic(); kinds = collections.Counter(); tool_names = []
    done = False; marker = False; errors = 0; turn_id = None
    with request('/api/chat', {'channel': channel, 'message':
        '这是一条部署连通性自检，不是个人信息，也不需要记忆或搜索。请只回复：连接正常。'}, timeout=120) as response:
        for line in response:
            event = json.loads(line); kind = event.get('type'); kinds[kind] += 1
            if kind == 'tool': tool_names.append(event.get('data', {}).get('name'))
            if kind == 'error': errors += 1
            if kind == 'done':
                done = True; turn_id = event['turn_id']
                marker = '连接正常' in event.get('data', {}).get('answer', '')
    print(json.dumps({'chat_done': done, 'expected_marker': marker, 'errors': errors,
        'tools': tool_names, 'event_counts': dict(kinds), 'seconds': round(time.monotonic()-start, 2)}))
    assert done and marker and not errors
    with request('/turn/' + turn_id) as response:
        assert response.status == 200 and b'<html' in response.read(4096).lower()
    with request('/api/turns/' + turn_id) as response:
        trace = json.load(response)['turn']
    print(json.dumps({'turn_page_and_trace_verified': True, 'turn_id': turn_id}))

if '--restart' in sys.argv:
    stable_paths = ['/api/history', '/api/mutations', '/api/reflections', '/api/turns?limit=1']
    def snapshots():
        result = {}
        for path in stable_paths:
            with request(path) as response:
                result[path] = hashlib.sha256(json.dumps(json.load(response), sort_keys=True).encode()).hexdigest()
        return result
    before = snapshots()
    journal_before = Path('/var/lib/deep-seeing/data/runtime/episode-index-sync.json').read_bytes()
    subprocess.run(['systemctl', 'restart', 'deep-seeing-hindsight'], check=True, capture_output=True)
    healthy = False
    for _ in range(40):
        try:
            with request('/health', base='http://127.0.0.1:8889', timeout=2) as response:
                healthy = json.load(response).get('status') == 'healthy'
            if healthy: break
        except Exception: pass
        time.sleep(1)
    assert healthy, 'Hindsight did not recover after restart'
    subprocess.run(['systemctl', 'restart', 'deep-seeing'], check=True, capture_output=True)
    after = None
    for _ in range(15):
        try:
            after = snapshots(); break
        except Exception: time.sleep(1)
    assert before == after, 'Persistent API data changed across restart'
    assert journal_before == Path('/var/lib/deep-seeing/data/runtime/episode-index-sync.json').read_bytes()
    with request('/api/graph') as response:
        graph = json.load(response)
    assert graph.get('available'), 'Graph unavailable after restart'
    print(json.dumps({'restart_persistence_verified': True, 'index_journal_unchanged': True,
        'graph_available': True, 'graph_nodes': len(graph.get('nodes', []))}))
