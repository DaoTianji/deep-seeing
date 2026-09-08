import runpy
import json
import tempfile
from pathlib import Path
import unittest

module = runpy.run_path(str(Path(__file__).with_name('score-longmemeval.py')))


class ScoreTests(unittest.TestCase):
    def test_recovery_preserves_invalid_attempt_and_cannot_replace_valid_score(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / 'judgments.jsonl'
            old = {'question_id': 'a', 'hypothesis_sha256': 'fixed', 'model': 'judge',
                   'label': None, 'attempts': 1, 'usage': {'total_tokens': 10}}
            repaired = dict(old, label=False, usage={'total_tokens': 12})
            path.write_text(json.dumps(old) + '\n')
            repair_path = Path(str(path) + '.repairs.jsonl')
            repair_path.write_text(json.dumps(repaired) + '\n')
            result = module['resolved_judgments'](path)[0]
            self.assertFalse(result['label'])
            self.assertEqual(result['attempts'], 2)
            self.assertEqual(result['recovery_tokens_reported'], 10)
            self.assertIsNone(json.loads(path.read_text())['label'])
            repair_path.write_text(json.dumps(repaired) + '\n' + json.dumps(dict(repaired, label=True)) + '\n')
            with self.assertRaisesRegex(ValueError, 'only an invalid verdict'):
                module['resolved_judgments'](path)

    def test_errors_stay_in_denominator_and_abstention_separate(self):
        refs = {'a': {'question_type': 'single-session-user', 'answer_session_ids': ['s']},
                'b_abs': {'question_type': 'single-session-user', 'answer_session_ids': []}}
        hs = [{'question_id': 'a', 'hypothesis': 'x', 'answer_seconds': 1, 'error': 'step limit',
               'candidate_session_ids': ['s'], 'read_session_ids': [], 'used_session_ids': []},
              {'question_id': 'b_abs', 'hypothesis': 'unknown', 'answer_seconds': 2}]
        js = [{'question_id': 'a', 'label': True}, {'question_id': 'b_abs', 'label': True}]
        s = module['summarize'](refs, hs, js)
        self.assertEqual(s['accuracy'], .5)
        self.assertEqual(s['inference_errors'], 1)
        self.assertEqual(s['abstention']['accuracy'], 1)
        self.assertEqual(s['retrieval']['answerable_items'], 1)
        self.assertEqual(s['retrieval']['candidate_session_recall'], 1)
        self.assertEqual(s['retrieval']['read_session_recall'], 0)

    def test_missing_judge_is_not_a_pass(self):
        s = module['summarize']({'a': {'question_type': 'multi-session', 'answer_session_ids': ['s']}},
                                [{'question_id': 'a', 'answer_seconds': 1}], [])
        self.assertEqual(s['accuracy'], 0)
        self.assertEqual(s['judge_errors_or_missing'], 1)

    def test_tool_time_units_and_multi_evidence_recall(self):
        s = module['summarize'](
            {'a': {'question_type': 'multi-session', 'answer_session_ids': ['s1', 's2']}},
            [{'question_id': 'a', 'answer_seconds': 12,
              'candidate_session_ids': ['s1', 's1', 'noise'],
              'read_session_ids': ['s1'], 'used_session_ids': [],
              'searches': [{'duration_ns': 1000000}, {'duration_ns': 2000000}],
              'reads': [{'duration_ns': 500000}]}],
            [{'question_id': 'a', 'label': False}])
        self.assertEqual(s['retrieval']['candidate_session_recall'], .5)
        self.assertEqual(s['retrieval']['read_session_recall'], .5)
        self.assertEqual(s['retrieval']['used_session_recall'], 0)
        self.assertEqual(s['mean_search_time_ms_per_question'], 3)
        self.assertEqual(s['mean_read_time_ms_per_question'], .5)
        self.assertEqual(s['accuracy'], 0)

    def test_official_function_loads_without_sdk_and_uses_abstention(self):
        path = 'data/evals/longmemeval-baseline/upstream/src/evaluation/evaluate_qa.py'
        if not Path(path).exists():
            self.skipTest('upstream not downloaded')
        fn = module['official_prompt'](path)
        p = fn('multi-session', 'Q', 'A', 'H', abstention=True)
        self.assertIn('unanswerable question', p)
        self.assertIn('Model Response: H', p)


if __name__ == '__main__':
    unittest.main()
