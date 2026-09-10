"""Small server-side probes. Only synthetic prompts; credentials stay local."""
import json
import os
from pathlib import Path
import subprocess
import sys
import urllib.request
import urllib.error

config={}
for line in Path('/etc/deep-seeing/memory/hindsight.env').read_text().splitlines():
    k,v=line.split('=',1);config[k]=json.loads(v)
key=config['HINDSIGHT_API_LLM_API_KEY']
if len(sys.argv)>1 and sys.argv[1]=='memory':
    env=dict(os.environ,OPENAI_API_KEY=key,OPENAI_BASE_URL='https://api.siliconflow.cn/v1',
        OPENAI_MODEL='deepseek-ai/DeepSeek-V4-Pro',HINDSIGHT_URL='http://127.0.0.1:8889')
    result=subprocess.run(['/opt/deep-seeing/ops/deep-seeing-memory-smoke','-run'],env=env,capture_output=True,text=True,timeout=400)
    print(result.stdout.replace(key,'[REDACTED]'))
    if result.returncode:print(result.stderr.replace(key,'[REDACTED]'));sys.exit(result.returncode)
else:
    payload={'model':'deepseek-ai/DeepSeek-V4-Pro','enable_thinking':False,'max_tokens':160,
        'messages':[{'role':'user','content':'Synthetic test. Return JSON with fact set to ORCHID.'}],
        'response_format':{'type':'json_schema','json_schema':{'name':'fixture','strict':True,
            'schema':{'type':'object','properties':{'fact':{'type':'string'}},'required':['fact'],'additionalProperties':False}}}}
    req=urllib.request.Request('https://api.siliconflow.cn/v1/chat/completions',data=json.dumps(payload).encode(),
        headers={'Authorization':'Bearer '+key,'Content-Type':'application/json'})
    try:
        with urllib.request.urlopen(req,timeout=60) as response: data=json.load(response)
        assert json.loads(data['choices'][0]['message']['content'])['fact']=='ORCHID'
        print(json.dumps({'structured_output':True,'usage':data.get('usage')}))
    except urllib.error.HTTPError as e:
        print(json.dumps({'structured_output':False,'http_status':e.code}));sys.exit(1)
