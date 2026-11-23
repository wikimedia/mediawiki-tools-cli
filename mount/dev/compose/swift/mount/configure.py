#!/usr/bin/env python3
"""Render development credentials without interpreting shell or INI syntax."""
import os
import pwd
from pathlib import Path

path = Path('/etc/swift/proxy-server.conf')
config = Path('/etc/swift/proxy-server.conf.template').read_text()
for name in (
    'SWIFT_MEDIA_KEY', 'SWIFT_THUMBOR_KEY', 'SWIFT_PRIVATE_KEY',
    'SWIFT_UPLOAD_HOST', 'SWIFT_SHARDED_CONTAINERS',
):
    value = os.environ.get(name, '')
    if any(char.isspace() for char in value) or '\x00' in value:
        raise SystemExit(f'{name} must not contain whitespace or NUL')
    if name != 'SWIFT_SHARDED_CONTAINERS' and not value:
        raise SystemExit(f'{name} must not be empty')
    config = config.replace(f'@{name}@', value)
path.write_text(config)
os.chmod(path, 0o640)
user = pwd.getpwnam('swift')
os.chown(path, user.pw_uid, user.pw_gid)
