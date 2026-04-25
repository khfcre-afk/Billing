#!/usr/bin/env python3
"""Minimal mock of Pterodactyl Panel API for end-to-end testing of the billing platform.
Intentionally permissive: returns plausible canned responses for every endpoint the Go client calls.
"""
import json
import re
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

_user_seq = 100
_server_seq = 200
_lock = threading.Lock()


def _alloc_attrs(i: int):
    return {
        "id": 1000 + i,
        "ip": "10.0.0.5",
        "port": 25565 + i,
        "assigned": False,
    }


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        sys.stderr.write("[mock] " + (fmt % args) + "\n")

    def _read_json(self):
        n = int(self.headers.get("Content-Length", "0") or 0)
        if n == 0:
            return {}
        try:
            return json.loads(self.rfile.read(n) or b"{}")
        except Exception:
            return {}

    def _send(self, status: int, body: dict | None):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        if body is None:
            self.send_header("Content-Length", "0")
            self.end_headers()
            return
        data = json.dumps(body).encode()
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        m = re.match(r"^/api/application/nodes/\d+/allocations", self.path)
        if m:
            allocs = [{"attributes": _alloc_attrs(i)} for i in range(1, 6)]
            return self._send(200, {"object": "list", "data": allocs})
        return self._send(404, {"error": "not found", "path": self.path})

    def do_POST(self):
        global _user_seq, _server_seq
        body = self._read_json()
        if self.path == "/api/application/users":
            with _lock:
                _user_seq += 1
                uid = _user_seq
            attrs = {
                "id": uid,
                "email": body.get("email", ""),
                "username": body.get("username", "u%d" % uid),
            }
            return self._send(201, {"object": "user", "attributes": attrs})

        if self.path == "/api/application/servers":
            with _lock:
                _server_seq += 1
                sid = _server_seq
            ident = "abc%05d" % sid
            attrs = {
                "id": sid,
                "identifier": ident,
                "uuid": "00000000-0000-4000-8000-%012d" % sid,
                "name": body.get("name", "srv-mock"),
                "suspended": False,
                "allocation": body.get("allocation", {}).get("default", 1000),
            }
            return self._send(201, {"object": "server", "attributes": attrs})

        if re.match(r"^/api/application/servers/\d+/(suspend|unsuspend)$", self.path):
            return self._send(204, None)

        if re.match(r"^/api/client/servers/[^/]+/power$", self.path):
            return self._send(204, None)

        return self._send(404, {"error": "not found", "path": self.path})

    def do_DELETE(self):
        if re.match(r"^/api/application/servers/\d+$", self.path):
            return self._send(204, None)
        return self._send(404, {"error": "not found", "path": self.path})


def main():
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 8081
    srv = ThreadingHTTPServer(("0.0.0.0", port), Handler)
    print("[mock] Pterodactyl mock listening on :%d" % port, flush=True)
    srv.serve_forever()


if __name__ == "__main__":
    main()
