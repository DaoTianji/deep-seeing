import argparse
import runpy
from pathlib import Path
import unittest

module = runpy.run_path(str(Path(__file__).with_name('score-longmemeval.py')))


class ScoreTests(unittest.TestCase):
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
