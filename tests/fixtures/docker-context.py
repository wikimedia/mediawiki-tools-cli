#!/usr/bin/env python3
"""Serve only Docker's ping API over an isolated native socket or named pipe.

This fixture imports no mwcli code. The Bash test drives the built CLI and
checks that requests actually reach this endpoint.
"""

import ctypes
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import shutil
import socket
import sys
import tempfile


root = Path(sys.argv[1]).resolve()
requests = root / "requests"
requests.write_text("")


def response(request):
    line = request.split(b"\r\n", 1)[0].decode("ascii")
    with requests.open("a") as log:
        log.write(line + "\n")
    if line not in ("HEAD /_ping HTTP/1.1", "GET /_ping HTTP/1.1"):
        return b"HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
    body = b"" if line.startswith("HEAD") else b"OK"
    return (b"HTTP/1.1 200 OK\r\nAPI-Version: 1.45\r\nOSType: linux\r\n"
            b"Content-Length: 2\r\nConnection: close\r\n\r\n" + body)


def windows_server():
    from ctypes import wintypes

    api = ctypes.WinDLL("kernel32", use_last_error=True)
    api.CreateNamedPipeW.argtypes = [wintypes.LPCWSTR] + [wintypes.DWORD] * 6 + [ctypes.c_void_p]
    api.CreateNamedPipeW.restype = wintypes.HANDLE
    api.ConnectNamedPipe.argtypes = [wintypes.HANDLE, ctypes.c_void_p]
    api.ReadFile.argtypes = [wintypes.HANDLE, ctypes.c_void_p, wintypes.DWORD,
                             ctypes.POINTER(wintypes.DWORD), ctypes.c_void_p]
    api.WriteFile.argtypes = api.ReadFile.argtypes
    api.FlushFileBuffers.argtypes = [wintypes.HANDLE]
    api.DisconnectNamedPipe.argtypes = [wintypes.HANDLE]
    api.CloseHandle.argtypes = [wintypes.HANDLE]
    name = f"mwcli-context-{os.getpid()}"
    pipe = rf"\\.\pipe\{name}"

    def create():
        handle = api.CreateNamedPipeW(pipe, 3, 0, 1, 65536, 65536, 0, None)
        if handle == wintypes.HANDLE(-1).value:
            raise ctypes.WinError(ctypes.get_last_error())
        return handle

    handle = create()

    def serve():
        nonlocal handle
        while True:
            try:
                if not api.ConnectNamedPipe(handle, None) and ctypes.get_last_error() != 535:
                    raise ctypes.WinError(ctypes.get_last_error())
                data = b""
                size = wintypes.DWORD()
                buffer = ctypes.create_string_buffer(65536)
                while b"\r\n\r\n" not in data:
                    if not api.ReadFile(handle, buffer, len(buffer), ctypes.byref(size), None):
                        raise ctypes.WinError(ctypes.get_last_error())
                    data += buffer.raw[:size.value]
                reply = response(data)
                if not api.WriteFile(handle, reply, len(reply), ctypes.byref(size), None):
                    raise ctypes.WinError(ctypes.get_last_error())
                api.FlushFileBuffers(handle)
            finally:
                api.DisconnectNamedPipe(handle)
                api.CloseHandle(handle)
            handle = create()

    return f"npipe:////./pipe/{name}", serve


def unix_server():
    # macOS has a short Unix socket path limit, independent of the fixture root.
    directory = Path(tempfile.mkdtemp(prefix="mwctx-", dir="/tmp"))
    import atexit
    atexit.register(shutil.rmtree, directory, ignore_errors=True)
    import signal
    signal.signal(signal.SIGTERM, lambda *_: sys.exit(0))
    listener = socket.socket(socket.AF_UNIX)
    listener.bind(str(directory / "docker.sock"))
    listener.listen()

    def serve():
        while True:
            connection, _ = listener.accept()
            with connection:
                connection.settimeout(10)
                request = b""
                while b"\r\n\r\n" not in request:
                    chunk = connection.recv(65536)
                    if not chunk:
                        break
                    request += chunk
                connection.sendall(response(request))

    return "unix://" + str(directory / "docker.sock"), serve


host, serve = windows_server() if os.name == "nt" else unix_server()
docker_dir = root / "docker"
metadata = docker_dir / "contexts/meta" / hashlib.sha256(b"desktop-linux").hexdigest()
metadata.mkdir(parents=True)
(metadata / "meta.json").write_text(json.dumps({
    "Name": "desktop-linux", "Endpoints": {"docker": {"Host": host, "SkipTLSVerify": False}},
}))
(docker_dir / "config.json").write_text('{"currentContext":"desktop-linux"}')
config_home = root / "config home"
(config_home / "mwcli").mkdir(parents=True)
(config_home / "mwcli/config.json").write_text(json.dumps({
    "telemetry": "no",
    "timer_last_update_checked": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
}))
(root / "bin").mkdir()
# Native executables that exit successfully with no arguments satisfy the
# Compose presence check even on runners without Docker Desktop.
if os.name == "nt":
    shutil.copy2(Path(os.environ["SystemRoot"]) / "System32/cmd.exe", root / "bin/docker-compose.exe")
else:
    stub = root / "bin/docker-compose"
    stub.write_text("#!/bin/sh\nexit 0\n")
    stub.chmod(0o755)
for filename, value in {
    "host": host, "docker-config": str(docker_dir), "config-home": str(config_home),
    "expected": str(config_home / "mwcli/mwdd/context-regression"),
}.items():
    (root / filename).write_text(value)
(root / "ready").touch()
serve()
