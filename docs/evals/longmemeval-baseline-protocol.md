# LongMemEval-S baseline protocol

Frozen baseline: `974d4e4e9f47896917fe079994a866bdfd260f4d`.
Branch: `codex/eval-longmemeval-baseline`.

## Scope and interpretation

Run all 500 questions in the authors' cleaned LongMemEval-S dataset. This is an
English, raw-session Episode memory / native T2 tool baseline, not a claim that
production T1 extraction, Bond, T3 reflection, or role learning has been evaluated.
The production retrieval algorithm, candidate formatting, read/evidence tools,
and 16-step Eino Agent are unchanged. In particular, the known permissive
two-character matching is deliberately preserved to measure the starting point.

Two configurations use the same configured answer model (`gpt-5.6-sol`):

1. **native**: every historical conversation session is inserted losslessly as
   one Episode, oldest first. Agent may search, read, and report recall evidence.
2. **no-memory**: question and question timestamp only, no history or tools.

Both receive the same neutral English benchmark persona. Native also receives
the existing production recall guidance. Native uses normal production model
defaults; the no-memory caller has a 2,048-token output ceiling. Thus no-memory is
a basic missing-information control, not a compute-matched comparison to another
memory system. This run does not establish superiority to BM25, vector RAG, or
full-context reading.

## Frozen assets

- Official repository: https://github.com/xiaowu0162/LongMemEval
- Upstream commit: `9e0b455f4ef0e2ab8f2e582289761153549043fc`.
- Dataset: `xiaowu0162/longmemeval-cleaned/longmemeval_s_cleaned.json`.
- Downloaded via `hf-mirror.com` after the original hostname timed out.
- Dataset bytes: 277,383,467.
- Dataset SHA-256 (matches repository LFS metadata):
  `d6f21ea9d60a0d56f34a05b609c79c88a451d2ae03597821ea3d5a9678c3a442`.
- Upstream `src/evaluation/evaluate_qa.py` SHA-256:
  `ecce9c4c79dc89d99534ac17b383a5cbb5b9f0c69ee98adaf0684742e3d95251`.
- 500 questions: knowledge-update 78, multi-session 133,
  single-session-assistant 56, single-session-preference 30,
  single-session-user 70, temporal-reasoning 133. Abstention is identified by the
  official `_abs` question-ID convention and reported separately.

The first two items are a technical smoke test, not a held-out set. No semantic
prompt or retrieval tuning follows their scores. The complete dataset is public;
this is a reproducible benchmark run, not proof of uncontaminated generalization.

## No leakage or production writes

The inference input type excludes question ID/type, answer, answer_session_ids,
source session IDs, and per-turn has_answer annotations. Only historical message
role/content, historical timestamps, question text and question timestamp cross
the boundary. Benchmark session identifiers are mapped back **after inference**
for diagnostics. Import timestamps are explicitly distinguished from event dates.

Each question gets a fresh temporary EpisodeStore and fresh in-memory STM. The
adapter never constructs the production App, opens production files, connects to
Redis/Neo4j, loads personal Soul/Bond, or invokes PostTurn/Review/Dream. Only the
three read/evidence tools are exposed. Gold answers are only available to the
separate scorer. Answer runs do not share state. Worker concurrency is bounded.

## Judging

Use the official `get_anscheck_prompt` function unchanged, loaded from its AST to
avoid importing unrelated SDK dependencies. Default judge is the upstream
`gpt-4o-2024-08-06`, temperature 0, max_tokens 10, one user message. Gateway routing
differs from the upstream SDK client; model identity must be recorded. If the
gateway cannot provide the official judge, report the actual alternate judge and
label the result non-identical to the official model protocol.

Actual run: the snapshot `gpt-4o-2024-08-06` returned HTTP 503 on both smoke
questions after bounded retries. The gateway's listed `gpt-4o` alias completed
both judge calls with valid yes/no responses. Full-run judging therefore uses
`gpt-4o` with the official prompt, temperature and output limit, but an unpinned
gateway snapshot. Do not describe this as an exact official-snapshot reproduction
or a submitted leaderboard result. The failed snapshot probes remain in data/.

Exact yes/no verdicts use the upstream yes/no interpretation. Unexpected judge
outputs are recorded as errors, not guessed as success. The scorer does not send
the full histories to the judge. Inference runtime failures remain in the
denominator and are reported independently. Only transport/rate-limit/timeouts
have up to three attempts; wrong answers and step exhaustion are never retried
for a better score. Every attempt failure is retained. First successful semantic
result wins; no best-of selection.

## Measurement and completion

- Overall question-weighted accuracy, Wilson 95% interval, per-type accuracy,
  type-macro average, and abstention performance.
- Gold-session recall at candidate, read, and used stages for answerable items;
  never equate retrieval recall with answer accuracy.
- Tool counts, no-search count, p50/p95 total answer latency, ingestion duration,
  and reported token usage. Token sums cover successful final inference attempts
  and returned judge usage; failed transport attempts may incur unreported cost.
- Both configurations must contain all 500 unique official question IDs and all
  500 valid judge verdicts. Partial smoke results are not the final benchmark.
- Save raw answers, traces, manifests and judgments under ignored `data/evals/`.
  Commit only protocol, harness/tests, and aggregate report without answer text.
- Low accuracy is a valid baseline outcome; never repair the algorithm mid-run.

## Reproduce

From the repository root, credentials are loaded from `.env` without printing them:

```sh
go test ./cmd/eval-longmemeval
go run ./cmd/eval-longmemeval -validate
go run ./cmd/eval-longmemeval -mode native -workers 4 \
  -out data/evals/longmemeval-baseline/native.jsonl
go run ./cmd/eval-longmemeval -mode no-memory -workers 4 \
  -out data/evals/longmemeval-baseline/no-memory.jsonl
python3 scripts/score-longmemeval.py \
  --data data/evals/longmemeval-baseline/longmemeval_s_cleaned.mirror.json \
  --hypotheses data/evals/longmemeval-baseline/native.jsonl --judge gpt-4o
python3 scripts/score-longmemeval.py \
  --data data/evals/longmemeval-baseline/longmemeval_s_cleaned.mirror.json \
  --hypotheses data/evals/longmemeval-baseline/no-memory.jsonl --judge gpt-4o
```

Output files are checkpointed after each question. Rerunning the identical command
resumes; changed dataset/model/protocol settings reject the checkpoint. Use a new
output path for a changed configuration. Do not run two writers against one file.
