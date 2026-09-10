#!/usr/bin/env python3
"""Prepare a NEW checkpoint retaining every non-quota result, including failures.
Never calls a model and never modifies the source run or selects better answers.
"""
import argparse
import hashlib
import json
from pathlib import Path


def quota_failure(error):
    return any(s in error.lower() for s in ('insufficient_user_quota', 'insufficient_quota', '用户额度不足', '预扣费额度失败'))


def prepare(source, target):
    source, target = Path(source), Path(target)
    outputs = [target, Path(str(target)+'.manifest.json'), Path(str(target)+'.recovery.json')]
    if source.resolve() == target.resolve() or any(p.exists() for p in outputs):
        raise ValueError('recovery requires a new, unused output path')
    raw = source.read_bytes()
    lines = raw.splitlines(keepends=True)
    rows = [json.loads(line) for line in lines]
    ids = [r['question_id'] for r in rows]
    if len(ids) != len(set(ids)):
        raise ValueError('duplicate source IDs')
    manifest = Path(str(source)+'.manifest.json').read_bytes()
    cfg = json.loads(manifest)
    if cfg['limit'] != 0:
        raise ValueError('not a full-run checkpoint')
    if any(r['mode'] != cfg['mode'] or r['model'] != cfg['model'] for r in rows):
        raise ValueError('source configuration mismatch')
    skipped = [r for r in rows if quota_failure(r.get('error', ''))]
    kept = [line for line, r in zip(lines, rows) if not quota_failure(r.get('error', ''))]
    audit = {'source': str(source), 'source_sha256': hashlib.sha256(raw).hexdigest(),
             'retained': len(kept), 'quota_recovery_ids': [r['question_id'] for r in skipped],
             'prior_quota_attempt_tokens_reported': sum(r.get('tokens', {}).get('total_tokens', 0) for r in skipped),
             'semantics': 'all non-quota results retained byte-for-byte, including max-step failures; no model calls'}
    target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    for path, content in zip(outputs, [b''.join(kept), manifest, (json.dumps(audit, indent=2)+'\n').encode()]):
        with path.open('xb') as f:
            path.chmod(0o600)
            f.write(content)
    return {'target': str(target), 'retained': len(kept), 'retry_quota': len(skipped)}


if __name__ == '__main__':
    p = argparse.ArgumentParser()
    p.add_argument('--source', required=True)
    p.add_argument('--target', required=True)
    a = p.parse_args()
    print(json.dumps(prepare(a.source, a.target), indent=2))
