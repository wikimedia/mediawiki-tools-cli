import importlib.util
from pathlib import Path
import unittest


spec = importlib.util.spec_from_file_location(
    'clean_recording_text', Path(__file__).with_name('clean-recording-text.py')
)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class CleanRecordingTextTest(unittest.TestCase):
    def test_tapes_keep_recording_variables_out_of_visible_commands(self):
        guides = Path(__file__).resolve().parents[1] / 'docs' / 'guides'
        for tape in guides.rglob('*.tape'):
            hidden = False
            for line in tape.read_text().splitlines():
                if line == 'Hide':
                    hidden = True
                elif line == 'Show':
                    hidden = False
                elif line.startswith('Type ') and not hidden:
                    self.assertNotIn('MWCLI_RECORD_', line, f'{tape}: {line}')

    def test_removes_harness_and_wrapped_setup_but_preserves_output(self):
        text = ('> mw() { "$MWCLI_RECORD_BIN" --no-interaction "$@"; }\n'
                'wrapped setup\n> export CONTEXT="$MWCLI_RECORD_CONTEXT"\n'
                'clear\n──────────\n'
                '> mw dev --context "$MWCLI_RECORD_CONTEXT" stop\n'
                'Container mysql Stopped\n')
        self.assertEqual(module.clean_text(text),
                         '──────────\n> mw dev stop\nContainer mysql Stopped\n')

    def test_preserves_real_contexts_credentials_and_shell_examples(self):
        text = ('> mw dev --context staging mysql cli -- --password=custom\n'
                '> mw() { echo example; }\n'
                '{"CONTEXT": "value", "output": "unchanged"}\n')
        self.assertEqual(module.clean_text(text), text)

    def test_legacy_dump_command_is_self_contained_and_cleanup_is_idempotent(self):
        text = ('> mw dev --context "$MWCLI_RECORD_CONTEXT" compose exec -- '
                '-e MYSQL_PWD="$MWCLI_RECORD_DB_PASSWORD" -T mysql mariadb -uroot -e "SELECT 1"\n')
        cleaned = module.clean_text(text)
        self.assertEqual(cleaned,
                         '> mw dev compose exec -- -T mysql mariadb -uroot -ptoor -e "SELECT 1"\n')
        self.assertEqual(module.clean_text(cleaned), cleaned)


if __name__ == '__main__':
    unittest.main()
