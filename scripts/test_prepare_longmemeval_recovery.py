import hashlib
import json
from pathlib import Path
import runpy
import tempfile
import unittest

module = runpy.run_path(str(Path(__file__).with_name('prepare-longmemeval-recovery.py')))


class RecoveryTests(unittest.TestCase):
    def test_preserves_answers_and_step_failures_but_retries_quota(self):
        with tempfile.TemporaryDirectory() as tmp:
            source, target = Path(tmp)/'original.jsonl', Path(tmp)/'recovered.jsonl'
            rows = [{'question_id': str(i), 'mode': 'bm25', 'model': 'fixed', 'error': e}
                    for i, e in enumerate(['', 'exceeds max steps', '403 insufficient_user_quota'])]
            source.write_text(''.join(json.dumps(r)+'\n' for r in rows))
            Path(str(source)+'.manifest.json').write_text(json.dumps({'mode': 'bm25', 'model': 'fixed', 'limit': 0}))
            before = source.read_bytes()
            result = module['prepare'](source, target)
            self.assertEqual(result['retained'], 2)
            self.assertEqual(result['retry_quota'], 1)
            self.assertEqual(source.read_bytes(), before)
            self.assertEqual([json.loads(s)['question_id'] for s in target.read_text().splitlines()], ['0', '1'])
            audit = json.loads(Path(str(target)+'.recovery.json').read_text())
            self.assertEqual(audit['source_sha256'], hashlib.sha256(before).hexdigest())
            with self.assertRaisesRegex(ValueError, 'unused output'):
                module['prepare'](source, target)
            with self.assertRaisesRegex(ValueError, 'unused output'):
                module['prepare'](source, source)


if __name__ == '__main__':
    unittest.main()
