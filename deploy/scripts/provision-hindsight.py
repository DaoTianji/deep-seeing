"""Run as root; API credential arrives on stdin, never command arguments."""
import json
import os
import pathlib
import subprocess
import sys

assert os.geteuid() == 0
root = pathlib.Path('/etc/deep-seeing/memory')
key = sys.stdin.read().strip()
assert key and '\n' not in key
password = (root / 'hindsight-db-password').read_text().strip()
def run(*args):
    result = subprocess.run(args, capture_output=True, text=True)
    if result.returncode:
        raise SystemExit('Operation failed: ' + ' '.join(args[:3]))
    return result.stdout.strip()
# Official slim Python distribution, isolated from the system interpreter.
# The registry image duplicated two 320 MB layers and repeatedly timed out.
assert pathlib.Path('/opt/deep-seeing/hindsight-0.9.2-locked/bin/python').is_file()
config = {
    'HINDSIGHT_API_DATABASE_URL': 'postgresql://hindsight:'+password+'@127.0.0.1:5457/hindsight',
    'HINDSIGHT_API_HOST':'127.0.0.1','HINDSIGHT_API_PORT':'8889',
    'HINDSIGHT_API_LLM_PROVIDER':'openai', 'HINDSIGHT_API_LLM_API_KEY':key,
    'HINDSIGHT_API_LLM_MODEL':'deepseek-ai/DeepSeek-V4-Pro',
    'HINDSIGHT_API_LLM_BASE_URL':'https://api.siliconflow.cn/v1',
    'HINDSIGHT_API_LLM_EXTRA_BODY':'{"enable_thinking":false}',
    'HINDSIGHT_API_LLM_MAX_CONCURRENT':'1','HINDSIGHT_API_LLM_MAX_RETRIES':'1',
    'HINDSIGHT_API_LLM_TIMEOUT':'120','HINDSIGHT_API_RETAIN_MAX_COMPLETION_TOKENS':'8192',
    'HINDSIGHT_API_RETAIN_CHUNK_SIZE':'4096','HINDSIGHT_API_RETAIN_MAX_CONCURRENT':'1',
    'HINDSIGHT_API_EMBEDDINGS_PROVIDER':'openai', 'HINDSIGHT_API_EMBEDDINGS_OPENAI_API_KEY':key,
    'HINDSIGHT_API_EMBEDDINGS_OPENAI_BASE_URL':'https://api.siliconflow.cn/v1',
    'HINDSIGHT_API_EMBEDDINGS_OPENAI_MODEL':'Qwen/Qwen3-Embedding-8B',
    'HINDSIGHT_API_EMBEDDINGS_OPENAI_DIMENSIONS':'1536','HINDSIGHT_API_EMBEDDINGS_OPENAI_BATCH_SIZE':'16',
    'HINDSIGHT_API_RERANKER_PROVIDER':'siliconflow','HINDSIGHT_API_RERANKER_SILICONFLOW_API_KEY':key,
    'HINDSIGHT_API_RERANKER_SILICONFLOW_MODEL':'Qwen/Qwen3-Reranker-8B',
    'HINDSIGHT_API_ENABLE_OBSERVATIONS':'false','HINDSIGHT_API_ENABLE_AUTO_CONSOLIDATION':'false',
    'HINDSIGHT_API_WORKER_ENABLED':'false','HINDSIGHT_API_OTEL_TRACES_ENABLED':'false',
    'HINDSIGHT_API_DB_POOL_MIN_SIZE':'1','HINDSIGHT_API_DB_POOL_MAX_SIZE':'8',
    'HINDSIGHT_ENABLE_CP':'false','HINDSIGHT_API_LOG_LEVEL':'warning',
}
env = root / 'hindsight.env'
fd = os.open(env, os.O_WRONLY|os.O_CREAT|os.O_EXCL, 0o600)
with os.fdopen(fd,'w') as f:
    f.write(''.join(k+'='+json.dumps(v)+'\n' for k,v in config.items()))
print('Configured Hindsight 0.9.2; credentials written with mode 0600')
