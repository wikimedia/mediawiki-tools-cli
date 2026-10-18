#!/usr/bin/env python3
"""Wait for this job's authenticated GitHub results without a long-lived token."""
from contextlib import closing
import http.client
import json
import os
import socket
import sys
import time


class LocalConnection(http.client.HTTPConnection):
    socket_path = "/run/mwcli-wait/status.sock"

    def connect(self):
        self.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        self.sock.settimeout(self.timeout)
        self.sock.connect(self.socket_path)


def main(dispatch=False):
    deadline, previous = time.monotonic() + (5 * 60 if dispatch else 40 * 60), None
    while time.monotonic() < deadline:
        try:
            with closing(LocalConnection("localhost", timeout=60)) as connection:
                connection.request("POST" if dispatch else "GET", "/dispatch" if dispatch else "/status", headers={"JOB-TOKEN": os.environ["CI_JOB_TOKEN"]})
                response = connection.getresponse()
                result = json.load(response)
            message = result["message"]
            if message != previous:
                print(message, result.get("url", ""), flush=True)
                previous = message
            if result["state"] == "success":
                return 0
            if result["state"] == "failed":
                return 1
        except (OSError, ValueError, http.client.HTTPException):
            if previous != "unavailable":
                print("Waiting for the local result service", flush=True)
                previous = "unavailable"
        time.sleep(15)
    print("Timed out contacting GitHub integration", file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main(dispatch=sys.argv[1:] == ["dispatch"]))
