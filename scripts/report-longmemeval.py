#!/usr/bin/env python3
"""Audit two completed LongMemEval runs and emit an answer-free report."""
import argparse
import hashlib
import json
import random
import runpy
import statistics
from collections import Counter
from pathlib import Path


def lines(path):
    return [json.loads(s) for s in Path(path).read_text().splitlines() if s.strip()]


def audited(path, judge):
    hs = lines(path)
    scorer = runpy.run_path(str(Path(__file__).with_name('score-longmemeval.py')))
    js = scorer['resolved_judgments'](str(path) + '.judgments-' + judge + '.jsonl')
    summary = json.loads(Path(str(path) + '.summary-' + judge + '.json').read_text())
    manifest = json.loads(Path(str(path) + '.manifest.json').read_text())
    jm = json.loads(Path(str(path) + '.judgments-' + judge + '.jsonl.manifest.json').read_text())
    hid = [x['question_id'] for x in hs]
    jid = [x['question_id'] for x in js]
    assert len(hid) == len(set(hid)) == 500, 'incomplete/duplicate hypotheses'
    assert len(jid) == len(set(jid)) == 500 and set(hid) == set(jid), 'incomplete judgments'
    assert summary['complete'] and summary['judge'] == judge, 'summary incomplete or wrong judge'
    assert manifest['limit'] == 0, 'not a full run'
    assert hashlib.sha256(Path(path).read_bytes()).hexdigest() == jm['hypotheses_sha256'], 'stale judgments'
    assert manifest['data_sha256'] == jm['data_sha256'] == summary['dataset_sha256'], 'dataset mismatch'
    hmap = {x['question_id']: x for x in hs}
    scores = {}
    for j in js:
        h = hmap[j['question_id']]
        assert isinstance(j['label'], bool), 'invalid verdict'
        assert hashlib.sha256(h['hypothesis'].encode()).hexdigest() == j['hypothesis_sha256'], 'answer changed'
        scores[j['question_id']] = int(j['label'] and not h.get('error'))
    assert sum(scores.values()) == summary['correct'], 'incorrect score aggregation'
    return hs, summary, manifest, scores


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--root', default='data/evals/longmemeval-baseline')
    p.add_argument('--out', default='docs/evals/longmemeval-baseline-results.md')
    p.add_argument('--judge', default='gpt-4o')
    a = p.parse_args()
    root = Path(a.root)
    h, n, nm, ns = audited(root / 'native.jsonl', a.judge)
    _, b, bm, bs = audited(root / 'no-memory.jsonl', a.judge)
    assert nm['model'] == bm['model'] and nm['data_sha256'] == bm['data_sha256']
    assert set(ns) == set(bs), 'paired IDs differ'
    delta = [ns[k] - bs[k] for k in sorted(ns)]
    rng = random.Random(20260908)
    draws = sorted(statistics.mean(rng.choices(delta, k=len(delta))) for _ in range(10000))
    interval = [draws[249], draws[9749]]
    win = sum(v == 1 for v in delta)
    loss = sum(v == -1 for v in delta)
    diagnostics = Counter()
    # Diagnostic funnel uses annotated source IDs after inference only.
    with (root / 'longmemeval_s_cleaned.mirror.json').open() as f:
        refs = {x['question_id']: set(x['answer_session_ids']) for x in json.load(f)}
    assert set(ns) == set(refs), 'result IDs do not match official dataset'
    for row in h:
        qid = row['question_id']
        if '_abs' in qid or not refs[qid]:
            continue
        gold = refs[qid]
        if not row.get('searches'):
            diagnostics['no_search'] += 1
        elif not (gold & set(row.get('candidate_session_ids') or [])):
            diagnostics['no_gold_candidate'] += 1
        elif not (gold & set(row.get('read_session_ids') or [])):
            diagnostics['candidate_not_read'] += 1
        elif not ns[qid]:
            diagnostics['read_some_gold_but_wrong'] += 1
        else:
            diagnostics['read_some_gold_and_correct'] += 1
    def pct(x): return f'{100*x:.1f}%'
    def ci(s): return '–'.join(pct(x) for x in s['accuracy_wilson_95'])
    report = [
        '# LongMemEval-S 完整基线结果', '',
        '**评分可信度提示：网关 gpt-4o 裁判出现可核验的误判；自动分数不是人工确认的准确率。请结合裁判审计与第二裁判复核解读，不用于榜单排名。**', '',
        f'测评日期：2026-09-08。基线分支：`codex/eval-longmemeval-baseline`。', '',
        '## 完整性与协议', '',
        '- 两组分别完成全部 500 个唯一题号、500 个有效裁判结果；校验结果和答案哈希一致。',
        f'- 答题模型：`{nm["model"]}`；裁判：`{a.judge}` 网关别名。',
        '- 使用官方清洗数据及官方分类评分提示、temperature=0、max_tokens=10。',
        '- 官方指定的 GPT-4o 日期快照不可用；网关别名未锁定快照。结果不是完全相同模型协议的官方榜单提交。',
        '- 原生组为原文会话 Episode + 生产搜索/读取/证据工具，不包含 T1 自动提炼、Bond、T3 反思或角色塑造。',
        '- 原生检索的宽松两字符匹配没有修复。无记忆组是信息缺失对照，不是强 RAG 或全文上下文对照。',
        '- 本次只有一次完整运行；同一题内的模型随机波动与跨次稳定性尚未测量。', '',
        '## 总分', '',
        '| 配置 | 自动判对题数 | 自动准确率 | 95% Wilson 区间 | 分类宏平均 |',
        '|---|---:|---:|---:|---:|',
        f'| 原生记忆 | {n["correct"]}/500 | {pct(n["accuracy"])} | {ci(n)} | {pct(n["task_macro_accuracy"])} |',
        f'| 无记忆 | {b["correct"]}/500 | {pct(b["accuracy"])} | {ci(b)} | {pct(b["task_macro_accuracy"])} |', '',
        f'配对差值：{100*statistics.mean(delta):+.1f} 个百分点；按题重采样 10,000 次的 95% 区间为 '
        f'{100*interval[0]:+.1f} 至 {100*interval[1]:+.1f} 个百分点。原生独对 {win} 题，无记忆独对 {loss} 题。', '',
        '该区间描述这套题上的样本不确定性；不涵盖模型跨运行波动、公开题集污染或裁判偏差。', '',
        '## 分项', '', '| 类别 | 题数 | 原生记忆 | 无记忆 |', '|---|---:|---:|---:|',
    ]
    for k, v in n['categories'].items():
        report.append(f'| {k} | {v["total"]} | {v["correct"]} / {pct(v["accuracy"])} | {b["categories"][k]["correct"]} / {pct(b["categories"][k]["accuracy"])} |')
    report += ['', f'无法回答子集：原生 {n["abstention"]["correct"]}/{n["abstention"]["total"]}；无记忆 {b["abstention"]["correct"]}/{b["abstention"]["total"]}。该子集已包含在总分中，不额外加权。', '',
               f'可回答子集：原生 {n["correct"] - n["abstention"]["correct"]}/470；无记忆 {b["correct"] - b["abstention"]["correct"]}/470。', '',
               '## 检索诊断', '',
               f'- 证据会话平均召回率：候选 {pct(n["retrieval"]["candidate_session_recall"])}；实际读取 {pct(n["retrieval"]["read_session_recall"])}；声明采用 {pct(n["retrieval"]["used_session_recall"])}。',
               '- 分母仅包括有标注证据的可回答题；这些是 session-level recall，不是回答正确率或任意排名 k 下的 Recall@k。',
               '- 以下互斥分组以至少一个标注证据命中为准；“读取部分证据但答错”仍可能缺少其他必要证据，不能直接归咎于推理：', '']
    labels = {'no_search': '没有搜索', 'no_gold_candidate': '搜索但未命中任何标注证据',
              'candidate_not_read': '命中证据候选但未读取',
              'read_some_gold_but_wrong': '读取至少部分证据但裁判判错或运行失败',
              'read_some_gold_and_correct': '读取至少部分证据且裁判判对'}
    for k, v in sorted(diagnostics.items()):
        report.append(f'  - {labels[k]}: {v}')
    report += ['', '## 成本与延迟', '', '| 指标 | 原生记忆 | 无记忆 |', '|---|---:|---:|',
               f'| 回答总延迟 p50 / p95 | {n["answer_seconds_p50"]:.2f}s / {n["answer_seconds_p95"]:.2f}s | {b["answer_seconds_p50"]:.2f}s / {b["answer_seconds_p95"]:.2f}s |',
               f'| 平均搜索 / 读取次数 | {n["mean_searches"]:.2f} / {n["mean_reads"]:.2f} | 0 / 0 |',
               f'| 每题平均累计搜索 / 读取耗时 | {n["mean_search_time_ms_per_question"]:.2f}ms / {n["mean_read_time_ms_per_question"]:.2f}ms | 0 / 0 |',
               f'| 答题模型总 tokens（输入及输出） | {n["inference_tokens_successful_final_attempts"]:,} | {b["inference_tokens_successful_final_attempts"]:,} |',
               f'| 裁判报告 tokens | {n["judge_tokens_reported"]:,} | {b["judge_tokens_reported"]:,} |',
               f'| 推理错误 / 裁判缺失 | {n["inference_errors"]} / {n["judge_errors_or_missing"]} | {b["inference_errors"]} / {b["judge_errors_or_missing"]} |', '',
               '延迟包含原生 Agent 多轮模型调用、工具和最终回答；不等于数据库检索耗时。四个并发题目运行时的测量可能受网关负载影响。Token 数不含无回执的失败调用成本，也不直接等同于账单。', '',
               '## 可复核产物', '',
               '- 固定协议与复现命令：[longmemeval-baseline-protocol.md](longmemeval-baseline-protocol.md)。',
               '- 裁判可靠性审计：[longmemeval-judge-audit.md](longmemeval-judge-audit.md)。',
               '- 原始数据、答案、工具轨迹、评分回执、配置与哈希：被忽略的 `data/evals/longmemeval-baseline/`。',
               '- 汇总脚本检查 500 题完整性、重复题号、推理与评分答案哈希一致、裁判有效性及汇总分数。',
               '- 基线实现未更改生产逻辑；未部署、未发布角色、未使用正式记忆。', '']
    Path(a.out).write_text('\n'.join(report))
    print(json.dumps({'report': a.out, 'native_accuracy': n['accuracy'], 'no_memory_accuracy': b['accuracy'],
                      'delta': statistics.mean(delta), 'paired_bootstrap_95': interval,
                      'diagnostic_counts': dict(diagnostics)}, indent=2))


if __name__ == '__main__':
    main()
