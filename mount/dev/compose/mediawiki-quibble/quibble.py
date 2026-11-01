"""Temporary compatibility for https://phabricator.wikimedia.org/T252883.

Remove this wrapper and its mounts when the bundled Quibble respects caller
environment overrides for browser tests and user commands.
"""

import contextvars
import functools
import sys


def install_compatibility(commands):
    """Give the inherited environment precedence only in affected commands."""
    import os

    targets = (
        (getattr(commands, 'BrowserTests', None), '_run_webdriver'),
        (getattr(commands, 'UserScripts', None), 'execute'),
    )
    if not callable(getattr(commands, 'run', None)) or any(
        not callable(getattr(cls, name, None)) for cls, name in targets
    ):
        raise RuntimeError(
            'The mw CLI Quibble compatibility wrapper does not support this '
            'Quibble API. Use the bundled image or update the wrapper.'
        )

    active = contextvars.ContextVar('mw_cli_quibble_overrides', default=False)
    original_run = commands.run

    @functools.wraps(original_run)
    def run(*args, **kwargs):
        if active.get() and kwargs.get('env') is not None:
            # Copy instead of mutating Quibble's dictionary. This preserves all
            # explicit overrides (including empty strings), without changing
            # Quibble's installation, API testing or backend environments.
            kwargs['env'] = {**kwargs['env'], **os.environ}
        return original_run(*args, **kwargs)

    def wrap(method):
        @functools.wraps(method)
        def execute(*args, **kwargs):
            token = active.set(True)
            try:
                return method(*args, **kwargs)
            finally:
                active.reset(token)
        return execute

    commands.run = run
    for cls, name in targets:
        setattr(cls, name, wrap(getattr(cls, name)))


def main():
    import quibble.cli
    import quibble.commands

    try:
        install_compatibility(quibble.commands)
    except RuntimeError as error:
        sys.exit(str(error))
    return quibble.cli.main()


if __name__ == '__main__':
    sys.exit(main())
