import hashlib
import json
from pathlib import Path
import runpy
import tempfile
import unittest

module = runpy.run_path(str(Path(__file__).with_name('report-longmemeval.py')))


class ReportAuditTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.path = Path(self.tmp.name) / 'native.jsonl'
        self.hs = [{'question_id': str(i), 'hypothesis': 'synthetic'} for i in range(500)]
        self.js = [{'question_id': str(i), 'label': True,
                    'hypothesis_sha256': hashlib.sha256(b'synthetic').hexdigest()}
                   for i in range(500)]
        self.write_rows(self.path, self.hs)
        self.jp = Path(str(self.path) + '.judgments-test.jsonl')
        self.write_rows(self.jp, self.js)
        self.write_json(str(self.path) + '.summary-test.json',
                        {'complete': True, 'judge': 'test', 'correct': 500, 'dataset_sha256': 'data'})
        self.write_json(str(self.path) + '.manifest.json', {'limit': 0, 'data_sha256': 'data'})
        self.write_json(str(self.jp) + '.manifest.json',
                        {'hypotheses_sha256': hashlib.sha256(self.path.read_bytes()).hexdigest(),
                         'data_sha256': 'data'})

    @staticmethod
    def write_json(path, value):
        Path(path).write_text(json.dumps(value))

    @staticmethod
    def write_rows(path, rows):
        Path(path).write_text(''.join(json.dumps(r) + '\n' for r in rows))

    def test_complete_matching_run_passes(self):
        _, _, _, scores = module['audited'](self.path, 'test')
        self.assertEqual(sum(scores.values()), 500)

    def test_incomplete_verdicts_rejected(self):
        self.write_rows(self.jp, self.js[:-1])
        with self.assertRaisesRegex(AssertionError, 'incomplete judgments'):
            module['audited'](self.path, 'test')

    def test_modified_answer_rejected(self):
        self.hs[0]['hypothesis'] = 'changed'
        self.write_rows(self.path, self.hs)
        with self.assertRaisesRegex(AssertionError, 'stale judgments'):
            module['audited'](self.path, 'test')

    def test_invalid_verdict_rejected(self):
        self.js[0]['label'] = None
        self.write_rows(self.jp, self.js)
        with self.assertRaisesRegex(AssertionError, 'invalid verdict'):
            module['audited'](self.path, 'test')


if __name__ == '__main__':
    unittest.main()
