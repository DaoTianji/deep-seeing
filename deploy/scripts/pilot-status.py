"""Read only counters and public progress from the isolated pilot."""
import collections,json,subprocess
from pathlib import Path
base=Path('/var/lib/deep-seeing-eval50')
p=base/'budget.json'
if p.exists():
    b=json.loads(p.read_text());calls=b['calls']
    print(json.dumps({'budget_cny':b['cap_nano_cny']/1e9,'conservative_spent_cny':round(b['charged_nano_cny']/1e9,4),
        'calls':len(calls),'states':dict(collections.Counter(c['status'] for c in calls)),
        'by_model':dict(collections.Counter(c['model'] for c in calls))}))
for case in sorted((base/'hindsight-pilot-50/cases').glob('*')):
    jp=case/'retain.json';mp=case/'sources.json'
    states=json.loads(jp.read_text()) if jp.exists() else {}
    sources=json.loads(mp.read_text()) if mp.exists() else {}
    print(json.dumps({'case':case.name,'sources':len(sources),'retains':dict(collections.Counter(states.values())),
        'answer_saved':(base/'hindsight-pilot-50'/(case.name+'.result.json')).exists(),
        'paused':(case/'paused.json').exists()}))
for unit in ['deep-seeing-pilot-budget','deep-seeing-pilot-hindsight','deep-seeing-pilot-run','deep-seeing']:
    r=subprocess.run(['systemctl','show',unit,'-p','ActiveState','-p','SubState','-p','ExecMainStatus'],capture_output=True,text=True)
    print(unit,r.stdout.strip().replace('\n',' '))
