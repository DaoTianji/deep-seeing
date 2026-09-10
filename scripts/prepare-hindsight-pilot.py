"""Freeze the first item of each of six categories, with full histories intact."""
import hashlib,json,os
from pathlib import Path
source=Path('data/evals/longmemeval-baseline/longmemeval_s_cleaned.mirror.json')
raw=source.read_bytes();assert hashlib.sha256(raw).hexdigest()=='d6f21ea9d60a0d56f34a05b609c79c88a451d2ae03597821ea3d5a9678c3a442'
seen=set();selected=[]
for row in json.loads(raw):
    if row['question_type'] not in seen:
        seen.add(row['question_type']);selected.append(row)
assert len(selected)==6
root=Path('data/evals/hindsight-pilot-50');root.mkdir(mode=0o700,exist_ok=True)
data=json.dumps(selected,ensure_ascii=False).encode();p=root/'pilot-data.json'
if p.exists():assert p.read_bytes()==data
else:
    with os.fdopen(os.open(p,os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600),'wb') as f:f.write(data)
print(json.dumps({'selected':[{'id':r['question_id'],'category':r['question_type'],'sessions':len(r['haystack_sessions'])} for r in selected],
    'sessions':sum(len(r['haystack_sessions']) for r in selected),'sha256':hashlib.sha256(data).hexdigest()}))
