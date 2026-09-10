import runpy
import unittest
from pathlib import Path

module = runpy.run_path(str(Path(__file__).with_name('report-longmemeval-controls.py')))


class ControlsReportTests(unittest.TestCase):
    def test_full_input_preserves_text_but_not_gold_labels(self):
        r = {'question_id': 'HIDDEN_ID', 'answer': 'HIDDEN_ANSWER', 'question_type': 'HIDDEN_TYPE',
             'question': 'Question?', 'question_date': '2023/05/30 (Tue) 12:00',
             'haystack_dates': ['2023/05/20 (Sat) 12:00', '2023/05/19 (Fri) 12:00'],
             'haystack_sessions': [[{'role': 'user', 'content': 'Later 中文', 'has_answer': True}],
                                   [{'role': 'assistant', 'content': 'Earlier'}]]}
        out = module['full_input'](r)
        self.assertLess(out.index('Earlier'), out.index('Later 中文'))
        self.assertTrue(out.endswith('Question: Question?'))
        for secret in ('HIDDEN_ID', 'HIDDEN_ANSWER', 'HIDDEN_TYPE', 'has_answer'):
            self.assertNotIn(secret, out)

    def test_paired_identical_scores_have_zero_interval(self):
        s = {'a': 0, 'b': 1}
        self.assertEqual(module['paired'](s, s), (0, 0, 0))


if __name__ == '__main__':
    unittest.main()
