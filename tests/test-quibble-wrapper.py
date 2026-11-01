"""Run locally with Python, or in the pinned image to also probe real Quibble."""

import contextlib
from concurrent.futures import ThreadPoolExecutor
import importlib.util
import os
from pathlib import Path
import types
import unittest
from unittest.mock import patch


path = Path(__file__).resolve().parents[1] / 'mount/dev/compose/mediawiki-quibble/quibble.py'
spec = importlib.util.spec_from_file_location('quibble_wrapper', path)
wrapper = importlib.util.module_from_spec(spec)
spec.loader.exec_module(wrapper)

DEFAULTS = {
    'MW_SERVER': 'http://localhost:8080',
    'MW_SCRIPT_PATH': '/',
    'MEDIAWIKI_USER': 'WikiAdmin',
    'MEDIAWIKI_PASSWORD': 'testwikijenkinspass',
    'FORCE_COLOR': '1',
    'DISPLAY': ':94',
}
OVERRIDES = {
    'MW_SERVER': 'https://example.test',
    'MW_SCRIPT_PATH': '/proxy/second/w',
    'MEDIAWIKI_USER': 'OtherAdmin',
    'MEDIAWIKI_PASSWORD': 'other-password',
    'FORCE_COLOR': '0',
    'DISPLAY': ':99',
    'CUSTOM_SETTING': 'custom value',
}


class CompatibilityTests(unittest.TestCase):
    def setUp(self):
        self.calls = []
        commands = types.SimpleNamespace()

        def run(*args, **kwargs):
            self.calls.append(kwargs.get('env'))
            return 17

        def execute(_self):
            env = {**os.environ, **DEFAULTS}
            result = commands.run(['test'], cwd='.', env=env)
            self.assertEqual(env, {**os.environ, **DEFAULTS})
            return result

        commands.run = run
        commands.BrowserTests = type('BrowserTests', (), {'_run_webdriver': execute})
        commands.UserScripts = type('UserScripts', (), {'execute': execute})
        wrapper.install_compatibility(commands)
        self.commands = commands

    def test_overrides_and_return_value(self):
        with patch.dict(os.environ, OVERRIDES, clear=True):
            self.assertEqual(self.commands.BrowserTests()._run_webdriver(), 17)
            self.assertEqual(self.commands.UserScripts().execute(), 17)
        for env in self.calls:
            self.assertEqual(env, OVERRIDES)

    def test_defaults_without_overrides(self):
        with patch.dict(os.environ, {}, clear=True):
            self.commands.BrowserTests()._run_webdriver()
        self.assertEqual(self.calls, [DEFAULTS])

    def test_empty_script_path(self):
        with patch.dict(os.environ, {'MW_SCRIPT_PATH': ''}, clear=True):
            self.commands.UserScripts().execute()
        self.assertEqual(self.calls[0]['MW_SCRIPT_PATH'], '')

    def test_unrelated_commands_unchanged(self):
        with patch.dict(os.environ, OVERRIDES, clear=True):
            self.commands.BrowserTests()._run_webdriver()
            env = {'MW_SERVER': 'backend', 'LANG': 'C.UTF-8'}
            self.commands.run(['backend'], cwd='.', env=env)
            self.commands.run(['inherit'], cwd='.')
        self.assertIs(self.calls[1], env)
        self.assertIsNone(self.calls[2])

    def test_exception_does_not_leak_overrides(self):
        def fail(_self):
            raise ValueError('test failure')

        commands = types.SimpleNamespace(
            run=self.commands.run,
            BrowserTests=type('BrowserTests', (), {'_run_webdriver': fail}),
            UserScripts=type('UserScripts', (), {'execute': fail}),
        )
        wrapper.install_compatibility(commands)
        with patch.dict(os.environ, OVERRIDES, clear=True):
            with self.assertRaisesRegex(ValueError, 'test failure'):
                commands.BrowserTests()._run_webdriver()
            env = {'MW_SERVER': 'backend'}
            commands.run(['backend'], cwd='.', env=env)
        self.assertIs(self.calls[0], env)

    def test_unsupported_api(self):
        with self.assertRaisesRegex(RuntimeError, 'bundled image'):
            wrapper.install_compatibility(types.SimpleNamespace())

    def test_other_threads_do_not_inherit_overrides(self):
        calls = []
        commands = types.SimpleNamespace(run=lambda *a, **kw: calls.append(kw['env']))

        def execute(_self):
            with ThreadPoolExecutor(max_workers=1) as executor:
                executor.submit(commands.run, ['backend'], env=DEFAULTS).result()
            commands.run(['browser'], env=DEFAULTS)

        commands.BrowserTests = type('BrowserTests', (), {'_run_webdriver': execute})
        commands.UserScripts = type('UserScripts', (), {'execute': execute})
        wrapper.install_compatibility(commands)
        with patch.dict(os.environ, OVERRIDES, clear=True):
            commands.BrowserTests()._run_webdriver()
        self.assertIs(calls[0], DEFAULTS)
        self.assertEqual(calls[1], OVERRIDES)


try:
    import quibble.commands as real_commands
except ImportError:
    real_commands = None


@unittest.skipIf(real_commands is None, 'requires the Quibble image')
class PinnedQuibbleTests(unittest.TestCase):
    def test_actual_browser_and_user_commands(self):
        # Exercise the actual installed methods and subprocess environment,
        # without requiring a browser, wiki or network in this unit test.
        with contextlib.ExitStack() as stack:
            stack.enter_context(patch.object(
                real_commands.BrowserTests, '_run_webdriver',
                real_commands.BrowserTests._run_webdriver,
            ))
            stack.enter_context(patch.object(
                real_commands.UserScripts, 'execute', real_commands.UserScripts.execute,
            ))
            run = stack.enter_context(patch.object(real_commands, 'run'))
            stack.enter_context(patch.dict(os.environ, OVERRIDES, clear=True))
            wrapper.install_compatibility(real_commands)
            real_commands.BrowserTests(
                '/workspace/src', [], ':94', 'http://localhost:8080', 'external',
            )._run_webdriver('/workspace/src', 'mediawiki/core')
            real_commands.UserScripts(
                '/workspace/src', ['true'], 'http://localhost:8080', 'external',
            ).execute()
            self.assertEqual(run.call_count, 2)
            for call in run.call_args_list:
                for key, value in OVERRIDES.items():
                    self.assertEqual(call.kwargs['env'][key], value)
                self.assertEqual(call.kwargs['env']['QUIBBLE_APACHE'], '1')


if __name__ == '__main__':
    unittest.main()
