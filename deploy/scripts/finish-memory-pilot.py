"""One-shot completion stage of the requested pilot, not a recurring task.
Wait for the runner, grade completed answers under its budget, stop test services.
"""
import collections,json,os,subprocess,time,sys
from pathlib import Path

base=Path('/var/lib/deep-seeing-eval50');root=base/'hindsight-pilot-50'
def state():return subprocess.check_output(['systemctl','show','deep-seeing-pilot-run','-p','ActiveState','--value'],text=True).strip()
score_status='existing_receipts'
if '--summarize-only' not in sys.argv:
    deadline=time.monotonic()+5*3600
    while state() in ['active','activating','deactivating'] and time.monotonic()<deadline:
        time.sleep(15)
    if state()=='active':
        subprocess.run(['systemctl','stop','deep-seeing-pilot-run'],check=True)
    env=dict(os.environ)
    for line in Path('/etc/deep-seeing/pilot50/runner.env').read_text().splitlines():
        k,v=line.split('=',1);env[k]=json.loads(v)
    score_status='not_run'
    try:
        r=subprocess.run(['/usr/bin/python3','/opt/deep-seeing/ops/score-hindsight-pilot.py','--root',str(root),
            '--data',str(base/'pilot-data.json'),'--official','/opt/deep-seeing/ops/evaluate_qa.py',
            '--scorer','/opt/deep-seeing/ops/score-longmemeval.py'],env=env,capture_output=True,text=True,timeout=420)
        score_status='complete' if r.returncode==0 else 'failed'
    except subprocess.TimeoutExpired:
        score_status='timeout'
    finally:
        subprocess.run(['systemctl','stop','deep-seeing-pilot-run','deep-seeing-pilot-hindsight'],check=False)
        subprocess.run(['systemctl','stop','deep-seeing-pilot-budget'],check=False)
rows=[json.loads(p.read_text()) for p in sorted(root.glob('*.result.json'))]
judgments=[json.loads(p.read_text()) for p in sorted(root.glob('*.judgment.json'))]
budget=json.loads((base/'budget.json').read_text())
summary={'fixed_cases':6,'completed_answers':len(rows),'score_stage':score_status,
 'budget_cny':budget['cap_nano_cny']/1e9,'execution_complete':len(rows)==6,
 'results_without_errors':sum(not r.get('error') for r in rows),
 'technical_failures':sum(bool(r.get('error')) for r in rows),'incomplete_cases':6-len(rows),
 'judged':sum(j.get('status')=='judged' for j in judgments),'correct':sum(j.get('correct') is True for j in judgments),
 'complete':len(rows)==6 and len(judgments)==6 and all(j.get('status')=='judged' for j in judgments),
 'conservative_cost_cny':budget['charged_nano_cny']/1e9,'api_calls':len(budget['calls']),
 'billing_states':dict(collections.Counter(c['status'] for c in budget['calls'])),
 'categories':[{'id':r['question_id'],'type':r['question_type'],'error':bool(r.get('error')),
    'answer_seconds':r['answer_seconds'],'ingest_seconds':r['ingest_seconds'],
    'searches':len(r.get('searches') or []),'reads':len(r.get('reads') or []),'used':len(r.get('used_session_ids') or [])} for r in rows],
 'judgments':[{k:v for k,v in j.items() if k!='usage'} for j in judgments]}
with os.fdopen(os.open(base/'summary.json',os.O_CREAT|os.O_WRONLY|os.O_TRUNC,0o600),'w') as f:
    json.dump(summary,f,indent=2);f.flush();os.fsync(f.fileno())
print(json.dumps(summary))
