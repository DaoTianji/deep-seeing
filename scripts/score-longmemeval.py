#!/usr/bin/env python3
"""Resumable official-prompt LongMemEval judging over an OpenAI-compatible gateway.

Only the official get_anscheck_prompt AST is loaded, without SDK dependencies.
No answer text is copied into the committed aggregate report.
"""
import argparse
import ast
import concurrent.futures
import hashlib
import json
import math
import os
from pathlib import Path
import shlex
import statistics
import time
import urllib.error
import urllib.request
from collections import Counter, defaultdict


def credentials():
    env = dict(os.environ)
    p = Path('.env')
    if p.exists():
        for line in p.read_text().splitlines():
            line = line.strip()
            if not line or line.startswith('#') or '=' not in line:
                continue
            k, v = line.removeprefix('export ').split('=', 1)
            values = shlex.split(v, comments=True)
            env.setdefault(k.strip(), ' '.join(values))
    return env


def sha(path):
    h = hashlib.sha256()
    with open(path, 'rb') as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b''):
            h.update(chunk)
    return h.hexdigest()


def official_prompt(path):
    if sha(path) != 'ecce9c4c79dc89d99534ac17b383a5cbb5b9f0c69ee98adaf0684742e3d95251':
        raise ValueError('official scorer version differs from the frozen protocol')
    tree = ast.parse(Path(path).read_text())
    fn = [n for n in tree.body if isinstance(n, ast.FunctionDef) and n.name == 'get_anscheck_prompt']
    if len(fn) != 1:
        raise ValueError('official prompt function missing')
    ns = {}
    exec(compile(ast.Module(body=fn, type_ignores=[]), str(path), 'exec'), ns)
    return ns['get_anscheck_prompt']


def rows(path):
    if not Path(path).exists():
        return []
    return [json.loads(line) for line in Path(path).read_text().splitlines() if line.strip()]


def resolved_judgments(path):
    original = rows(path)
    done = {r['question_id']: r for r in original}
    if len(done) != len(original):
        raise ValueError('duplicate original judgments')
    for repair in rows(str(path) + '.repairs.jsonl'):
        qid = repair['question_id']
        previous = done.get(qid)
        if not previous or previous.get('label') is not None:
            raise ValueError('repair may replace only an invalid verdict')
        if any(previous[k] != repair[k] for k in ('hypothesis_sha256', 'model')):
            raise ValueError('repair changed answer or judge')
        merged = dict(repair)
        merged['attempts'] += previous.get('attempts', 0)
        merged['recovery_tokens_reported'] = previous.get('recovery_tokens_reported', 0) + previous.get('usage', {}).get('total_tokens', 0)
        done[qid] = merged
    return list(done.values())


def call(env, model, prompt, max_tokens):
    base = env.get('OPENAI_BASE_URL', env.get('AI_GATEWAY_BASE_URL', '')).rstrip('/')
    key = env.get('OPENAI_API_KEY', env.get('AI_GATEWAY_API_KEY', ''))
    if not base or not key:
        raise ValueError('missing gateway configuration')
    payload = {'model': model, 'messages': [{'role': 'user', 'content': prompt}],
               'n': 1, 'temperature': 0, 'max_tokens': max_tokens}
    req = urllib.request.Request(base + '/chat/completions', json.dumps(payload).encode(),
                                 {'Authorization': 'Bearer ' + key, 'Content-Type': 'application/json'})
    started = time.monotonic()
    with urllib.request.urlopen(req, timeout=120) as response:
        result = json.load(response)
    return result['choices'][0]['message']['content'].strip(), result.get('usage', {}), time.monotonic() - started


def judge_one(ref, hyp, prompt_fn, env, args):
    prompt = prompt_fn(ref['question_type'], ref['question'], ref['answer'],
                       hyp['hypothesis'], abstention='_abs' in ref['question_id'])
    result = {'question_id': ref['question_id'], 'question_type': ref['question_type'],
              'hypothesis_sha256': hashlib.sha256(hyp['hypothesis'].encode()).hexdigest(),
              'model': args.judge, 'label': None, 'attempts': 0}
    errors = []
    for attempt in range(1, 4):
        result['attempts'] = attempt
        try:
            answer, usage, seconds = call(env, args.judge, prompt, args.max_tokens)
            result.update({'response': answer, 'usage': usage, 'seconds': seconds})
            if answer.lower().rstrip('.') not in ('yes', 'no'):
                result['error'] = 'invalid judge verdict'
                break
            # Upstream evaluates `yes in eval_response.lower()`.
            result['label'] = 'yes' in answer.lower()
            break
        except (urllib.error.URLError, TimeoutError, OSError) as e:
            code = getattr(e, 'code', None)
            errors.append(type(e).__name__ + (':' + str(code) if code else ''))
            if code and code not in (408, 429, 500, 502, 503, 504):
                result['error'] = errors[-1]
                break
            if attempt == 3:
                result['error'] = errors[-1]
                break
            time.sleep(attempt * 2)
    if errors:
        result['attempt_errors'] = errors
    return result


def pct(values, p):
    values = sorted(values)
    return values[max(0, math.ceil(len(values) * p) - 1)] if values else 0


def wilson(k, n):
    if not n:
        return [0, 0]
    z = 1.96
    center = (k / n + z*z / (2*n)) / (1 + z*z/n)
    delta = z * math.sqrt(k/n * (1-k/n)/n + z*z/(4*n*n)) / (1+z*z/n)
    return [center-delta, center+delta]


def summarize(refs, hypotheses, judgments):
    scores = {j['question_id']: j for j in judgments}
    bytype = defaultdict(list)
    correct = 0
    abstention = []
    candidate_hits, read_hits, used_hits = [], [], []
    for h in hypotheses:
        qid = h['question_id']
        j = scores.get(qid, {})
        good = j.get('label') is True and not h.get('error')
        correct += good
        bytype[refs[qid]['question_type']].append(good)
        if '_abs' in qid:
            abstention.append(good)
        else:
            gold = set(refs[qid]['answer_session_ids'])
            if gold:
                for field, dest in [('candidate_session_ids', candidate_hits), ('read_session_ids', read_hits), ('used_session_ids', used_hits)]:
                    dest.append(len(set(h.get(field) or []) & gold) / len(gold))
    def mean(v): return statistics.mean(v) if v else 0
    return {
        'total': len(hypotheses), 'correct': correct, 'accuracy': correct / len(hypotheses) if hypotheses else 0,
        'accuracy_wilson_95': wilson(correct, len(hypotheses)),
        'task_macro_accuracy': mean([mean(v) for v in bytype.values()]),
        'categories': {k: {'total': len(v), 'correct': sum(v), 'accuracy': mean(v)} for k, v in sorted(bytype.items())},
        'abstention': {'total': len(abstention), 'correct': sum(abstention), 'accuracy': mean(abstention)},
        'retrieval': {'answerable_items': len(candidate_hits), 'candidate_session_recall': mean(candidate_hits),
                      'read_session_recall': mean(read_hits), 'used_session_recall': mean(used_hits)},
        'inference_errors': sum(bool(h.get('error')) for h in hypotheses),
        'judge_errors_or_missing': sum(scores.get(h['question_id'], {}).get('label') is None for h in hypotheses),
        'mean_searches': mean([len(h.get('searches') or []) for h in hypotheses]),
        'mean_reads': mean([len(h.get('reads') or []) for h in hypotheses]),
        'mean_search_time_ms_per_question': mean([sum(s.get('duration_ns', 0) for s in (h.get('searches') or [])) / 1e6 for h in hypotheses]),
        'mean_read_time_ms_per_question': mean([sum(s.get('duration_ns', 0) for s in (h.get('reads') or [])) / 1e6 for h in hypotheses]),
        'no_search_items': sum(not h.get('searches') for h in hypotheses),
        'answer_seconds_p50': pct([h['answer_seconds'] for h in hypotheses], .5),
        'answer_seconds_p95': pct([h['answer_seconds'] for h in hypotheses], .95),
        'ingest_seconds_sum': sum(h.get('ingest_seconds', 0) for h in hypotheses),
        'inference_tokens_successful_final_attempts': sum(h.get('tokens', {}).get('total_tokens', 0) for h in hypotheses),
        'judge_tokens_reported': sum(j.get('usage', {}).get('total_tokens', 0) + j.get('recovery_tokens_reported', 0) for j in judgments),
        'inference_attempts': sum(h.get('attempts', 0) for h in hypotheses),
        'judge_attempts': sum(j.get('attempts', 0) for j in judgments),
    }


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--data', required=True)
    p.add_argument('--hypotheses', required=True)
    p.add_argument('--upstream', default='data/evals/longmemeval-baseline/upstream/src/evaluation/evaluate_qa.py')
    p.add_argument('--judge', default='gpt-4o-2024-08-06')
    p.add_argument('--max-tokens', type=int, default=10)
    p.add_argument('--workers', type=int, default=4)
    p.add_argument('--summarize-only', action='store_true')
    p.add_argument('--allow-partial', action='store_true')
    p.add_argument('--retry-invalid', action='store_true', help='append recovery records for invalid verdicts only; never rerun valid scores')
    args = p.parse_args()
    with open(args.data) as f:
        data = json.load(f)
    refs = {r['question_id']: r for r in data}
    # Drop all histories before opening the judge network path.
    for r in refs.values():
        for k in ('haystack_sessions', 'haystack_dates', 'haystack_session_ids'):
            r.pop(k, None)
    del data
    hyps = rows(args.hypotheses)
    hyp_ids = [h['question_id'] for h in hyps]
    if len(set(hyp_ids)) != len(hyp_ids) or not set(hyp_ids).issubset(refs):
        raise ValueError('duplicate or unknown hypothesis IDs')
    if not args.allow_partial and (len(refs) != 500 or set(hyp_ids) != set(refs)):
        raise ValueError('full evaluation requires exactly all 500 IDs')
    path = Path(args.hypotheses + '.judgments-' + args.judge.replace('/', '_') + '.jsonl')
    manifest = {'data_sha256': sha(args.data), 'hypotheses_sha256': sha(args.hypotheses),
                'official_script_sha256': sha(args.upstream), 'judge': args.judge,
                'max_tokens': args.max_tokens, 'temperature': 0}
    mp = Path(str(path) + '.manifest.json')
    if mp.exists() and json.loads(mp.read_text()) != manifest:
        raise ValueError('judge manifest mismatch: use a new hypothesis file')
    if not mp.exists():
        mp.write_text(json.dumps(manifest, indent=2) + '\n')
        mp.chmod(0o600)
    previous = resolved_judgments(path)
    done = {r['question_id']: r for r in previous}
    if len(done) != len(previous) or not set(done).issubset(hyp_ids):
        raise ValueError('invalid judgment checkpoint IDs')
    if not args.summarize_only:
        fn = official_prompt(args.upstream)
        env = credentials()
        if args.retry_invalid and set(done) != set(hyp_ids):
            raise ValueError('finish original pass before invalid-verdict recovery')
        jobs = [h for h in hyps if h['question_id'] not in done or (args.retry_invalid and done[h['question_id']].get('label') is None)]
        output_path = Path(str(path) + '.repairs.jsonl') if args.retry_invalid else path
        with output_path.open('a') as f, concurrent.futures.ThreadPoolExecutor(max_workers=args.workers) as executor:
            output_path.chmod(0o600)
            future_map = {executor.submit(judge_one, refs[h['question_id']], h, fn, env, args): h for h in jobs}
            for future in concurrent.futures.as_completed(future_map):
                r = future.result()
                f.write(json.dumps(r, ensure_ascii=False) + '\n')
                f.flush()
                os.fsync(f.fileno())
                done[r['question_id']] = r
                print(f"judged={len(done)}/{len(hyps)} id={r['question_id']} label={r['label']} error={r.get('error', '')}", flush=True)
        done = {r['question_id']: r for r in resolved_judgments(path)}
    summary = summarize(refs, hyps, list(done.values()))
    summary.update({'judge': args.judge, 'official_prompt': True, 'dataset_sha256': manifest['data_sha256'],
                    'complete': len(hyps) == 500 and summary['judge_errors_or_missing'] == 0})
    Path(args.hypotheses + '.summary-' + args.judge.replace('/', '_') + '.json').write_text(json.dumps(summary, indent=2) + '\n')
    print(json.dumps(summary, indent=2))
    if summary['judge_errors_or_missing']:
        raise SystemExit(2)


if __name__ == '__main__':
    main()
