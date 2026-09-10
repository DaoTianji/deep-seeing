"""Four bounded, synthetic provider probes. Never prints credentials or vectors."""
import json
import pathlib
import urllib.request
import urllib.error
import time

values = {}
for line in pathlib.Path('.env.local').read_text().splitlines():
    if '=' in line and not line.lstrip().startswith('#'):
        k, v = line.split('=', 1)
        values[k.strip()] = v.strip().strip('\"\'')
key = values['SILICONFLOW_API_KEY']

def request(path, payload):
    started = time.monotonic()
    req = urllib.request.Request('https://api.siliconflow.cn/v1/' + path,
        data=json.dumps(payload).encode(), headers={'Authorization': 'Bearer ' + key, 'Content-Type': 'application/json'})
    try:
        with urllib.request.urlopen(req, timeout=120) as response:
            result = json.load(response)
    except urllib.error.HTTPError as error:
        raise SystemExit('Provider HTTP ' + str(error.code)) from None
    print(json.dumps({'endpoint': path, 'elapsed_seconds': round(time.monotonic()-started, 2),
                      'usage': result.get('usage')}, ensure_ascii=False), flush=True)
    return result

base = {'model': 'deepseek-ai/DeepSeek-V4-Pro', 'max_tokens': 160, 'enable_thinking': False}
reply = request('chat/completions', {**base, 'messages': [{'role':'user', 'content':'这是纯虚构连通性测试，只回复：连接成功。'}]})
assert reply['choices'][0]['message'].get('content'), 'Missing answer'
call = request('chat/completions', {**base, 'messages': [{'role':'user', 'content':'Call lookup with topic=test. Do not answer directly.'}],
    'tools': [{'type':'function','function': {'name':'lookup','description':'Synthetic lookup','parameters': {'type':'object','properties':{'topic':{'type':'string'}},'required':['topic']}}}],
    'tool_choice': {'type':'function', 'function':{'name':'lookup'}}})
assert call['choices'][0]['message']['tool_calls'][0]['function']['name'] == 'lookup'
embedded = request('embeddings', {'model':'Qwen/Qwen3-Embedding-8B','input':['虚构的小林喜欢蓝色。','小林偏爱的颜色'], 'dimensions':1536})
assert len(embedded['data']) == 2 and len(embedded['data'][0]['embedding']) == 1536
ranked = request('rerank', {'model':'Qwen/Qwen3-Reranker-8B','query':'小林喜欢什么颜色？','documents':['虚构的小林喜欢蓝色。','小林明天吃面条。'],'top_n':2,'return_documents':False})
assert ranked['results'][0]['index'] == 0
print('PASS: answer, tool call, 1536-dimensional embedding, reranking')
