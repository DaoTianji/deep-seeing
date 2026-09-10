"""Independent official-prompt grading through the same 50-CNY cost gate."""
import argparse,importlib.util,json,os,time,urllib.request
from pathlib import Path

p=argparse.ArgumentParser();p.add_argument('--data',required=True);p.add_argument('--root',required=True);p.add_argument('--official',required=True);p.add_argument('--scorer',required=True);a=p.parse_args()
spec=importlib.util.spec_from_file_location('scorer',a.scorer);module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
prompt_fn=module.official_prompt(a.official)
assert os.environ['OPENAI_BASE_URL']=='http://127.0.0.1:8890/v1'
root=Path(a.root);refs=json.loads(Path(a.data).read_text());assert len(refs)==6
judged=[]
for ref in refs:
    path=root/(ref['question_id']+'.result.json');jp=root/(ref['question_id']+'.judgment.json')
    if not path.exists():continue
    result=json.loads(path.read_text())
    if result.get('error'):continue
    if jp.exists():
        row=json.loads(jp.read_text());judged.append(row);continue
    row={'question_id':ref['question_id'],'category':ref['question_type'],'status':'uncertain'}
    # Never silently repeat an uncertain paid judgment after restart.
    with os.fdopen(os.open(jp,os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600),'w') as f:json.dump(row,f);f.flush();os.fsync(f.fileno())
    prompt=prompt_fn(ref['question_type'],ref['question'],ref['answer'],result['hypothesis'],abstention='_abs' in ref['question_id'])
    payload={'model':'deepseek-ai/DeepSeek-V4-Pro','messages':[{'role':'user','content':prompt}], 'temperature':0,'max_tokens':16,'enable_thinking':False}
    req=urllib.request.Request(os.environ['OPENAI_BASE_URL']+'/chat/completions',data=json.dumps(payload).encode(),headers={'Authorization':'Bearer '+os.environ['OPENAI_API_KEY'],'Content-Type':'application/json'})
    try:
        with urllib.request.urlopen(req,timeout=60) as response:data=json.load(response)
        verdict=data['choices'][0]['message']['content'].strip().lower().rstrip('.')
        if verdict not in ['yes','no']:raise ValueError('invalid verdict')
        row.update(status='judged',correct=verdict=='yes',usage=data.get('usage',{}))
    except Exception as e:row.update(error=type(e).__name__)
    with jp.open('w') as f:json.dump(row,f);f.flush();os.fsync(f.fileno())
    judged.append(row)
valid=[r for r in judged if r['status']=='judged']
print(json.dumps({'fixed_cases':6,'completed_answers':len(list(root.glob('*.result.json'))),'valid_judgments':len(valid),'correct':sum(r['correct'] for r in valid),'judgments':judged}))
