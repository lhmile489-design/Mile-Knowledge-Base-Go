"""Local fake GitHub for the M5 acceptance script (技术设计 16.8).

The e2e build points both the API and codeload bases at this server through
SKILLS_FAKE_GITHUB, so the real GitHub hosts are never contacted. The state
file is re-read on every request: the script rewrites it to move a repository
from one commit to the next.

Usage: python -I fake-github.py <state.json> <port>
"""

import json
import os
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, unquote, urlparse

STATE_PATH = sys.argv[1]
PORT = int(sys.argv[2])
LOCK = threading.Lock()


def load_state():
    with LOCK:
        with open(STATE_PATH, encoding="utf-8") as handle:
            return json.load(handle)


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):  # keep the console quiet
        pass

    def reply(self, code, body, content_type="application/json"):
        data = body if isinstance(body, bytes) else body.encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def repo(self, segments, state):
        if len(segments) >= 3 and segments[0] == "repos":
            name = f"{segments[1]}/{segments[2]}"
        elif len(segments) >= 2:
            name = f"{segments[0]}/{segments[1]}"
        else:
            return None
        return state.get("repos", {}).get(name)

    def do_GET(self):
        sys.stderr.write(self.path + "\n")
        sys.stderr.flush()
        parsed = urlparse(self.path)
        segments = [unquote(part) for part in parsed.path.split("/") if part]
        query = parse_qs(parsed.query)
        try:
            state = load_state()
            if segments[:1] == ["repos"] and len(segments) == 3:
                entry = self.repo(segments, state)
                if entry is None:
                    return self.reply(404, "{}")
                return self.reply(200, json.dumps({"default_branch": entry.get("ref", "main")}))
            if segments[:1] == ["repos"] and len(segments) == 5 and segments[3] == "commits":
                entry = self.repo(segments, state)
                if entry is None:
                    return self.reply(404, "{}")
                return self.reply(200, entry["head"], "text/plain")
            if segments[:1] == ["repos"] and len(segments) == 4 and segments[3] == "commits":
                entry = self.repo(segments, state)
                if entry is None:
                    return self.reply(404, "{}")
                path = (query.get("path") or [""])[0]
                sha = entry.get("paths", {}).get(path, "")
                if not sha:
                    return self.reply(200, "[]")
                return self.reply(200, json.dumps([{"sha": sha}]))
            if len(segments) == 4 and segments[2] == "tar.gz":
                entry = self.repo(segments, state)
                sha = segments[3]
                archive = (entry or {}).get("archives", {}).get(sha, "")
                if not archive or not os.path.exists(archive):
                    return self.reply(404, "{}")
                with open(archive, "rb") as handle:
                    return self.reply(200, handle.read(), "application/octet-stream")
            return self.reply(404, "{}")
        except Exception as exc:  # a broken fixture must not kill the server
            return self.reply(500, json.dumps({"error": str(exc)}))


class Server(ThreadingHTTPServer):
    # Windows lets a second server bind the same port when SO_REUSEADDR is on;
    # two responders make the run flaky, so a busy port must fail loudly.
    allow_reuse_address = False
    allow_reuse_port = False


if __name__ == "__main__":
    try:
        Server(("127.0.0.1", PORT), Handler).serve_forever()
    except OSError as exc:
        sys.stderr.write(f"fake GitHub cannot bind 127.0.0.1:{PORT}: {exc}\n")
        sys.exit(2)
