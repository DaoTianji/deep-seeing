import importlib.util
from pathlib import Path
import tempfile
import concurrent.futures
import json
import unittest

spec = importlib.util.spec_from_file_location('gate', Path(__file__).with_name('pilot-budget-proxy.py'))
gate = importlib.util.module_from_spec(spec); spec.loader.exec_module(gate)

class BudgetTests(unittest.TestCase):
    def test_amended_cap_preserves_spend_and_receipts(self):
        with tempfile.TemporaryDirectory() as d:
            path=Path(d)/'ledger.json'
            b=gate.Budget(path);b.reserve('deepseek-ai/DeepSeek-V4-Pro',gate.CAP)
            with self.assertRaises(ValueError): gate.Budget(path,2*gate.CAP)
            state=json.loads(path.read_text());state['cap_nano_cny']=2*gate.CAP
            path.write_text(json.dumps(state))
            b=gate.Budget(path,2*gate.CAP)
            self.assertEqual(b.state['charged_nano_cny'],gate.CAP)
            self.assertEqual(len(b.state['calls']),1)
            self.assertEqual(b.reserve('deepseek-ai/DeepSeek-V4-Pro',gate.CAP),1)
            self.assertIsNone(b.reserve('deepseek-ai/DeepSeek-V4-Pro',1))
            self.assertEqual(len(gate.Budget(path,2*gate.CAP).state['calls']),2)
            with self.assertRaises(ValueError): gate.Budget(path)
            with self.assertRaises(ValueError): gate.Budget(path,3*gate.CAP)
    def test_output_cap_and_reasoning(self):
        p, bound = gate.bounded({'model':'deepseek-ai/DeepSeek-V4-Pro','messages':[{'role':'user','content':'你好'}], 'max_tokens':999999}, 'chat/completions')
        self.assertEqual(p['max_tokens'],8192); self.assertTrue(p['enable_thinking'])
        self.assertGreater(bound,8192*24000)
    def test_endpoint_isolation(self):
        with self.assertRaises(ValueError): gate.bounded({'model':'gpt-other'},'chat/completions')
        with self.assertRaises(ValueError): gate.bounded({'model':'Qwen/Qwen3-Embedding-8B','input':[[1,2]]},'embeddings')
    def test_durable_unknown_and_limit(self):
        with tempfile.TemporaryDirectory() as d:
            b=gate.Budget(Path(d)/'ledger.json')
            self.assertEqual(b.reserve('deepseek-ai/DeepSeek-V4-Pro',gate.CAP),0)
            self.assertIsNone(b.reserve('deepseek-ai/DeepSeek-V4-Pro',1))
            b=gate.Budget(Path(d)/'ledger.json');self.assertIsNone(b.reserve('deepseek-ai/DeepSeek-V4-Pro',1))
            b.settle(0,{'prompt_tokens':100,'completion_tokens':10})
            self.assertEqual(b.state['charged_nano_cny'],100*12000+10*24000)
    def test_missing_usage_never_refunds(self):
        with tempfile.TemporaryDirectory() as d:
            b=gate.Budget(Path(d)/'ledger.json');b.reserve('Qwen/Qwen3-Reranker-8B',10000)
            b.settle(0,{'prompt_tokens':1});self.assertEqual(b.state['charged_nano_cny'],10000)
    def test_concurrent_reservations_share_one_cap(self):
        with tempfile.TemporaryDirectory() as d:
            b=gate.Budget(Path(d)/'ledger.json')
            amount=gate.CAP//10
            with concurrent.futures.ThreadPoolExecutor(max_workers=16) as pool:
                receipts=list(pool.map(lambda _:b.reserve('deepseek-ai/DeepSeek-V4-Pro',amount),range(40)))
            self.assertEqual(sum(x is not None for x in receipts),10)
            self.assertEqual(b.state['charged_nano_cny'],gate.CAP)
            restored=gate.Budget(Path(d)/'ledger.json')
            self.assertIsNone(restored.reserve('deepseek-ai/DeepSeek-V4-Pro',1))
    def test_reported_usage_above_bound_fails_closed(self):
        with tempfile.TemporaryDirectory() as d:
            b=gate.Budget(Path(d)/'ledger.json')
            i=b.reserve('deepseek-ai/DeepSeek-V4-Pro',1)
            b.settle(i,{'prompt_tokens':10,'completion_tokens':1})
            self.assertEqual(b.state['calls'][i]['status'],'bound_violation')
            self.assertIsNone(b.reserve('deepseek-ai/DeepSeek-V4-Pro',1))

if __name__=='__main__': unittest.main()
