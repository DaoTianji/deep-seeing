"""Loopback-only, fail-closed cost gate for the six-case memory pilot.

Prices: CNY 12/M input, 24/M output, .28/M embedding/rerank; no cache credit.
Reserve a UTF-8-byte input upper estimate plus output cap BEFORE forwarding.
Unknown/failed receipts retain the reservation. Ledger contains no prompt/body/key.
This budget covers this proxy's calls, not other activity on the same account.
"""
import hashlib
import http.server
import json
import os
from pathlib import Path
import threading
import time
import urllib.error
import urllib.request

class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None

MODELS = {'deepseek-ai/DeepSeek-V4-Pro': (12000, 24000),
          'Qwen/Qwen3-Embedding-8B': (280, 0), 'Qwen/Qwen3-Reranker-8B': (280, 0)}
CAP = 50_000_000_000  # nano-CNY

def bounded(payload, endpoint):
    p = dict(payload)
    model = p.get('model')
    expected = {'chat/completions': 'deepseek-ai/DeepSeek-V4-Pro',
                'embeddings': 'Qwen/Qwen3-Embedding-8B', 'rerank': 'Qwen/Qwen3-Reranker-8B'}
    if expected.get(endpoint) != model:
        raise ValueError('endpoint/model denied')
    if p.get('n', 1) != 1: raise ValueError('multiple completions denied')
    if endpoint == 'chat/completions':
        for m in p.get('messages', []):
            if isinstance(m.get('content'), list): raise ValueError('multimodal denied')
        p['max_tokens'] = min(int(p.pop('max_completion_tokens', p.get('max_tokens', 8192))), 8192)
        if p['max_tokens'] < 1: raise ValueError('invalid output cap')
        p.setdefault('enable_thinking', True)
        if p['enable_thinking']: p.setdefault('reasoning_effort', 'high')
        if p.get('stream'): p['stream_options'] = {'include_usage': True}
    raw = json.dumps(p, ensure_ascii=False).encode()
    # Every UTF-8 byte can conservatively require one token; include framing.
    input_bound = len(raw) + 4096
    if endpoint == 'rerank':
        docs = p.get('documents', [])
        if not all(isinstance(d, str) for d in docs) or not isinstance(p.get('query'), str):
            raise ValueError('text-only rerank required')
        input_bound += len(p['query'].encode()) * len(docs) + 256 * len(docs)
    if endpoint == 'embeddings':
        items = p.get('input', [])
        if isinstance(items, str): items = [items]
        if not isinstance(items, list) or not all(isinstance(x, str) for x in items):
            raise ValueError('text-only embeddings required')
        input_bound += 256 * len(items)
    a, b = MODELS[model]
    return p, input_bound * a + p.get('max_tokens', 0) * b

class Budget:
    def __init__(self, path, cap=CAP):
        if cap not in (CAP, 2*CAP): raise ValueError('unauthorized cap')
        self.cap = cap
        self.path = Path(path); self.lock = threading.Lock()
        self.state = {'cap_nano_cny': cap, 'charged_nano_cny': 0, 'calls': []}
        if self.path.exists(): self.state = json.loads(self.path.read_text())
        if self.state['cap_nano_cny'] != cap: raise ValueError('budget mismatch')
    def save(self):
        temp = self.path.with_suffix('.tmp')
        fd = os.open(temp, os.O_CREAT | os.O_TRUNC | os.O_WRONLY, 0o600)
        with os.fdopen(fd, 'w') as f:
            json.dump(self.state, f); f.flush(); os.fsync(f.fileno())
        os.replace(temp, self.path)
    def reserve(self, model, amount):
        with self.lock:
            if self.state['charged_nano_cny'] + amount > self.cap:
                return None
            i = len(self.state['calls'])
            self.state['calls'].append({'model': model, 'reserved': amount, 'charged': amount,
                                        'status': 'uncertain', 'at': time.time()})
            self.state['charged_nano_cny'] += amount; self.save(); return i
    def settle(self, i, usage):
        with self.lock:
            row = self.state['calls'][i]
            # Only ordinary, fully reported chat/embedding usage is refunded.
            # Rerank providers vary in their billing units: retain upper estimate.
            if not usage or 'Reranker' in row['model']: return
            if 'prompt_tokens' not in usage: return
            inp = usage['prompt_tokens']; out = usage.get('completion_tokens', 0)
            if not isinstance(inp, int) or not isinstance(out, int) or min(inp, out) < 0: return
            a, b = MODELS[row['model']]; actual = inp*a + out*b
            if actual > row['reserved']:
                self.state['charged_nano_cny'] = self.cap
                row['status'] = 'bound_violation'; self.save(); return
            self.state['charged_nano_cny'] += actual-row['charged']
            row.update(charged=actual, status='metered', input_tokens=inp, output_tokens=out)
            self.save()

def serve():
    key = os.environ['SILICONFLOW_API_KEY']; token = os.environ['PILOT_PROXY_TOKEN']
    cap_cny = os.environ.get('PILOT_BUDGET_CAP_CNY', '50')
    if cap_cny not in ('50', '100'): raise ValueError('unsupported cap')
    budget = Budget(os.environ['PILOT_BUDGET_LEDGER'], int(cap_cny)*1_000_000_000)
    class Handler(http.server.BaseHTTPRequestHandler):
        def log_message(self, *args): pass
        def error(self, status, message):
            body = json.dumps({'error': {'message': message, 'type': 'pilot_budget'}}).encode()
            self.send_response(status); self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(body))); self.end_headers(); self.wfile.write(body)
        def do_POST(self):
            if self.headers.get('Authorization') != 'Bearer '+token:
                return self.error(401, 'denied')
            endpoint = self.path.removeprefix('/v1/')
            try:
                size = int(self.headers.get('Content-Length', '0'))
                if size < 1 or size > 2_000_000: raise ValueError('payload limit')
                p, reserve = bounded(json.loads(self.rfile.read(size)), endpoint)
            except Exception: return self.error(400, 'invalid pilot request')
            index = budget.reserve(p['model'], reserve)
            if index is None: return self.error(402, 'pilot budget exhausted; do not retry')
            req = urllib.request.Request('https://api.siliconflow.cn/v1/'+endpoint,
                data=json.dumps(p).encode(), headers={'Authorization': 'Bearer '+key, 'Content-Type': 'application/json'})
            usage = None
            try:
                with urllib.request.build_opener(NoRedirect()).open(req, timeout=240) as resp:
                    self.send_response(200); self.send_header('Content-Type', resp.headers.get('Content-Type', 'application/json'))
                    self.end_headers()
                    if p.get('stream'):
                        for line in resp:
                            if line.startswith(b'data: ') and line.strip() != b'data: [DONE]':
                                try: usage = json.loads(line[6:]).get('usage') or usage
                                except Exception: pass
                            self.wfile.write(line); self.wfile.flush()
                    else:
                        data = resp.read(16_000_000)
                        try: usage = json.loads(data).get('usage')
                        except Exception: pass
                        self.wfile.write(data)
            except urllib.error.HTTPError as e:
                self.error(e.code, 'upstream HTTP '+str(e.code))
            except Exception:
                # Keep reservation on timeout/disconnect; do not replay.
                pass
            finally:
                budget.settle(index, usage)
    http.server.ThreadingHTTPServer(('127.0.0.1', 8890), Handler).serve_forever()

if __name__ == '__main__': serve()
